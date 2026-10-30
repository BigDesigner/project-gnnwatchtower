package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gnnwatchtower/internal/database"
)

var (
	signerMu         sync.RWMutex
	updatePrivateKey ed25519.PrivateKey
	updatePublicKey  ed25519.PublicKey
)

// InitUpdateSigner initializes or loads the appliance Ed25519 signing key pair
func InitUpdateSigner(dataDir string) error {
	signerMu.Lock()
	defer signerMu.Unlock()

	if dataDir == "" {
		dataDir = "./data"
	}
	_ = os.MkdirAll(dataDir, 0700)
	keyPath := filepath.Join(dataDir, "agent_signer.key")

	// 1. Try to read existing private key
	if keyData, err := os.ReadFile(keyPath); err == nil && len(keyData) == ed25519.PrivateKeySize {
		updatePrivateKey = ed25519.PrivateKey(keyData)
		updatePublicKey = updatePrivateKey.Public().(ed25519.PublicKey)
		log.Printf("[GNNwatchtower Security] Loaded existing Ed25519 Update Signing Key (Public: %s...)", hex.EncodeToString(updatePublicKey)[:12])
		syncPublicKeyToDB(hex.EncodeToString(updatePublicKey))
		return nil
	}

	// 2. Generate a new Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed generating Ed25519 update signing key: %w", err)
	}

	updatePrivateKey = priv
	updatePublicKey = pub

	// Write private key with strict 0600 permissions
	if err := os.WriteFile(keyPath, []byte(priv), 0600); err != nil {
		log.Printf("[GNNwatchtower Security] UYARI: Ed25519 anahtar dosyaya yazılamadı: %v", err)
	}

	pubHex := hex.EncodeToString(pub)
	log.Printf("[GNNwatchtower Security] Generated fresh Ed25519 Update Signing Key (Public: %s...)", pubHex[:12])
	syncPublicKeyToDB(pubHex)
	return nil
}

func syncPublicKeyToDB(pubHex string) {
	if database.DB != nil {
		_, _ = database.DB.Exec(`
			INSERT INTO system_settings (key, value, description)
			VALUES ('agent_update_public_key', $1, 'Ajan uzaktan güncelleme Ed25519 doğrulama açık anahtarı')
			ON CONFLICT(key) DO UPDATE SET value = $1
		`, pubHex)
	}

}

// GetUpdatePublicKeyHex returns the 32-byte Ed25519 public key as a 64-char hex string
func GetUpdatePublicKeyHex() string {
	signerMu.RLock()
	defer signerMu.RUnlock()
	if len(updatePublicKey) == 0 {
		return ""
	}
	return hex.EncodeToString(updatePublicKey)
}

// SignAgentUpdate signs target version and SHA256 using Ed25519
func SignAgentUpdate(targetVersion, sha256Hex string) (string, error) {
	signerMu.RLock()
	defer signerMu.RUnlock()

	if len(updatePrivateKey) == 0 {
		return "", fmt.Errorf("ed25519 update signing key is not initialized")
	}

	cleanSHA := strings.ToLower(strings.TrimSpace(sha256Hex))
	cleanVer := strings.TrimSpace(targetVersion)
	payload := []byte(fmt.Sprintf("GNNWATCHTOWER_UPDATE:v%s:%s", cleanVer, cleanSHA))

	sig := ed25519.Sign(updatePrivateKey, payload)
	return hex.EncodeToString(sig), nil
}

// VerifyAgentUpdateSignature verifies whether an Ed25519 signature is valid for a given version and SHA256
func VerifyAgentUpdateSignature(pubKeyHex, targetVersion, sha256Hex, sigHex string) bool {
	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(sigHex))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}

	cleanSHA := strings.ToLower(strings.TrimSpace(sha256Hex))
	cleanVer := strings.TrimSpace(targetVersion)
	payload := []byte(fmt.Sprintf("GNNWATCHTOWER_UPDATE:v%s:%s", cleanVer, cleanSHA))

	return ed25519.Verify(ed25519.PublicKey(pubBytes), payload, sigBytes)
}
