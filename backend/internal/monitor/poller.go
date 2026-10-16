package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/incident"
	"gnnwatchtower/internal/models"
)

// Poller manages continuous device & service checks
type Poller struct {
	ticker       *time.Ticker
	stopChan     chan struct{}
	mu           sync.Mutex
	icmpProbe    func(string, time.Duration) (bool, *float64, error)
	websiteProbe func(string, time.Duration) (bool, float64, int, string)
	serviceProbe func(models.ServiceMonitored, string)
	notify       func(string, string, string, string)
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
	rows, err := database.DB.Query(`SELECT id,name,ip_address,device_type,monitoring_mode,monitoring_profile,check_interval_seconds,status,last_checked_at FROM devices WHERE monitoring_profile NOT IN ('paused','maintenance')`)
	if err != nil {
		log.Printf("[Monitor] Cannot load inventory: %v", err)
		return
	}
	var devices []models.Device
	for rows.Next() {
		var d models.Device
		var checked sql.NullTime
		if err = rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.DeviceType, &d.MonitoringMode, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status, &checked); err != nil {
			rows.Close()
			return
		}
		if checked.Valid {
			d.LastCheckedAt = &checked.Time
		}
		if d.LastCheckedAt == nil || time.Since(*d.LastCheckedAt) >= models.CheckCadence(d) {
			devices = append(devices, d)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return
	}
	rows.Close()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	for _, d := range devices {
		wg.Add(1)
		sem <- struct{}{}
		go func(d models.Device) { defer wg.Done(); defer func() { <-sem }(); p.checkSingleDevice(d) }(d)
	}
	wg.Wait()
}

func (p *Poller) checkSingleDevice(dev models.Device) {
	// Re-read policy before dispatch; never trust a stale caller's category/profile/target.
	var checked sql.NullTime
	err := database.DB.QueryRow(`SELECT name,ip_address,device_type,monitoring_mode,monitoring_profile,check_interval_seconds,status,last_checked_at FROM devices WHERE id=?`, dev.ID).Scan(&dev.Name, &dev.IPAddress, &dev.DeviceType, &dev.MonitoringMode, &dev.MonitoringProfile, &dev.CheckIntervalSeconds, &dev.Status, &checked)
	if err != nil || dev.MonitoringProfile == "paused" || dev.MonitoringProfile == "maintenance" {
		return
	}
	if checked.Valid && time.Since(checked.Time) < models.CheckCadence(dev) {
		return
	}
	safe := dev
	if err = models.NormalizeDevice(&safe); err != nil || safe.MonitoringMode != dev.MonitoringMode {
		_, _ = database.DB.Exec(`UPDATE devices SET status='unknown',last_checked_at=CURRENT_TIMESTAMP,last_probe_latency_ms=NULL,probe_error='Geçersiz hedef veya izleme politikası' WHERE id=?`, dev.ID)
		return
	}
	var online bool
	var rtt *float64
	httpResult := ""
	if dev.MonitoringMode == models.ModeWebsite {
		probe := p.websiteProbe
		if probe == nil {
			probe = checkWebsiteReachability
		}
		ok, latency, code, proto := probe(dev.IPAddress, 3*time.Second)
		online = ok
		if code > 0 {
			rtt = &latency
			httpResult = fmt.Sprintf("HTTP %d · %s", code, proto)
		} else {
			httpResult = "HTTP yanıtı alınamadı"
		}
	} else {
		probe := p.icmpProbe
		if probe == nil {
			probe = ProbeICMP
		}
		online, rtt, err = probe(dev.IPAddress, time.Second)
	}
	if err != nil {
		_, _ = database.DB.Exec(`UPDATE devices SET status='unknown',last_checked_at=CURRENT_TIMESTAMP,last_probe_latency_ms=NULL,probe_error='Yerel ICMP kontrolü çalıştırılamadı' WHERE id=? AND monitoring_profile NOT IN ('paused','maintenance') AND monitoring_mode=? AND ip_address=?`, dev.ID, dev.MonitoringMode, dev.IPAddress)
		return
	}
	status := "offline"
	if online {
		status = "online"
	}
	if err = p.recordObservation(dev, status, rtt, httpResult); err != nil {
		log.Printf("[Monitor] Observation persistence failed: %v", err)
		return
	}
	if dev.MonitoringMode == models.ModeServer {
		p.pollDeviceServices(dev.ID, dev.IPAddress)
	}
}

