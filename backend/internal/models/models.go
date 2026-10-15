package models

import (
	"time"
)

// Device represents a monitored hardware/server/VM/NAS in the network
type Device struct {
	ID                   int        `json:"id"`
	Name                 string     `json:"name"`
	IPAddress            string     `json:"ip_address"`
	MACAddress           string     `json:"mac_address"`
	DeviceType           string     `json:"device_type"` // windows, ubuntu, esxi, synology, switch, router, other
	OSVersion            string     `json:"os_version"`
	Purpose              string     `json:"purpose"`
	Location             string     `json:"location"`
	OperatorNotes        string     `json:"operator_notes"`
	IsAgentInstalled     bool       `json:"is_agent_installed"`
	AgentToken           string     `json:"agent_token,omitempty"`
	MonitoringProfile    string     `json:"monitoring_profile"` // active, dormant_24h, dormant_30d, maintenance, paused
	CheckIntervalSeconds int        `json:"check_interval_seconds"`
	Status               string     `json:"status"` // online, offline, warning, maintenance, unknown
	LastSeenAt           *time.Time `json:"last_seen_at"`
	LastStatusChangeAt   time.Time  `json:"last_status_change_at"`
	ResponseTimeMS       int        `json:"response_time_ms"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`

	AgentVersion         string     `json:"agent_version,omitempty"`
	AgentUpgradeStatus   string     `json:"agent_upgrade_status,omitempty"` // none, pending, downloading, verifying, applying, failed, success
	AgentUpgradeTarget   string     `json:"agent_upgrade_target,omitempty"`
	AgentUpgradeError    string     `json:"agent_upgrade_error,omitempty"`
	AgentLastUpgradeAt   *time.Time `json:"agent_last_upgrade_at,omitempty"`
	MachineUUID          string     `json:"machine_uuid,omitempty"`

	LatestCPU   *float64 `json:"latest_cpu,omitempty"`
	LatestRAM   *float64 `json:"latest_ram,omitempty"`
	LatestDisk  *float64 `json:"latest_disk,omitempty"`
	LatestTemp  *float64 `json:"latest_temp,omitempty"`
	LatestNetRx *float64 `json:"latest_net_rx,omitempty"`
	LatestNetTx *float64 `json:"latest_net_tx,omitempty"`

	Services []ServiceMonitored `json:"services,omitempty"`
}

// ServiceMonitored represents a network or system service monitored on a device
type ServiceMonitored struct {
	ID                 int        `json:"id"`
	DeviceID           int        `json:"device_id"`
	Name               string     `json:"name"`       // e.g. "RDP", "SSH", "Docker", "MSSQL"
	CheckType          string     `json:"check_type"` // tcp_port, http, agent_service, snmp
	TargetPort         int        `json:"target_port"`
	TargetPath         string     `json:"target_path"`
	ExpectedStatusCode int        `json:"expected_status_code"`
	Status             string     `json:"status"` // online, offline, degraded
	ResponseTimeMS     int        `json:"response_time_ms"`
	LastCheckAt        *time.Time `json:"last_check_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// DowntimeIncident represents an offline period with root cause classification
type DowntimeIncident struct {
	ID                      int        `json:"id"`
	DeviceID                int        `json:"device_id"`
	DeviceName              string     `json:"device_name,omitempty"`
	DeviceIP                string     `json:"device_ip,omitempty"`
	StartedAt               time.Time  `json:"started_at"`
	ResolvedAt              *time.Time `json:"resolved_at"`
	DurationSeconds         *int       `json:"duration_seconds"`
	ClassifiedCause         string     `json:"classified_cause"` // power_outage, planned_maintenance, system_crash, network_failure, unclassified
	AutoDetectionHypothesis string     `json:"auto_detection_hypothesis"`
	OperatorConfirmed       bool       `json:"operator_confirmed"`
	OperatorNotes           string     `json:"operator_notes"`
	ClassifiedBy            string     `json:"classified_by"`
	CreatedAt               time.Time  `json:"created_at"`
}

// SystemMetric records performance telemetry
type SystemMetric struct {
	ID                 int64     `json:"id"`
	DeviceID           int       `json:"device_id"`
	CPUUsagePct        float64   `json:"cpu_usage_pct"`
	RAMUsagePct        float64   `json:"ram_usage_pct"`
	DiskUsagePct       float64   `json:"disk_usage_pct"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
	NetRxKBps          float64   `json:"net_rx_kbps"`
	NetTxKBps          float64   `json:"net_tx_kbps"`
	RecordedAt         time.Time `json:"recorded_at"`
}

