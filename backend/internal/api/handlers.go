package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/backup"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/incident"
	"gnnwatchtower/internal/models"
	"gnnwatchtower/internal/monitor"
	"gnnwatchtower/internal/version"
)

// GetDashboardStats returns high-level system summary
func GetDashboardStats(c *gin.Context) {
	devices, err := loadDevices()
	if err != nil {
		c.JSON(500, gin.H{"error": "Envanter okunamadı"})
		return
	}
	counts := map[string]int{"online": 0, "offline": 0, "unknown": 0, "stale": 0, "maintenance": 0, "paused": 0}
	categories := map[string]int{"server": 0, "website": 0, "icmp": 0}
	for _, d := range devices {
		counts[d.ObservationState]++
		categories[d.MonitoringMode]++
	}
	var ratio *float64
	observed := counts["online"] + counts["offline"]
	if observed > 0 {
		r := float64(counts["online"]) * 100 / float64(observed)
		ratio = &r
	}
	var company, location string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key='client_company_name'").Scan(&company)
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key='client_location'").Scan(&location)
	c.JSON(200, gin.H{"total_devices": len(devices), "status_counts": counts, "category_counts": categories, "current_reachability_percentage": ratio, "reachability_observed_count": observed, "client_company_name": company, "client_location": location})
}

func GetDevices(c *gin.Context) {
	devices, err := loadDevices()
	if err != nil {
		c.JSON(500, gin.H{"error": "Envanter okunamadı"})
		return
	}
	c.JSON(200, devices)
}

func getServices(devID int) []models.ServiceMonitored {
	var list []models.ServiceMonitored
	if database.DB == nil {
		return list
	}
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

// CreateDevice manually registers a validated monitoring target
func CreateDevice(c *gin.Context) {
	var d models.Device
	if c.ShouldBindJSON(&d) != nil {
		c.JSON(400, gin.H{"error": "Geçersiz cihaz verisi"})
		return
	}
	if err := models.NormalizeDevice(&d); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var token any
	if d.MonitoringMode == models.ModeServer {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			c.JSON(500, gin.H{"error": "Kayıt anahtarı oluşturulamadı"})
			return
		}
		token = hex.EncodeToString(b)
	}
	var id int
	err := database.DB.QueryRow(`INSERT INTO devices(name,ip_address,mac_address,device_type,os_version,purpose,location,operator_notes,monitoring_mode,monitoring_profile,check_interval_seconds,agent_token,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'unknown') RETURNING id`, d.Name, d.IPAddress, d.MACAddress, d.DeviceType, d.OSVersion, d.Purpose, d.Location, d.OperatorNotes, d.MonitoringMode, d.MonitoringProfile, d.CheckIntervalSeconds, token).Scan(&id)
	if err != nil {
		c.JSON(409, gin.H{"error": "Hedef zaten kayıtlı veya cihaz kaydedilemedi"})
		return
	}
	incident.DispatchAlert(d.Name, d.IPAddress, "YENİ CİHAZ EKLENDİ", "Cihaz envantere kaydedildi; ilk kontrol bekleniyor.")
	result := gin.H{"id": id, "message": "Cihaz eklendi"}
	if token != nil {
		result["agent_token"] = token
	}
	c.JSON(201, result)
}

