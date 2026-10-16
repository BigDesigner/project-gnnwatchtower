package monitor

import (
	"fmt"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
	"path/filepath"
	"testing"
	"time"
)

func TestICMPBoundaryCadenceIncidents(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	res, err := db.Exec(`INSERT INTO devices(name,ip_address,device_type,monitoring_mode) VALUES('NAS fixture','192.0.2.20','synology','icmp')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err = db.Exec(`INSERT INTO services_monitored(device_id,name,target_port) VALUES(?,'Legacy HTTPS',443)`, id); err != nil {
		t.Fatal(err)
	}
	calls := 0
	online := true
	var events []string
	rtt := 1.25
	p := Poller{icmpProbe: func(target string, timeout time.Duration) (bool, *float64, error) {
		calls++
		if target != "192.0.2.20" {
			t.Fatal("unexpected target")
		}
		return online, &rtt, nil
	}, websiteProbe: func(string, time.Duration) (bool, float64, int, string) {
		t.Fatal("ICMP dispatched HTTP")
		return false, 0, 0, ""
	}, serviceProbe: func(models.ServiceMonitored, string) { t.Fatal("ICMP dispatched service") }, notify: func(_, _, event, _ string) { events = append(events, event) }}
	check := func() {
		t.Helper()
		if _, err := db.Exec(`UPDATE devices SET last_checked_at=NULL WHERE id=?`, id); err != nil {
			t.Fatal(err)
		}
		p.checkSingleDevice(models.Device{ID: int(id)})
	}
	check()
	p.pollDevices()
	if calls != 1 {
		t.Fatalf("cadence added probes: %d", calls)
	}
	online = false
	check()
	check()
	var count, open int
	db.QueryRow(`SELECT COUNT(*),SUM(resolved_at IS NULL) FROM downtime_incidents`).Scan(&count, &open)
	if count != 1 || open != 1 || len(events) != 1 || events[0] != "DOWNTIME" {
		t.Fatalf("repeated failure incidents=%d open=%d events=%v", count, open, events)
	}
	online = true
	check()
	check()
	db.QueryRow(`SELECT COUNT(*) FROM downtime_incidents WHERE resolved_at IS NULL`).Scan(&open)
	if open != 0 || len(events) != 2 || events[1] != "RECOVERY" {
		t.Fatalf("recovery %v open=%d", events, open)
	}
	var duration int
	if err = db.QueryRow(`SELECT duration_seconds FROM downtime_incidents`).Scan(&duration); err != nil || duration < 0 {
		t.Fatal("duration", err)
	}
	before := calls
	db.Exec(`UPDATE devices SET monitoring_profile='maintenance',last_checked_at=NULL WHERE id=?`, id)
	p.pollDevices()
	p.checkSingleDevice(models.Device{ID: int(id)})
	if calls != before {
		t.Fatal("maintenance was probed")
	}
	db.Exec(`UPDATE devices SET monitoring_profile='active',last_checked_at=NULL WHERE id=?`, id)
	p.icmpProbe = func(string, time.Duration) (bool, *float64, error) {
		return false, nil, fmt.Errorf("fixture local error")
	}
	p.checkSingleDevice(models.Device{ID: int(id)})
	var status string
	db.QueryRow(`SELECT status FROM devices WHERE id=?`, id).Scan(&status)
	if status != "unknown" || len(events) != 2 {
		t.Fatal("local execution failure created outage", status, events)
	}
	db.Exec(`UPDATE devices SET monitoring_profile='dormant_24h',last_checked_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	p.icmpProbe = func(string, time.Duration) (bool, *float64, error) {
		t.Fatal("offline retry ignored dormant interval")
		return false, nil, nil
	}
	p.pollDevices()
}

func TestRTTRequiresMeasuredValue(t *testing.T) {
	for _, s := range []string{"time<1ms", "time<1 ms", "no RTT", "time=0 ms"} {
		if parseRTT(s) >= 0 {
			t.Fatal("fabricated latency", s)
		}
	}
	if parseRTT("64 bytes from fixture: time=1.234 ms") != 1.234 {
		t.Fatal("real RTT parsing")
	}
}
