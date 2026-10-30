package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/database"
)

var (
	activeSessions = make(map[string]sessionInfo)
	sessionsMu     sync.RWMutex
)

func init() {
	// Periodic cleanup of expired sessions and login attempts every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			now := time.Now()
			sessionsMu.Lock()
			for token, sess := range activeSessions {
				if now.After(sess.ExpiresAt) {
					delete(activeSessions, token)
				}
			}
			sessionsMu.Unlock()

			attemptsMu.Lock()
			for _, entries := range []map[string]*loginAttempt{loginAttempts, userAttempts} {
				for ip, att := range entries {
					if now.After(att.lockedUntil) && att.failedCount > 0 {
						delete(entries, ip)
					}
				}
			}
			attemptsMu.Unlock()
		}
	}()
}

type sessionInfo struct {
	Username           string
	Role               string
	MustChangePassword bool
	ExpiresAt          time.Time
}

type loginAttempt struct {
	failedCount int
	lockedUntil time.Time
	updatedAt   time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	userAttempts  = make(map[string]*loginAttempt)
	attemptsMu    sync.Mutex
)

// Login handles operator login with rate limiting and transparent bcrypt migration
func Login(c *gin.Context) {
	select {
	case loginSlots <- struct{}{}:
		defer func() { <-loginSlots }()
	default:
		c.JSON(429, gin.H{"error": "Giriş kapasitesi dolu"})
		return
	}
	ip := c.ClientIP()

	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kullanıcı adı ve şifre zorunludur"})
		return
	}

	if len(input.Username) > 128 || len(input.Password) > 72 {
		c.JSON(400, gin.H{"error": "Geçersiz giriş uzunluğu"})
		return
	}
	attemptsMu.Lock()
	now := time.Now()
	for _, entries := range []map[string]*loginAttempt{loginAttempts, userAttempts} {
		for key, att := range entries {
			if now.Sub(att.updatedAt) > 10*time.Minute && now.After(att.lockedUntil) {
				delete(entries, key)
			}
		}
	}
	if (loginAttempts[ip] == nil && len(loginAttempts) >= 4096) || (userAttempts[input.Username] == nil && len(userAttempts) >= 4096) {
		attemptsMu.Unlock()
		c.JSON(429, gin.H{"error": "Giriş kapasitesi dolu"})
		return
	}
	// Fix R04: Check IP attempt limit
	if att, exists := loginAttempts[ip]; exists {
		if time.Now().Before(att.lockedUntil) {
			remaining := int(time.Until(att.lockedUntil).Seconds())
			attemptsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("Çok fazla hatalı giriş denemesi. Lütfen %d saniye bekleyin.", remaining),
			})
			return
		}
	}
	// Fix R04: Check Username attempt limit (prevents IP rotation bypass)
	if uAtt, exists := userAttempts[input.Username]; exists {
		if time.Now().Before(uAtt.lockedUntil) {
			remaining := int(time.Until(uAtt.lockedUntil).Seconds())
			attemptsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("Bu hesap için çok fazla hatalı giriş denemesi yapıldı. Lütfen %d saniye bekleyin.", remaining),
			})
			return
		}
	}
	attemptsMu.Unlock()

	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Veritabanı hazır değil"})
		return
	}

	var id int
	var role, storedHash string
	var mustChange int
	err := database.DB.QueryRow(`
		SELECT id, role, password_hash, must_change_password 
		FROM users 
		WHERE username = $1
	`, input.Username).Scan(&id, &role, &storedHash, &mustChange)

	if err != nil || !database.VerifyPassword(input.Password, storedHash) {
		attemptsMu.Lock()
		att := loginAttempts[ip]
		if att == nil {
			att = &loginAttempt{}
			loginAttempts[ip] = att
		}
		att.updatedAt = time.Now()
		att.failedCount++
		if att.failedCount >= 5 {
			att.lockedUntil = time.Now().Add(5 * time.Minute)
		}

		uAtt := userAttempts[input.Username]
		if uAtt == nil {
			uAtt = &loginAttempt{}
			userAttempts[input.Username] = uAtt
		}
		uAtt.updatedAt = time.Now()
		uAtt.failedCount++
		if uAtt.failedCount >= 5 {
			uAtt.lockedUntil = time.Now().Add(5 * time.Minute)
		}
		attemptsMu.Unlock()

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Hatalı kullanıcı adı veya şifre"})
		return
	}

	// Login successful - clear failed attempts for this IP and username
	attemptsMu.Lock()
	delete(loginAttempts, ip)
	delete(userAttempts, input.Username)
	attemptsMu.Unlock()

	// Transparently upgrade legacy SHA-256 hash to bcrypt
	if !strings.HasPrefix(storedHash, "$2a$") && !strings.HasPrefix(storedHash, "$2b$") && !strings.HasPrefix(storedHash, "$2y$") {
		newBcryptHash, hErr := database.HashPassword(input.Password)
		if hErr == nil {
			_, _ = database.DB.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", newBcryptHash, id)
		}
	}

	// If password was default 'admin', ensure must_change_password = 1
	if input.Password == "admin" {
		mustChange = 1
		_, _ = database.DB.Exec("UPDATE users SET must_change_password = 1 WHERE id = $1", id)
	}

	// Generate session token
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(500, gin.H{"error": "Oturum oluşturulamadı"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	sessionsMu.Lock()
	for key, sess := range activeSessions {
		if time.Now().After(sess.ExpiresAt) {
			delete(activeSessions, key)
		}
	}
	if len(activeSessions) >= 4096 {
		sessionsMu.Unlock()
		c.JSON(429, gin.H{"error": "Oturum kapasitesi dolu"})
		return
	}
	activeSessions[token] = sessionInfo{
		Username:           input.Username,
		Role:               role,
		MustChangePassword: mustChange == 1,
		ExpiresAt:          time.Now().Add(7 * 24 * time.Hour), // 7 days session
	}
	sessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"token":                token,
		"username":             input.Username,
		"role":                 role,
		"must_change_password": mustChange == 1,
		"message":              "Giriş başarılı",
	})
}