func UpdateDevice(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"error": "Geçersiz cihaz kimliği"})
		return
	}
	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(500, gin.H{"error": "Kayıt açılamadı"})
		return
	}
	defer tx.Rollback()
	var d models.Device
	err = tx.QueryRow(`SELECT id,name,ip_address,device_type,monitoring_mode,COALESCE(os_version,''),COALESCE(purpose,''),COALESCE(location,''),COALESCE(operator_notes,''),monitoring_profile,check_interval_seconds FROM devices WHERE id=?`, id).Scan(&d.ID, &d.Name, &d.IPAddress, &d.DeviceType, &d.MonitoringMode, &d.OSVersion, &d.Purpose, &d.Location, &d.OperatorNotes, &d.MonitoringProfile, &d.CheckIntervalSeconds)
	if err != nil {
		c.JSON(404, gin.H{"error": "Cihaz bulunamadı"})
		return
	}
	oldMode, oldAddress, oldProfile := d.MonitoringMode, d.IPAddress, d.MonitoringProfile
	// Decode over existing metadata so omitted fields remain intact.
	if c.ShouldBindJSON(&d) != nil {
		c.JSON(400, gin.H{"error": "Geçersiz cihaz verisi"})
		return
	}
	if d.MonitoringMode != oldMode || d.ID != id {
		c.JSON(400, gin.H{"error": "Mevcut kaydın izleme kategorisi değiştirilemez"})
		return
	}
	if err = models.NormalizeDevice(&d); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	_, err = tx.Exec(`UPDATE devices SET name=?,ip_address=?,device_type=?,os_version=?,purpose=?,location=?,operator_notes=?,monitoring_profile=?,check_interval_seconds=?,status=CASE WHEN ip_address!=? THEN 'unknown' ELSE status END,last_checked_at=CASE WHEN ip_address!=? THEN NULL ELSE last_checked_at END,last_probe_latency_ms=CASE WHEN ip_address!=? THEN NULL ELSE last_probe_latency_ms END,updated_at=CURRENT_TIMESTAMP WHERE id=?`, d.Name, d.IPAddress, d.DeviceType, d.OSVersion, d.Purpose, d.Location, d.OperatorNotes, d.MonitoringProfile, d.CheckIntervalSeconds, d.IPAddress, d.IPAddress, d.IPAddress, id)
	// Resuming a deliberately suspended monitor requires a new observation.
	if err == nil && (oldAddress != d.IPAddress || ((oldProfile == "maintenance" || oldProfile == "paused") && d.MonitoringProfile != "maintenance" && d.MonitoringProfile != "paused")) {
		_, err = tx.Exec(`UPDATE devices SET last_checked_at=NULL,last_probe_latency_ms=NULL,status='unknown',probe_error='',http_result='' WHERE id=?`, id)
	}
	if err != nil {
		c.JSON(409, gin.H{"error": "Hedef zaten kayıtlı veya güncelleme başarısız"})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(500, gin.H{"error": "Güncelleme kaydedilemedi"})
		return
	}
	c.JSON(200, gin.H{"message": "Cihaz güncellendi"})
}

