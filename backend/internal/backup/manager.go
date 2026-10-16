package backup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
)

// BackupBundle represents the complete exportable state of the monitoring platform
type BackupBundle struct {
	Version       string                    `json:"version"`
	ExportedAt    time.Time                 `json:"exported_at"`
	Settings      []models.SystemSetting    `json:"settings"`
	AlertChannels []models.AlertChannel     `json:"alert_channels"`
	Devices       []DeviceExportData        `json:"devices"`
	Incidents     []models.DowntimeIncident `json:"incidents"`
}

type DeviceExportData struct {
	Device   models.Device             `json:"device"`
	Services []models.ServiceMonitored `json:"services"`
}

// ExportAll exports all configurations, devices, alert channels, and logs to a JSON bundle
func ExportAll() (*BackupBundle, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	bundle := &BackupBundle{
		Version:    "1.1.0",
		ExportedAt: time.Now(),
	}

	// 1. Settings
	sRows, err := database.DB.Query("SELECT key, value, description, updated_at FROM system_settings")
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var s models.SystemSetting
			var desc sql.NullString
			if err := sRows.Scan(&s.Key, &s.Value, &desc, &s.UpdatedAt); err == nil {
				if desc.Valid {
					s.Description = desc.String
				}
				bundle.Settings = append(bundle.Settings, s)
			}
		}
	}

	// 2. Alert Channels
	aRows, err := database.DB.Query("SELECT id, name, type, config_json, is_enabled, created_at FROM alert_channels")
	if err == nil {
		defer aRows.Close()
		for aRows.Next() {
			var a models.AlertChannel
			if err := aRows.Scan(&a.ID, &a.Name, &a.Type, &a.Config, &a.IsEnabled, &a.CreatedAt); err == nil {
				bundle.AlertChannels = append(bundle.AlertChannels, a)
			}
		}
	}

	// 3. Devices & their Monitored Services
	dRows, err := database.DB.Query(`
		SELECT id, name, ip_address, mac_address, device_type, os_version, purpose, 
		       location, operator_notes, is_agent_installed, agent_token, monitoring_profile, 
		       check_interval_seconds, status, created_at, updated_at, monitoring_mode, last_checked_at, last_probe_latency_ms
		FROM devices
	`)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d models.Device
			var mac, osVer, purp, loc, notes, token sql.NullString
			var checked sql.NullTime
			var rtt sql.NullFloat64
			if err := dRows.Scan(&d.ID, &d.Name, &d.IPAddress, &mac, &d.DeviceType, &osVer, &purp, &loc, &notes,
				&d.IsAgentInstalled, &token, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status,
				&d.CreatedAt, &d.UpdatedAt, &d.MonitoringMode, &checked, &rtt); err == nil {
				if checked.Valid {
					d.LastCheckedAt = &checked.Time
				}
				if rtt.Valid {
					v := rtt.Float64
					d.LastProbeLatencyMS = &v
				}
				if mac.Valid {
					d.MACAddress = mac.String
				}
				if osVer.Valid {
					d.OSVersion = osVer.String
				}
				if purp.Valid {
					d.Purpose = purp.String
				}
				if loc.Valid {
					d.Location = loc.String
				}
				if notes.Valid {
					d.OperatorNotes = notes.String
				}
				if token.Valid {
					d.AgentToken = token.String
				}

				// Load services after closing the inventory cursor.
				var services []models.ServiceMonitored
				bundle.Devices = append(bundle.Devices, DeviceExportData{
					Device:   d,
					Services: services,
				})
			}
		}
	}

	if dRows != nil {
		dRows.Close()
	}
	for i := range bundle.Devices {
		bundle.Devices[i].Services = getServicesForDevice(bundle.Devices[i].Device.ID)
	}

	// 4. Incidents
	iRows, err := database.DB.Query(`
		SELECT id, device_id, started_at, resolved_at, duration_seconds, classified_cause, 
		       auto_detection_hypothesis, operator_confirmed, operator_notes, classified_by, created_at
		FROM downtime_incidents
		ORDER BY started_at DESC
		LIMIT 1000
	`)
	if err == nil {
		defer iRows.Close()
		for iRows.Next() {
			var inc models.DowntimeIncident
			var resAt sql.NullTime
			var dur sql.NullInt32
			var hypo, notes, by sql.NullString
			if err := iRows.Scan(&inc.ID, &inc.DeviceID, &inc.StartedAt, &resAt, &dur, &inc.ClassifiedCause,
				&hypo, &inc.OperatorConfirmed, &notes, &by, &inc.CreatedAt); err == nil {
				if resAt.Valid {
					inc.ResolvedAt = &resAt.Time
				}
				if dur.Valid {
					d := int(dur.Int32)
					inc.DurationSeconds = &d
				}
				if hypo.Valid {
					inc.AutoDetectionHypothesis = hypo.String
				}
				if notes.Valid {
					inc.OperatorNotes = notes.String
				}
				if by.Valid {
					inc.ClassifiedBy = by.String
				}
				bundle.Incidents = append(bundle.Incidents, inc)
			}
		}
	}

	return bundle, nil
}

