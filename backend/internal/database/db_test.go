package database

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashPasswordAndVerify(t *testing.T) {
	password := "SecretP@ssw0rd2026!"
	hash := HashPassword(password)

	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Fatalf("expected bcrypt hash prefix ($2a$ or $2b$), got: %s", hash)
	}

	if !VerifyPassword(password, hash) {
		t.Fatalf("expected password verification to succeed for valid password")
	}

	if VerifyPassword("WrongPassword", hash) {
		t.Fatalf("expected password verification to fail for wrong password")
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
