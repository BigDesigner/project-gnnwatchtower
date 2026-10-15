package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestEd25519UpdateSigningAndVerification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gnn-signer-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Initialize update signer in temporary directory
	if err := InitUpdateSigner(tmpDir); err != nil {
		t.Fatalf("InitUpdateSigner failed: %v", err)
	}

	pubKeyHex := GetUpdatePublicKeyHex()
	if len(pubKeyHex) != 64 { // 32 bytes in hex = 64 chars
		t.Fatalf("expected 64-char hex public key, got %d chars: %s", len(pubKeyHex), pubKeyHex)
	}

	targetVersion := "0.4.3"
	sha256Hex := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// 2. Sign legitimate update
	sigHex, err := SignAgentUpdate(targetVersion, sha256Hex)
	if err != nil {
		t.Fatalf("SignAgentUpdate failed: %v", err)
	}
	if len(sigHex) != 128 { // 64 bytes in hex = 128 chars
		t.Fatalf("expected 128-char hex signature, got %d chars: %s", len(sigHex), sigHex)
	}

	// 3. Verify valid signature passes
	if !VerifyAgentUpdateSignature(pubKeyHex, targetVersion, sha256Hex, sigHex) {
		t.Fatalf("expected valid signature to pass verification, but it failed")
	}

	// 4. Verify tampered version fails verification
	if VerifyAgentUpdateSignature(pubKeyHex, "0.4.4", sha256Hex, sigHex) {
		t.Fatalf("expected tampered version to fail verification, but it passed")
	}

	// 5. Verify tampered SHA-256 binary digest fails verification (MITM binary substitution defense)
	tamperedSHA := "0000000098fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if VerifyAgentUpdateSignature(pubKeyHex, targetVersion, tamperedSHA, sigHex) {
		t.Fatalf("expected tampered SHA256 to fail verification, but it passed")
	}

	// 6. Verify rogue key cannot sign updates accepted by pinned key
	_, roguePriv, _ := ed25519.GenerateKey(rand.Reader)
	roguePayload := []byte("GNNWATCHTOWER_UPDATE:v0.4.3:" + sha256Hex)
	rogueSig := ed25519.Sign(roguePriv, roguePayload)
	rogueSigHex := hex.EncodeToString(rogueSig)

	if VerifyAgentUpdateSignature(pubKeyHex, targetVersion, sha256Hex, rogueSigHex) {
		t.Fatalf("expected signature from untrusted private key to fail verification, but it passed")
	}

	// 7. Verify private key file permissions are 0600
	keyPath := filepath.Join(tmpDir, "agent_signer.key")
	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat private key file: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("expected 0600 permissions on private key file, got %o", perm)
	}
}
