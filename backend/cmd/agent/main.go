package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ServerHeartbeatResponse struct {
	Status        string `json:"status"`
	Action        string `json:"action,omitempty"` // "upgrade" or "none"
	TargetVersion string `json:"target_version,omitempty"`
	DownloadURL   string `json:"download_url,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Signature     string `json:"signature,omitempty"`
}

type AgentReportPayload struct {
	Token          string  `json:"token"`
	Hostname       string  `json:"hostname"`
	LocalIP        string  `json:"local_ip"`
	MACAddress     string  `json:"mac_address,omitempty"`
	OSName         string  `json:"os_name"`
	OSVersion      string  `json:"os_version"`
	Architecture   string  `json:"arch,omitempty"`
	AgentVersion   string  `json:"agent_version,omitempty"`
	MachineUUID    string  `json:"machine_uuid,omitempty"`
	UpgradeStatus  string  `json:"upgrade_status,omitempty"`
	UpgradeError   string  `json:"upgrade_error,omitempty"`
	CPUUsagePct    float64 `json:"cpu_usage_pct"`
	RAMUsagePct    float64 `json:"ram_usage_pct"`
	DiskUsagePct   float64 `json:"disk_usage_pct"`
	NetRxKBps      float64 `json:"net_rx_kbps"`
	NetTxKBps      float64 `json:"net_tx_kbps"`
	TempCelsius    float64 `json:"temp_celsius"`
	IsShuttingDown bool    `json:"is_shutting_down"`
	ShutdownReason string  `json:"shutdown_reason"`
}

var (
	isUpgrading     bool
	upgradeMu       sync.Mutex
	lastUpgradeStat string
	lastUpgradeErr  string
)

func getLocalIP(serverURL string) string {
	u, err := url.Parse(serverURL)
	if err == nil && u.Hostname() != "" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		targetPort := u.Port()
		if targetPort == "" {
			targetPort = "3800"
		}
		conn, err := net.Dial("udp", net.JoinHostPort(u.Hostname(), targetPort))
		if err == nil {
			defer conn.Close()
			localAddr := conn.LocalAddr().(*net.UDPAddr)
			if localAddr != nil && !localAddr.IP.IsLoopback() {
				return localAddr.IP.String()
			}
		}
	}

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if localAddr != nil && !localAddr.IP.IsLoopback() {
			return localAddr.IP.String()
		}
	}

	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}
	return ""
}

func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if (iface.Flags&net.FlagUp) != 0 && (iface.Flags&net.FlagLoopback) == 0 {
				hw := iface.HardwareAddr.String()
				if hw != "" {
					return hw
				}
			}
		}
	}
	return ""
}

func loadAgentKey() string {
	keyPath := getAgentKeyPath()
	data, err := os.ReadFile(keyPath)
	if err == nil {
		k := strings.TrimSpace(string(data))
		if len(k) >= 16 {
			// Migrate legacy permissions
			_ = secureFilePermissions(keyPath)
			return k
		}
	}
	return ""
}

func secureFilePermissions(path string) error {
	if runtime.GOOS == "windows" {
		// Fix R11: Use locale-independent well-known SIDs:
		// *S-1-5-18 = Local SYSTEM
		// *S-1-5-32-544 = Builtin Administrators
		// /inheritance:r removes inherited ACEs
		// /grant:r replaces existing grants for these principals
		cmd := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", "*S-1-5-18:(F)", "/grant:r", "*S-1-5-32-544:(F)")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[GNNwatchtower Agent] Windows ACL ayarlama hatasi (%s): %v | %s", path, err, strings.TrimSpace(string(out)))
			return fmt.Errorf("icacls error: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return os.Chmod(path, 0600)
}

func getSignerKeyPath() string {
	dir := os.Getenv("ProgramFiles")
	if dir != "" && runtime.GOOS == "windows" {
		return fmt.Sprintf("%s\\GNNwatchtower-Agent\\signer.pub", dir)
	}
	return "/opt/gnnwatchtower-agent/signer.pub"
}

func loadSignerKey() ed25519.PublicKey {
	keyPath := getSignerKeyPath()
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil
	}
	hexStr := strings.TrimSpace(string(data))
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil || len(keyBytes) != ed25519.PublicKeySize {
		return nil
	}
	_ = secureFilePermissions(keyPath)
	return ed25519.PublicKey(keyBytes)
}

func saveSignerKey(pubKeyHex string) error {
	k := strings.TrimSpace(pubKeyHex)
	if k == "" {
		return nil
	}
	keyPath := getSignerKeyPath()
	dir := os.Getenv("ProgramFiles")
	if dir != "" && runtime.GOOS == "windows" {
		_ = os.MkdirAll(fmt.Sprintf("%s\\GNNwatchtower-Agent", dir), 0700)
	} else if runtime.GOOS == "linux" {
		_ = os.MkdirAll("/opt/gnnwatchtower-agent", 0700)
	}
	if err := os.WriteFile(keyPath, []byte(k), 0600); err != nil {
		return err
	}
	return secureFilePermissions(keyPath)
}

func saveAgentKey(token string) error {
	keyPath := getAgentKeyPath()
	dir := os.Getenv("ProgramFiles")
	if dir != "" && runtime.GOOS == "windows" {
		_ = os.MkdirAll(fmt.Sprintf("%s\\GNNwatchtower-Agent", dir), 0700)
	} else if runtime.GOOS == "linux" {
		_ = os.MkdirAll("/opt/gnnwatchtower-agent", 0700)
	}
	if err := os.WriteFile(keyPath, []byte(token), 0600); err != nil {
		return err
	}
	return secureFilePermissions(keyPath)
}

func loadEnrollKey() string {
	keyPath := getEnrollKeyPath()
	data, err := os.ReadFile(keyPath)
	if err == nil {
		k := strings.TrimSpace(string(data))
		if len(k) >= 8 {
			_ = secureFilePermissions(keyPath)
			return k
		}
	}
	return ""
}

func saveEnrollKey(key string) error {
	k := strings.TrimSpace(key)
	if k == "" {
		return nil
	}
	keyPath := getEnrollKeyPath()
	dir := os.Getenv("ProgramFiles")
	if dir != "" && runtime.GOOS == "windows" {
		_ = os.MkdirAll(fmt.Sprintf("%s\\GNNwatchtower-Agent", dir), 0700)
	} else if runtime.GOOS == "linux" {
		_ = os.MkdirAll("/opt/gnnwatchtower-agent", 0700)
	}
	if err := os.WriteFile(keyPath, []byte(k), 0600); err != nil {
		return err
	}
	return secureFilePermissions(keyPath)
}

func autoEnroll(serverURL, enrollKey string) (string, error) {
	if enrollKey == "" {
		enrollKey = loadEnrollKey()
	}
	log.Printf("[GNNwatchtower Agent] Token bulunamadi. Otomatik kayit (Auto-Enrollment) baslatiliyor...")

	hostname, _ := os.Hostname()
	reqBody := map[string]interface{}{
		"enrollment_key": enrollKey,
		"hostname":       hostname,
		"local_ip":       getLocalIP(serverURL),
		"mac_address":    getMACAddress(),
		"os_name":        runtime.GOOS,
		"os_version":     getOSVersionDetails(),
		"arch":           runtime.GOARCH,
		"machine_uuid":   getMachineUUID(),
		"agent_version":  Version,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	enrollURL := fmt.Sprintf("%s/api/agent/enroll", serverURL)
	req, err := http.NewRequest("POST", enrollURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if enrollKey != "" {
		req.Header.Set("X-Enrollment-Key", enrollKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kayit istegi gonderilemedi (%s): %w", enrollURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("sunucu kaydi onaylamadi (Status %d): %s", resp.StatusCode, string(body))
	}

	var enrollResp struct {
		Status          string `json:"status"`
		Token           string `json:"token"`
		UpdatePublicKey string `json:"update_public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&enrollResp); err != nil {
		return "", fmt.Errorf("kayit cevabi cozumlenemedi: %w", err)
	}

	if enrollResp.Token == "" {
		return "", fmt.Errorf("sunucu bos token dondurdu")
	}

	if enrollResp.UpdatePublicKey != "" {
		if err := saveSignerKey(enrollResp.UpdatePublicKey); err != nil {
			log.Printf("[UYARI] Ajan dogrulama acik anahtari (Signer Key) kaydedilemedi: %v", err)
		} else {
			log.Printf("[OK] Ajan guncelleme dogrulama acik anahtari (Ed25519) guvenle tescil edildi.")
		}
	}

	if err := saveAgentKey(enrollResp.Token); err != nil {
		log.Printf("[UYARI] agent.key dosyasina yazilamadi: %v", err)
	} else {
		log.Printf("[OK] Ajan basariyla kaydedildi. Kimlik anahtari guvenle saklandi (%s).", getAgentKeyPath())
	}

	return enrollResp.Token, nil
}

