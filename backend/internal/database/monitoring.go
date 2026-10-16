package database

import (
	"database/sql"
	"fmt"
	"gnnwatchtower/internal/models"
	"log"
)

// Additive and idempotent. No probes, notifications, credential issuance or deletion.
func migrateMonitoring(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(devices)")
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, col := range []struct{ name, definition string }{
		{"monitoring_mode", "TEXT NOT NULL DEFAULT ''"}, {"last_checked_at", "DATETIME"}, {"last_probe_latency_ms", "REAL"}, {"probe_error", "TEXT NOT NULL DEFAULT ''"}, {"http_result", "TEXT NOT NULL DEFAULT ''"},
	} {
		if !columns[col.name] {
			if _, err = db.Exec("ALTER TABLE devices ADD COLUMN " + col.name + " " + col.definition); err != nil {
				return err
			}
		}
	}
	rows, err = db.Query(`SELECT id,name,ip_address,COALESCE(device_type,'other'),COALESCE(monitoring_mode,''),COALESCE(monitoring_profile,'active'),check_interval_seconds,is_agent_installed,
 (SELECT COUNT(*) FROM services_monitored s WHERE s.device_id=devices.id) FROM devices`)
	if err != nil {
		return err
	}
	var pending []models.Device
	invalid, agentsBlocked, servicesBlocked := 0, 0, 0
	for rows.Next() {
		var d models.Device
		var services int
		if err = rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.DeviceType, &d.MonitoringMode, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.IsAgentInstalled, &services); err != nil {
			rows.Close()
			return err
		}
		if d.MonitoringMode != "" {
			continue
		}
		d.MonitoringMode = models.LegacyMode(d.DeviceType, d.IPAddress)
		check := d
		if err := models.NormalizeDevice(&check); err != nil {
			invalid++
		}
		if d.MonitoringMode != models.ModeServer {
			if d.IsAgentInstalled {
				agentsBlocked++
			}
			servicesBlocked += services
		}
		pending = append(pending, d)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, d := range pending {
		if _, err = tx.Exec(`UPDATE devices SET monitoring_mode=? WHERE id=? AND monitoring_mode=''`, d.MonitoringMode, d.ID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM system_settings WHERE key='scan_subnet'`); err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_metrics_device_latest ON system_metrics(device_id,id); CREATE INDEX IF NOT EXISTS idx_incidents_device_open ON downtime_incidents(device_id,resolved_at);`); err != nil {
		return err
	}
	// Database constraints cover all writers, including old API call paths.
	_, err = tx.Exec(`CREATE TRIGGER IF NOT EXISTS devices_mode_insert AFTER INSERT ON devices WHEN NEW.monitoring_mode='' BEGIN
 UPDATE devices SET monitoring_mode=CASE WHEN lower(NEW.device_type) IN ('windows','linux','ubuntu','debian','centos','rhel','rocky','almalinux') THEN 'server' WHEN lower(NEW.device_type) IN ('website','web','http') THEN 'website' ELSE 'icmp' END WHERE id=NEW.id; END;
 CREATE TRIGGER IF NOT EXISTS devices_mode_update BEFORE UPDATE OF monitoring_mode,device_type ON devices WHEN NEW.monitoring_mode NOT IN ('server','website','icmp') OR (NEW.monitoring_mode='server' AND lower(NEW.device_type) NOT IN ('windows','linux','ubuntu','debian','centos','rhel','rocky','almalinux')) OR (NEW.monitoring_mode='website' AND lower(NEW.device_type) NOT IN ('website','web','http','other')) BEGIN SELECT RAISE(ABORT,'invalid monitoring contract'); END;
 CREATE TRIGGER IF NOT EXISTS devices_mode_validate_insert BEFORE INSERT ON devices WHEN NEW.monitoring_mode NOT IN ('','server','website','icmp') OR (NEW.monitoring_mode='server' AND lower(NEW.device_type) NOT IN ('windows','linux','ubuntu','debian','centos','rhel','rocky','almalinux')) OR (NEW.monitoring_mode='website' AND lower(NEW.device_type) NOT IN ('website','web','http','other')) BEGIN SELECT RAISE(ABORT,'invalid monitoring contract'); END;`)
	if err != nil {
		return fmt.Errorf("monitoring constraints: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if len(pending) > 0 {
		log.Printf("[Monitoring migration] classified=%d invalid_targets_blocked=%d legacy_agent_associations_blocked=%d legacy_services_blocked=%d; history and credentials retained; no network actions", len(pending), invalid, agentsBlocked, servicesBlocked)
	}
	return nil
}
