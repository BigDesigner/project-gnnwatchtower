package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/database"
)

var (
	activeSessions = make(map[string]sessionInfo)
	sessionsMu     sync.RWMutex
)

type sessionInfo struct {
	Username  string
	Role      string
	ExpiresAt time.Time
}

// Login handles operator login
func Login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kullanıcı adı ve şifre zorunludur"})
		return
	}

	hash := database.HashPassword(input.Password)

	var id int
	var role string
	err := database.DB.QueryRow(`
		SELECT id, role FROM users 
		WHERE username = $1 AND password_hash = $2
	`, input.Username, hash).Scan(&id, &role)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Hatalı kullanıcı adı veya şifre"})
		return
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
		"token":    token,
		"username": input.Username,
		"role":     role,
		"message":  "Giriş başarılı",
	})
}

// ChangePassword allows admin to update their password
func ChangePassword(c *gin.Context) {
	var input struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Eksik bilgi"})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "admin"
	}

	currHash := database.HashPassword(input.CurrentPassword)
	var id int
	err := database.DB.QueryRow("SELECT id FROM users WHERE username = $1 AND password_hash = $2", username, currHash).Scan(&id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Mevcut şifre hatalı"})
		return
	}

	newHash := database.HashPassword(input.NewPassword)
	_, err = database.DB.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", newHash, id)
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
