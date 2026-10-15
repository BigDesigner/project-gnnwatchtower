package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	var isOnline bool
	var latency float64

	isWeb := strings.EqualFold(dev.DeviceType, "website") ||
		strings.EqualFold(dev.DeviceType, "web") ||
		strings.EqualFold(dev.DeviceType, "http") ||
		strings.HasPrefix(dev.IPAddress, "http://") ||
		strings.HasPrefix(dev.IPAddress, "https://")

	var statusCode int
	var proto string
	if isWeb {
		isOnline, latency, statusCode, proto = checkWebsiteReachability(dev.IPAddress, 3*time.Second)
	} else {
		isOnline, latency = PingHost(dev.IPAddress, 1*time.Second)
	}

	responseTime := int(latency)

	newStatus := "offline"
	if isOnline {
		newStatus = "online"
	}

	var notesUpdate string
	if isWeb {
		if statusCode > 0 {
			notesUpdate = fmt.Sprintf("HTTP %d • %s", statusCode, proto)
		} else {
			notesUpdate = "Bağlantı Hatası"
		}
	}

	// Check status transition
	if dev.Status != newStatus {
		if isWeb && notesUpdate != "" {
			_, _ = database.DB.Exec(`UPDATE devices SET operator_notes = $1 WHERE id = $2`, notesUpdate, dev.ID)
		}
		p.handleStatusTransition(dev, newStatus, responseTime)
	} else if isOnline {
		// Update ping time and last seen
		if isWeb && notesUpdate != "" {
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET last_seen_at = CURRENT_TIMESTAMP, response_time_ms = $1, operator_notes = $2, updated_at = CURRENT_TIMESTAMP 
				WHERE id = $3
			`, responseTime, notesUpdate, dev.ID)
		} else {
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET last_seen_at = CURRENT_TIMESTAMP, response_time_ms = $1, updated_at = CURRENT_TIMESTAMP 
				WHERE id = $2
			`, responseTime, dev.ID)
		}
	} else if isWeb && notesUpdate != "" {
		_, _ = database.DB.Exec(`UPDATE devices SET operator_notes = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, notesUpdate, dev.ID)
	}

	// Poll Monitored Services on this device
	p.pollDeviceServices(dev.ID, dev.IPAddress)
}

func isDisallowedHostOrIP(hostStr string) bool {
	h := strings.ToLower(strings.TrimSpace(hostStr))
	if idx := strings.Index(h, "://"); idx != -1 {
		h = h[idx+3:]
	}
	if slashIdx := strings.Index(h, "/"); slashIdx != -1 {
		h = h[:slashIdx]
	}
	if colonIdx := strings.LastIndex(h, ":"); colonIdx != -1 && !strings.Contains(h[colonIdx:], "]") {
		h = h[:colonIdx]
	}
	h = strings.Trim(h, "[]")

	if h == "" || h == "metadata.google.internal" || h == "instance-data" || h == "169.254.169.254" {
		return true
	}

	ips, err := net.LookupIP(h)
	if err != nil {
		ip := net.ParseIP(h)
		if ip != nil {
			ips = []net.IP{ip}
		} else {
			return false
		}
	}

	for _, ip := range ips {
		// Link-local unicast (169.254.0.0/16, fe80::/10) or multicast
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
		// Explicitly block AWS/GCP/Azure/OpenStack metadata IP
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return true
		}
	}
	return false
}

func checkWebsiteReachability(targetURL string, timeout time.Duration) (bool, float64, int, string) {
	urlStr := targetURL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil || isDisallowedHostOrIP(parsedURL.Hostname()) {
		return false, 0, 0, ""
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if isDisallowedHostOrIP(req.URL.Hostname()) {
				return fmt.Errorf("SSRF protection: redirect to link-local or cloud metadata address blocked")
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Get(urlStr)
	if err != nil {
		// If https failed and URL was plain host, fallback to http
		if !strings.HasPrefix(targetURL, "https://") && !strings.HasPrefix(targetURL, "http://") {
			fallbackURL, errFallback := url.Parse("http://" + targetURL)
			if errFallback == nil && !isDisallowedHostOrIP(fallbackURL.Hostname()) {
				respHttp, errHttp := client.Get("http://" + targetURL)
				if errHttp == nil {
					defer respHttp.Body.Close()
					latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
					return respHttp.StatusCode < 500, latencyMs, respHttp.StatusCode, "HTTP"
				}
			}
		}
		return false, 0, 0, ""
	}
	defer resp.Body.Close()

	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	proto := "HTTP"
	if resp.TLS != nil {
		proto = "HTTPS"
	}
	// Status < 500 means server is up (including 200 OK, 301/302 Redirect, 401/403 Auth)
	return resp.StatusCode < 500, latencyMs, resp.StatusCode, proto
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
		_ = tx.QueryRow(`
			INSERT INTO downtime_incidents (device_id, started_at, classified_cause, auto_detection_hypothesis)
			VALUES ($1, CURRENT_TIMESTAMP, 'unclassified', 'Lost network response')
			RETURNING id
		`, dev.ID).Scan(&incidentID)

		if err := tx.Commit(); err == nil {
			// Check correlation: Did multiple devices drop right now?
			if incidentID > 0 {
				incident.EvaluateSimultaneousOutages(dev.ID, incidentID)
			}
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

		if err := tx.Commit(); err == nil {
			incident.DispatchAlert(dev.Name, dev.IPAddress, "RECOVERY", fmt.Sprintf("Device recovered and is online (Latency: %dms).", respTime))
		}
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
