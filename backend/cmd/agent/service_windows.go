//go:build windows

package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "GNNwatchtowerAgent"
	serviceDisplayName = "GNNwatchtower Telemetry Agent"
	serviceDesc        = "Enterprise Network & Server Telemetry Service for GNNwatchtower Monitoring Appliance"
)

var elog *eventlog.Log

func initEventLog() {
	var err error
	elog, err = eventlog.Open(serviceName)
	if err != nil {
		_ = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
		elog, _ = eventlog.Open(serviceName)
	}
}

func logEvent(eventType uint32, msg string) {
	if elog != nil {
		switch eventType {
		case eventlog.Error:
			_ = elog.Error(1001, msg)
		case eventlog.Warning:
			_ = elog.Warning(1002, msg)
		default:
			_ = elog.Info(1000, msg)
		}
	}
	log.Println(msg)
}

type windowsAgentService struct {
	serverURL string
	token     string
	interval  int
}

func (m *windowsAgentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	initEventLog()
	logEvent(eventlog.Info, fmt.Sprintf("[GNNwatchtower Agent] Service starting (v%s)...", Version))

	if m.serverURL == "" || m.serverURL == "http://127.0.0.1:3800" || m.serverURL == "http://localhost:3800" {
		if saved := loadServerURL(); saved != "" && saved != "http://127.0.0.1:3800" && saved != "http://localhost:3800" {
			m.serverURL = saved
		}
	}
	if m.interval <= 0 {
		m.interval = 15
	}

	stopChan := make(chan struct{})
	go runAgentLoop(m.serverURL, m.token, m.interval, stopChan)

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	logEvent(eventlog.Info, "[GNNwatchtower Agent] Service is running.")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				logEvent(eventlog.Info, "[GNNwatchtower Agent] Stop/Shutdown signal received. Emitting pre-shutdown telemetry...")
				hostname, _ := os.Hostname()
				sendPreShutdown(m.serverURL, m.token, hostname, "Windows SCM Stop / Graceful OS Shutdown")
				close(stopChan)
				break loop
			default:
				log.Printf("Unexpected service control request #%d", c)
			}
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	logEvent(eventlog.Info, "[GNNwatchtower Agent] Service stopped cleanly.")
	return
}

func runAsWindowsService(serverURL, token string, interval int) error {
	return svc.Run(serviceName, &windowsAgentService{
		serverURL: serverURL,
		token:     token,
		interval:  interval,
	})
}

func runWindowsServiceIfUnderSCM(serverURL, token string, intervalSec int) bool {
	isService, err := svc.IsWindowsService()
	if err == nil && isService {
		_ = runAsWindowsService(serverURL, token, intervalSec)
		return true
	}
	return false
}

func handleServiceCommand(cmd, serverURL, token, enrollKey string) bool {
	if serverURL != "" {
		saveServerURL(serverURL)
	}
	if token != "" {
		_ = saveAgentKey(token)
	}
	if enrollKey != "" {
		_ = saveEnrollKey(enrollKey)
	}
	switch strings.ToLower(cmd) {
	case "install":
		if err := installWindowsService(serverURL, token, enrollKey); err != nil {
			log.Fatalf("[FATAL] Windows Service kurulum hatasi: %v", err)
		}
		os.Exit(0)
	case "uninstall":
		if err := uninstallWindowsService(); err != nil {
			log.Fatalf("[FATAL] Windows Service kaldirma hatasi: %v", err)
		}
		os.Exit(0)
	case "start":
		if err := startWindowsService(); err != nil {
			log.Fatalf("[FATAL] Windows Service baslatma hatasi: %v", err)
		}
		os.Exit(0)
	case "stop":
		if err := stopWindowsService(); err != nil {
			log.Fatalf("[FATAL] Windows Service durdurma hatasi: %v", err)
		}
		os.Exit(0)
	case "restart":
		if err := restartOSService(); err != nil {
			log.Fatalf("[FATAL] Windows Service yeniden baslatma hatasi: %v", err)
		}
		os.Exit(0)
	case "status", "diag":
		runDiagnostics(serverURL, token)
		os.Exit(0)
	default:
		return false
	}
	return true
}

