package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestMonitoringAPIIsolation(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	const session = "synthetic-monitoring-session"
	sessionsMu.Lock()
	activeSessions[session] = sessionInfo{Username: "admin", Role: "admin", ExpiresAt: time.Now().Add(time.Hour)}
	sessionsMu.Unlock()
	defer func() { sessionsMu.Lock(); delete(activeSessions, session); sessionsMu.Unlock() }()
	router := SetupRouter(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("test")}})
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		router.ServeHTTP(w, r)
		return w
	}
	for _, token := range []string{"", session} {
		w := request("POST", "/api/discovery/scan", `{"subnet":"192.0.2.0/24"}`, token)
		if w.Code != 404 {
			t.Fatalf("retired endpoint: %d %s", w.Code, w.Body)
		}
	}
	w := request("POST", "/api/devices", `{"name":"NAS fixture","ip_address":"192.0.2.40","device_type":"synology","monitoring_mode":"icmp"}`, session)
	if w.Code != 201 || bytes.Contains(w.Body.Bytes(), []byte("agent_token")) {
		t.Fatalf("icmp create: %d %s", w.Code, w.Body)
	}
	var created struct {
		ID int `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created.ID
	for _, body := range []string{`{"name":"bad","ip_address":"https://example.com","device_type":"esxi","monitoring_mode":"icmp"}`, `{"name":"bad","ip_address":"192.0.2.41","device_type":"esxi","monitoring_mode":"server"}`, `{"name":"bad","ip_address":"192.0.2.41","device_type":"nas","monitoring_mode":"website"}`, `{"name":"bad","ip_address":"fixture.local","device_type":"nas","monitoring_mode":"icmp"}`} {
		if w := request("POST", "/api/devices", body, session); w.Code != 400 {
			t.Fatalf("mode bypass: %d %s", w.Code, w.Body)
		}
	}
	// Even historical credentials and installed flags grant no capabilities to appliances.
	db.Exec(`UPDATE devices SET agent_token='legacy-fixture-token',is_agent_installed=1 WHERE id=?`, id)
	for _, endpoint := range []struct{ method, path, body string }{
		{"POST", fmt.Sprintf("/api/devices/%d/services", id), `{"name":"HTTPS","target_port":443}`},
		{"GET", fmt.Sprintf("/api/devices/%d/telemetry", id), ""},
		{"POST", fmt.Sprintf("/api/agent/%d/upgrade", id), "{}"},
		{"POST", "/api/agent/report", `{"token":"legacy-fixture-token","cpu_usage_pct":90}`},
	} {
		w := request(endpoint.method, endpoint.path, endpoint.body, session)
		if w.Code != 403 {
			t.Fatalf("capability bypass %s: %d %s", endpoint.path, w.Code, w.Body)
		}
	}
	var enrollmentKey string
	db.QueryRow(`SELECT value FROM system_settings WHERE key='agent_enrollment_key'`).Scan(&enrollmentKey)
	body, _ := json.Marshal(map[string]any{"hostname": "NAS fixture", "local_ip": "192.0.2.40", "os_name": "Linux", "enrollment_key": enrollmentKey})
	if w := request("POST", "/api/agent/enroll", string(body), ""); w.Code != 403 {
		t.Fatalf("enroll bypass: %d %s", w.Code, w.Body)
	}
	if w := request("POST", "/api/agent/upgrade-all", "{}", session); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var pending, metrics, services int
	db.QueryRow(`SELECT COUNT(*) FROM devices WHERE agent_upgrade_status='pending'`).Scan(&pending)
	db.QueryRow(`SELECT COUNT(*) FROM system_metrics`).Scan(&metrics)
	db.QueryRow(`SELECT COUNT(*) FROM services_monitored`).Scan(&services)
	if pending+metrics+services != 0 {
		t.Fatal("forbidden side effects", pending, metrics, services)
	}
	if w := request("PUT", fmt.Sprintf("/api/devices/%d", id), `{"monitoring_mode":"server","device_type":"linux"}`, session); w.Code != 400 {
		t.Fatal("mode conversion allowed")
	}
	if w := request("PUT", fmt.Sprintf("/api/devices/%d", id), `{"ip_address":"192.0.2.42","location":"Fixture room"}`, session); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var target string
	db.QueryRow(`SELECT ip_address FROM devices WHERE id=?`, id).Scan(&target)
	if target != "192.0.2.42" {
		t.Fatal("IP edit ignored")
	}
	request("PUT", "/api/settings", `{"scan_subnet":"192.0.2.0/24"}`, session)
	var retired int
	db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE key='scan_subnet'`).Scan(&retired)
	if retired != 0 {
		t.Fatal("retired setting restored")
	}
	w = request("GET", "/api/devices", "", session)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var list []models.Device
	if err = json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].MonitoringMode != "icmp" || list[0].AgentToken != "" || list[0].LatestCPU != nil || list[0].IsAgentInstalled {
		t.Fatalf("inventory leakage: %s", w.Body)
	}
	if gw := checkGatewayHealth(); gw["status"] != "unknown" || gw["ip"] != nil {
		t.Fatal("fabricated gateway", gw)
	}
}

