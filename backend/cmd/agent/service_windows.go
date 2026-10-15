//go:build windows

package main

import (
	"fmt"
	"log"
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

func handleServiceCommand(cmd, serverURL, token string) bool {
	if serverURL != "" {
		saveServerURL(serverURL)
	}
	if token != "" {
		_ = saveAgentKey(token)
	}
	switch strings.ToLower(cmd) {
	case "install":
		if err := installWindowsService(serverURL, token); err != nil {
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

func installWindowsService(serverURL, token string) error {
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
	if err != nil {
		return fmt.Errorf("servis durdurulamadi: %w", err)
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

func saveServerURL(serverURL string) {
	if serverURL == "" {
		return
	}
	p := filepath.Join(getAgentDir(), "server.url")
	_ = os.WriteFile(p, []byte(strings.TrimSpace(serverURL)), 0644)
}

func loadServerURL() string {
	p := filepath.Join(getAgentDir(), "server.url")
	data, err := os.ReadFile(p)
	if err == nil {
		u := strings.TrimSpace(string(data))
		if u != "" {
			return u
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
