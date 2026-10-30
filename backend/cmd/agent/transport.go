package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gnnwatchtower/internal/tlsbootstrap"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var trustMu sync.Mutex
var trustedAgentTransport *http.Transport
var trustOrigin string

func trustPath() string { return filepath.Join(getAgentDir(), "server-cert.pem") }
func migrationTrust(server *url.URL) error {
	key := loadSignerKey()
	if key == nil {
		return fmt.Errorf("HTTPS migration requires the existing signer.pub or the authenticated installer")
	}
	nonce := make([]byte, 32)
	if _, e := rand.Read(nonce); e != nil {
		return e
	}
	target := *server
	target.Path = "/api/agent/signer-key"
	target.RawQuery = "nonce=" + hex.EncodeToString(nonce)
	target.User = nil
	// This request contains no token, enrollment key or telemetry. Existing pin authenticates its response.
	client := http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, e := client.Get(target.String())
	if e != nil {
		return fmt.Errorf("TLS trust bootstrap unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("TLS bootstrap not ready")
	}
	var bundle tlsbootstrap.Bundle
	if e = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&bundle); e != nil {
		return e
	}
	if bundle.Authority != server.Host || bundle.Nonce != hex.EncodeToString(nonce) || !bundle.Verify(key) {
		return fmt.Errorf("TLS trust signature mismatch")
	}
	certs := x509.NewCertPool()
	if !certs.AppendCertsFromPEM([]byte(bundle.Certificate)) {
		return fmt.Errorf("invalid TLS certificate")
	}
	f, e := os.CreateTemp(getAgentDir(), ".server-cert-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.WriteString(bundle.Certificate); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = secureFilePermissions(f.Name()); e != nil {
		return e
	}
	return os.Rename(f.Name(), trustPath())
}

type tlsOnlyTransport struct{}

func (tlsOnlyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if (r.URL.Scheme != "https" && r.URL.Scheme != "http") || r.URL.User != nil {
		return nil, fmt.Errorf("invalid appliance URL")
	}
	trustMu.Lock()
	if trustedAgentTransport == nil || trustOrigin != r.URL.Host {
		pemData, _ := os.ReadFile(trustPath())
		if len(bytes.TrimSpace(pemData)) == 0 && r.URL.Scheme == "http" {
			if e := migrationTrust(r.URL); e != nil {
				trustMu.Unlock()
				return nil, e
			}
			pemData, _ = os.ReadFile(trustPath())
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if len(pemData) > 0 && !roots.AppendCertsFromPEM(pemData) {
			trustMu.Unlock()
			return nil, fmt.Errorf("invalid appliance TLS trust file")
		}
		if trustedAgentTransport != nil {
			trustedAgentTransport.CloseIdleConnections()
		}
		trustedAgentTransport = &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second}
		trustOrigin = r.URL.Host
	}
	transport := trustedAgentTransport
	trustMu.Unlock()
	request := r.Clone(r.Context())
	target := *r.URL
	target.Scheme = "https"
	request.URL = &target
	// HTTP-configured legacy agents send credentials only after a verified TLS handshake.
	return transport.RoundTrip(request)
}
func sameAgentOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 || len(via) == 0 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("agent redirect outside trusted origin denied")
	}
	return nil
}
