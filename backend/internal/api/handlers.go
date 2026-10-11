package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/backup"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/discovery"
	"gnnwatchtower/internal/models"
)

// GetDashboardStats returns high-level system summary
func GetDashboardStats(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not ready"})
		return
	}

	var totalDevices, onlineDevices, offlineDevices, warningDevices int
	_ = database.DB.QueryRow(`
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END), 0)
		FROM devices
	`).Scan(&totalDevices, &onlineDevices, &offlineDevices, &warningDevices)

	var activeProfiles, dormantProfiles, maintenanceProfiles int
	_ = database.DB.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN monitoring_profile = 'active' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN monitoring_profile IN ('dormant_24h', 'dormant_30d') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN monitoring_profile = 'maintenance' THEN 1 ELSE 0 END), 0)
		FROM devices
	`).Scan(&activeProfiles, &dormantProfiles, &maintenanceProfiles)

	var unclassifiedIncidents int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*) FROM downtime_incidents WHERE classified_cause = 'unclassified'
	`).Scan(&unclassifiedIncidents)

	// Fetch real average temperature from recent system metrics (last 2 hours)
	var avgTemp sql.NullFloat64
	_ = database.DB.QueryRow(`
		SELECT AVG(temperature_celsius)
		FROM system_metrics
		WHERE recorded_at >= datetime('now', '-2 hours') AND temperature_celsius > 0
	`).Scan(&avgTemp)

	var avgTemperature *float64
	if avgTemp.Valid {
		v := math.Round(avgTemp.Float64*10) / 10
		avgTemperature = &v
	}

	// Fetch client branding
	var clientCompany, clientLocation string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'client_company_name'").Scan(&clientCompany)
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'client_location'").Scan(&clientLocation)
	if clientCompany == "" {
		clientCompany = "Müşteri Altyapısı"
	}
	if clientLocation == "" {
		clientLocation = "Sistem Odası"
	}

	// Calculate overall average uptime in past 24h
	uptimePct := 100.0
	if totalDevices > 0 {
		uptimePct = float64(onlineDevices) / float64(totalDevices) * 100.0
	}

	// Probe gateway status
	gateway := checkGatewayHealth()

	c.JSON(http.StatusOK, gin.H{
		"client_company_name":    clientCompany,
		"client_location":        clientLocation,
		"total_devices":          totalDevices,
		"online_devices":         onlineDevices,
		"offline_devices":        offlineDevices,
		"warning_devices":        warningDevices,
		"active_profiles":        activeProfiles,
		"dormant_profiles":       dormantProfiles,
		"maintenance_profiles":   maintenanceProfiles,
		"unclassified_incidents":  unclassifiedIncidents,
		"uptime_percentage":      fmt.Sprintf("%.1f", uptimePct),
		"avg_temperature":        avgTemperature,
		"gateway":                gateway,
	})
}

// GetDevices returns all devices with nested services
func GetDevices(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not ready"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, name, ip_address, mac_address, device_type, os_version, purpose, 
		       location, operator_notes, is_agent_installed, agent_token, monitoring_profile, 
		       check_interval_seconds, status, last_seen_at, last_status_change_at, 
		       response_time_ms, created_at, updated_at
		FROM devices
		ORDER BY id ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		var mac, osVer, purp, loc, notes, token sql.NullString
		var lastSeen sql.NullTime

		if err := rows.Scan(&d.ID, &d.Name, &d.IPAddress, &mac, &d.DeviceType, &osVer, &purp, &loc, &notes,
			&d.IsAgentInstalled, &token, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status,
			&lastSeen, &d.LastStatusChangeAt, &d.ResponseTimeMS, &d.CreatedAt, &d.UpdatedAt); err == nil {
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
			if lastSeen.Valid {
				d.LastSeenAt = &lastSeen.Time
			}

			// Attach services
			d.Services = getServices(d.ID)
			devices = append(devices, d)
		}
	}

	if devices == nil {
		devices = []models.Device{}
	}

	c.JSON(http.StatusOK, devices)
}

