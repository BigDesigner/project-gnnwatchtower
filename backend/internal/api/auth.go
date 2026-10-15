package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
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
			for ip, att := range loginAttempts {
				if now.After(att.lockedUntil) && att.failedCount > 0 {
					delete(loginAttempts, ip)
				}
			}
			attemptsMu.Unlock()
		}
	}()
}

type sessionInfo struct {
	Username  string
	Role      string
	ExpiresAt time.Time
}

type loginAttempt struct {
	failedCount int
	lockedUntil time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	attemptsMu    sync.Mutex
)

// Login handles operator login with rate limiting and transparent bcrypt migration
func Login(c *gin.Context) {
	ip := c.ClientIP()

	attemptsMu.Lock()
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
	attemptsMu.Unlock()

	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kullanıcı adı ve şifre zorunludur"})
		return
	}

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
		att.failedCount++
		if att.failedCount >= 5 {
			att.lockedUntil = time.Now().Add(5 * time.Minute)
		}
		attemptsMu.Unlock()

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Hatalı kullanıcı adı veya şifre"})
		return
	}

	// Login successful - clear failed attempts for this IP
	attemptsMu.Lock()
	delete(loginAttempts, ip)
	attemptsMu.Unlock()

	// Transparently upgrade legacy SHA-256 hash to bcrypt
	if !strings.HasPrefix(storedHash, "$2a$") && !strings.HasPrefix(storedHash, "$2b$") && !strings.HasPrefix(storedHash, "$2y$") {
		newBcryptHash := database.HashPassword(input.Password)
		_, _ = database.DB.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", newBcryptHash, id)
	}

	// Generate session token
	tokenBytes := make([]byte, 24)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	sessionsMu.Lock()
	activeSessions[token] = sessionInfo{
		Username:  input.Username,
		Role:      role,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days session
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

	newHash := database.HashPassword(input.NewPassword)
	_, err = database.DB.Exec("UPDATE users SET password_hash = $1, must_change_password = 0 WHERE id = $2", newHash, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Şifre güncellenemedi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Şifre başarıyla değiştirildi."})
}

// AuthMiddleware validates session token
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
		c.Next()
	}
}
