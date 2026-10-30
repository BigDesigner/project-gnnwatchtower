// Package tlsbootstrap authenticates appliance TLS trust using an existing agent signer pin.
package tlsbootstrap

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
)

type Bundle struct {
	Authority   string `json:"authority"`
	Nonce       string `json:"nonce"`
	Certificate string `json:"certificate"`
	Signature   string `json:"signature"`
}

func (b Bundle) Payload() []byte {
	data, _ := json.Marshal([]string{"GNNWATCHTOWER_TLS_V1", b.Authority, b.Nonce, b.Certificate})
	return data
}
func (b Bundle) Verify(key ed25519.PublicKey) bool {
	sig, e := hex.DecodeString(b.Signature)
	return e == nil && len(key) == ed25519.PublicKeySize && ed25519.Verify(key, b.Payload(), sig)
}