func main() {
	serverURL := flag.String("server", "", "GNNwatchtower Appliance URL (Orn: http://10.38.1.13:3800)")
	token := flag.String("token", "", "GNNwatchtower Agent Token (Opsiyonel, bos ise otomatik kayit yapilir)")
	enrollKeyFlag := flag.String("enroll-key", "", "GNNwatchtower Ajan Kayıt Anahtarı (Agent Enrollment Key)")
	signerKeyFlag := flag.String("signer-key", "", "GNNwatchtower Ajan Güncelleme Doğrulama Açık Anahtarı (Ed25519 Public Key)")
	requireHTTPSFlag := flag.Bool("require-https", false, "Yalnızca şifrelenmiş HTTPS bağlantılarına izin verir")
	serviceCmd := flag.String("service", "", "Servis islemleri: install, uninstall, start, stop, restart, status, diag")
	intervalSec := flag.Int("interval", 15, "Periyodik heartbeat araligi (saniye)")
	showVersion := flag.Bool("version", false, "Ajan surum numarasini gosterir")
	diagFlag := flag.Bool("diag", false, "Etkilesimli servis kontrol ve teshis konsolunu acar")
	statusFlag := flag.Bool("status", false, "Ajan ve servis durumunu gosterir")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GNNwatchtower Enterprise Telemetry Agent v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if *signerKeyFlag != "" {
		_ = saveSignerKey(*signerKeyFlag)
	}

	if *enrollKeyFlag != "" {
		_ = saveEnrollKey(*enrollKeyFlag)
	}

	// 1. Resolve effective server URL (CLI flag -> Saved server.url / SCM ImagePath -> Default)
	resolvedServerURL := *serverURL
	if resolvedServerURL == "" || resolvedServerURL == "http://127.0.0.1:3800" || resolvedServerURL == "http://localhost:3800" {
		if saved := loadServerURL(); saved != "" && saved != "http://127.0.0.1:3800" && saved != "http://localhost:3800" {
			resolvedServerURL = saved
		}
	}
	if resolvedServerURL != "" && resolvedServerURL != "http://127.0.0.1:3800" && resolvedServerURL != "http://localhost:3800" {
		saveServerURL(resolvedServerURL)
	}
	if resolvedServerURL == "" {
		resolvedServerURL = "http://127.0.0.1:3800"
	}

	if (*requireHTTPSFlag || os.Getenv("REQUIRE_HTTPS") == "true" || os.Getenv("REQUIRE_HTTPS") == "1") && strings.HasPrefix(strings.ToLower(resolvedServerURL), "http://") {
		log.Fatalf("[FATAL] Guvenlik ilkesi ihlali: -require-https aktif ancak sunucu adresi sifrelenmemis HTTP (%s) olarak belirtildi!", resolvedServerURL)
	}

	// 2. Check if invoked as service management command
	if *serviceCmd != "" {
		if handleServiceCommand(*serviceCmd, resolvedServerURL, *token, *enrollKeyFlag) {
			os.Exit(0)
		}
	}

	// 3. Resolve agent token (CLI flag -> Local Key file -> Auto-Enrollment)
	resolvedToken := *token
	if resolvedToken == "" {
		resolvedToken = loadAgentKey()
	}

	// 4. If -diag or -status is passed, launch interactive diagnostics console
	if *diagFlag || *statusFlag {
		runDiagnostics(resolvedServerURL, resolvedToken)
		os.Exit(0)
	}

	// 5. Windows Service Control Manager runtime check
	if runWindowsServiceIfUnderSCM(resolvedServerURL, resolvedToken, *intervalSec) {
		return
	}

	// 6. If run interactively on Windows with no arguments (e.g. double clicked in Explorer)
	if len(os.Args) == 1 && runtime.GOOS == "windows" {
		runDiagnostics(resolvedServerURL, resolvedToken)
		os.Exit(0)
	}

	if resolvedToken == "" {
		newToken, err := autoEnroll(resolvedServerURL, *enrollKeyFlag)
		if err != nil {
			log.Printf("[UYARI] Ajan otomatik kaydi yapilamadi: %v", err)
		} else {
			resolvedToken = newToken
		}
	} else if *token != "" {
		_ = saveAgentKey(*token)
	}

	hostname, _ := os.Hostname()
	log.Printf("[GNNwatchtower Agent v%s] Baslatildi -> Hedef: %s (Cihaz: %s | %s/%s)", Version, resolvedServerURL, hostname, runtime.GOOS, runtime.GOARCH)

	// Graceful shutdown channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Println("[GNNwatchtower Agent] Kapatma sinyali alindi! Merkeze Pre-Shutdown bildirimi gonderiliyor...")
		sendPreShutdown(resolvedServerURL, resolvedToken, hostname, "OS Graceful Shutdown / Restart")
		log.Println("[GNNwatchtower Agent] Pre-shutdown tamamlandi. Temiz cikis yapiliyor.")
		os.Exit(0)
	}()

	runAgentLoop(resolvedServerURL, resolvedToken, *intervalSec, nil)
}

