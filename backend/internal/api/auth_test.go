package api

import (
	"bytes"
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