func DeleteDevice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz cihaz ID"})
		return
	}

	tx, e := database.DB.Begin()
	if e != nil {
		c.JSON(500, gin.H{"error": "Silme işlemi başlatılamadı"})
		return
	}
	defer tx.Rollback()
	for _, table := range []string{"services_monitored", "downtime_incidents", "system_metrics"} {
		if _, e = tx.Exec("DELETE FROM "+table+" WHERE device_id=?", id); e != nil {
			c.JSON(500, gin.H{"error": "Bağlı kayıtlar silinemedi"})
			return
		}
	}
	_, err = tx.Exec("DELETE FROM devices WHERE id=?", id)
	if err == nil {
		err = tx.Commit()
	}
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

	if !requireServerMode(c, devID) {
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

	if input.TargetPort < 1 || input.TargetPort > 65535 || (input.CheckType != "tcp_port" && input.CheckType != "http") || (input.TargetPath != "" && !strings.HasPrefix(input.TargetPath, "/")) {
		c.JSON(400, gin.H{"error": "Geçersiz servis hedefi"})
		return
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

	_, err = database.DB.Exec("DELETE FROM services_monitored WHERE id = $1 AND device_id=$2", sID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Servis izlemeden kaldırıldı."})
}

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

	switch input.ClassifiedCause {
	case "power_outage", "planned_maintenance", "system_crash", "network_failure", "unclassified":
	default:
		c.JSON(400, gin.H{"error": "Geçersiz olay sınıflandırması"})
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
	rows, err := database.DB.Query("SELECT key, value FROM system_settings WHERE key != 'scan_subnet'")
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
		if k == "scan_subnet" {
			continue
		}
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

		if k == "agent_enrollment_key" {
			strVal = strings.TrimSpace(strVal)
			if strVal != "" {
				matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]{16,64}$`, strVal)
				if !matched {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Ajan kayıt anahtarı yalnızca 16-64 karakter alfanümerik, tire ve alt çizgi içerebilir."})
					return
				}
			}
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

	var companyName, location string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'client_company_name'").Scan(&companyName)
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		companyName = "Müşteri Altyapısı"
	}
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'client_location'").Scan(&location)
	location = strings.TrimSpace(location)
	if location == "" {
		location = "Merkez Sistem Odası"
	}

	testTimestamp := time.Now().Format("2006-01-02 15:04:05")

	switch strings.ToLower(req.Channel) {
	case "telegram":
		tgMsg := incident.FormatTelegramAlert(
			companyName,
			location,
			"Test Cihazı",
			"127.0.0.1",
			"TEST BAĞLANTISI",
			"GNNwatchtower Telegram entegrasyonu ve bildirim tablosu başarıyla doğrulandı.",
			testTimestamp,
		)
		if err := incident.SendTelegram(req.TelegramBotToken, req.TelegramChatID, tgMsg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Telegram test bildirimi başarıyla iletildi!"})

	case "discord":
		discMsg := incident.FormatDiscordAlert(
			companyName,
			location,
			"Test Cihazı",
			"127.0.0.1",
			"TEST BAĞLANTISI",
			"GNNwatchtower Discord entegrasyonu ve bildirim tablosu başarıyla doğrulandı.",
			testTimestamp,
		)
		if err := incident.SendDiscord(req.DiscordWebhookURL, discMsg); err != nil {
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
			"Test Cihazı",
			"127.0.0.1",
			"TEST BAĞLANTISI",
			"GNNwatchtower e-posta entegrasyonu başarıyla doğrulandı.",
			testTimestamp,
			companyName,
			location,
		); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Brevo test e-postası başarıyla iletildi!"})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bilinmeyen bildirim kanalı"})
	}
}

// AgentEnroll handles zero-touch agent enrollment
func AgentEnroll(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Veritabanı hazır değil"})
		return
	}

	var req models.AgentEnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz kayıt verisi: " + err.Error()})
		return
	}

	clientIP := c.ClientIP()
	targetIP := strings.TrimSpace(req.LocalIP)
	if targetIP == "" || targetIP == "127.0.0.1" || strings.HasPrefix(targetIP, "127.") || targetIP == "::1" {
		targetIP = clientIP
	}

	// Fix R06: Ensure targetIP is a valid IP address literal (strictly reject URLs, loopback or unparseable formats)
	parsedIP := net.ParseIP(targetIP)
	if parsedIP == nil || !models.ValidIP(targetIP) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz IP adresi biçimi. Yalnızca geçerli IPv4 veya IPv6 adresleri kabul edilir."})
		return
	}
	targetIP = parsedIP.String()

	// Validate Agent Enrollment Key (Fail-closed for unknown devices, self-healing for registered fleet)
	var serverEnrollKey string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_enrollment_key'").Scan(&serverEnrollKey)
	serverEnrollKey = strings.TrimSpace(serverEnrollKey)

	reqEnrollKey := strings.TrimSpace(c.GetHeader("X-Enrollment-Key"))
	if reqEnrollKey == "" {
		reqEnrollKey = strings.TrimSpace(req.EnrollmentKey)
	}

	keyMatches := (serverEnrollKey != "" && reqEnrollKey != "" && reqEnrollKey == serverEnrollKey)

	// Check if this is an already-registered device in our fleet (by machine_uuid, IP, or hostname)
	var preDevID int
	if req.MachineUUID != "" {
		_ = database.DB.QueryRow("SELECT id FROM devices WHERE machine_uuid = $1", req.MachineUUID).Scan(&preDevID)
	}
	if preDevID == 0 {
		_ = database.DB.QueryRow("SELECT id FROM devices WHERE ip_address = $1", targetIP).Scan(&preDevID)
	}
	if preDevID == 0 && req.Hostname != "" {
		_ = database.DB.QueryRow("SELECT id FROM devices WHERE LOWER(name) = LOWER($1)", req.Hostname).Scan(&preDevID)
	}

	if preDevID > 0 && !requireServerMode(c, preDevID) {
		return
	}
	if !keyMatches && preDevID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Geçersiz veya eksik ajan kayıt anahtarı (Enrollment Key)"})
		return
	}

	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	agentToken := hex.EncodeToString(tokenBytes)

	name := strings.TrimSpace(req.Hostname)
	if name == "" {
		name = "Agent-" + targetIP
	}
	if len(name) > 64 {
		name = name[:64]
	}

	devType := "linux"
	if strings.Contains(strings.ToLower(req.OSName), "windows") {
		devType = "windows"
	}

	agentVer := req.AgentVersion
	if agentVer == "" {
		agentVer = version.Get()
	}

	// 1. Check if device already exists by machine_uuid or ip_address
	var devID int
	var existingName string

	if req.MachineUUID != "" {
		_ = database.DB.QueryRow(`
			SELECT id, name FROM devices WHERE machine_uuid = $1
		`, req.MachineUUID).Scan(&devID, &existingName)
	}

	if devID == 0 {
		_ = database.DB.QueryRow(`
			SELECT id, name FROM devices WHERE ip_address = $1
		`, targetIP).Scan(&devID, &existingName)
	}

	if devID == 0 && req.Hostname != "" {
		_ = database.DB.QueryRow(`
			SELECT id, name FROM devices WHERE LOWER(name) = LOWER($1)
		`, req.Hostname).Scan(&devID, &existingName)
	}

	if devID > 0 {
		if !requireServerMode(c, devID) {
			return
		}
		// Fix R01: Prevent rogue device hijacking and token overwrites.
		var currentToken string
		var registeredIP string
		var registeredAgent bool
		if err := database.DB.QueryRow("SELECT COALESCE(agent_token,''), ip_address,is_agent_installed FROM devices WHERE id = $1", devID).Scan(&currentToken, &registeredIP, &registeredAgent); err != nil {
			c.JSON(500, gin.H{"error": "Ajan kaydı okunamadı"})
			return
		}
		currentToken = strings.TrimSpace(currentToken)

		reqAgentToken := strings.TrimSpace(c.GetHeader("X-Agent-Token"))
		if reqAgentToken == "" {
			reqAgentToken = strings.TrimSpace(req.AgentToken)
		}

		isAuthorizedReEnroll := (reqAgentToken != "" && reqAgentToken == currentToken) || (!registeredAgent && keyMatches)

		if !isAuthorizedReEnroll {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Bu cihaz zaten kayıtlıdır. Mevcut cihazın ajan anahtarını yenilemek için geçerli X-Agent-Token sunulmalıdır.",
			})
			return
		}

		// Existing device enrolled!
		_, updateErr := database.DB.Exec(`
			UPDATE devices 
			SET agent_token = $1,
			    is_agent_installed = 1,
			    agent_version = $2,
			    machine_uuid = COALESCE(NULLIF($3, ''), machine_uuid),
			    mac_address = COALESCE(NULLIF($4, ''), mac_address),
			    os_version = $5,
			    device_type = $6,
			    status = 'online',
			    agent_upgrade_error = 'key_healed',
			    last_seen_at = CURRENT_TIMESTAMP,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $7
		`, agentToken, agentVer, req.MachineUUID, req.MACAddress, req.OSName+" "+req.OSVersion, devType, devID)
		if updateErr != nil {
			c.JSON(500, gin.H{"error": "Ajan kaydı güncellenemedi"})
			return
		}

		log.Printf("[GNNwatchtower Core] Kayıtlı cihaz '%s' (%s) imza anahtarı ve oturum tokenını güncelledi.", existingName, targetIP)
	} else {
		// New device created via Auto-Enrollment!
		res, insErr := database.DB.Exec(`
			INSERT INTO devices (name, ip_address, mac_address, device_type, os_version, is_agent_installed, agent_token, agent_version, machine_uuid, status, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, 'online', CURRENT_TIMESTAMP)
		`, name, targetIP, req.MACAddress, devType, req.OSName+" "+req.OSVersion, agentToken, agentVer, req.MachineUUID)
		if insErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ajan kaydedilemedi: " + insErr.Error()})
			return
		}
		id64, _ := res.LastInsertId()
		devID = int(id64)

		incident.DispatchAlert(name, targetIP, "YENİ AJAN OTOMATİK KAYIT (AUTO-ENROLL)", fmt.Sprintf("Yeni %s sunucusu (%s - v%s) sıfır-dokunuş (zero-touch) ile sisteme kaydedildi.", req.OSName, targetIP, agentVer))
	}

	c.JSON(http.StatusOK, models.AgentEnrollResponse{
		Status:               "enrolled",
		Token:                agentToken,
		DeviceID:             devID,
		CheckIntervalSeconds: 15,
		ServerVersion:        version.Get(),
		UpdatePublicKey:      GetUpdatePublicKeyHex(),
	})
}

// AgentHeartbeat receives periodic telemetry and graceful shutdown events from Windows/Ubuntu agents
func AgentHeartbeat(c *gin.Context) {
	var report models.AgentReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := c.GetHeader("X-Agent-Token")
	if token == "" {
		token = report.Token
	}

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ajan yetkilendirme tokenı zorunludur"})
		return
	}

	var devID int
	var devName string
	var devType string
	var upgStatus string
	var upgTarget string

	err := database.DB.QueryRow(`
		SELECT id, name, device_type, COALESCE(agent_upgrade_status, 'none'), COALESCE(agent_upgrade_target, '')
		FROM devices 
		WHERE agent_token = $1
	`, token).Scan(&devID, &devName, &devType, &upgStatus, &upgTarget)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Geçersiz veya yetkilendirilmemiş ajan tokenı"})
		return
	}

	if !requireServerMode(c, devID) {
		return
	}

	var profile string
	if database.DB.QueryRow("SELECT monitoring_profile FROM devices WHERE id=?", devID).Scan(&profile) != nil {
		c.JSON(404, gin.H{"error": "Cihaz bulunamadı"})
		return
	}
	if profile == "paused" || profile == "maintenance" {
		c.JSON(200, models.AgentUpgradeInstruction{Status: "monitoring_suspended", Action: "none"})
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

		incident.DispatchAlert(devName, report.LocalIP, "PLANLI KAPATMA / YENİDEN BAŞLATMA", "Ajan işletim sistemi temiz kapanma bildirimi iletti: "+reason)

		c.JSON(http.StatusOK, models.AgentUpgradeInstruction{Status: "ack_shutdown", Action: "none"})
		return
	}

	// Handle agent upgrade feedback or automatic detection upon restart
	if report.UpgradeStatus == "success" || ((upgStatus == "pending" || upgStatus == "applying" || upgStatus == "downloading") && upgTarget != "" && report.AgentVersion != "" && report.AgentVersion == upgTarget) {
		if upgStatus != "success" && upgStatus != "none" {
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET agent_upgrade_status = 'success',
				    agent_upgrade_target = '',
				    agent_upgrade_error = '',
				    agent_last_upgrade_at = CURRENT_TIMESTAMP,
				    agent_version = COALESCE(NULLIF($1, ''), agent_version)
				WHERE id = $2
			`, report.AgentVersion, devID)
			incident.DispatchAlert(devName, report.LocalIP, "AJAN BAŞARIYLA GÜNCELLENDİ", fmt.Sprintf("Ajan başarıyla v%s sürümüne yükseltildi.", report.AgentVersion))
		} else {
			// Already notified; prevent notification spam loop and clear target
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET agent_upgrade_status = 'none',
				    agent_upgrade_target = '',
				    agent_upgrade_error = '',
				    agent_version = COALESCE(NULLIF($1, ''), agent_version)
				WHERE id = $2
			`, report.AgentVersion, devID)
		}
	} else if report.UpgradeStatus != "" && report.UpgradeStatus != "none" {
		if report.UpgradeStatus != upgStatus && upgStatus != "pending" {
			_, _ = database.DB.Exec(`
				UPDATE devices 
				SET agent_upgrade_status = $1,
				    agent_upgrade_error = $2
				WHERE id = $3
			`, report.UpgradeStatus, report.UpgradeError, devID)
		}
	}

	// Autonomous Zero-Touch Signer Key Healing:
	// If the agent reported a missing Signer Key error and upgrade is NOT yet pending,
	// challenge the agent ONCE with HTTP 401 Unauthorized so it invokes autoEnroll() to fetch signer.pub.
	// But if upgStatus == "pending", DO NOT challenge; proceed to dispatch the upgrade instruction!
	if (strings.Contains(report.UpgradeError, "Signer Key") || strings.Contains(report.UpgradeError, "anahtari")) && upgStatus != "pending" {
		var healedCount int
		_ = database.DB.QueryRow("SELECT COUNT(*) FROM devices WHERE id = $1 AND agent_upgrade_error = 'key_healed'", devID).Scan(&healedCount)
		if healedCount == 0 {
			_, _ = database.DB.Exec("UPDATE devices SET agent_upgrade_error = 'key_healed' WHERE id = $1", devID)
			log.Printf("[GNNwatchtower Core] Ajan '%s' (%s) imza anahtarini otomatik tescil etmek uzere 401 ile yeniden kayda yonlendirildi", devName, report.LocalIP)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Imza anahtari tescili icin otomatik yeniden kayit tetiklendi"})
			return
		}
	}

	// Auto-resolve any open downtime or planned_maintenance incidents for this device
	_, _ = database.DB.Exec(`
		UPDATE downtime_incidents 
		SET resolved_at = CURRENT_TIMESTAMP, 
		    duration_seconds = CAST((strftime('%s', 'now') - strftime('%s', started_at)) AS INTEGER)
		WHERE device_id = $1 AND resolved_at IS NULL
	`, devID)

	// Normal heartbeat update
	agentVer := report.AgentVersion
	if agentVer == "" {
		agentVer = "0.4.2"
	}
	_, _ = database.DB.Exec(`
		UPDATE devices 
		SET status = 'online',
		    is_agent_installed = 1,
		    agent_version = $1,
		    os_version = COALESCE(NULLIF($2, ''), os_version),
		    last_seen_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, agentVer, report.OSName+" "+report.OSVersion, devID)

	// Save telemetry metrics
	_, _ = database.DB.Exec(`
		INSERT INTO system_metrics (device_id, cpu_usage_pct, ram_usage_pct, disk_usage_pct, temperature_celsius, net_rx_kbps, net_tx_kbps, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
	`, devID, report.CPUUsagePct, report.RAMUsagePct, report.DiskUsagePct, report.TempCelsius, report.NetRxKBps, report.NetTxKBps)

	// Check if this device has a pending upgrade instruction!
	if upgStatus == "pending" {
		platform := "linux"
		if devType == "windows" || strings.Contains(strings.ToLower(report.OSName), "windows") {
			platform = "windows"
		}

		targetVer := upgTarget
		if targetVer == "" {
			targetVer = version.Get()
		}

		checksum := getBinaryChecksum(platform)
		sig, _ := SignAgentUpdate(targetVer, checksum)

		c.JSON(http.StatusOK, models.AgentUpgradeInstruction{
			Status:          "ok",
			Action:          "upgrade",
			TargetVersion:   targetVer,
			DownloadURL:     fmt.Sprintf("/api/agent/download/%s", platform),
			SHA256:          checksum,
			Signature:       sig,
			UpdatePublicKey: GetUpdatePublicKeyHex(),
		})
		return
	}

	c.JSON(http.StatusOK, models.AgentUpgradeInstruction{
		Status: "ok",
		Action: "none",
	})
}

// GetAgentSignerKey returns the appliance Ed25519 update signing public key
func GetAgentSignerKey(c *gin.Context) {
	pubKey := GetUpdatePublicKeyHex()
	if pubKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Ajan imza anahtarı henüz başlatılmadı"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"public_key": pubKey,
		"algorithm":  "Ed25519",
	})
}