func runAgentLoop(serverURL, token string, intervalSec int, stopChan <-chan struct{}) {
	if intervalSec <= 0 {
		intervalSec = 15
	}
	if serverURL == "" || serverURL == "http://127.0.0.1:3800" || serverURL == "http://localhost:3800" {
		if saved := loadServerURL(); saved != "" && saved != "http://127.0.0.1:3800" && saved != "http://localhost:3800" {
			serverURL = saved
		}
	}
	hostname, _ := os.Hostname()

	// Check for upgrade marker from previous upgrade
	checkUpgradeMarker()

	if token == "" {
		token = loadAgentKey()
	}
	if token == "" {
		if newToken, err := autoEnroll(serverURL, loadEnrollKey()); err == nil {
			token = newToken
		}
	}

	// Initial heartbeat immediately
	resp, newToken := sendHeartbeat(serverURL, token, hostname)
	if newToken != "" {
		token = newToken
	}
	handleServerInstruction(serverURL, token, resp)

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			if token == "" {
				token = loadAgentKey()
			}
			resp, newToken := sendHeartbeat(serverURL, token, hostname)
			if newToken != "" {
				token = newToken
			}
			handleServerInstruction(serverURL, token, resp)
		}
	}
}

func sendHeartbeat(serverURL, token, hostname string) (*ServerHeartbeatResponse, string) {
	if token == "" {
		token = loadAgentKey()
	}
	cpuPct, ramPct, diskPct, netRx, netTx, temp := getSystemMetrics()

	upgradeMu.Lock()
	upgStat := lastUpgradeStat
	upgErr := lastUpgradeErr
	upgradeMu.Unlock()

	report := &AgentReportPayload{
		Token:          token,
		Hostname:       hostname,
		LocalIP:        getLocalIP(serverURL),
		MACAddress:     getMACAddress(),
		OSName:         runtime.GOOS,
		OSVersion:      getOSVersionDetails(),
		Architecture:   runtime.GOARCH,
		AgentVersion:   Version,
		MachineUUID:    getMachineUUID(),
		UpgradeStatus:  upgStat,
		UpgradeError:   upgErr,
		CPUUsagePct:    math.Round(cpuPct*10) / 10,
		RAMUsagePct:    math.Round(ramPct*10) / 10,
		DiskUsagePct:   math.Round(diskPct*10) / 10,
		NetRxKBps:      math.Round(netRx*10) / 10,
		NetTxKBps:      math.Round(netTx*10) / 10,
		TempCelsius:    math.Round(temp*10) / 10,
		IsShuttingDown: false,
	}

	body, err := json.Marshal(report)
	if err != nil {
		return nil, token
	}

	url := fmt.Sprintf("%s/api/agent/report", serverURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, token
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", token)

	client := &http.Client{Timeout: 6 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("[UYARI] Heartbeat gonderilemedi (%s): %v", url, err)
		logToFile(fmt.Sprintf("[UYARI] Heartbeat gonderilemedi: %v", err))
		return nil, token
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		var sResp ServerHeartbeatResponse
		if err := json.NewDecoder(res.Body).Decode(&sResp); err == nil {
			logToFile(fmt.Sprintf("[OK] Heartbeat iletildi (HTTP 200 OK) -> Hedef: %s", serverURL))
			return &sResp, token
		}
	} else if res.StatusCode == http.StatusUnauthorized {
		log.Printf("[UYARI] Sunucu token'i reddetti (401). Yeniden kayit denenecek...")
		logToFile("[UYARI] Sunucu token'i reddetti (401)")
		if newToken, err := autoEnroll(serverURL, loadEnrollKey()); err == nil {
			_ = saveAgentKey(newToken)
			return nil, newToken
		}
	} else {
		logToFile(fmt.Sprintf("[UYARI] Sunucu HTTP %d dondu", res.StatusCode))
	}
	return nil, token
}

func sendPreShutdown(serverURL, token, hostname, reason string) {
	report := &AgentReportPayload{
		Token:          token,
		Hostname:       hostname,
		LocalIP:        getLocalIP(serverURL),
		OSName:         runtime.GOOS,
		OSVersion:      getOSVersionDetails(),
		AgentVersion:   Version,
		IsShuttingDown: true,
		ShutdownReason: reason,
	}

	body, err := json.Marshal(report)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/api/agent/report", serverURL)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if req != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", token)
		client := &http.Client{Timeout: 3 * time.Second}
		_, _ = client.Do(req)
	}
}

