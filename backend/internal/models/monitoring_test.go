package models

import (
	"testing"
	"time"
)

func TestMonitoringContract(t *testing.T) {
	for _, tc := range []struct {
		kind, address, mode string
		valid               bool
	}{
		{"synology", "192.0.2.10", "icmp", true}, {"esxi", "2001:db8::10", "icmp", true},
		{"windows", "192.0.2.11", "icmp", true}, {"ubuntu", "192.0.2.12", "server", true},
		{"esxi", "192.0.2.10", "server", false}, {"nas", "https://example.com", "website", false},
		{"nas", "example.com", "icmp", false}, {"nas", "192.0.2.10:443", "icmp", false},
		{"nas", "0.0.0.0", "icmp", false}, {"nas", "224.0.0.1", "icmp", false},
		{"nas", "192.0.2.10", "plugin", false}, {"website", "https://user:pass@example.com", "website", false},
		{"website", "ftp://example.com", "website", false}, {"website", "example.com", "website", true},
	} {
		t.Run(tc.kind+tc.address+tc.mode, func(t *testing.T) {
			d := Device{Name: "Fixture", DeviceType: tc.kind, IPAddress: tc.address, MonitoringMode: tc.mode}
			err := NormalizeDevice(&d)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	for _, kind := range []string{"synology", "esxi", "qnap", "nas", "storage", "router", "unknown-agent"} {
		if LegacyMode(kind, "192.0.2.10") != ModeICMP {
			t.Fatal("appliance gained capabilities", kind)
		}
	}
}

func TestFreshnessAndProfiles(t *testing.T) {
	now := time.Now()
	recent := now.Add(-time.Second)
	old := now.Add(-time.Hour)
	d := Device{MonitoringMode: ModeServer, MonitoringProfile: "active", Status: "online", CheckIntervalSeconds: 15, IsAgentInstalled: true}
	if ObservationState(d, now) != "unknown" {
		t.Fatal("new record cannot be online")
	}
	d.LastCheckedAt = &old
	d.LatestMetricAt = &old
	if ObservationState(d, now) != "stale" || TelemetryFresh(d, now) {
		t.Fatal("stale state")
	}
	d.LastCheckedAt = &recent
	d.LatestMetricAt = &recent
	if ObservationState(d, now) != "online" || !TelemetryFresh(d, now) {
		t.Fatal("fresh sample")
	}
	d.MonitoringMode = ModeICMP
	if TelemetryFresh(d, now) {
		t.Fatal("appliance telemetry")
	}
	d.MonitoringProfile = "maintenance"
	if ObservationState(d, now) != "maintenance" {
		t.Fatal("maintenance")
	}
	d.MonitoringMode = ModeServer
	d.LatestMetricAt = &old
	d.MonitoringProfile = "dormant_24h"
	d.LastCheckedAt = &old
	if TelemetryFresh(d, now) {
		t.Fatal("dormant cadence made stale agent metric current")
	}
	if ObservationState(d, now) != "online" {
		t.Fatal("cadence ignored")
	}
}
