package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/backup"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/discovery"
	"gnnwatchtower/internal/incident"
	"gnnwatchtower/internal/models"
	"gnnwatchtower/internal/monitor"
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
		SELECT 
			d.id, d.name, d.ip_address, d.mac_address, d.device_type, d.os_version, d.purpose, 
			d.location, d.operator_notes, d.is_agent_installed, d.agent_token, d.monitoring_profile, 
			d.check_interval_seconds, d.status, d.last_seen_at, d.last_status_change_at, 
			d.response_time_ms, d.created_at, d.updated_at,
			m.cpu_usage_pct, m.ram_usage_pct, m.disk_usage_pct, m.temperature_celsius,
			m.net_rx_kbps, m.net_tx_kbps
		FROM devices d
		LEFT JOIN (
			SELECT sm1.device_id, sm1.cpu_usage_pct, sm1.ram_usage_pct, sm1.disk_usage_pct, 
			       sm1.temperature_celsius, sm1.net_rx_kbps, sm1.net_tx_kbps
			FROM system_metrics sm1
			JOIN (
				SELECT device_id, MAX(id) as max_id
				FROM system_metrics
				GROUP BY device_id
			) sm2 ON sm1.id = sm2.max_id
		) m ON d.id = m.device_id
		ORDER BY d.id ASC
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
		var cpu, ram, disk, temp, netRx, netTx sql.NullFloat64

		if err := rows.Scan(&d.ID, &d.Name, &d.IPAddress, &mac, &d.DeviceType, &osVer, &purp, &loc, &notes,
			&d.IsAgentInstalled, &token, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status,
			&lastSeen, &d.LastStatusChangeAt, &d.ResponseTimeMS, &d.CreatedAt, &d.UpdatedAt,
			&cpu, &ram, &disk, &temp, &netRx, &netTx); err == nil {
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

			if cpu.Valid {
				v := math.Round(cpu.Float64*10) / 10
				d.LatestCPU = &v
			}
			if ram.Valid {
				v := math.Round(ram.Float64*10) / 10
				d.LatestRAM = &v
			}
			if disk.Valid {
				v := math.Round(disk.Float64*10) / 10
				d.LatestDisk = &v
			}
			if temp.Valid && temp.Float64 > 0 {
				v := math.Round(temp.Float64*10) / 10
				d.LatestTemp = &v
			}
			if netRx.Valid {
				v := math.Round(netRx.Float64*10) / 10
				d.LatestNetRx = &v
			}
			if netTx.Valid {
				v := math.Round(netTx.Float64*10) / 10
				d.LatestNetTx = &v
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

	input.IPAddress = strings.TrimSpace(input.IPAddress)
	input.Name = strings.TrimSpace(input.Name)
	if strings.EqualFold(input.DeviceType, "website") && !strings.HasPrefix(input.IPAddress, "http://") && !strings.HasPrefix(input.IPAddress, "https://") {
		input.IPAddress = "https://" + input.IPAddress
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

	incident.DispatchAlert(input.Name, input.IPAddress, "YENİ CİHAZ EKLENDİ", fmt.Sprintf("Envantere yeni bir cihaz kaydedildi (Tip: %s, Profil: %s).", input.DeviceType, input.MonitoringProfile))

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

	var devName, devIP string
	_ = database.DB.QueryRow(`
		SELECT d.name, d.ip_address 
		FROM downtime_incidents i 
		JOIN devices d ON d.id = i.device_id 
		WHERE i.id = $1
	`, incID).Scan(&devName, &devIP)

	causeLabel := input.ClassifiedCause
	switch input.ClassifiedCause {
	case "power_outage":
		causeLabel = "Elektrik Kesintisi"
	case "planned_maintenance":
		causeLabel = "Planlı Bakım"
	case "system_crash":
		causeLabel = "Sistem Çökmesi (OS Crash)"
	case "network_failure":
		causeLabel = "Ağ / Switch Bağlantı Arızası"
	}

	detail := fmt.Sprintf("Teşhis: %s. Operatör: %s.", causeLabel, input.OperatorName)
	if input.OperatorNotes != "" {
		detail += fmt.Sprintf(" Not: %s", input.OperatorNotes)
	}
	incident.DispatchAlert(devName, devIP, "KESİNTİ TEŞHİSİ KAYDEDİLDİ", detail)

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

// GetSettings lists system configuration keys as a direct key-value map
func GetSettings(c *gin.Context) {
	rows, err := database.DB.Query("SELECT key, value FROM system_settings")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	settingsMap := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			settingsMap[k] = v
		}
	}
	c.JSON(http.StatusOK, settingsMap)
}

// UpdateSettings updates key-value configurations with bulletproof atomic batch transaction
func UpdateSettings(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not ready"})
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Veritabanı işlemi başlatılamadı: " + err.Error()})
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SQL hazırlama hatası: " + err.Error()})
		return
	}
	defer stmt.Close()

	for k, rawVal := range input {
		// Ignore empty keys or numerical array indices (e.g., "0", "1")
		if k == "" || len(k) < 2 {
			continue
		}

		var strVal string
		switch v := rawVal.(type) {
		case string:
			strVal = v
		case float64:
			strVal = fmt.Sprintf("%.0f", v)
		case bool:
			strVal = fmt.Sprintf("%v", v)
		case map[string]interface{}:
			// If frontend passed an object with {key, value}, extract the string value
			if val, ok := v["value"].(string); ok {
				strVal = val
			} else {
				continue
			}
		default:
			continue
		}

		if _, err := stmt.Exec(k, strVal); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ayar kaydedilemedi: " + err.Error()})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "İşlem onaylanamadı: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ayarlar başarıyla kaydedildi."})
}

type TestAlertRequest struct {
	Channel           string `json:"channel"`
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramChatID    string `json:"telegram_chat_id"`
	DiscordWebhookURL string `json:"discord_webhook_url"`
	BrevoAPIKey       string `json:"brevo_api_key"`
	BrevoSenderName   string `json:"brevo_sender_name"`
	BrevoSenderEmail  string `json:"brevo_sender_email"`
	BrevoRecipient    string `json:"brevo_recipient_email"`
}

// TestAlert sends an instant verification message to Telegram, Discord or Brevo
func TestAlert(c *gin.Context) {
	var req TestAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz istek parametreleri"})
		return
	}

	// Fallback to database settings if input fields are empty
	if database.DB != nil {
		if req.TelegramBotToken == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'telegram_bot_token'").Scan(&req.TelegramBotToken)
		}
		if req.TelegramChatID == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'telegram_chat_id'").Scan(&req.TelegramChatID)
		}
		if req.DiscordWebhookURL == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'discord_webhook_url'").Scan(&req.DiscordWebhookURL)
		}
		if req.BrevoAPIKey == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'brevo_api_key'").Scan(&req.BrevoAPIKey)
		}
		if req.BrevoSenderName == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'brevo_sender_name'").Scan(&req.BrevoSenderName)
		}
		if req.BrevoSenderEmail == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'brevo_sender_email'").Scan(&req.BrevoSenderEmail)
		}
		if req.BrevoRecipient == "" {
			_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'brevo_recipient_email'").Scan(&req.BrevoRecipient)
		}
	}

	testMsg := fmt.Sprintf("✅ [GNNwatchtower] Test bildirimi başarıyla alındı!\nZaman: %s\nBu bildirim kanal entegrasyonunu doğrulamak için gönderilmiştir.", time.Now().Format("2006-01-02 15:04:05"))

	switch strings.ToLower(req.Channel) {
	case "telegram":
		if err := incident.SendTelegram(req.TelegramBotToken, req.TelegramChatID, testMsg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Telegram test bildirimi başarıyla iletildi!"})

	case "discord":
		if err := incident.SendDiscord(req.DiscordWebhookURL, "**[GNNwatchtower] Test Bildirimi**\n"+testMsg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Discord test bildirimi başarıyla iletildi!"})

	case "brevo":
		if err := incident.SendBrevoEmail(
			req.BrevoAPIKey,
			req.BrevoSenderName,
			req.BrevoSenderEmail,
			req.BrevoRecipient,
			"Test Sunucusu",
			"127.0.0.1",
			"TEST",
			"GNNwatchtower e-posta entegrasyonu başarıyla doğrulandı.",
			time.Now().Format("2006-01-02 15:04:05"),
		); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Brevo test e-postası başarıyla iletildi!"})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bilinmeyen bildirim kanalı"})
	}
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
		clientIP := c.ClientIP()
		targetIP := report.LocalIP
		if targetIP == "" || targetIP == "127.0.0.1" || strings.HasPrefix(targetIP, "127.") || targetIP == "::1" {
			targetIP = clientIP
		}

		var existingID int
		var existingName string
		findErr := database.DB.QueryRow(`
			SELECT id, name FROM devices WHERE ip_address = $1
		`, targetIP).Scan(&existingID, &existingName)

		if findErr == nil {
			devID = existingID
			devName = existingName
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET agent_token = $1, is_agent_installed = 1, status = 'online', last_seen_at = CURRENT_TIMESTAMP 
				WHERE id = $2
			`, report.Token, devID)
		} else {
			name := report.Hostname
			if name == "" {
				name = "Agent-" + targetIP
			}
			devType := "linux"
			if strings.Contains(strings.ToLower(report.OSName), "windows") {
				devType = "windows"
			}
			res, insErr := database.DB.Exec(`
				INSERT INTO devices (name, ip_address, device_type, os_version, is_agent_installed, agent_token, status, last_seen_at)
				VALUES ($1, $2, $3, $4, 1, $5, 'online', CURRENT_TIMESTAMP)
				ON CONFLICT (ip_address) DO UPDATE SET
					agent_token = EXCLUDED.agent_token,
					is_agent_installed = 1,
					status = 'online',
					last_seen_at = CURRENT_TIMESTAMP
			`, name, targetIP, devType, report.OSName+" "+report.OSVersion, report.Token)
			if insErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ajan yetkilendirilemedi: " + insErr.Error()})
				return
			}
			id64, _ := res.LastInsertId()
			devID = int(id64)
			devName = name

			if devID <= 0 {
				_ = database.DB.QueryRow("SELECT id FROM devices WHERE ip_address = $1", targetIP).Scan(&devID)
			}
			incident.DispatchAlert(devName, targetIP, "YENİ AJAN AKTİF", fmt.Sprintf("Yeni %s ajanı başarıyla sisteme bağlandı ve telemetri göndermeye başladı.", report.OSName))
		}
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

		incident.DispatchAlert(devName, report.LocalIP, "PLANLI KAPATMA / YENİDEN BAŞLATMA", "Ajan işletim sistemi temiz kapanma bildirimi iletti: "+reason)

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
		INSERT INTO system_metrics (device_id, cpu_usage_pct, ram_usage_pct, disk_usage_pct, temperature_celsius, net_rx_kbps, net_tx_kbps, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
	`, devID, report.CPUUsagePct, report.RAMUsagePct, report.DiskUsagePct, report.TempCelsius, report.NetRxKBps, report.NetTxKBps)

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
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err == nil {
			token = hex.EncodeToString(bytes)
		} else {
			token = fmt.Sprintf("agent-%d", time.Now().UnixNano())
		}
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

