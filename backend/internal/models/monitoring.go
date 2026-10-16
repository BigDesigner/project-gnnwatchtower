package models

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	ModeServer  = "server"
	ModeWebsite = "website"
	ModeICMP    = "icmp"
)

func ServerType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "windows", "linux", "ubuntu", "debian", "centos", "rhel", "rocky", "almalinux":
		return true
	}
	return false
}

// LegacyMode never grants agent capabilities to an unknown inventory type.
// Ambiguous addresses remain in ICMP mode and validation prevents probing them.
func LegacyMode(t, address string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "website" || t == "web" || t == "http" {
		return ModeWebsite
	}
	if ServerType(t) {
		return ModeServer
	}
	if (t == "" || t == "other") && (strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://")) {
		return ModeWebsite
	}
	return ModeICMP
}

func ValidIP(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.Equal(net.IPv4bcast) && !ip.IsLinkLocalUnicast()
}

// NormalizeDevice is shared by manual entry, migration checks and restore.
// It performs no DNS lookup or network activity.
func NormalizeDevice(d *Device) error {
	d.Name, d.IPAddress = strings.TrimSpace(d.Name), strings.TrimSpace(d.IPAddress)
	d.DeviceType = strings.ToLower(strings.TrimSpace(d.DeviceType))
	if d.DeviceType == "" {
		d.DeviceType = "other"
	}
	if d.MonitoringMode == "" {
		d.MonitoringMode = LegacyMode(d.DeviceType, d.IPAddress)
	}
	if d.Name == "" || len(d.Name) > 128 || len(d.IPAddress) > 2048 || len(d.DeviceType) > 64 || len(d.OperatorNotes) > 4096 || len(d.Location) > 256 {
		return fmt.Errorf("Cihaz adı, hedef veya açıklama uzunluğu geçersiz")
	}
	switch d.MonitoringMode {
	case ModeServer:
		if !ServerType(d.DeviceType) {
			return fmt.Errorf("Ajan izleme yalnızca Windows ve Linux cihazları içindir")
		}
		if !ValidIP(d.IPAddress) {
			return fmt.Errorf("Geçerli bir tekil IP adresi girin")
		}
	case ModeICMP:
		if d.DeviceType == "website" || d.DeviceType == "web" || d.DeviceType == "http" || !ValidIP(d.IPAddress) {
			return fmt.Errorf("IP cihazları için URL veya ana makine adı yerine geçerli tekil IP adresi girin")
		}
	case ModeWebsite:
		if d.DeviceType != "website" && d.DeviceType != "web" && d.DeviceType != "http" && d.DeviceType != "other" {
			return fmt.Errorf("Web izleme için cihaz türü website olmalıdır")
		}
		if !strings.Contains(d.IPAddress, "://") {
			d.IPAddress = "https://" + d.IPAddress
		}
		u, err := url.Parse(d.IPAddress)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
			return fmt.Errorf("Kimlik bilgisi içermeyen geçerli HTTP(S) URL girin")
		}
		if ip := net.ParseIP(u.Hostname()); ip != nil && (!ValidIP(u.Hostname()) || ip.IsLoopback()) {
			return fmt.Errorf("Bu web hedefi kullanılamaz")
		}
		if strings.EqualFold(u.Hostname(), "metadata.google.internal") || strings.EqualFold(u.Hostname(), "instance-data") {
			return fmt.Errorf("Bu web hedefi kullanılamaz")
		}
		d.DeviceType = "website"
	default:
		return fmt.Errorf("Geçersiz izleme kategorisi")
	}
	if d.MonitoringProfile == "" {
		d.MonitoringProfile = "active"
	}
	switch d.MonitoringProfile {
	case "active", "dormant_24h", "dormant_30d", "maintenance", "paused":
	default:
		return fmt.Errorf("Geçersiz izleme profili")
	}
	if d.CheckIntervalSeconds == 0 {
		d.CheckIntervalSeconds = 15
	}
	if d.CheckIntervalSeconds < 15 || d.CheckIntervalSeconds > 86400 {
		return fmt.Errorf("Kontrol aralığı 15–86400 saniye olmalıdır")
	}
	return nil
}

func CheckCadence(d Device) time.Duration {
	switch d.MonitoringProfile {
	case "dormant_24h":
		return 24 * time.Hour
	case "dormant_30d":
		return 30 * 24 * time.Hour
	}
	n := d.CheckIntervalSeconds
	if n < 15 {
		n = 15
	}
	return time.Duration(n) * time.Second
}

func ObservationState(d Device, now time.Time) string {
	if d.MonitoringProfile == "maintenance" || d.MonitoringProfile == "paused" {
		return d.MonitoringProfile
	}
	if d.LastCheckedAt == nil {
		return "unknown"
	}
	if now.Sub(*d.LastCheckedAt) < -30*time.Second {
		return "unknown"
	}
	if now.Sub(*d.LastCheckedAt) > 2*CheckCadence(d)+30*time.Second {
		return "stale"
	}
	if d.Status == "online" || d.Status == "offline" {
		return d.Status
	}
	return "unknown"
}

func TelemetryFresh(d Device, now time.Time) bool {
	return d.MonitoringMode == ModeServer && d.IsAgentInstalled && d.MonitoringProfile != "paused" && d.MonitoringProfile != "maintenance" && d.LatestMetricAt != nil && now.Sub(*d.LatestMetricAt) >= -30*time.Second && now.Sub(*d.LatestMetricAt) <= 90*time.Second
}

func ValidateService(s ServiceMonitored) error {
	if strings.TrimSpace(s.Name) == "" || len(s.Name) > 128 || s.TargetPort < 1 || s.TargetPort > 65535 || (s.CheckType != "tcp_port" && s.CheckType != "http" && s.CheckType != "agent_service") || len(s.TargetPath) > 2048 || (s.TargetPath != "" && !strings.HasPrefix(s.TargetPath, "/")) || s.ExpectedStatusCode < 0 || s.ExpectedStatusCode > 599 {
		return fmt.Errorf("Geçersiz servis yapılandırması")
	}
	return nil
}