// ImportAll restores a backup bundle into the database
func ImportAll(data []byte) error {
	var bundle BackupBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("invalid backup JSON format: %w", err)
	}

	if bundle.Version != "" && bundle.Version != "1.0.0" && bundle.Version != "1.1.0" && !strings.HasPrefix(bundle.Version, "0.") {
		return fmt.Errorf("Desteklenmeyen yedek sürümü")
	}
	seenTargets := map[string]bool{}
	seenIDs := map[int]bool{}
	for _, entry := range bundle.Devices {
		d := entry.Device
		if err := models.NormalizeDevice(&d); err != nil {
			return err
		}
		if seenTargets[d.IPAddress] || (d.ID > 0 && seenIDs[d.ID]) {
			return fmt.Errorf("Yedekte yinelenen cihaz kimliği veya hedef var")
		}
		seenTargets[d.IPAddress] = true
		if d.ID > 0 {
			seenIDs[d.ID] = true
		}
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Restore Settings
	keyRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-]{16,64}$`)
	for _, s := range bundle.Settings {
		if s.Key == "scan_subnet" {
			continue
		}
		// Fix V02: Enforce strict format check on restored enrollment keys
		if s.Key == "agent_enrollment_key" {
			val := strings.TrimSpace(s.Value)
			if val != "" && !keyRegex.MatchString(val) {
				return fmt.Errorf("güvenlik ihlali: yedek paketindeki 'agent_enrollment_key' geçersiz veya güvensiz karakterler içeriyor")
			}
		}
		_, err := tx.Exec(`
			INSERT INTO system_settings (key, value, description, updated_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, description = EXCLUDED.description, updated_at = CURRENT_TIMESTAMP
		`, s.Key, s.Value, s.Description)
		if err != nil {
			return fmt.Errorf("failed restoring setting %s: %w", s.Key, err)
		}
	}

	// 2. Restore Alert Channels
	for _, a := range bundle.AlertChannels {
		configJSON := a.Config
		if configJSON == "" {
			configJSON = "{}"
		}
		_, _ = tx.Exec(`
			INSERT INTO alert_channels (name, type, config_json, is_enabled)
			VALUES ($1, $2, $3, $4)
		`, a.Name, a.Type, configJSON, a.IsEnabled)
	}

	// Restore metadata transactionally. Restored observations start unknown;
	// historical incidents retain their evidence, and imports never issue credentials.
	idMap := map[int]int{}
	for _, entry := range bundle.Devices {
		d := entry.Device
		if err := models.NormalizeDevice(&d); err != nil {
			return fmt.Errorf("geçersiz yedek cihazı: %w", err)
		}
		var existingMode string
		err := tx.QueryRow("SELECT monitoring_mode FROM devices WHERE ip_address=?", d.IPAddress).Scan(&existingMode)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && existingMode != d.MonitoringMode {
			return fmt.Errorf("yedek mevcut cihazın izleme kategorisini değiştiremez")
		}
		var token any
		installed := false
		if d.MonitoringMode == models.ModeServer && d.AgentToken != "" {
			if !regexp.MustCompile(`^[a-fA-F0-9]{32,64}$`).MatchString(d.AgentToken) {
				return fmt.Errorf("geçersiz ajan anahtarı")
			}
			token = d.AgentToken
			installed = d.IsAgentInstalled
		}
		var id int
		err = tx.QueryRow(`INSERT INTO devices(name,ip_address,mac_address,device_type,os_version,purpose,location,operator_notes,is_agent_installed,agent_token,monitoring_mode,monitoring_profile,check_interval_seconds,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'unknown')
  ON CONFLICT(ip_address) DO UPDATE SET name=excluded.name,mac_address=excluded.mac_address,device_type=excluded.device_type,os_version=excluded.os_version,purpose=excluded.purpose,location=excluded.location,operator_notes=excluded.operator_notes,monitoring_profile=excluded.monitoring_profile,check_interval_seconds=excluded.check_interval_seconds,updated_at=CURRENT_TIMESTAMP RETURNING id`, d.Name, d.IPAddress, d.MACAddress, d.DeviceType, d.OSVersion, d.Purpose, d.Location, d.OperatorNotes, installed, token, d.MonitoringMode, d.MonitoringProfile, d.CheckIntervalSeconds).Scan(&id)
		if err != nil {
			return fmt.Errorf("cihaz geri yüklenemedi: %w", err)
		}
		idMap[d.ID] = id
		for _, s := range entry.Services {
			if err := models.ValidateService(s); err != nil {
				return err
			}
			// Preserved legacy service records cannot execute outside server mode.
			_, err = tx.Exec(`INSERT INTO services_monitored(device_id,name,check_type,target_port,target_path,expected_status_code) SELECT ?,?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM services_monitored WHERE device_id=? AND name=? AND check_type=? AND target_port=? AND COALESCE(target_path,'')=?)`, id, s.Name, s.CheckType, s.TargetPort, s.TargetPath, s.ExpectedStatusCode, id, s.Name, s.CheckType, s.TargetPort, s.TargetPath)
			if err != nil {
				return err
			}
		}
	}
	for _, i := range bundle.Incidents {
		id, ok := idMap[i.DeviceID]
		if !ok {
			return fmt.Errorf("olay kaydının cihazı yedekte bulunamadı")
		}
		if i.StartedAt.IsZero() || (i.ResolvedAt != nil && i.ResolvedAt.Before(i.StartedAt)) {
			return fmt.Errorf("geçersiz olay zamanları")
		}
		_, err = tx.Exec(`INSERT INTO downtime_incidents(device_id,started_at,resolved_at,duration_seconds,classified_cause,auto_detection_hypothesis,operator_confirmed,operator_notes,classified_by) SELECT ?,?,?,?,?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM downtime_incidents WHERE device_id=? AND julianday(started_at)=julianday(?))`, id, backupTime(&i.StartedAt), backupTime(i.ResolvedAt), i.DurationSeconds, i.ClassifiedCause, i.AutoDetectionHypothesis, i.OperatorConfirmed, i.OperatorNotes, i.ClassifiedBy, id, backupTime(&i.StartedAt))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func backupTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format("2006-01-02 15:04:05.999999999")
}

func getServicesForDevice(devID int) []models.ServiceMonitored {
	var list []models.ServiceMonitored
	rows, err := database.DB.Query(`
		SELECT id, device_id, name, check_type, target_port, target_path, expected_status_code, status, response_time_ms
		FROM services_monitored WHERE device_id = $1
	`, devID)
	if err != nil {
		return list
	}
	defer rows.Close()

	for rows.Next() {
		var s models.ServiceMonitored
		var targetPath sql.NullString
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.Name, &s.CheckType, &s.TargetPort, &targetPath, &s.ExpectedStatusCode, &s.Status, &s.ResponseTimeMS); err == nil {
			if targetPath.Valid {
				s.TargetPath = targetPath.String
			}
			list = append(list, s)
		}
	}
	return list
}

// EnsureBackupPermissions enforces 0700 on backupDir and migrates all existing archives to 0600
func EnsureBackupPermissions(backupDir string) error {
	if backupDir == "" {
		backupDir = "./backups"
	}
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(backupDir, 0700); err != nil {
		log.Printf("[Backup Engine] UYARI: Yedek dizini izinleri (0700) uygulanamadi: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				filePath := filepath.Join(backupDir, entry.Name())
				if err := os.Chmod(filePath, 0600); err != nil {
					log.Printf("[Backup Engine] UYARI: Eski yedek dosyasi (%s) 0600 yapilamadi: %v", filePath, err)
				}
			}
		}
	}
	return nil
}

// StartDailyBackupScheduler runs background backup every day at 03:00 and keeps 30 days
func StartDailyBackupScheduler(backupDir string) {
	if backupDir == "" {
		backupDir = "./backups"
	}
	_ = EnsureBackupPermissions(backupDir)

	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			// Run at 03:00 AM (between 03:00 and 03:30)
			if now.Hour() == 3 && now.Minute() < 35 {
				filename := filepath.Join(backupDir, fmt.Sprintf("gnnwatchtower-auto-backup-%s.json", now.Format("2006-01-02")))
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					bundle, err := ExportAll()
					if err == nil {
						data, _ := json.MarshalIndent(bundle, "", "  ")
						_ = os.WriteFile(filename, data, 0600)
						log.Printf("[Backup Engine] Automated daily backup saved to %s (permissions: 0600)", filename)
						cleanOldBackups(backupDir, 30*24*time.Hour)
					}
				}
			}
		}
	}()
}

func cleanOldBackups(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
			log.Printf("[Backup Engine] Removed expired backup: %s", entry.Name())
		}
	}
}
