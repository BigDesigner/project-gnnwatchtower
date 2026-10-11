package backup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
)

// BackupBundle represents the complete exportable state of the monitoring platform
type BackupBundle struct {
	Version       string                    `json:"version"`
	ExportedAt    time.Time                 `json:"exported_at"`
	Settings      []models.SystemSetting    `json:"settings"`
	AlertChannels []models.AlertChannel    `json:"alert_channels"`
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
		Version:    "1.0.0",
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
		       check_interval_seconds, status, created_at, updated_at
		FROM devices
	`)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d models.Device
			var mac, osVer, purp, loc, notes, token sql.NullString
			if err := dRows.Scan(&d.ID, &d.Name, &d.IPAddress, &mac, &d.DeviceType, &osVer, &purp, &loc, &notes,
				&d.IsAgentInstalled, &token, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status,
				&d.CreatedAt, &d.UpdatedAt); err == nil {
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

				// Fetch services for this device
				services := getServicesForDevice(d.ID)
				bundle.Devices = append(bundle.Devices, DeviceExportData{
					Device:   d,
					Services: services,
				})
			}
		}
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

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Restore Settings
	for _, s := range bundle.Settings {
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

	// 3. Restore Devices & Services
	for _, dData := range bundle.Devices {
		var devID int
		err := tx.QueryRow(`
			INSERT INTO devices (name, ip_address, mac_address, device_type, os_version, purpose, location, operator_notes, is_agent_installed, agent_token, monitoring_profile, check_interval_seconds, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'unknown')
			ON CONFLICT (ip_address) DO UPDATE SET
				name = EXCLUDED.name,
				mac_address = EXCLUDED.mac_address,
				device_type = EXCLUDED.device_type,
				os_version = EXCLUDED.os_version,
				purpose = EXCLUDED.purpose,
				location = EXCLUDED.location,
				operator_notes = EXCLUDED.operator_notes,
				monitoring_profile = EXCLUDED.monitoring_profile,
				updated_at = CURRENT_TIMESTAMP
			RETURNING id
		`, dData.Device.Name, dData.Device.IPAddress, dData.Device.MACAddress, dData.Device.DeviceType,
			dData.Device.OSVersion, dData.Device.Purpose, dData.Device.Location, dData.Device.OperatorNotes,
			dData.Device.IsAgentInstalled, dData.Device.AgentToken, dData.Device.MonitoringProfile,
			dData.Device.CheckIntervalSeconds).Scan(&devID)

		if err != nil {
			continue
		}

		// Restore Services
		for _, s := range dData.Services {
			_, _ = tx.Exec(`
				INSERT INTO services_monitored (device_id, name, check_type, target_port, target_path, expected_status_code)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, devID, s.Name, s.CheckType, s.TargetPort, s.TargetPath, s.ExpectedStatusCode)
		}
	}

	return tx.Commit()
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

// StartDailyBackupScheduler runs background backup every day at 03:00 and keeps 30 days
func StartDailyBackupScheduler(backupDir string) {
	if backupDir == "" {
		backupDir = "./backups"
	}
	_ = os.MkdirAll(backupDir, 0755)

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
						_ = os.WriteFile(filename, data, 0644)
						log.Printf("[Backup Engine] Automated daily backup saved to %s", filename)
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

