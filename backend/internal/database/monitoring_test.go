package database

import (
	"path/filepath"
	"testing"
)

func TestMonitoringMigrationPreservesLegacyHistory(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`DROP TRIGGER devices_mode_insert; DROP TRIGGER devices_mode_update; DROP TRIGGER devices_mode_validate_insert;
 INSERT INTO devices(id,name,ip_address,device_type,monitoring_mode,is_agent_installed,agent_token) VALUES(71,'Legacy NAS','192.0.2.71','synology','',1,'legacy-fixture');
 INSERT INTO devices(id,name,ip_address,device_type,monitoring_mode) VALUES(72,'Legacy web','https://example.com','web','');
 INSERT INTO devices(id,name,ip_address,device_type,monitoring_mode) VALUES(73,'Legacy Linux','192.0.2.73','ubuntu','');
 INSERT INTO devices(id,name,ip_address,device_type,monitoring_mode) VALUES(74,'Ambiguous NAS','https://example.net','nas','');
 INSERT INTO services_monitored(device_id,name,target_port) VALUES(71,'Legacy HTTPS',443);
 INSERT INTO downtime_incidents(device_id,started_at) VALUES(71,CURRENT_TIMESTAMP);
 INSERT INTO system_metrics(device_id,cpu_usage_pct) VALUES(71,42);
 INSERT INTO system_settings(key,value) VALUES('scan_subnet','192.0.2.0/24');`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = migrateMonitoring(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, expected := range []struct {
		id   int
		mode string
	}{{71, "icmp"}, {72, "website"}, {73, "server"}, {74, "icmp"}} {
		var mode string
		if err = db.QueryRow(`SELECT monitoring_mode FROM devices WHERE id=?`, expected.id).Scan(&mode); err != nil || mode != expected.mode {
			t.Fatal(expected, mode, err)
		}
	}
	var token string
	db.QueryRow(`SELECT agent_token FROM devices WHERE id=71`).Scan(&token)
	if token != "legacy-fixture" {
		t.Fatal("legacy credential lost")
	}
	for _, table := range []string{"services_monitored", "downtime_incidents", "system_metrics"} {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE device_id=71").Scan(&count)
		if count != 1 {
			t.Fatal("history lost", table, count)
		}
	}
	var retired int
	db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE key='scan_subnet'`).Scan(&retired)
	if retired != 0 {
		t.Fatal("retired settings retained")
	}
	var checked any
	db.QueryRow(`SELECT last_checked_at FROM devices WHERE id=71`).Scan(&checked)
	if checked != nil {
		t.Fatal("migration fabricated check")
	}
}