// A completed observation and its incident transition commit atomically.
func (p *Poller) recordObservation(dev models.Device, status string, rtt *float64, httpResult string) error {
	p.mu.Lock()
	locked := true
	defer func() {
		if locked {
			p.mu.Unlock()
		}
	}()
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous, mode, profile, address, previousProbeNote string
	if err = tx.QueryRow(`SELECT status,monitoring_mode,monitoring_profile,ip_address,probe_error FROM devices WHERE id=?`, dev.ID).Scan(&previous, &mode, &profile, &address, &previousProbeNote); err != nil {
		return err
	}
	if mode != dev.MonitoringMode || address != dev.IPAddress || profile == "paused" || profile == "maintenance" {
		return nil
	}
	observedOnline := status == "online"
	probeNote := ""
	// A recent authenticated report proves reachability even if one ICMP reply is lost.
	// Recheck inside the transaction so a concurrent heartbeat cannot be overwritten.
	if status == "offline" && mode == models.ModeServer {
		var installed bool
		var reported sql.NullTime
		if err = tx.QueryRow(`SELECT d.is_agent_installed,m.recorded_at FROM devices d LEFT JOIN system_metrics m ON m.id=(SELECT MAX(id) FROM system_metrics WHERE device_id=d.id) WHERE d.id=?`, dev.ID).Scan(&installed, &reported); err != nil {
			return err
		}
		if installed && reported.Valid && time.Since(reported.Time) >= -30*time.Second && time.Since(reported.Time) <= 90*time.Second {
			status = "online"
			rtt = nil
			probeNote = "ICMP yanıtı yok; güncel ajan raporu var"
		}
	}
	// Confirm loss on the next scheduled observation, without sending retries.
	const pendingICMP = "ICMP yanıtı alınamadı; doğrulama bekleniyor"
	if status == "offline" && mode == models.ModeICMP && previous != "offline" && !(previous == "unknown" && previousProbeNote == pendingICMP) {
		status = "unknown"
		probeNote = pendingICMP
		rtt = nil
	}
	response := 0
	if rtt != nil {
		response = int(*rtt)
	}
	_, err = tx.Exec(`UPDATE devices SET status=?,last_checked_at=CURRENT_TIMESTAMP,last_probe_latency_ms=?,probe_error=?,http_result=?,response_time_ms=?,last_seen_at=CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE last_seen_at END,last_status_change_at=CASE WHEN status!=? THEN CURRENT_TIMESTAMP ELSE last_status_change_at END,updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, rtt, probeNote, httpResult, response, observedOnline, status, dev.ID)
	if err != nil {
		return err
	}
	event := ""
	incidentID := 0
	if status == "offline" && previous != "offline" {
		var count int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM downtime_incidents WHERE device_id=? AND resolved_at IS NULL`, dev.ID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			res, e := tx.Exec(`INSERT INTO downtime_incidents(device_id,started_at,classified_cause,auto_detection_hypothesis) VALUES(?,CURRENT_TIMESTAMP,'unclassified','İzleme yöntemiyle yanıt alınamadı; güç durumu doğrulanmadı')`, dev.ID)
			if e != nil {
				return e
			}
			id, _ := res.LastInsertId()
			incidentID = int(id)
			event = "DOWNTIME"
		}
	} else if status == "online" {
		res, e := tx.Exec(`UPDATE downtime_incidents SET resolved_at=CURRENT_TIMESTAMP,duration_seconds=MAX(0,CAST(strftime('%s','now')-strftime('%s',started_at) AS INTEGER)) WHERE device_id=? AND resolved_at IS NULL`, dev.ID)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			event = "RECOVERY"
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	p.mu.Unlock()
	locked = false
	if event != "" {
		if p.notify != nil {
			p.notify(dev.Name, dev.IPAddress, event, "Erişilebilirlik durumu değişti")
		} else {
			if incidentID > 0 {
				incident.EvaluateSimultaneousOutages(dev.ID, incidentID)
			}
			incident.DispatchAlert(dev.Name, dev.IPAddress, event, "Erişilebilirlik durumu değişti; donanım sağlığı ölçülmedi.")
		}
	}
	return nil
}

func isDisallowedHostOrIP(hostStr string) bool {
	h := strings.TrimSpace(strings.ToLower(hostStr))
	if strings.Contains(h, "://") {
		u, err := url.Parse(h)
		if err != nil {
			return true
		}
		h = u.Hostname()
	} else if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	if h == "" || h == "metadata.google.internal" || h == "instance-data" {
		return true
	}
	ip := net.ParseIP(h)
	var ips []net.IP
	if ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.LookupIP(h)
		if err != nil {
			return false
		}
		ips = resolved
	}
	for _, ip := range ips {
		if ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() || ip.Equal(net.IPv4bcast) {
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

func (p *Poller) pollDeviceServices(deviceID int, ipAddress string) {
	rows, err := database.DB.Query(`
		SELECT id, name, check_type, target_port, target_path, expected_status_code
		FROM services_monitored
		WHERE device_id = $1 AND EXISTS (SELECT 1 FROM devices WHERE id=$1 AND monitoring_mode='server' AND monitoring_profile NOT IN ('paused','maintenance'))
	`, deviceID)
	if err != nil {
		return
	}
	defer rows.Close()

	var services []models.ServiceMonitored
	for rows.Next() {
		var s models.ServiceMonitored
		var targetPath sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.CheckType, &s.TargetPort, &targetPath, &s.ExpectedStatusCode); err != nil {
			continue
		}
		if targetPath.Valid {
			s.TargetPath = targetPath.String
		}

		services = append(services, s)
	}
	rows.Close()
	for _, s := range services {
		if p.serviceProbe != nil {
			p.serviceProbe(s, ipAddress)
		} else {
			checkSingleService(s, ipAddress)
		}
	}
}

func checkSingleService(s models.ServiceMonitored, ip string) {
	var allowed int
	if database.DB.QueryRow(`SELECT COUNT(*) FROM services_monitored s JOIN devices d ON d.id=s.device_id WHERE s.id=? AND d.monitoring_mode='server' AND d.monitoring_profile NOT IN ('paused','maintenance') AND d.ip_address=?`, s.ID, ip).Scan(&allowed) != nil || allowed != 1 {
		return
	}
	startTime := time.Now()
	serviceStatus := "offline"
	respTime := 0

	switch s.CheckType {
	case "tcp_port":
		addr := net.JoinHostPort(ip, strconv.Itoa(s.TargetPort))
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			_ = conn.Close()
			serviceStatus = "online"
			respTime = int(time.Since(startTime).Milliseconds())
		}
	case "http":
		url := fmt.Sprintf("http://%s:%d%s", ip, s.TargetPort, s.TargetPath)
		client := http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
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