// CreateDevice adds a new device manually or from discovery
func CreateDevice(c *gin.Context) {
	var input struct {
		Name                 string `json:"name" binding:"required"`
		IPAddress            string `json:"ip_address" binding:"required"`
		MACAddress           string `json:"mac_address"`
		DeviceType           string `json:"device_type"`
		OSVersion            string `json:"os_version"`
		Purpose              string `json:"purpose"`
		Location             string `json:"location"`
		OperatorNotes        string `json:"operator_notes"`
		MonitoringProfile    string `json:"monitoring_profile"`
		CheckIntervalSeconds int    `json:"check_interval_seconds"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.MonitoringProfile == "" {
		input.MonitoringProfile = "active"
	}
	if input.CheckIntervalSeconds == 0 {
		input.CheckIntervalSeconds = 15
	}
	if input.DeviceType == "" {
		input.DeviceType = "other"
	}

	// Generate agent token
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	agentToken := hex.EncodeToString(tokenBytes)

	var id int
	err := database.DB.QueryRow(`
		INSERT INTO devices (name, ip_address, mac_address, device_type, os_version, purpose, location, operator_notes, agent_token, monitoring_profile, check_interval_seconds, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'unknown')
		RETURNING id
	`, input.Name, input.IPAddress, input.MACAddress, input.DeviceType, input.OSVersion, input.Purpose,
		input.Location, input.OperatorNotes, agentToken, input.MonitoringProfile, input.CheckIntervalSeconds).Scan(&id)

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Cihaz IP adresi zaten kayıtlı veya veritabanı hatası: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "agent_token": agentToken, "message": "Cihaz başarıyla eklendi."})
}

// UpdateDevice modifies operator metadata and monitoring configurations
func UpdateDevice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz cihaz ID"})
		return
	}

	var input struct {
		Name                 string `json:"name"`
		DeviceType           string `json:"device_type"`
		OSVersion            string `json:"os_version"`
		Purpose              string `json:"purpose"`
		Location             string `json:"location"`
		OperatorNotes        string `json:"operator_notes"`
		MonitoringProfile    string `json:"monitoring_profile"`
		CheckIntervalSeconds int    `json:"check_interval_seconds"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = database.DB.Exec(`
		UPDATE devices
		SET name = COALESCE(NULLIF($1, ''), name),
		    device_type = COALESCE(NULLIF($2, ''), device_type),
		    os_version = $3,
		    purpose = $4,
		    location = $5,
		    operator_notes = $6,
		    monitoring_profile = COALESCE(NULLIF($7, ''), monitoring_profile),
		    check_interval_seconds = CASE WHEN $8 > 0 THEN $8 ELSE check_interval_seconds END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $9
	`, input.Name, input.DeviceType, input.OSVersion, input.Purpose, input.Location,
		input.OperatorNotes, input.MonitoringProfile, input.CheckIntervalSeconds, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cihaz bilgileri güncellendi."})
}

// DeleteDevice deletes a device
func DeleteDevice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz cihaz ID"})
		return
	}

	_, err = database.DB.Exec("DELETE FROM devices WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cihaz silindi."})
}

// AddServiceToDevice attaches a port/service to monitor on a device
func AddServiceToDevice(c *gin.Context) {
	idStr := c.Param("id")
	devID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz cihaz ID"})
		return
	}

	var input struct {
		Name               string `json:"name" binding:"required"`
		CheckType          string `json:"check_type"`
		TargetPort         int    `json:"target_port" binding:"required"`
		TargetPath         string `json:"target_path"`
		ExpectedStatusCode int    `json:"expected_status_code"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.CheckType == "" {
		input.CheckType = "tcp_port"
	}
	if input.ExpectedStatusCode == 0 {
		input.ExpectedStatusCode = 200
	}

	var servID int
	err = database.DB.QueryRow(`
		INSERT INTO services_monitored (device_id, name, check_type, target_port, target_path, expected_status_code)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, devID, input.Name, input.CheckType, input.TargetPort, input.TargetPath, input.ExpectedStatusCode).Scan(&servID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": servID, "message": "Servis izlemeye eklendi."})
}