# Calisan eski ajani durdur (Text file busy ve curl 23 yazma hatasini onler)
systemctl stop gnnwatchtower-agent.service 2>/dev/null || true
pkill -9 -f gnnwatchtower-agent 2>/dev/null || true
sleep 1

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

echo "[GNNwatchtower] Ajan ikili dosyasi indiriliyor..."
rm -f "$INSTALL_DIR/gnnwatchtower-agent.tmp"
if ! curl -fsSL "$SERVER_URL/api/agent/download/linux" -o "$INSTALL_DIR/gnnwatchtower-agent.tmp"; then
    echo "[HATA] Ajan ikili dosyasi sunucudan indirilemedi! Lutfen sunucuda 'update.sh' calistirildigindan emin olunuz."
    exit 1
fi

chmod +x "$INSTALL_DIR/gnnwatchtower-agent.tmp"
mv -f "$INSTALL_DIR/gnnwatchtower-agent.tmp" "$INSTALL_DIR/gnnwatchtower-agent"

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
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err == nil {
			token = hex.EncodeToString(bytes)
		} else {
			token = fmt.Sprintf("agent-%d", time.Now().UnixNano())
		}
	}

	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, host)

	ps1 := fmt.Sprintf(`$ErrorActionPreference = "Stop"
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[HATA] Windows Ajan kurulumu Yonetici olarak calistirilmalidir (Run PowerShell as Administrator)!" -ForegroundColor Red
    exit 1
}

Write-Host "[GNNwatchtower] Windows Ajan kurulumu baslatiliyor..." -ForegroundColor Cyan

$serverUrl = "%s"
$token = "%s"
$installDir = "C:\Program Files\GNNwatchtower-Agent"
$taskName = "GNNwatchtowerAgent"

# Calisan eski ajani ve gorevi durdur
Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
Stop-Process -Name "gnnwatchtower-agent" -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

if (!(Test-Path -Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$agentExe = "$installDir\gnnwatchtower-agent.exe"
$agentTmp = "$installDir\gnnwatchtower-agent.tmp"

Write-Host "[GNNwatchtower] Ajan indiriliyor..."
try {
    Invoke-WebRequest -Uri "$serverUrl/api/agent/download/windows" -OutFile $agentTmp -UseBasicParsing
} catch {
    Write-Host "[HATA] Windows Ajan ikili dosyasi indirilemedi! Lutfen sunucuda 'update.sh' calistirildigindan emin olunuz." -ForegroundColor Red
    exit 1
}

if ((Get-Item $agentTmp).Length -lt 50000) {
    Remove-Item $agentTmp -Force -ErrorAction SilentlyContinue
    Write-Host "[HATA] Indirilen dosya boyutu gecersiz. Lutfen sunucuda 'update.sh' calistiriniz." -ForegroundColor Red
    exit 1
}

Move-Item -Path $agentTmp -Destination $agentExe -Force

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
		"/opt/gnnwatchtower/bin/" + filename,
		"./bin/" + filename,
		filename,
	}

	var foundPath string
	for _, p := range paths {
		if f, err := os.Stat(p); err == nil && !f.IsDir() && f.Size() > 10000 {
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Ajan ikili dosyası henüz oluşturulmadı. Lütfen sunucuda 'update.sh' çalıştırınız.",
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.File(foundPath)
}

// checkGatewayHealth tests default gateway reachability and ping latency
func checkGatewayHealth() gin.H {
	var gwIP string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'default_gateway'").Scan(&gwIP)
	}
	if gwIP == "" {
		gwIP = "192.168.1.1"
	}

	isOnline, latencyMs := monitor.PingHost(gwIP, 1*time.Second)

	status := "offline"
	if isOnline {
		status = "online"
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
		SELECT cpu_usage_pct, ram_usage_pct, disk_usage_pct, temperature_celsius, net_rx_kbps, net_tx_kbps, recorded_at
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
		var cpu, ram, disk, temp, netRx, netTx sql.NullFloat64
		var recordedAt time.Time
		if err := rows.Scan(&cpu, &ram, &disk, &temp, &netRx, &netTx, &recordedAt); err == nil {
			points = append(points, gin.H{
				"cpu":         math.Round(cpu.Float64*10) / 10,
				"ram":         math.Round(ram.Float64*10) / 10,
				"disk":        math.Round(disk.Float64*10) / 10,
				"temp":        math.Round(temp.Float64*10) / 10,
				"net_rx":      math.Round(netRx.Float64*10) / 10,
				"net_tx":      math.Round(netTx.Float64*10) / 10,
				"recorded_at": recordedAt.Format("15:04:05"),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id": devID,
		"metrics":   points,
	})
}


