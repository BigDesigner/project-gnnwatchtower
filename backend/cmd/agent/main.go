package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type AgentReport struct {
	Token          string  `json:"token"`
	Hostname       string  `json:"hostname"`
	OSName         string  `json:"os_name"`
	OSVersion      string  `json:"os_version"`
	CPUUsagePct    float64 `json:"cpu_usage_pct"`
	RAMUsagePct    float64 `json:"ram_usage_pct"`
	DiskUsagePct   float64 `json:"disk_usage_pct"`
	IsShuttingDown bool    `json:"is_shutting_down"`
	ShutdownReason string  `json:"shutdown_reason"`
}

func main() {
	serverURL := flag.String("server", "http://127.0.0.1:3800", "GNNwatchtower Server URL")
	token := flag.String("token", "", "GNNwatchtower Agent Authentication Token")
	intervalSec := flag.Int("interval", 15, "Report interval in seconds")
	flag.Parse()

	if *token == "" {
		log.Fatal("[FATAL] -token parameter is required")
	}

	hostname, _ := os.Hostname()
	log.Printf("[GNNwatchtower Agent] Starting on %s (%s/%s) -> Target: %s", hostname, runtime.GOOS, runtime.GOARCH, *serverURL)

	// Graceful shutdown channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Pre-shutdown hook: send instant signal to central appliance
	go func() {
		<-shutdownChan
		log.Println("[GNNwatchtower Agent] OS Shutdown signal received! Emitting Pre-Shutdown signal...")
		sendReport(*serverURL, &AgentReport{
			Token:          *token,
			Hostname:       hostname,
			OSName:         runtime.GOOS,
			OSVersion:      runtime.GOARCH,
			IsShuttingDown: true,
			ShutdownReason: "OS Graceful Shutdown / Restart",
		})
		log.Println("[GNNwatchtower Agent] Pre-shutdown signal sent. Exiting cleanly.")
		os.Exit(0)
	}()

	ticker := time.NewTicker(time.Duration(*intervalSec) * time.Second)
	defer ticker.Stop()

	// Initial report
	sendHeartbeat(*serverURL, *token, hostname)

	for range ticker.C {
		sendHeartbeat(*serverURL, *token, hostname)
	}
}

func sendHeartbeat(serverURL, token, hostname string) {
	// Sample memory metrics via Go runtime
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	report := &AgentReport{
		Token:          token,
		Hostname:       hostname,
		OSName:         runtime.GOOS,
		OSVersion:      runtime.GOARCH,
		CPUUsagePct:    0.0, // Baseline idle
		RAMUsagePct:    float64(m.Alloc) / 1024.0 / 1024.0, // MB
		DiskUsagePct:   0.0,
		IsShuttingDown: false,
	}

	sendReport(serverURL, report)
}

func sendReport(serverURL string, report *AgentReport) {
	body, err := json.Marshal(report)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/api/agent/report", serverURL)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[WARN] Failed sending report to %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("[OK] Heartbeat acknowledged by server (%s)", resp.Status)
	} else {
		log.Printf("[WARN] Server returned status %d", resp.StatusCode)
	}
}
