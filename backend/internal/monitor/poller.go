package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/incident"
	"gnnwatchtower/internal/models"
)

// Poller manages continuous device & service checks
type Poller struct {
	ticker   *time.Ticker
	stopChan chan struct{}
	mu       sync.Mutex
}

var GlobalPoller *Poller

func StartPoller(interval time.Duration) *Poller {
	p := &Poller{
		ticker:   time.NewTicker(interval),
		stopChan: make(chan struct{}),
	}
	GlobalPoller = p

	go p.run()
	log.Printf("[Monitor] Device polling engine started with interval %v", interval)
	return p
}

func (p *Poller) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	close(p.stopChan)
}

func (p *Poller) run() {
	// Initial poll on startup
	p.pollDevices()

	for {
		select {
		case <-p.ticker.C:
			p.pollDevices()
		case <-p.stopChan:
			return
		}
	}
}

func (p *Poller) pollDevices() {
	if database.DB == nil {
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, name, ip_address, device_type, monitoring_profile, check_interval_seconds, status, last_seen_at
		FROM devices
		WHERE monitoring_profile != 'paused' AND monitoring_profile != 'maintenance'
	`)
	if err != nil {
		log.Printf("[Monitor] Query error fetching devices: %v", err)
		return
	}
	defer rows.Close()

	var devicesToCheck []models.Device
	now := time.Now()

	for rows.Next() {
		var d models.Device
		var lastSeen sql.NullTime
		if err := rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.DeviceType, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status, &lastSeen); err == nil {
			if lastSeen.Valid {
				d.LastSeenAt = &lastSeen.Time
			}

			// Check based on profile frequency
			shouldCheck := false
			switch d.MonitoringProfile {
			case "active":
				shouldCheck = true
			case "dormant_24h":
				if d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) >= 24*time.Hour {
					shouldCheck = true
				}
			case "dormant_30d":
				if d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) >= 30*24*time.Hour {
					shouldCheck = true
				}
			default:
				shouldCheck = true
			}

			if shouldCheck {
				devicesToCheck = append(devicesToCheck, d)
			}
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)

	for _, dev := range devicesToCheck {
		wg.Add(1)
		sem <- struct{}{}

		go func(target models.Device) {
			defer wg.Done()
			defer func() { <-sem }()
			p.checkSingleDevice(target)
		}(dev)
	}

	wg.Wait()
}

func (p *Poller) checkSingleDevice(dev models.Device) {
	startTime := time.Now()
	isOnline := false
	responseTime := 0

	// 1. Try TCP ports check or ICMP
	ports := []int{80, 443, 22, 3389, 5000, 902}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dev.IPAddress, port), 800*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			isOnline = true
			responseTime = int(time.Since(startTime).Milliseconds())
			break
		}
	}

	// Fallback to ICMP ping check
	if !isOnline {
		d := net.Dialer{Timeout: 800 * time.Millisecond}
		conn, err := d.Dial("ip4:icmp", dev.IPAddress)
		if err == nil {
			_ = conn.Close()
			isOnline = true
			responseTime = int(time.Since(startTime).Milliseconds())
		}
	}

	newStatus := "offline"
	if isOnline {
		newStatus = "online"
	}

	// Check status transition
	if dev.Status != newStatus {
		p.handleStatusTransition(dev, newStatus, responseTime)
	} else if isOnline {
		// Update ping time and last seen
		_, _ = database.DB.Exec(`
			UPDATE devices 
			SET last_seen_at = CURRENT_TIMESTAMP, response_time_ms = $1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = $2
		`, responseTime, dev.ID)
	}

	// Poll Monitored Services on this device
	p.pollDeviceServices(dev.ID, dev.IPAddress)
}

func (p *Poller) handleStatusTransition(dev models.Device, newStatus string, respTime int) {
	log.Printf("[Monitor] Status changed for %s (%s): %s -> %s", dev.Name, dev.IPAddress, dev.Status, newStatus)

	tx, err := database.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	if newStatus == "offline" {
		// Device went down -> Record incident
		_, _ = tx.Exec(`
			UPDATE devices 
			SET status = 'offline', last_status_change_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP 
			WHERE id = $1
		`, dev.ID)

		var incidentID int
		err := tx.QueryRow(`
			INSERT INTO downtime_incidents (device_id, started_at, classified_cause, auto_detection_hypothesis)
			VALUES ($1, CURRENT_TIMESTAMP, 'unclassified', 'Lost network response')
			RETURNING id
		`, dev.ID).Scan(&incidentID)

		if err == nil {
			_ = tx.Commit()
			// Check correlation: Did multiple devices drop right now?
			incident.EvaluateSimultaneousOutages(dev.ID, incidentID)
			incident.DispatchAlert(dev.Name, dev.IPAddress, "DOWNTIME", "Device stopped responding.")
		}
	} else if newStatus == "online" {
		// Device recovered -> Close active incident
		_, _ = tx.Exec(`
			UPDATE devices 
			SET status = 'online', last_seen_at = CURRENT_TIMESTAMP, last_status_change_at = CURRENT_TIMESTAMP, response_time_ms = $1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = $2
		`, respTime, dev.ID)

		// Find open incident
		var incID int
		var startedAt time.Time
		err := tx.QueryRow(`
			SELECT id, started_at FROM downtime_incidents 
			WHERE device_id = $1 AND resolved_at IS NULL 
			ORDER BY started_at DESC LIMIT 1
		`, dev.ID).Scan(&incID, &startedAt)

		if err == nil {
			duration := int(time.Since(startedAt).Seconds())
			_, _ = tx.Exec(`
				UPDATE downtime_incidents 
				SET resolved_at = CURRENT_TIMESTAMP, duration_seconds = $1 
				WHERE id = $2
			`, duration, incID)
		}

		_ = tx.Commit()
		incident.DispatchAlert(dev.Name, dev.IPAddress, "RECOVERY", fmt.Sprintf("Device recovered and is online (Latency: %dms).", respTime))
	}
}

func (p *Poller) pollDeviceServices(deviceID int, ipAddress string) {
	rows, err := database.DB.Query(`
		SELECT id, name, check_type, target_port, target_path, expected_status_code
		FROM services_monitored
		WHERE device_id = $1
	`, deviceID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var s models.ServiceMonitored
		var targetPath sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.CheckType, &s.TargetPort, &targetPath, &s.ExpectedStatusCode); err != nil {
			continue
		}
		if targetPath.Valid {
			s.TargetPath = targetPath.String
		}

		go checkSingleService(s, ipAddress)
	}
}

func checkSingleService(s models.ServiceMonitored, ip string) {
	startTime := time.Now()
	serviceStatus := "offline"
	respTime := 0

	switch s.CheckType {
	case "tcp_port":
		addr := fmt.Sprintf("%s:%d", ip, s.TargetPort)
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			_ = conn.Close()
			serviceStatus = "online"
			respTime = int(time.Since(startTime).Milliseconds())
		}
	case "http":
		url := fmt.Sprintf("http://%s:%d%s", ip, s.TargetPort, s.TargetPath)
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == s.ExpectedStatusCode || (s.ExpectedStatusCode == 0 && resp.StatusCode >= 200 && resp.StatusCode < 400) {
				serviceStatus = "online"
			} else {
				serviceStatus = "degraded"
			}
			respTime = int(time.Since(startTime).Milliseconds())
		}
	}

	_, _ = database.DB.Exec(`
		UPDATE services_monitored 
		SET status = $1, response_time_ms = $2, last_check_at = CURRENT_TIMESTAMP 
		WHERE id = $3
	`, serviceStatus, respTime, s.ID)
}
