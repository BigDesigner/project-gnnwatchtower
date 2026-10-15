package database

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashPasswordAndVerify(t *testing.T) {
	password := "SecretP@ssw0rd2026!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("expected HashPassword to succeed: %v", err)
	}

	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Fatalf("expected bcrypt hash prefix ($2a$ or $2b$), got: %s", hash)
	}

	if !VerifyPassword(password, hash) {
		t.Fatalf("expected password verification to succeed for valid password")
	}

	if VerifyPassword("WrongPassword", hash) {
		t.Fatalf("expected password verification to fail for wrong password")
	}

	// Fix R05: Test that passwords > 72 bytes return an error and do NOT fallback to SHA-256
	longPassword := strings.Repeat("a", 73)
	_, err = HashPassword(longPassword)
	if err == nil {
		t.Fatalf("expected error for password > 72 bytes, got nil")
	}
}

func TestLegacySHA256BackwardCompatibility(t *testing.T) {
	password := "LegacyAdmin123"
	// Compute legacy hash
	h := sha256.New()
	h.Write([]byte(password + "_gnnwatchtower_salt_2026"))
	legacyHash := hex.EncodeToString(h.Sum(nil))

	if !VerifyPassword(password, legacyHash) {
		t.Fatalf("expected legacy SHA-256 hash to verify successfully")
	}

	if VerifyPassword("InvalidPass", legacyHash) {
		t.Fatalf("expected legacy verification to fail for invalid password")
	}
}

func TestInitialAdminPasswordEnforcesChange(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	var adminHash string
	var mustChange int
	err = db.QueryRow("SELECT password_hash, must_change_password FROM users WHERE username = 'admin'").Scan(&adminHash, &mustChange)
	if err != nil {
		t.Fatalf("failed querying admin user: %v", err)
	}

	// Initial password must be 'admin'
	if !VerifyPassword("admin", adminHash) {
		t.Fatalf("expected initial admin password to be 'admin', but verification failed")
	}

	// Must enforce password change
	if mustChange != 1 {
		t.Fatalf("expected must_change_password=1 for initial admin, got %d", mustChange)
	}
}