func installWindowsService(serverURL, token, enrollKey string) error {
	if serverURL != "" {
		saveServerURL(serverURL)
	}
	if token != "" {
		_ = saveAgentKey(token)
	}
	if enrollKey != "" {
		_ = saveEnrollKey(enrollKey)
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "GNNwatchtower-Agent")
	_ = os.MkdirAll(installDir, 0755)
	targetExe := filepath.Join(installDir, "gnnwatchtower-agent.exe")

	if strings.ToLower(exePath) != strings.ToLower(targetExe) {
		data, err := os.ReadFile(exePath)
		if err == nil {
			_ = os.WriteFile(targetExe, data, 0755)
		}
		exePath = targetExe
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("SCM baglanti hatasi (Yonetici yetkisi gerekli): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("servis '%s' zaten sistemde kayitli", serviceName)
	}

	args := []string{"-server", serverURL}
	if token != "" {
		args = append(args, "-token", token)
	}

	cfg := mgr.Config{
		DisplayName:      serviceDisplayName,
		Description:      serviceDesc,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		DelayedAutoStart: false,
	}

	s, err = m.CreateService(serviceName, exePath, cfg, args...)
	if err != nil {
		return fmt.Errorf("CreateService basarisiz: %w", err)
	}
	defer s.Close()

	// Event log source installation
	_ = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)

	// Set recovery actions: restart service after 10s, 30s, 60s
	recovery := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	_ = s.SetRecoveryActions(recovery, 86400)

	log.Printf("[OK] Windows Hizmeti '%s' basariyla kuruldu (%s).", serviceName, exePath)
	return nil
}

func uninstallWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("servis '%s' bulunamadi: %w", serviceName, err)
	}
	defer s.Close()

	_ = s.Delete()
	_ = eventlog.Remove(serviceName)
	log.Printf("[OK] Windows Hizmeti '%s' basariyla kaldirildi.", serviceName)
	return nil
}

func startWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("servise erisilemedi: %w", err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("servis baslatilamadi: %w", err)
	}
	log.Printf("[OK] Windows Hizmeti '%s' baslatildi.", serviceName)
	return nil
}

func stopWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("servise erisilemedi: %w", err)
	}
	defer s.Close()

	_, err = s.Control(svc.Stop)
	if err != nil && !strings.Contains(err.Error(), "has not been started") {
		return fmt.Errorf("servis durdurulamadi: %w", err)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		st, qErr := s.Query()
		if qErr == nil && st.State == svc.Stopped {
			break
		}
	}

	log.Printf("[OK] Windows Hizmeti '%s' durduruldu.", serviceName)
	return nil
}

func getOSServiceStatus() (string, bool) {
	m, err := mgr.Connect()
	if err != nil {
		return "SCM erisilemedi (Yonetici yetkisi gerekebilir)", false
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return "Kurulu Degil (Not Installed)", false
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return "Durum Alinamadi", false
	}

	switch status.State {
	case svc.Running:
		return "Calisiyor (Running)", true
	case svc.Stopped:
		return "Durduruldu (Stopped)", false
	case svc.StartPending:
		return "Baslatiliyor (StartPending)", false
	case svc.StopPending:
		return "Durduruluyor (StopPending)", false
	case svc.Paused:
		return "Duraklatildi (Paused)", false
	default:
		return fmt.Sprintf("Bilinmeyen (%d)", status.State), false
	}
}

func startOSService() error {
	return startWindowsService()
}

func stopOSService() error {
	return stopWindowsService()
}

func restartOSService() error {
	_ = stopWindowsService()
	time.Sleep(1 * time.Second)
	return startWindowsService()
}

func getMachineUUID() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		guid, _, err := k.GetStringValue("MachineGuid")
		if err == nil && guid != "" {
			return strings.TrimSpace(guid)
		}
	}
	return ""
}

func getOSVersionDetails() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		pname, _, _ := k.GetStringValue("ProductName")
		build, _, _ := k.GetStringValue("CurrentBuild")
		if pname != "" {
			return fmt.Sprintf("%s (Build %s)", pname, build)
		}
	}
	return "Windows Server"
}

func getAgentDir() string {
	programFiles := os.Getenv("ProgramFiles")
	if programFiles != "" {
		p := filepath.Join(programFiles, "GNNwatchtower-Agent")
		_ = os.MkdirAll(p, 0755)
		return p
	}
	exe, err := os.Executable()
	if err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

func getAgentKeyPath() string {
	return filepath.Join(getAgentDir(), "agent.key")
}

func getEnrollKeyPath() string {
	return filepath.Join(getAgentDir(), "enroll.key")
}

func saveServerURL(serverURL string) {
	if serverURL == "" {
		return
	}
	u, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return
	}
	cleanURL := u.Scheme + "://" + u.Host

	p := filepath.Join(getAgentDir(), "server.url")
	_ = os.WriteFile(p, []byte(cleanURL), 0644)

	syncServiceRegistryURL(cleanURL)
}