// GetCurrentUser returns session status and whether password change is mandatory
func GetCurrentUser(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetString("role")
	mustChange := c.GetBool("must_change_password")

	if database.DB != nil && username != "" {
		var dbMustChange int
		if err := database.DB.QueryRow("SELECT must_change_password FROM users WHERE username = $1", username).Scan(&dbMustChange); err == nil {
			mustChange = dbMustChange == 1
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"username":             username,
		"role":                 role,
		"must_change_password": mustChange,
	})
}

// Logout invalidates the active bearer token on the server
func Logout(c *gin.Context) {
	token := c.GetString("token")
	if token != "" {
		sessionsMu.Lock()
		delete(activeSessions, token)
		sessionsMu.Unlock()
	}
	c.JSON(http.StatusOK, gin.H{"message": "Oturum başarıyla sonlandırıldı."})
}

// ChangePassword allows operator to update their password
func ChangePassword(c *gin.Context) {
	var input struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Eksik bilgi"})
		return
	}

	if len(input.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Yeni şifre en az 8 karakter olmalıdır"})
		return
	}

	// Fix R05: Enforce max 72 bytes length for bcrypt safely
	if len([]byte(input.NewPassword)) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Yeni şifre en fazla 72 bayt uzunluğunda olabilir"})
		return
	}

	if input.NewPassword == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Yeni parola varsayılan 'admin' parolası olamaz"})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "admin"
	}

	var id int
	var storedHash string
	err := database.DB.QueryRow("SELECT id, password_hash FROM users WHERE username = $1", username).Scan(&id, &storedHash)
	if err != nil || !database.VerifyPassword(input.CurrentPassword, storedHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Mevcut şifre hatalı"})
		return
	}

	newHash, err := database.HashPassword(input.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = database.DB.Exec("UPDATE users SET password_hash = $1, must_change_password = 0 WHERE id = $2", newHash, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Şifre güncellenemedi"})
		return
	}

	// Clean up initial bootstrap password file if it exists
	_ = os.Remove(database.BootstrapPasswordPath)

	// Fix R08: Revoke all other active sessions for this user except current session
	token := c.GetString("token")
	if token != "" {
		sessionsMu.Lock()
		for tok, sess := range activeSessions {
			if sess.Username == username {
				if tok == token {
					sess.MustChangePassword = false
					activeSessions[tok] = sess
				} else {
					delete(activeSessions, tok)
				}
			}
		}
		sessionsMu.Unlock()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Şifre başarıyla değiştirildi.",
		"must_change_password": false,
	})
}

// AuthMiddleware validates session token and enforces mandatory password change
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Yetkilendirme gerekli"})
			c.Abort()
			return
		}

		sessionsMu.RLock()
		sess, ok := activeSessions[token]
		sessionsMu.RUnlock()

		if !ok || time.Now().After(sess.ExpiresAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Geçersiz veya süresi dolmuş oturum"})
			c.Abort()
			return
		}

		c.Set("username", sess.Username)
		c.Set("role", sess.Role)
		c.Set("token", token)
		c.Set("must_change_password", sess.MustChangePassword)

		// If user is required to change password, reject access to general APIs
		if sess.MustChangePassword {
			path := c.Request.URL.Path
			if path != "/api/auth/change-password" && path != "/api/auth/me" {
				c.JSON(http.StatusForbidden, gin.H{
					"error":                "Zorunlu güvenlik güncellemesi: Devam etmeden önce varsayılan parolanızı değiştirmelisiniz.",
					"must_change_password": true,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

var loginSlots = make(chan struct{}, 8)