// TriggerAgentUpgrade queues an upgrade instruction for a single device
func TriggerAgentUpgrade(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Veritabanı hazır değil"})
		return
	}

	id := c.Param("id")
	numericID, e := strconv.Atoi(id)
	if e != nil {
		c.JSON(400, gin.H{"error": "Geçersiz cihaz kimliği"})
		return
	}
	if !requireServerMode(c, numericID) {
		return
	}
	var targetVersion string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_latest_version'").Scan(&targetVersion)
	if targetVersion == "" {
		targetVersion = version.Get()
	}

	res, err := database.DB.Exec(`
		UPDATE devices 
		SET agent_upgrade_status = 'pending',
		    agent_upgrade_target = $1,
		    agent_upgrade_error = ''
		WHERE id = $2 AND is_agent_installed = 1 AND monitoring_mode='server'
	`, targetVersion, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cihaz bulunamadı veya üzerinde ajan kurulu değil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Ajan güncelleme talimatı kuyruğa alındı. Bir sonraki heartbeat döngüsünde uygulanacaktır.",
		"target_version": targetVersion,
	})
}

// TriggerAllAgentsUpgrade queues upgrade instructions for all devices with agents
func TriggerAllAgentsUpgrade(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Veritabanı hazır değil"})
		return
	}

	var targetVersion string
	_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_latest_version'").Scan(&targetVersion)
	if targetVersion == "" {
		targetVersion = version.Get()
	}

	res, err := database.DB.Exec(`
		UPDATE devices 
		SET agent_upgrade_status = 'pending',
		    agent_upgrade_target = $1,
		    agent_upgrade_error = ''
		WHERE is_agent_installed = 1 AND monitoring_mode='server'
	`, targetVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	count, _ := res.RowsAffected()
	c.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("%d adet ajana uzaktan güncelleme talimatı iletildi.", count),
		"queued_count":   count,
		"target_version": targetVersion,
	})
}

