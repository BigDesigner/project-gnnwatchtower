package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func completeAgentInstaller(c *gin.Context, script, platform string) {
	cert := publicTLSCertificate()
	block, _ := pem.Decode([]byte(cert))
	key := GetUpdatePublicKeyHex()
	checksum := getBinaryChecksum(platform)
	if block == nil || len(key) != 64 || len(checksum) != 64 {
		c.JSON(503, gin.H{"error": "Kurulum için sertifika, imza anahtarı veya ajan ikilisi hazır değil"})
		return
	}
	fp := sha256.Sum256(block.Bytes)
	for marker, value := range map[string]string{"__WT_CERT_B64__": base64.StdEncoding.EncodeToString([]byte(cert)), "__WT_CERT_FP__": hex.EncodeToString(fp[:]), "__WT_SIGNER__": key, "__WT_SHA__": checksum} {
		script = strings.ReplaceAll(script, marker, value)
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, fmt.Sprint(script))
}
