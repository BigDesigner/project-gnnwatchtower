package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gnnwatchtower/internal/api"
	"gnnwatchtower/internal/backup"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/monitor"
	"gnnwatchtower/internal/version"
	"runtime"
)

// ensureSelfSignedCert creates a zero-dependency self-signed ECDSA certificate for HTTPS
func ensureSelfSignedCert(certPath, keyPath string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil
		}
	}

	dir := filepath.Dir(certPath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GNNwatchtower Appliance"},
			CommonName:   "gnnwatchtower.local",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
		DNSNames:              []string{"localhost", "gnnwatchtower", "gnnwatchtower.local"},
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				template.IPAddresses = append(template.IPAddresses, ipnet.IP)
			}
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}); err != nil {
		return err
	}

	log.Printf("[GNNwatchtower TLS] Generated self-signed SSL/TLS certificate (%s, %s)", certPath, keyPath)
	return nil
}

//go:embed dist/*
var embeddedDist embed.FS

func main() {
	// 1. Resolve configuration
	resetAdmin := flag.String("reset-admin", "", "Admin kullanıcısının parolasını doğrudan konsoldan sıfırlar (Örn: -reset-admin 'YeniParolaniz')")
	showVersion := flag.Bool("version", false, "Sunucu sürüm numarasını gösterir")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GNNwatchtower Server Appliance v%s (%s/%s)\n", version.Get(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	log.Println("======================================================")
	log.Printf(" GNNwatchtower Network Monitoring Appliance v%s (Port 3800)\n", version.Get())
	log.Println(" Architecture: Single Binary • Pure Go SQLite WAL")
	log.Println(" License: GNU Affero General Public License v3 (AGPL-3.0)")
	log.Println("======================================================")

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3800"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		if _, err := os.Stat("/opt/gnnwatchtower/data"); err == nil {
			dbPath = "/opt/gnnwatchtower/data/gnnwatchtower.db"
		} else {
			dbPath = "./data/gnnwatchtower.db"
		}
	}

	// 2. Initialize Pure Go SQLite Database (WAL Mode)
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[FATAL] SQLite initialization failed: %v", err)
	}
	defer db.Close()

	// If -reset-admin flag is provided, reset password and exit
	if *resetAdmin != "" {
		newPass := *resetAdmin
		if len([]byte(newPass)) > 72 {
			log.Fatalf("[HATA] Parola 72 bayttan uzun olamaz.")
		}
		if len(newPass) < 6 {
			log.Fatalf("[HATA] Parola en az 6 karakter olmalıdır.")
		}
		newHash, err := database.HashPassword(newPass)
		if err != nil {
			log.Fatalf("[HATA] Parola hash üretilemedi: %v", err)
		}
		_, err = db.Exec("UPDATE users SET password_hash = $1, must_change_password = 0 WHERE username = 'admin'", newHash)
		if err != nil {
			log.Fatalf("[HATA] Parola güncellenemedi: %v", err)
		}
		_ = os.Remove("./data/initial_admin_password.txt")
		log.Println("[GNNwatchtower Auth] ======================================================")
		log.Println("[GNNwatchtower Auth] BAŞARILI: 'admin' kullanıcısının parolası güncellendi!")
		log.Println("[GNNwatchtower Auth] Yeni belirlediğiniz parola ile doğrudan giriş yapabilirsiniz.")
		log.Println("[GNNwatchtower Auth] ======================================================")
		return
	}

	// 3. Initialize Ed25519 Update Signer
	if err := api.InitUpdateSigner("./data"); err != nil {
		log.Printf("[WARN] Failed initializing update signer: %v", err)
	}

	// 4. Start Polling Engine (Goroutine worker pool)
	poller := monitor.StartPoller(15 * time.Second)
	defer poller.Stop()
	log.Println("[GNNwatchtower Core] Polling Engine started (15s baseline interval)")

	// 4. Start Nightly Automated Backup Worker (03:00 Backup Rotation)
	backup.StartDailyBackupScheduler("./backups")
	log.Println("[GNNwatchtower Core] Nightly 03:00 Backup Scheduler started")

	// 5. Mount Embedded Web UI Assets (dist/)
	var webFS fs.FS
	subFS, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		log.Printf("[WARN] Failed extracting embedded dist folder: %v", err)
	} else {
		webFS = subFS
		log.Println("[GNNwatchtower Core] Embedded Web UI mounted successfully")
	}

	// 6. Setup Gin Router
	router := api.SetupRouter(webFS)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 7. Start HTTP / HTTPS Server in background
	tlsEnabled := os.Getenv("TLS_ENABLED") == "true" || os.Getenv("TLS_ENABLED") == "1"
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	if tlsEnabled || certFile != "" || keyFile != "" {
		if certFile == "" {
			certFile = "./data/cert.pem"
		}
		if keyFile == "" {
			keyFile = "./data/key.pem"
		}
		if err := ensureSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatalf("[FATAL] TLS certificate generation/load failed: %v", err)
		}
		go func() {
			log.Printf("[GNNwatchtower Appliance] Secure Dashboard & API listening on https://0.0.0.0:%s", port)
			if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[FATAL] HTTPS Server failed: %v", err)
			}
		}()
	} else {
		go func() {
			log.Printf("[GNNwatchtower Appliance] Dashboard & API listening on http://0.0.0.0:%s", port)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[FATAL] HTTP Server failed: %v", err)
			}
		}()
	}

	// 8. Graceful Shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[GNNwatchtower] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[WARN] Server forced to shutdown: %v", err)
	}

	// Checkpoint SQLite WAL mode on clean exit
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		log.Printf("[WARN] WAL checkpoint failed: %v", err)
	}

	log.Println("[GNNwatchtower] Appliance exited cleanly.")
}