// GetAgentVersions returns information about available agent binaries and checksums
func GetAgentVersions(c *gin.Context) {
	var latestVersion string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_latest_version'").Scan(&latestVersion)
	}
	if latestVersion == "" {
		latestVersion = version.Get()
	}

	winHash := getBinaryChecksum("windows")
	linuxHash := getBinaryChecksum("linux")
	winSig, _ := SignAgentUpdate(latestVersion, winHash)
	linuxSig, _ := SignAgentUpdate(latestVersion, linuxHash)

	c.JSON(http.StatusOK, gin.H{
		"latest_version":    latestVersion,
		"sha256_windows":    winHash,
		"sha256_linux":      linuxHash,
		"signature_windows": winSig,
		"signature_linux":   linuxSig,
		"update_public_key": GetUpdatePublicKeyHex(),
		"download_windows":  "/api/agent/download/windows",
		"download_linux":    "/api/agent/download/linux",
	})
}

func getBinaryChecksum(platform string) string {
	var filename string
	if platform == "windows" {
		filename = "gnnwatchtower-agent.exe"
	} else {
		filename = "gnnwatchtower-agent"
	}

	var paths []string
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(exeDir, "bin", filename), filepath.Join(exeDir, filename))
	}
	paths = append(paths,
		"/opt/gnnwatchtower/bin/"+filename,
		"./bin/"+filename,
		"../bin/"+filename,
		filename,
	)

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil && len(data) > 10000 {
			h := sha256.Sum256(data)
			return hex.EncodeToString(h[:])
		}
	}
	return ""
}