func handleServerInstruction(serverURL, token string, resp *ServerHeartbeatResponse) {
	if resp == nil {
		return
	}

	if resp.Action == "upgrade" && resp.TargetVersion != "" {
		if resp.TargetVersion == Version {
			setUpgradeState("success", "")
			return
		}
		upgradeMu.Lock()
		if isUpgrading {
			upgradeMu.Unlock()
			return
		}
		isUpgrading = true
		upgradeMu.Unlock()

		go performUpgrade(serverURL, token, resp.TargetVersion, resp.DownloadURL, resp.SHA256, resp.Signature)
	}
}

func performUpgrade(serverURL, token, targetVer, downloadURL, expectedSHA256, signatureHex string) {
	log.Printf("[GNNwatchtower Agent] Uzaktan guncelleme talimati alindi! Hedef Surum: v%s", targetVer)
	logToFile(fmt.Sprintf("[BILGI] Uzaktan guncelleme talimati alindi -> Hedef: v%s", targetVer))

	setUpgradeState("downloading", "")
	hostname, _ := os.Hostname()
	_, _ = sendHeartbeat(serverURL, token, hostname)

	exePath, err := os.Executable()
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Executable path error: %v", err))
		logToFile(fmt.Sprintf("[HATA] Executable path alinamadi: %v", err))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	cleanServerURL := strings.TrimRight(serverURL, "/")
	fullDownloadURL := downloadURL
	if !strings.HasPrefix(fullDownloadURL, "http://") && !strings.HasPrefix(fullDownloadURL, "https://") {
		if !strings.HasPrefix(fullDownloadURL, "/") {
			fullDownloadURL = "/" + fullDownloadURL
		}
		fullDownloadURL = cleanServerURL + fullDownloadURL
	}

	// Fix R03: Parse URLs and enforce strict origin pinning (scheme, host, port must match exactly)
	parsedServer, sErr := url.Parse(cleanServerURL)
	if sErr != nil || parsedServer.Host == "" {
		setUpgradeState("failed", "Guvenlik: Gecersiz sunucu adresi")
		logToFile("[HATA] Guvenlik: Gecersiz sunucu adresi")
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	parsedDownload, dErr := url.Parse(fullDownloadURL)
	if dErr != nil || parsedDownload.Host == "" {
		setUpgradeState("failed", "Guvenlik: Gecersiz indirme adresi")
		logToFile("[HATA] Guvenlik: Gecersiz indirme adresi")
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	if !strings.EqualFold(parsedServer.Scheme, parsedDownload.Scheme) || !strings.EqualFold(parsedServer.Host, parsedDownload.Host) {
		setUpgradeState("failed", "Guvenlik ihlali: Indirme adresi tanimli sunucu ile eslesmiyor")
		logToFile(fmt.Sprintf("[HATA] Guvenlik: Gecersiz indirme origin: %s != %s", parsedDownload.Host, parsedServer.Host))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	// Fix R03: Enforce mandatory 64-char SHA-256 (no empty checksum bypass)
	cleanExpectedSHA := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if len(cleanExpectedSHA) != 64 {
		setUpgradeState("failed", "Guvenlik ihlali: Guncelleme paketi resmi SHA-256 ozeti icermiyor veya gecersiz")
		logToFile("[HATA] Guvenlik: 64-karakter SHA-256 ozeti eksik veya gecersiz")
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	// Fix R03: Enforce mandatory Ed25519 digital signature verification!
	// This proves authenticity independently of the transport channel, completely defeating HTTP MITM substitution!
	signerPubKey := loadSignerKey()
	if signerPubKey == nil {
		setUpgradeState("failed", "Guvenlik: Ajan guncelleme dogrulama anahtari (Signer Key) bulunamadi")
		logToFile("[HATA] Guvenlik ihlali: Dogrulama anahtari (Signer Key) yuklenemedi, guncelleme durduruldu")
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	cleanSig := strings.TrimSpace(signatureHex)
	sigBytes, sErr := hex.DecodeString(cleanSig)
	if sErr != nil || len(sigBytes) != ed25519.SignatureSize {
		setUpgradeState("failed", "Guvenlik ihlali: Guncelleme paketi gecersiz veya eksik Ed25519 imzasi iceriyor")
		logToFile(fmt.Sprintf("[HATA] Guvenlik: Ed25519 imza uzunlugu veya formati hatali (len=%d)", len(sigBytes)))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	expectedMsg := []byte(fmt.Sprintf("GNNWATCHTOWER_UPDATE:v%s:%s", strings.TrimSpace(targetVer), cleanExpectedSHA))
	if !ed25519.Verify(signerPubKey, expectedMsg, sigBytes) {
		setUpgradeState("failed", "Guvenlik ihlali: Guncelleme paketi dijital imza dogrulamasini gecemedi (SAHTE VEYA DEGISTIRILMIS PAKET)!")
		logToFile(fmt.Sprintf("[HATA] Guvenlik ihlali: Ed25519 imza gecersiz! Hedef: v%s, SHA: %s", targetVer, cleanExpectedSHA))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		log.Printf("[HATA] Guncelleme paketi dijital imza dogrulamasini GECEMEDI! Ag uzerinde paket degistirilmis olabilir.")
		return
	}
	log.Printf("[GNNwatchtower Agent] Ed25519 dijital imza dogrulandi (Authenticity Verified).")

	tmpFile := exePath + ".update"
	_ = os.Remove(tmpFile)

	// Fix R03: Validate redirects to ensure download cannot be redirected to another domain
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !strings.EqualFold(req.URL.Host, parsedServer.Host) || !strings.EqualFold(req.URL.Scheme, parsedServer.Scheme) {
				return fmt.Errorf("guvenlik: harici yonlendirme reddedildi (%s)", req.URL.Host)
			}
			if len(via) >= 5 {
				return fmt.Errorf("cok fazla yonlendirme")
			}
			return nil
		},
	}
	resp, err := client.Get(fullDownloadURL)
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Download error: %v", err))
		logToFile(fmt.Sprintf("[HATA] Indirme hatasi: %v", err))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		setUpgradeState("failed", fmt.Sprintf("HTTP %d download failure", resp.StatusCode))
		logToFile(fmt.Sprintf("[HATA] Indirme basarisiz (HTTP %d)", resp.StatusCode))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Cannot create file: %v", err))
		logToFile(fmt.Sprintf("[HATA] Dosya olusturulamadi: %v", err))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)
	_, err = io.Copy(writer, resp.Body)
	out.Close()

	if err != nil {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("Write error: %v", err))
		logToFile(fmt.Sprintf("[HATA] Yazma hatasi: %v", err))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		return
	}

	// Verify SHA-256 Checksum (MANDATORY SECURITY INVARIANT)
	computedSHA := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(computedSHA, cleanExpectedSHA) {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("SHA256 uyusmazligi! Beklenen: %s, Hesaplanan: %s", cleanExpectedSHA, computedSHA))
		logToFile(fmt.Sprintf("[HATA] SHA-256 uyusmazligi: beklenen %s, hesaplanan %s", cleanExpectedSHA, computedSHA))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		log.Printf("[HATA] Guncelleme ikili dosyasinin SHA256 ozeti dogrulanamadi!")
		return
	}

	log.Printf("[OK] Yeni ajan ikili dosyasi SHA256 ile basariyla dogrulandi (%s).", computedSHA)
	logToFile(fmt.Sprintf("[OK] Yeni binary SHA-256 ile dogrulandi (%s). Yeniden baslatiliyor...", computedSHA[:8]))
	setUpgradeState("applying", "")
	_, _ = sendHeartbeat(serverURL, token, hostname)

	// Persist marker file before swap so restarted process reports success immediately
	writeUpgradeMarker(targetVer)

	// Apply OS atomic swap & restart
	if err := applyOSUpgrade(tmpFile, exePath); err != nil {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("Swap error: %v", err))
		logToFile(fmt.Sprintf("[HATA] Swap hatasi: %v", err))
		_, _ = sendHeartbeat(serverURL, token, hostname)
		log.Printf("[HATA] Guncelleme uygulanamadi: %v", err)
	}
}

func setUpgradeState(status, errMsg string) {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()
	lastUpgradeStat = status
	lastUpgradeErr = errMsg
	if status == "failed" {
		isUpgrading = false
	}
}

func logToFile(msg string) {
	logPath := filepath.Join(getAgentDir(), "agent.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil && fi.Size() > 2*1024*1024 {
		_ = f.Close()
		_ = os.Rename(logPath, logPath+".old")
		f, err = os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

func checkUpgradeMarker() {
	markerPath := filepath.Join(getAgentDir(), "upgrade.marker")
	data, err := os.ReadFile(markerPath)
	if err == nil {
		_ = os.Remove(markerPath)
		var marker struct {
			Status  string `json:"status"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &marker); err == nil && marker.Status == "success" {
			setUpgradeState("success", "")
			logToFile(fmt.Sprintf("[OK] Ajan guncellemesi basariyla tamamlandi (v%s)", Version))
		}
	}
}

func writeUpgradeMarker(targetVer string) {
	markerPath := filepath.Join(getAgentDir(), "upgrade.marker")
	data, _ := json.Marshal(map[string]string{
		"status":  "success",
		"version": targetVer,
	})
	_ = os.WriteFile(markerPath, data, 0644)
}
