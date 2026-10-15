//go:build !windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runAsWindowsService(serverURL, token string, interval int) error {
	return fmt.Errorf("Windows Service yalnizca Windows isletim sistemlerinde calisir")
}

func runWindowsServiceIfUnderSCM(serverURL, token string, intervalSec int) bool {
	return false
}

func logEvent(eventType uint32, msg string) {
	log.Println(msg)
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
		if err := installLinuxService(serverURL, token); err != nil {
			log.Fatalf("[FATAL] Linux systemd servis kurulum hatasi: %v", err)
		}
		os.Exit(0)
	case "uninstall":
		if err := uninstallLinuxService(); err != nil {
			log.Fatalf("[FATAL] Linux systemd servis kaldirma hatasi: %v", err)
		}
		os.Exit(0)
	case "start":
		if err := startOSService(); err != nil {
			log.Fatalf("[FATAL] Servis baslatilamadi: %v", err)
		}
		log.Println("[OK] gnnwatchtower-agent servisi baslatildi.")
		os.Exit(0)
	case "stop":
		if err := stopOSService(); err != nil {
			log.Fatalf("[FATAL] Servis durdurulamadi: %v", err)
		}
		log.Println("[OK] gnnwatchtower-agent servisi durduruldu.")
		os.Exit(0)
	case "restart":
		if err := restartOSService(); err != nil {
			log.Fatalf("[FATAL] Servis yeniden baslatma hatasi: %v", err)
		}
		log.Println("[OK] gnnwatchtower-agent servisi yeniden baslatildi.")
		os.Exit(0)
	case "status", "diag":
		runDiagnostics(serverURL, token)
		os.Exit(0)
	default:
		return false
	}
	return true
}

func installLinuxService(serverURL, token string) error {
	if serverURL != "" {
		saveServerURL(serverURL)
	}
	if token != "" {
		_ = saveAgentKey(token)
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	installDir := "/opt/gnnwatchtower-agent"
	_ = os.MkdirAll(installDir, 0755)
	targetExe := filepath.Join(installDir, "gnnwatchtower-agent")

	if exePath != targetExe {
		data, err := os.ReadFile(exePath)
		if err == nil {
			_ = os.WriteFile(targetExe, data, 0755)
		}
		exePath = targetExe
	}

	args := fmt.Sprintf("-server %s", serverURL)
	if token != "" {
		args += fmt.Sprintf(" -token %s", token)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=GNNwatchtower Telemetry Agent
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s %s
Restart=always
RestartSec=5s
StandardOutput=journal
StandardError=journal
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, exePath, args)

	servicePath := "/etc/systemd/system/gnnwatchtower-agent.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("systemd dosyasi yazilamadi: %w", err)
	}

	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "enable", "gnnwatchtower-agent.service").Run()
	_ = exec.Command("systemctl", "restart", "gnnwatchtower-agent.service").Run()

	log.Printf("[OK] Linux Systemd servisi tescil edildi ve baslatildi: %s", servicePath)
	return nil
}

func uninstallLinuxService() error {
	_ = exec.Command("systemctl", "stop", "gnnwatchtower-agent.service").Run()
	_ = exec.Command("systemctl", "disable", "gnnwatchtower-agent.service").Run()
	_ = os.Remove("/etc/systemd/system/gnnwatchtower-agent.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	log.Println("[OK] Linux Systemd servisi kaldirildi.")
	return nil
}

func getMachineUUID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id
			}
		}
	}
	return ""
}

func getOSVersionDetails() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				val := strings.TrimPrefix(line, "PRETTY_NAME=")
				return strings.Trim(val, "\"")
			}
		}
	}
	return "Linux"
}

func getAgentDir() string {
	installDir := "/opt/gnnwatchtower-agent"
	if _, err := os.Stat(installDir); err == nil {
		return installDir
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

func getOSServiceStatus() (string, bool) {
	out, err := exec.Command("systemctl", "is-active", "gnnwatchtower-agent").Output()
	state := strings.TrimSpace(string(out))
	if err == nil && state == "active" {
		return "Calisiyor (Active/Running)", true
	}
	if state != "" {
		return fmt.Sprintf("Durduruldu (%s)", state), false
	}
	return "Kurulu Degil veya Inaktif", false
}

func startOSService() error {
	return exec.Command("systemctl", "start", "gnnwatchtower-agent").Run()
}

func stopOSService() error {
	return exec.Command("systemctl", "stop", "gnnwatchtower-agent").Run()
}

func restartOSService() error {
	return exec.Command("systemctl", "restart", "gnnwatchtower-agent").Run()
}

func applyOSUpgrade(updateFile, currentExe string) error {
	log.Printf("[GNNwatchtower Agent] Applying atomic upgrade to %s...", currentExe)

	if err := os.Chmod(updateFile, 0755); err != nil {
		return fmt.Errorf("chmod error: %w", err)
	}

	oldFile := currentExe + ".old"
	_ = os.Remove(oldFile)
	_ = os.Rename(currentExe, oldFile)

	if err := os.Rename(updateFile, currentExe); err != nil {
		_ = os.Rename(oldFile, currentExe)
		return fmt.Errorf("binary swap error: %w", err)
	}

	// Trigger restart via systemd
	_ = exec.Command("systemctl", "restart", "gnnwatchtower-agent.service").Start()
	log.Println("[GNNwatchtower Agent] Upgrade applied. Restarting systemd service now.")
	os.Exit(0)
	return nil
}