// GenerateLinuxAgentScript outputs bash script for 1-click curl install on Ubuntu/Debian
func GenerateLinuxAgentScript(c *gin.Context) {
	proto := c.GetHeader("X-Forwarded-Proto")
	scheme := "http"
	if proto == "https" || c.Request.TLS != nil {
		scheme = "https"
	}

	rawHost := c.Request.Host
	cleanHost := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == ':' || r == '-' {
			return r
		}
		return -1
	}, rawHost)
	if cleanHost == "" {
		cleanHost = "127.0.0.1:3800"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, cleanHost)

	var enrollKey string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_enrollment_key'").Scan(&enrollKey)
	}

	cleanEnrollKey := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, enrollKey)

	enrollFlag := ""
	if cleanEnrollKey != "" {
		enrollFlag = fmt.Sprintf(" -enroll-key '%s'", cleanEnrollKey)
	}

	script := fmt.Sprintf(`#!/usr/bin/env bash
# ==============================================================================
# GNNwatchtower Telemetry Agent Installer (Linux / Ubuntu / Debian)
# Enterprise Zero-Touch Auto-Enrollment
# ==============================================================================
set -euo pipefail

SERVER_URL='%s'
INSTALL_DIR="/opt/gnnwatchtower-agent"
BIN_TARGET="$INSTALL_DIR/gnnwatchtower-agent"

echo "[GNNwatchtower] Kurumsal Linux Ajan kurulumu baslatiliyor..."

# Root kontrolu
if [ "$EUID" -ne 0 ]; then
    echo "[HATA] Lutfen kurulumu root veya 'sudo' ile calistiriniz."
    exit 1
fi

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Eski calisan ajani durdur
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop gnnwatchtower-agent.service 2>/dev/null || true
fi
pkill -9 -f gnnwatchtower-agent 2>/dev/null || true
sleep 1

echo "[GNNwatchtower] Ajan ikili dosyasi indiriliyor..."
rm -f "$BIN_TARGET.tmp"
if ! curl -fsSL "$SERVER_URL/api/agent/download/linux" -o "$BIN_TARGET.tmp"; then
    echo "[HATA] Ajan dosyasi sunucudan indirilemedi! Lutfen sunucu erisimini kontrol ediniz."
    exit 1
fi

chmod 755 "$BIN_TARGET.tmp"
mv -f "$BIN_TARGET.tmp" "$BIN_TARGET"

echo "[GNNwatchtower] Sistem servisi yapilandiriliyor ve baslatiliyor..."
"$BIN_TARGET" -service install -server "$SERVER_URL"%s
"$BIN_TARGET" -service start

echo "[GNNwatchtower] Ajan basariyla kuruldu, otomatik olarak kaydoldu (Auto-Enrollment) ve aktif calisiyor!"
`, serverURL, enrollFlag)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, script)
}