// AlertChannel configures outgoing alert dispatchers
type AlertChannel struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // telegram, discord, webhook, email
	Config    string    `json:"config"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// SystemSetting represents key-value system configurations
type SystemSetting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AgentReport represents incoming payload from mini-agent
type AgentReport struct {
	Token          string          `json:"token"`
	Hostname       string          `json:"hostname"`
	LocalIP        string          `json:"local_ip"`
	MACAddress     string          `json:"mac_address,omitempty"`
	OSName         string          `json:"os_name"`
	OSVersion      string          `json:"os_version"`
	Architecture   string          `json:"arch,omitempty"`
	AgentVersion   string          `json:"agent_version,omitempty"`
	MachineUUID    string          `json:"machine_uuid,omitempty"`
	UpgradeStatus  string          `json:"upgrade_status,omitempty"` // downloading, verifying, applying, failed, success
	UpgradeError   string          `json:"upgrade_error,omitempty"`
	CPUUsagePct    float64         `json:"cpu_usage_pct"`
	RAMUsagePct    float64         `json:"ram_usage_pct"`
	DiskUsagePct   float64         `json:"disk_usage_pct"`
	NetRxKBps      float64         `json:"net_rx_kbps"`
	NetTxKBps      float64         `json:"net_tx_kbps"`
	TempCelsius    float64         `json:"temp_celsius"`
	UptimeSeconds  uint64          `json:"uptime_seconds"`
	Services       map[string]bool `json:"services"` // e.g. {"nginx": true, "docker": false}
	IsShuttingDown bool            `json:"is_shutting_down"`
	ShutdownReason string          `json:"shutdown_reason"` // graceful_shutdown, reboot, etc.
}

// AgentEnrollRequest represents zero-touch agent enrollment
type AgentEnrollRequest struct {
	EnrollmentKey string `json:"enrollment_key"`
	Hostname      string `json:"hostname" binding:"required"`
	LocalIP       string `json:"local_ip"`
	MACAddress    string `json:"mac_address"`
	OSName        string `json:"os_name"`
	OSVersion     string `json:"os_version"`
	Architecture  string `json:"arch"`
	MachineUUID   string `json:"machine_uuid"`
	AgentVersion  string `json:"agent_version"`
	AgentToken    string `json:"agent_token,omitempty"`
}

// AgentEnrollResponse represents enrollment approval and token credentials
type AgentEnrollResponse struct {
	Status               string `json:"status"`
	Token                string `json:"token"`
	DeviceID             int    `json:"device_id"`
	CheckIntervalSeconds int    `json:"check_interval_seconds"`
	ServerVersion        string `json:"server_version"`
}

// AgentUpgradeInstruction represents command payload sent in heartbeat response
type AgentUpgradeInstruction struct {
	Status        string `json:"status"`
	Action        string `json:"action"` // "upgrade" or "none"
	TargetVersion string `json:"target_version,omitempty"`
	DownloadURL   string `json:"download_url,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

// DiscoveryHost represents an auto-discovered network node
type DiscoveryHost struct {
	IPAddress           string `json:"ip_address"`
	MACAddress          string `json:"mac_address"`
	Vendor              string `json:"vendor"`
	Hostname            string `json:"hostname"`
	OpenPorts           []int  `json:"open_ports"`
	SuggestedDeviceType string `json:"suggested_device_type"`
	AlreadyMonitored    bool   `json:"already_monitored"`
}
