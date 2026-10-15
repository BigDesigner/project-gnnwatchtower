package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/database"
)

func TestLoginRateLimiter(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init in-memory db: %v", err)
	}
	defer db.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", Login)

	// Clean up attempts for test IP
	testIP := "192.0.2.10"
	attemptsMu.Lock()
	delete(loginAttempts, testIP)
	attemptsMu.Unlock()

	// Perform 5 invalid logins
	for i := 1; i <= 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"nonexistent","password":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", testIP)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 Unauthorized, got %d", i, w.Code)
		}
	}

	// 6th attempt from the same IP must be rejected with 429 Too Many Requests
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"nonexistent","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", testIP)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: expected 429 Too Many Requests, got %d", w.Code)
	}

	// Reset attempts
	attemptsMu.Lock()
	delete(loginAttempts, testIP)
	attemptsMu.Unlock()
}

func TestMandatoryPasswordChangeFlow(t *testing.T) {
	attemptsMu.Lock()
	loginAttempts = make(map[string]*loginAttempt)
	attemptsMu.Unlock()

	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init in-memory db: %v", err)
	}
	defer db.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", Login)

	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	{
		protected.GET("/auth/me", GetCurrentUser)
		protected.POST("/auth/change-password", ChangePassword)
		protected.GET("/test-endpoint", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"data": "access granted"})
		})
	}

	// 1. Login with initial admin credentials
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/auth/login", loginBody)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK login, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Token              string `json:"token"`
		MustChangePassword bool   `json:"must_change_password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if !loginResp.MustChangePassword {
		t.Fatalf("expected must_change_password=true for initial admin, got false")
	}

	token := loginResp.Token

	// 2. Access /api/test-endpoint should be BLOCKED with 403 Forbidden
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/test-endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden before password change, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Access /api/auth/me should be ALLOWED and report must_change_password=true
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/auth/me, got %d", w.Code)
	}

	// 4. Attempting to change to 'admin' again must be rejected with 400
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/auth/change-password", bytes.NewBufferString(`{"current_password":"admin","new_password":"admin"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when setting new password to 'admin', got %d", w.Code)
	}

	// 5. Change password to secure password
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/auth/change-password", bytes.NewBufferString(`{"current_password":"admin","new_password":"SecurePassword123!"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK password change, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Access /api/test-endpoint should now be GRANTED with 200 OK
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/test-endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after password change, got %d: %s", w.Code, w.Body.String())
	}
}