// DeleteService removes a monitored service
func DeleteService(c *gin.Context) {
	sIDStr := c.Param("serviceId")
	sID, err := strconv.Atoi(sIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz servis ID"})
		return
	}

	_, err = database.DB.Exec("DELETE FROM services_monitored WHERE id = $1", sID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Servis izlemeden kaldırıldı."})
}

// TriggerDiscovery scans subnet for active hosts
func TriggerDiscovery(c *gin.Context) {
	var input struct {
		Subnet string `json:"subnet"`
	}
	_ = c.ShouldBindJSON(&input)

	subnet := input.Subnet
	if subnet == "" {
		// Read from settings
		var val string
		err := database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'scan_subnet'").Scan(&val)
		if err == nil && val != "" {
			subnet = val
		} else {
			subnet = "192.168.1.0/24"
		}
	}

	hosts, err := discovery.ScanSubnet(subnet)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subnet":  subnet,
		"count":   len(hosts),
		"devices": hosts,
	})
}

// GetIncidents returns historical downtime log with auto & operator causes
func GetIncidents(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT i.id, i.device_id, d.name, i.started_at, i.resolved_at, i.duration_seconds,
		       i.classified_cause, i.auto_detection_hypothesis, i.operator_confirmed, 
		       i.operator_notes, i.classified_by, i.created_at
		FROM downtime_incidents i
		JOIN devices d ON d.id = i.device_id
		ORDER BY i.started_at DESC
		LIMIT 200
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var list []models.DowntimeIncident
	for rows.Next() {
		var inc models.DowntimeIncident
		var resAt sql.NullTime
		var dur sql.NullInt32
		var hypo, notes, by sql.NullString

		if err := rows.Scan(&inc.ID, &inc.DeviceID, &inc.DeviceName, &inc.StartedAt, &resAt, &dur,
			&inc.ClassifiedCause, &hypo, &inc.OperatorConfirmed, &notes, &by, &inc.CreatedAt); err == nil {
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
			list = append(list, inc)
		}
	}

	if list == nil {
		list = []models.DowntimeIncident{}
	}

	c.JSON(http.StatusOK, list)
}

