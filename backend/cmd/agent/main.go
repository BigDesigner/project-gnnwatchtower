package main

import (
	"bytes"
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
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
)

type ServerHeartbeatResponse struct {
	Status        string `json:"status"`
	Action        string `json:"action,omitempty"` // "upgrade" or "none"
	TargetVersion string `json:"target_version,omitempty"`
	DownloadURL   string `json:"download_url,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
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
			return k
		}
	}
	return ""
}

func saveAgentKey(token string) error {
	keyPath := getAgentKeyPath()
	dir := os.Getenv("ProgramFiles")
	if dir != "" && runtime.GOOS == "windows" {
		_ = os.MkdirAll(fmt.Sprintf("%s\\GNNwatchtower-Agent", dir), 0755)
	} else if runtime.GOOS == "linux" {
		_ = os.MkdirAll("/opt/gnnwatchtower-agent", 0755)
	}
	return os.WriteFile(keyPath, []byte(token), 0600)
}

func autoEnroll(serverURL string) (string, error) {
	log.Printf("[GNNwatchtower Agent] Token bulunamadi. Wazuh tarzi otomatik kayit (Auto-Enrollment) baslatiliyor...")

	hostname, _ := os.Hostname()
	reqBody := map[string]interface{}{
		"hostname":      hostname,
		"local_ip":      getLocalIP(serverURL),
		"mac_address":   getMACAddress(),
		"os_name":       runtime.GOOS,
		"os_version":    getOSVersionDetails(),
		"arch":          runtime.GOARCH,
		"machine_uuid":  getMachineUUID(),
		"agent_version": Version,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	enrollURL := fmt.Sprintf("%s/api/agent/enroll", serverURL)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(enrollURL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("kayit istegi gonderilemedi (%s): %w", enrollURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("sunucu kaydi onaylamadi (Status %d): %s", resp.StatusCode, string(body))
	}

	var enrollResp struct {
		Status string `json:"status"`
		Token  string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&enrollResp); err != nil {
		return "", fmt.Errorf("kayit cevabi cozumlenemedi: %w", err)
	}

	if enrollResp.Token == "" {
		return "", fmt.Errorf("sunucu bos token dondurdu")
	}

	if err := saveAgentKey(enrollResp.Token); err != nil {
		log.Printf("[UYARI] agent.key dosyasina yazilamadi: %v", err)
	} else {
		log.Printf("[OK] Ajan basariyla kaydedildi. Kimlik anahtari guvenle saklandi (%s).", getAgentKeyPath())
	}

	return enrollResp.Token, nil
}

func main() {
	serverURL := flag.String("server", "http://127.0.0.1:3800", "GNNwatchtower Appliance URL")
	token := flag.String("token", "", "GNNwatchtower Agent Token (Opsiyonel, bos ise otomatik kayit yapilir)")
	serviceCmd := flag.String("service", "", "Servis islemleri: install, uninstall, start, stop, status")
	intervalSec := flag.Int("interval", 15, "Periyodik heartbeat araligi (saniye)")
	showVersion := flag.Bool("version", false, "Ajan surum numarasini gosterir")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GNNwatchtower Enterprise Telemetry Agent v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// 1. Check if invoked as service management command
	if *serviceCmd != "" {
		if handleServiceCommand(*serviceCmd, *serverURL, *token) {
			os.Exit(0)
		}
	}

	// 2. Windows Service Control Manager runtime check
	if runtime.GOOS == "windows" {
		isService, err := svc.IsWindowsService()
		if err == nil && isService {
			_ = runAsWindowsService(*serverURL, *token, *intervalSec)
			return
		}
	}

	// 3. Resolve agent token (CLI flag -> Local Key file -> Auto-Enrollment)
	resolvedToken := *token
	if resolvedToken == "" {
		resolvedToken = loadAgentKey()
	}

	if resolvedToken == "" {
		newToken, err := autoEnroll(*serverURL)
		if err != nil {
			log.Fatalf("[FATAL] Ajan kaydi basarisiz: %v. Lutfen sunucunun (%s) erisilebilir oldugundan emin olunuz.", err, *serverURL)
		}
		resolvedToken = newToken
	} else if *token != "" {
		_ = saveAgentKey(*token)
	}

	hostname, _ := os.Hostname()
	log.Printf("[GNNwatchtower Agent v%s] Baslatildi -> Hedef: %s (Cihaz: %s | %s/%s)", Version, *serverURL, hostname, runtime.GOOS, runtime.GOARCH)

	// Graceful shutdown channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Println("[GNNwatchtower Agent] Kapatma sinyali alindi! Merkeze Pre-Shutdown bildirimi gonderiliyor...")
		sendPreShutdown(*serverURL, resolvedToken, hostname, "OS Graceful Shutdown / Restart")
		log.Println("[GNNwatchtower Agent] Pre-shutdown tamamlandi. Temiz cikis yapiliyor.")
		os.Exit(0)
	}()

	runAgentLoop(*serverURL, resolvedToken, *intervalSec, nil)
}

func runAgentLoop(serverURL, token string, intervalSec int, stopChan <-chan struct{}) {
	hostname, _ := os.Hostname()

	// Initial heartbeat immediately
	resp := sendHeartbeat(serverURL, token, hostname)
	handleServerInstruction(serverURL, token, resp)

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			resp := sendHeartbeat(serverURL, token, hostname)
			handleServerInstruction(serverURL, token, resp)
		}
	}
}

func sendHeartbeat(serverURL, token, hostname string) *ServerHeartbeatResponse {
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
		return nil
	}

	url := fmt.Sprintf("%s/api/agent/report", serverURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", token)

	client := &http.Client{Timeout: 6 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("[UYARI] Heartbeat gonderilemedi (%s): %v", url, err)
		return nil
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		var sResp ServerHeartbeatResponse
		if err := json.NewDecoder(res.Body).Decode(&sResp); err == nil {
			return &sResp
		}
	} else if res.StatusCode == http.StatusUnauthorized {
		log.Printf("[UYARI] Sunucu token'i reddetti (401). Yeniden kayit denenecek...")
		if newToken, err := autoEnroll(serverURL); err == nil {
			_ = saveAgentKey(newToken)
		}
	}
	return nil
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

	if resp.Action == "upgrade" && resp.TargetVersion != "" && resp.TargetVersion != Version {
		upgradeMu.Lock()
		if isUpgrading {
			upgradeMu.Unlock()
			return
		}
		isUpgrading = true
		upgradeMu.Unlock()

		go performUpgrade(serverURL, token, resp.TargetVersion, resp.DownloadURL, resp.SHA256)
	}
}

func performUpgrade(serverURL, token, targetVer, downloadURL, expectedSHA256 string) {
	log.Printf("[GNNwatchtower Agent] Uzaktan guncelleme talimati alindi! Hedef Surum: v%s", targetVer)

	setUpgradeState("downloading", "")
	hostname, _ := os.Hostname()
	sendHeartbeat(serverURL, token, hostname)

	exePath, err := os.Executable()
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Executable path error: %v", err))
		sendHeartbeat(serverURL, token, hostname)
		return
	}

	fullDownloadURL := downloadURL
	if !strings.HasPrefix(fullDownloadURL, "http") {
		fullDownloadURL = strings.TrimRight(serverURL, "/") + downloadURL
	}

	tmpFile := exePath + ".update"
	_ = os.Remove(tmpFile)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(fullDownloadURL)
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Download error: %v", err))
		sendHeartbeat(serverURL, token, hostname)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		setUpgradeState("failed", fmt.Sprintf("HTTP %d download failure", resp.StatusCode))
		sendHeartbeat(serverURL, token, hostname)
		return
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		setUpgradeState("failed", fmt.Sprintf("Cannot create file: %v", err))
		sendHeartbeat(serverURL, token, hostname)
		return
	}

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)
	_, err = io.Copy(writer, resp.Body)
	out.Close()

	if err != nil {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("Write error: %v", err))
		sendHeartbeat(serverURL, token, hostname)
		return
	}

	// Verify SHA-256 Checksum
	computedSHA := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(computedSHA, expectedSHA256) {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("SHA256 uyusmazligi! Beklenen: %s, Hesaplanan: %s", expectedSHA256, computedSHA))
		sendHeartbeat(serverURL, token, hostname)
		log.Printf("[HATA] Guncelleme ikili dosyasinin SHA256 ozeti dogrulanamadi!")
		return
	}

	log.Printf("[OK] Yeni ajan ikili dosyasi SHA256 ile basariyla dogrulandi (%s).", computedSHA)
	setUpgradeState("applying", "")
	sendHeartbeat(serverURL, token, hostname)

	// Apply OS atomic swap & restart
	if err := applyOSUpgrade(tmpFile, exePath); err != nil {
		_ = os.Remove(tmpFile)
		setUpgradeState("failed", fmt.Sprintf("Swap error: %v", err))
		sendHeartbeat(serverURL, token, hostname)
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