// GenerateWindowsAgentScript outputs enterprise powershell script for Windows Server
func GenerateWindowsAgentScript(c *gin.Context) {
	proto := c.GetHeader("X-Forwarded-Proto")
	scheme := "http"
	if proto == "https" || c.Request.TLS != nil {
		scheme = "https"
	}

	rawHost := c.Request.Host
	cleanHost := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == ':' || r == '-' {
			return r
		}
		return -1
	}, rawHost)
	if cleanHost == "" {
		cleanHost = "127.0.0.1:3800"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, cleanHost)

	var enrollKey string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'agent_enrollment_key'").Scan(&enrollKey)
	}

	cleanEnrollKey := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, enrollKey)

	enrollFlag := ""
	if cleanEnrollKey != "" {
		enrollFlag = fmt.Sprintf(" -enroll-key '%s'", cleanEnrollKey)
	}

	ps1 := fmt.Sprintf(`$ErrorActionPreference = "Stop"

# Administrator yetki kontrolu
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[HATA] Windows Ajan kurulumu Yonetici olarak calistirilmalidir (Run PowerShell as Administrator)!" -ForegroundColor Red
    exit 1
}

Write-Host "[GNNwatchtower] Windows Kurumsal Ajan kurulumu baslatiliyor..." -ForegroundColor Cyan

$serverUrl = "%s"
$installDir = "C:\Program Files\GNNwatchtower-Agent"
$agentExe = "$installDir\gnnwatchtower-agent.exe"
$agentTmp = "$installDir\gnnwatchtower-agent.tmp"

# Calisan eski ajani durdur
try {
    if (Test-Path $agentExe) {
        & "$agentExe" -service stop 2>$null
    }
} catch {}
Stop-Process -Name "gnnwatchtower-agent" -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

if (!(Test-Path -Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

Write-Host "[GNNwatchtower] Ajan ikili dosyasi indiriliyor..."
try {
    Invoke-WebRequest -Uri "$serverUrl/api/agent/download/windows" -OutFile $agentTmp -UseBasicParsing
} catch {
    Write-Host "[HATA] Windows Ajan ikili dosyasi indirilemedi! Lutfen sunucu erisimini kontrol ediniz." -ForegroundColor Red
    exit 1
}

if ((Get-Item $agentTmp).Length -lt 50000) {
    Remove-Item $agentTmp -Force -ErrorAction SilentlyContinue
    Write-Host "[HATA] Indirilen dosya boyutu gecersiz." -ForegroundColor Red
    exit 1
}

Move-Item -Path $agentTmp -Destination $agentExe -Force

Write-Host "[GNNwatchtower] Yerel Windows Hizmeti tescil ediliyor..."
try {
    & "$agentExe" -service install -server "$serverUrl"%s
} catch {
    Write-Host "[BILGI] Servis zaten kurulu veya tescil edildi, baslatiliyor..." -ForegroundColor Yellow
}

try {
    & "$agentExe" -service start
} catch {
    Start-Service -Name "GNNwatchtowerAgent" -ErrorAction SilentlyContinue
}

Write-Host "[GNNwatchtower] Windows Ajan basariyla kuruldu, otomatik olarak kaydoldu (Auto-Enrollment) ve baslatildi!" -ForegroundColor Green
Write-Host ""
Write-Host "[BILGI] Hizmet durumunu izlemek ve ag/port erisimini test etmek icin:" -ForegroundColor Cyan
Write-Host "        & '$agentExe' -diag" -ForegroundColor Yellow
`, serverURL, enrollFlag)

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

	var paths []string
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(exeDir, "bin", filename), filepath.Join(exeDir, filename))
	}
	paths = append(paths,
		"/opt/gnnwatchtower/bin/"+filename,
		"./bin/"+filename,
		"../bin/"+filename,
		filename,
	)

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
	var ip string
	if database.DB != nil {
		_ = database.DB.QueryRow("SELECT value FROM system_settings WHERE key='default_gateway'").Scan(&ip)
	}
	if !models.ValidIP(ip) {
		return gin.H{"status": "unknown", "ip": nil, "latency_ms": nil}
	}
	online, rtt, err := monitor.ProbeICMP(ip, time.Second)
	status := "offline"
	if err != nil {
		status = "unknown"
	} else if online {
		status = "online"
	}
	return gin.H{"ip": ip, "status": status, "latency_ms": rtt}
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

	if !requireServerMode(c, devID) {
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

	points := []gin.H{}
	for rows.Next() {
		var cpu, ram, disk, temp, netRx, netTx sql.NullFloat64
		var recordedAt time.Time
		if err := rows.Scan(&cpu, &ram, &disk, &temp, &netRx, &netTx, &recordedAt); err == nil {
			points = append(points, gin.H{
				"cpu":         telemetryNumber(cpu),
				"ram":         telemetryNumber(ram),
				"disk":        telemetryNumber(disk),
				"temp":        telemetryNumber(temp),
				"net_rx":      telemetryNumber(netRx),
				"net_tx":      telemetryNumber(netTx),
				"recorded_at": recordedAt.UTC().Format(time.RFC3339),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id": devID,
		"metrics":   points,
	})
}