// ClassifyIncident allows operator to tag root cause (Power Cut vs Planned Maintenance vs Crash)
func ClassifyIncident(c *gin.Context) {
	idStr := c.Param("id")
	incID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz incident ID"})
		return
	}

	var input struct {
		ClassifiedCause string `json:"classified_cause" binding:"required"` // power_outage, planned_maintenance, system_crash, network_failure
		OperatorNotes   string `json:"operator_notes"`
		OperatorName    string `json:"operator_name"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.OperatorName == "" {
		input.OperatorName = "Sistem Operatörü"
	}

	_, err = database.DB.Exec(`
		UPDATE downtime_incidents
		SET classified_cause = $1,
		    operator_notes = $2,
		    operator_confirmed = true,
		    classified_by = $3
		WHERE id = $4
	`, input.ClassifiedCause, input.OperatorNotes, input.OperatorName, incID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kesinti nedeni başarıyla kaydedildi."})
}

// ExportBackup generates a full downloadable JSON backup bundle
func ExportBackup(c *gin.Context) {
	bundle, err := backup.ExportAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("gnnwatchtower-backup-%s.json", time.Now().Format("2006-01-02-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, bundle)
}

// ImportBackup handles restore of a JSON backup bundle
func ImportBackup(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		// Fallback to direct raw JSON body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil || len(body) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Yedek dosyası veya JSON verisi bulunamadı"})
			return
		}
		if err := backup.ImportAll(body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Yedek başarıyla geri yüklendi."})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dosya okunamadı"})
		return
	}

	if err := backup.ImportAll(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Yedek geri yüklenirken hata oluştu: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tüm cihazlar, ayarlar ve loglar başarıyla geri yüklendi."})
}

// GetSettings lists system configuration keys
func GetSettings(c *gin.Context) {
	rows, err := database.DB.Query("SELECT key, value, description, updated_at FROM system_settings")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var settings []models.SystemSetting
	for rows.Next() {
		var s models.SystemSetting
		var desc sql.NullString
		if err := rows.Scan(&s.Key, &s.Value, &desc, &s.UpdatedAt); err == nil {
			if desc.Valid {
				s.Description = desc.String
			}
			settings = append(settings, s)
		}
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates key-value configurations
func UpdateSettings(c *gin.Context) {
	var input map[string]string
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for k, v := range input {
		_, _ = database.DB.Exec(`
			INSERT INTO system_settings (key, value, updated_at)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
		`, k, v)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ayarlar kaydedildi."})
}

// AgentHeartbeat receives periodic telemetry and graceful shutdown events from Windows/Ubuntu agents
func AgentHeartbeat(c *gin.Context) {
	var report models.AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var devID int
	var devName string
	err := database.DB.QueryRow(`
		SELECT id, name FROM devices WHERE agent_token = $1
	`, report.Token).Scan(&devID, &devName)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Geçersiz ajan erişim anahtarı"})
		return
	}

	// If agent is reporting graceful shutdown
	if report.IsShuttingDown {
		reason := report.ShutdownReason
		if reason == "" {
			reason = "Kullanıcı veya Güncelleme Tarafından Başlatılan Kapatma"
		}

		_, _ = database.DB.Exec(`
			UPDATE devices 
			SET status = 'offline', last_status_change_at = CURRENT_TIMESTAMP 
			WHERE id = $1
		`, devID)

		// Record planned maintenance incident directly!
		_, _ = database.DB.Exec(`
			INSERT INTO downtime_incidents (device_id, started_at, classified_cause, auto_detection_hypothesis, operator_notes)
			VALUES ($1, CURRENT_TIMESTAMP, 'planned_maintenance', 'Ajan Kapanma Bildirimi: ' || $2, 'Ajan tarafından temiz kapanma doğrulandı')
		`, devID, reason)

		c.JSON(http.StatusOK, gin.H{"status": "ack_shutdown"})
		return
	}

	// Normal heartbeat update
	_, _ = database.DB.Exec(`
		UPDATE devices 
		SET status = 'online',
		    is_agent_installed = 1,
		    os_version = COALESCE(NULLIF($1, ''), os_version),
		    last_seen_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, report.OSName+" "+report.OSVersion, devID)

	// Save telemetry metrics
	_, _ = database.DB.Exec(`
		INSERT INTO system_metrics (device_id, cpu_usage_pct, ram_usage_pct, disk_usage_pct, recorded_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	`, devID, report.CPUUsagePct, report.RAMUsagePct, report.DiskUsagePct)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getServices(devID int) []models.ServiceMonitored {
	var list []models.ServiceMonitored
	rows, err := database.DB.Query(`
		SELECT id, device_id, name, check_type, target_port, target_path, expected_status_code, status, response_time_ms, last_check_at, created_at
		FROM services_monitored WHERE device_id = $1
	`, devID)
	if err != nil {
		return list
	}
	defer rows.Close()

	for rows.Next() {
		var s models.ServiceMonitored
		var targetPath sql.NullString
		var lastCheck sql.NullTime
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.Name, &s.CheckType, &s.TargetPort, &targetPath,
			&s.ExpectedStatusCode, &s.Status, &s.ResponseTimeMS, &lastCheck, &s.CreatedAt); err == nil {
			if targetPath.Valid {
				s.TargetPath = targetPath.String
			}
			if lastCheck.Valid {
				s.LastCheckAt = &lastCheck.Time
			}
			list = append(list, s)
		}
	}
	return list
}

// GenerateLinuxAgentScript outputs bash script for 1-click curl install on Ubuntu/Debian
func GenerateLinuxAgentScript(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "echo 'Hata: token parametresi eksik'; exit 1")
		return
	}

	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, host)

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -e

echo "[GNNwatchtower] Ajan kurulumu baslatiliyor..."
SERVER_URL="%s"
TOKEN="%s"
INSTALL_DIR="/opt/gnnwatchtower-agent"

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

echo "[GNNwatchtower] Ajan ikili dosyasi indiriliyor..."
curl -sSL "$SERVER_URL/api/agent/download/linux" -o "$INSTALL_DIR/gnnwatchtower-agent" || true
chmod +x "$INSTALL_DIR/gnnwatchtower-agent"

echo "[GNNwatchtower] Systemd servisi yapilandiriliyor..."
cat <<EOF > /etc/systemd/system/gnnwatchtower-agent.service
[Unit]
Description=GNNwatchtower Mini-Agent
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/gnnwatchtower-agent -server $SERVER_URL -token $TOKEN
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable gnnwatchtower-agent.service
systemctl restart gnnwatchtower-agent.service