func syncServiceRegistryURL(targetURL string) {
	if targetURL == "" || targetURL == "http://127.0.0.1:3800" || targetURL == "http://localhost:3800" {
		return
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+serviceName, registry.SET_VALUE|registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		exePath, _ := os.Executable()
		if exePath == "" {
			exePath = filepath.Join(os.Getenv("ProgramFiles"), "GNNwatchtower-Agent", "gnnwatchtower-agent.exe")
		}
		newImagePath := fmt.Sprintf(`"%s" -server %s`, exePath, targetURL)
		_ = k.SetStringValue("ImagePath", newImagePath)
	}
}

func loadServerURL() string {
	// 1. Check getAgentDir()/server.url
	p := filepath.Join(getAgentDir(), "server.url")
	if data, err := os.ReadFile(p); err == nil {
		u := strings.TrimSpace(string(data))
		if u != "" && u != "http://127.0.0.1:3800" && u != "http://localhost:3800" {
			return u
		}
	}
	// 2. Check exe directory/server.url
	if exe, err := os.Executable(); err == nil {
		p2 := filepath.Join(filepath.Dir(exe), "server.url")
		if data, err := os.ReadFile(p2); err == nil {
			u := strings.TrimSpace(string(data))
			if u != "" && u != "http://127.0.0.1:3800" && u != "http://localhost:3800" {
				return u
			}
		}
	}
	// 3. Fallback: check Windows Service ImagePath in Registry
	if regURL := loadServerURLFromRegistry(); regURL != "" {
		saveServerURL(regURL)
		return regURL
	}
	return ""
}

func updateServiceWithCurrentBinary() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	targetExe := filepath.Join(getAgentDir(), "gnnwatchtower-agent.exe")
	if strings.EqualFold(exePath, targetExe) {
		return fmt.Errorf("zaten kurulu olan servis dosyasi (%s) calistiriliyor", targetExe)
	}

	log.Printf("[*] Servis durduruluyor...")
	_ = stopWindowsService()
	time.Sleep(1 * time.Second)

	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("calisan dosya okunamadi: %w", err)
	}

	oldFile := targetExe + ".old"
	_ = os.Remove(oldFile)
	_ = os.Rename(targetExe, oldFile)

	if err := os.WriteFile(targetExe, data, 0755); err != nil {
		_ = os.Rename(oldFile, targetExe)
		return fmt.Errorf("yeni binary yazilamadi: %w", err)
	}

	// Update Registry ImagePath
	savedURL := loadServerURL()
	if savedURL != "" {
		syncServiceRegistryURL(savedURL)
	}

	log.Printf("[*] Servis yeni ikili dosya ile baslatiliyor...")
	return startWindowsService()
}

func loadServerURLFromRegistry() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+serviceName, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		imagePath, _, err := k.GetStringValue("ImagePath")
		if err == nil && imagePath != "" {
			parts := strings.Split(imagePath, " ")
			for i, p := range parts {
				if p == "-server" && i+1 < len(parts) {
					u := strings.Trim(parts[i+1], `"' `)
					if strings.HasPrefix(u, "http") {
						return u
					}
				}
			}
		}
	}
	return ""
}

func applyOSUpgrade(updateFile, currentExe string) error {
	logEvent(eventlog.Info, fmt.Sprintf("[GNNwatchtower Agent] Applying atomic upgrade to %s...", currentExe))

	oldFile := currentExe + ".old"
	_ = os.Remove(oldFile)

	// Step 1: Atomic rename of running executable
	if err := os.Rename(currentExe, oldFile); err != nil {
		return fmt.Errorf("current binary rename failed: %w", err)
	}

	// Step 2: Move new binary into place
	if err := os.Rename(updateFile, currentExe); err != nil {
		// Rollback
		_ = os.Rename(oldFile, currentExe)
		return fmt.Errorf("new binary swap failed: %w", err)
	}

	// Step 3: Trigger service restart asynchronously
	cmd := exec.Command("cmd.exe", "/C", "timeout /t 2 /nobreak > NUL & net stop GNNwatchtowerAgent & net start GNNwatchtowerAgent")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()

	logEvent(eventlog.Info, "[GNNwatchtower Agent] Upgrade applied. Restarting service now.")
	os.Exit(0)
	return nil
}