func TestDashboardReconcilesModesAndFreshness(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, kind := range []string{"windows", "linux", "website", "nas", "esxi", "router"} {
		target := "192.0.2." + fmt.Sprint(50+len(kind))
		if kind == "website" {
			target = "https://example.com"
		}
		_, err = db.Exec(`INSERT INTO devices(name,ip_address,device_type) VALUES(?,?,?)`, kind, target, kind)
		if err != nil {
			t.Fatal(err)
		}
	}
	db.Exec(`UPDATE devices SET status='online',last_checked_at=CURRENT_TIMESTAMP WHERE device_type='windows'`)
	db.Exec(`UPDATE devices SET status='offline',last_checked_at=CURRENT_TIMESTAMP WHERE device_type='nas'`)
	db.Exec(`UPDATE devices SET status='online',last_checked_at=datetime('now','-1 day') WHERE device_type='linux'`)
	db.Exec(`UPDATE devices SET monitoring_profile='maintenance' WHERE device_type='esxi'`)
	d, err := loadDevices()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	modes := map[string]int{}
	for _, device := range d {
		counts[device.ObservationState]++
		modes[device.MonitoringMode]++
	}
	if len(d) != 6 || modes["server"] != 2 || modes["website"] != 1 || modes["icmp"] != 3 || counts["online"] != 1 || counts["offline"] != 1 || counts["stale"] != 1 || counts["maintenance"] != 1 || counts["unknown"] != 2 {
		t.Fatal("category/state reconciliation", modes, counts)
	}
}

func TestPendingServerEnrollmentAndReenrollment(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`INSERT INTO devices(name,ip_address,device_type,monitoring_mode,agent_token) VALUES('Pending fixture','192.0.2.88','linux','server','pending-fixture-token')`); err != nil {
		t.Fatal(err)
	}
	var key string
	db.QueryRow(`SELECT value FROM system_settings WHERE key='agent_enrollment_key'`).Scan(&key)
	router := SetupRouter(nil)
	enroll := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"hostname": "Pending fixture", "local_ip": "192.0.2.88", "os_name": "Linux", "enrollment_key": key, "agent_token": token})
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/agent/enroll", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w
	}
	w := enroll("")
	if w.Code != 200 {
		t.Fatalf("pending server enrollment: %d %s", w.Code, w.Body)
	}
	var result models.AgentEnrollResponse
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Token == "" {
		t.Fatal("missing token", err)
	}
	if w = enroll(""); w.Code != 409 {
		t.Fatal("global key alone overwrote enrolled identity", w.Code, w.Body)
	}
	var stored string
	db.QueryRow(`SELECT agent_token FROM devices WHERE ip_address='192.0.2.88'`).Scan(&stored)
	if stored != result.Token {
		t.Fatal("rejected enrollment changed token")
	}
	if w = enroll(result.Token); w.Code != 200 {
		t.Fatalf("legitimate token refresh: %d %s", w.Code, w.Body)
	}
}