echo "[GNNwatchtower] Ajan basariyla kuruldu ve arka planda calisiyor!"
systemctl status gnnwatchtower-agent.service --no-pager
`, serverURL, token)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// GenerateWindowsAgentScript outputs powershell script for 1-click install on Windows Server
func GenerateWindowsAgentScript(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusBadRequest, "Write-Host 'Hata: token parametresi eksik'; exit 1")
		return
	}

	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, host)

	ps1 := fmt.Sprintf(`$ErrorActionPreference = "Stop"
Write-Host "[GNNwatchtower] Windows Ajan kurulumu baslatiliyor..." -ForegroundColor Cyan

$serverUrl = "%s"
$token = "%s"
$installDir = "C:\Program Files\GNNwatchtower-Agent"

if (!(Test-Path -Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$agentExe = "$installDir\gnnwatchtower-agent.exe"
Write-Host "[GNNwatchtower] Ajan indiriliyor..."
Invoke-WebRequest -Uri "$serverUrl/api/agent/download/windows" -OutFile $agentExe -UseBasicParsing

Write-Host "[GNNwatchtower] Servis zamanlanmis gorev olarak olusturuluyor..."
$taskName = "GNNwatchtowerAgent"
$action = New-ScheduledTaskAction -Execute $agentExe -Argument "-server $serverUrl -token $token"
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0

Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -User "NT AUTHORITY\SYSTEM" -RunLevel Highest | Out-Null
Start-ScheduledTask -TaskName $taskName

Write-Host "[GNNwatchtower] Windows Ajan basariyla kuruldu ve baslatildi!" -ForegroundColor Green
`, serverURL, token)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, ps1)
}

// DownloadAgentBinary serves the agent binary
func DownloadAgentBinary(c *gin.Context) {
	platform := c.Param("platform")
	var filename string
	if platform == "windows" {
		filename = "gnnwatchtower-agent.exe"
	} else {
		filename = "gnnwatchtower-agent"
	}

	paths := []string{
		"./bin/" + filename,
		"/opt/gnnwatchtower/bin/" + filename,
		filename,
	}

	var foundPath string
	for _, p := range paths {
		if f, err := os.Stat(p); err == nil && !f.IsDir() {
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ajan ikili dosyası henüz oluşturulmadı. Lütfen 'go build ./cmd/agent' çalıştırınız."})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.File(foundPath)
}

// checkGatewayHealth tests default gateway latency
func checkGatewayHealth() gin.H {
	var gwIP string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'default_gateway'").Scan(&gwIP)
	}
	if gwIP == "" {
		gwIP = "192.168.1.1"
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(gwIP, "53"), 350*time.Millisecond)
	if err != nil {
		// Try port 80 if 53 is not listening
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(gwIP, "80"), 350*time.Millisecond)
	}

	status := "offline"
	latencyMs := 0.0
	if err == nil {
		_ = conn.Close()
		status = "online"
		latencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	}

	return gin.H{
		"ip":         gwIP,
		"status":     status,
		"latency_ms": fmt.Sprintf("%.1f", latencyMs),
	}
}

// GetGatewayHealth returns live gateway status
func GetGatewayHealth(c *gin.Context) {
	c.JSON(http.StatusOK, checkGatewayHealth())
}

// GetDeviceTelemetry returns real time-series metrics for a device
func GetDeviceTelemetry(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not ready"})
		return
	}

	idStr := c.Param("id")
	devID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz cihaz ID"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT cpu_usage_pct, ram_usage_pct, disk_usage_pct, temperature_celsius, recorded_at
		FROM system_metrics
		WHERE device_id = $1
		ORDER BY recorded_at DESC
		LIMIT 24
	`, devID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var points []gin.H
	for rows.Next() {
		var cpu, ram, disk, temp sql.NullFloat64
		var recordedAt time.Time
		if err := rows.Scan(&cpu, &ram, &disk, &temp, &recordedAt); err == nil {
			points = append(points, gin.H{
				"cpu":         cpu.Float64,
				"ram":         ram.Float64,
				"disk":        disk.Float64,
				"temp":        temp.Float64,
				"recorded_at": recordedAt.Format("15:04:05"),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id": devID,
		"metrics":   points,
	})
}


