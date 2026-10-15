//go:build !windows

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
		if err := installLinuxService(serverURL, token, enrollKey); err != nil {
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

func installLinuxService(serverURL, token, enrollKey string) error {
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

	syncServiceSystemdURL(cleanURL)
}

func syncServiceSystemdURL(targetURL string) {
	if targetURL == "" || targetURL == "http://127.0.0.1:3800" || targetURL == "http://localhost:3800" {
		return
	}
	servicePath := "/etc/systemd/system/gnnwatchtower-agent.service"
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return
	}
	lines := strings.Split(string(content), "\n")
	modified := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") {
			parts := strings.Fields(line)
			serverIdx := -1
			for idx, part := range parts {
				if part == "-server" {
					serverIdx = idx
					break
				}
			}
			if serverIdx != -1 && serverIdx+1 < len(parts) {
				parts[serverIdx+1] = targetURL
				lines[i] = strings.Join(parts, " ")
				modified = true
			}
		}
	}
	if modified {
		_ = os.WriteFile(servicePath, []byte(strings.Join(lines, "\n")), 0644)
		_ = exec.Command("systemctl", "daemon-reload").Run()
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
	// 3. Check systemd service file
	servicePath := "/etc/systemd/system/gnnwatchtower-agent.service"
	if data, err := os.ReadFile(servicePath); err == nil {
		parts := strings.Fields(string(data))
		for i, p := range parts {
			if p == "-server" && i+1 < len(parts) {
				u := strings.Trim(parts[i+1], `"' `)
				if strings.HasPrefix(u, "http") {
					return u
				}
			}
		}
	}
	return ""
}

func updateServiceWithCurrentBinary() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	targetExe := filepath.Join(getAgentDir(), "gnnwatchtower-agent")
	if exePath == targetExe {
		return fmt.Errorf("zaten kurulu olan servis dosyasi (%s) calistiriliyor", targetExe)
	}

	log.Printf("[*] Servis durduruluyor...")
	_ = stopOSService()

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

	savedURL := loadServerURL()
	if savedURL != "" {
		syncServiceSystemdURL(savedURL)
	}

	log.Printf("[*] Servis yeni ikili dosya ile baslatiliyor...")
	return startOSService()
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

	// Trigger restart via detached child process so systemd doesn't terminate restart when parent exits
	cmd := exec.Command("/bin/sh", "-c", "sleep 1 && systemctl restart gnnwatchtower-agent.service")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
	log.Println("[GNNwatchtower Agent] Upgrade applied. Restarting systemd service detached.")
	os.Exit(0)
	return nil
}
