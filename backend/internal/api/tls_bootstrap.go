package api

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"github.com/gin-gonic/gin"
	"gnnwatchtower/internal/tlsbootstrap"
	"os"
	"strings"
	"sync"
)

var certificateMu sync.RWMutex
var applianceCertificate string

// ConfigureTLSCertificate publishes only a parsed public certificate, never a key.
func ConfigureTLSCertificate(path string) error {
	data, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if len(data) > 65536 {
		return fmt.Errorf("certificate too large")
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("invalid public certificate")
	}
	if _, e = x509.ParseCertificate(block.Bytes); e != nil {
		return e
	}
	// Re-encode public certificate blocks only; a combined PEM must never disclose its private key.
	public := []byte{}
	remaining := data
	for len(remaining) > 0 {
		next, rest := pem.Decode(remaining)
		if next == nil {
			break
		}
		remaining = rest
		if next.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(next.Bytes); err != nil {
			return err
		}
		public = append(public, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: next.Bytes})...)
	}
	certificateMu.Lock()
	applianceCertificate = string(public)
	certificateMu.Unlock()
	return nil
}
func publicTLSCertificate() string {
	certificateMu.RLock()
	defer certificateMu.RUnlock()
	return applianceCertificate
}
func signedTLSBootstrap(c *gin.Context) bool {
	nonce := c.Query("nonce")
	if nonce == "" {
		return false
	}
	n, e := hex.DecodeString(nonce)
	if e != nil || len(n) != 32 || len(c.Request.Host) > 255 || strings.ContainsAny(c.Request.Host, "\r\n/") {
		c.JSON(400, gin.H{"error": "Invalid bootstrap challenge"})
		return true
	}
	b := tlsbootstrap.Bundle{Authority: c.Request.Host, Nonce: nonce, Certificate: publicTLSCertificate()}
	signerMu.RLock()
	defer signerMu.RUnlock()
	if b.Certificate == "" || len(updatePrivateKey) != ed25519.PrivateKeySize {
		c.JSON(503, gin.H{"error": "TLS trust unavailable"})
		return true
	}
	b.Signature = hex.EncodeToString(ed25519.Sign(updatePrivateKey, b.Payload()))
	c.Header("Cache-Control", "no-store")
	c.JSON(200, b)
	return true
}
