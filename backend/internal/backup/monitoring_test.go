package backup

import (
	"encoding/json"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
	"path/filepath"
	"testing"
	"time"
)

func TestMonitoringBackupRoundTrip(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`INSERT INTO devices(id,name,ip_address,device_type,monitoring_mode,last_checked_at,last_probe_latency_ms) VALUES(41,'NAS fixture','192.0.2.41','synology','icmp',CURRENT_TIMESTAMP,1.25);
 INSERT INTO services_monitored(device_id,name,target_port) VALUES(41,'Legacy HTTPS',443);
 INSERT INTO downtime_incidents(device_id,started_at,classified_cause) VALUES(41,datetime('now','-1 hour'),'unclassified');`)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Version != "1.1.0" || len(bundle.Devices) != 1 || bundle.Devices[0].Device.MonitoringMode != "icmp" || bundle.Devices[0].Device.LastCheckedAt == nil || bundle.Devices[0].Device.LastProbeLatencyMS == nil || len(bundle.Incidents) != 1 {
		t.Fatalf("incomplete export: %+v", bundle)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = ImportAll(data); err != nil {
			t.Fatal(err)
		}
	}
	var devices, services, incidents int
	db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&devices)
	db.QueryRow(`SELECT COUNT(*) FROM services_monitored`).Scan(&services)
	db.QueryRow(`SELECT COUNT(*) FROM downtime_incidents`).Scan(&incidents)
	if devices != 1 || services != 1 || incidents != 1 {
		t.Fatal("non-idempotent restore", devices, services, incidents)
	}
	// Old bundles infer safe modes and cannot restore scanning or grant appliance tokens.
	old := BackupBundle{Version: "1.0.0", Settings: []models.SystemSetting{{Key: "scan_subnet", Value: "192.0.2.0/24"}}, Devices: []DeviceExportData{{Device: models.Device{ID: 99, Name: "Legacy ESXi", IPAddress: "192.0.2.99", DeviceType: "esxi", IsAgentInstalled: true, AgentToken: "legacy-agent"}}}, Incidents: []models.DowntimeIncident{{DeviceID: 99, StartedAt: time.Now().Add(-time.Hour), ClassifiedCause: "unclassified"}}}
	data, _ = json.Marshal(old)
	if err = ImportAll(data); err != nil {
		t.Fatal(err)
	}
	var mode string
	var token any
	var checked any
	if err = db.QueryRow(`SELECT monitoring_mode,agent_token,last_checked_at FROM devices WHERE ip_address='192.0.2.99'`).Scan(&mode, &token, &checked); err != nil || mode != "icmp" || token != nil || checked != nil {
		t.Fatal("unsafe old restore", mode, token, checked, err)
	}
	old.Devices[0].Device.MonitoringMode = "server"
	old.Settings = []models.SystemSetting{{Key: "rollback_fixture", Value: "must rollback"}}
	data, _ = json.Marshal(old)
	if err = ImportAll(data); err == nil {
		t.Fatal("unsafe mode accepted")
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE key='rollback_fixture'`).Scan(&count)
	if count != 0 {
		t.Fatal("partial restore committed")
	}
}
