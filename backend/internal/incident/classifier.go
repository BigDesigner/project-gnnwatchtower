package incident

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gnnwatchtower/internal/database"
)

// EvaluateSimultaneousOutages checks if multiple devices failed in the same minute
func EvaluateSimultaneousOutages(triggerDeviceID int, incidentID int) {
	if database.DB == nil {
		return
	}

	// Look for outages within 60 seconds
	var count int
	err := database.DB.QueryRow(`
		SELECT COUNT(DISTINCT device_id)
		FROM downtime_incidents
		WHERE started_at >= datetime('now', '-60 seconds')
		  AND resolved_at IS NULL
	`).Scan(&count)

	if err != nil {
		return
	}

	// If 3 or more devices failed simultaneously, classify as power outage / core switch failure
	if count >= 3 {
		log.Printf("[Incident Engine] Simultaneous failure detected: %d devices down together. Classifying as power outage.", count)

		hypothesis := fmt.Sprintf("Toplu Kesinti Korelasyonu: %d cihaz ayni anda kesintiye ugradi (Olası Elektrik Kesintisi veya Switch Arızası)", count)

		_, _ = database.DB.Exec(`
			UPDATE downtime_incidents
			SET classified_cause = 'power_outage',
			    auto_detection_hypothesis = $1
			WHERE started_at >= datetime('now', '-90 seconds')
			  AND resolved_at IS NULL
			  AND classified_cause = 'unclassified'
		`, hypothesis)
	}
}

func isChannelEnabled(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "true" || v == "1" || v == "yes"
}

// padRight pads string with spaces to reach exact visual rune width (supports UTF-8 / Turkish characters)
func padRight(s string, targetWidth int) string {
	runes := []rune(s)
	if len(runes) > targetWidth {
		if targetWidth > 3 {
			return string(runes[:targetWidth-3]) + "..."
		}
		return string(runes[:targetWidth])
	}
	return s + strings.Repeat(" ", targetWidth-len(runes))
}

// BuildAsciiTable formats a 36-character wide UTF-8 box table for Telegram and Discord code blocks
func BuildAsciiTable(company, device, ip, status, timestamp string) string {
	col1Width := 9
	col2Width := 20

	row := func(param, val string) string {
		return fmt.Sprintf("│ %s │ %s │", padRight(param, col1Width), padRight(val, col2Width))
	}

	return fmt.Sprintf(
`┌───────────┬──────────────────────┐
│ Parametre │ Değer                │
├───────────┼──────────────────────┤
%s
%s
%s
%s
%s
└───────────┴──────────────────────┘`,
		row("Müşteri", company),
		row("Cihaz", device),
		row("IP", ip),
		row("Durum", status),
		row("Zaman", timestamp),
	)
}

// FormatTelegramAlert constructs a rich HTML notification with customer company name and ASCII table
func FormatTelegramAlert(company, location, device, ip, alertType, detail, timestamp string) string {
	escapedComp := html.EscapeString(company)
	escapedLoc := html.EscapeString(location)
	escapedType := html.EscapeString(alertType)
	escapedDetail := html.EscapeString(detail)

	table := BuildAsciiTable(company, device, ip, alertType, timestamp)

	locStr := ""
	if escapedLoc != "" {
		locStr = fmt.Sprintf(" (%s)", escapedLoc)
	}

	return fmt.Sprintf(
`🏢 <b>MÜŞTERİ: %s</b>%s
🚨 <b>BİLDİRİM: %s</b>

<pre>
%s
</pre>
📋 <b>Detay:</b> %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🛡️ <i>GNNwatchtower Appliance</i>`,
		escapedComp, locStr, escapedType, table, escapedDetail,
	)
}

// FormatDiscordAlert constructs a Discord webhook payload with color-coded embed, ASCII table and fields
func FormatDiscordAlert(company, location, device, ip, alertType, detail, timestamp string) string {
	table := BuildAsciiTable(company, device, ip, alertType, timestamp)

	color := 0x0284C7 // Cyan / Info default
	upperType := strings.ToUpper(alertType)
	if strings.Contains(upperType, "DOWN") || strings.Contains(upperType, "KESİNTİ") || strings.Contains(upperType, "HATA") || strings.Contains(upperType, "CRASH") || strings.Contains(upperType, "OFFLINE") {
		color = 0xDC2626 // Red
	} else if strings.Contains(upperType, "ONLINE") || strings.Contains(upperType, "RECOVERY") || strings.Contains(upperType, "BAŞARI") || strings.Contains(upperType, "ÇÖZÜLDÜ") {
		color = 0x16A34A // Green
	} else if strings.Contains(upperType, "BAKIM") || strings.Contains(upperType, "PLANLI") || strings.Contains(upperType, "UYARI") {
		color = 0xF59E0B // Orange / Amber
	}

	locDisplay := location
	if locDisplay == "" {
		locDisplay = "Merkez Sistem Odası"
	}

	payload := map[string]interface{}{
		"content": fmt.Sprintf("🚨 **[GNNwatchtower] %s Bildirimi** — **%s**", alertType, company),
		"embeds": []map[string]interface{}{
			{
				"title": fmt.Sprintf("[%s] %s (%s)", alertType, device, ip),
				"description": fmt.Sprintf("```\n%s\n```\n**Detay:** %s", table, detail),
				"color": color,
				"fields": []map[string]interface{}{
					{"name": "🏢 Müşteri Şirketi", "value": fmt.Sprintf("**%s**", company), "inline": true},
					{"name": "📍 Lokasyon / Şube", "value": locDisplay, "inline": true},
					{"name": "⚠️ Olay Türü", "value": fmt.Sprintf("`%s`", alertType), "inline": true},
					{"name": "🖥️ Cihaz Adı", "value": fmt.Sprintf("`%s`", device), "inline": true},
					{"name": "🌐 IP Adresi", "value": fmt.Sprintf("`%s`", ip), "inline": true},
					{"name": "⏰ Zaman", "value": timestamp, "inline": true},
				},
				"footer": map[string]string{
					"text": fmt.Sprintf("GNNwatchtower Appliance • %s", company),
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	return string(jsonBytes)
}

// DispatchAlert sends notifications to configured external channels (Brevo Email, Telegram, Discord)
func DispatchAlert(deviceName, ip, alertType, detail string) {
	if database.DB == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Read settings
	settings := getAlertSettings()
	companyName := strings.TrimSpace(settings["client_company_name"])
	if companyName == "" {
		companyName = "Müşteri Altyapısı"
	}
	location := strings.TrimSpace(settings["client_location"])
	if location == "" {
		location = "Merkez Sistem Odası"
	}

	// 1. Brevo Email Integration
	if isChannelEnabled(settings["brevo_enabled"]) && settings["brevo_api_key"] != "" && settings["brevo_recipient_email"] != "" {
		go func() {
			_ = SendBrevoEmail(
				settings["brevo_api_key"],
				settings["brevo_sender_name"],
				settings["brevo_sender_email"],
				settings["brevo_recipient_email"],
				deviceName, ip, alertType, detail, timestamp,
				companyName, location,
			)
		}()
	}

	// 2. Telegram Integration
	if isChannelEnabled(settings["telegram_enabled"]) && settings["telegram_bot_token"] != "" && settings["telegram_chat_id"] != "" {
		tgMsg := FormatTelegramAlert(companyName, location, deviceName, ip, alertType, detail, timestamp)
		go func() {
			_ = SendTelegram(settings["telegram_bot_token"], settings["telegram_chat_id"], tgMsg)
		}()
	}

	// 3. Discord Webhook Integration
	if isChannelEnabled(settings["discord_enabled"]) && settings["discord_webhook_url"] != "" {
		discMsg := FormatDiscordAlert(companyName, location, deviceName, ip, alertType, detail, timestamp)
		go func() {
			_ = SendDiscord(settings["discord_webhook_url"], discMsg)
		}()
	}
}

func getAlertSettings() map[string]string {
	result := make(map[string]string)
	rows, err := database.DB.Query(`
		SELECT key, value FROM system_settings 
		WHERE key IN (
			'client_company_name', 'client_location',
			'brevo_enabled', 'brevo_api_key', 'brevo_sender_name', 'brevo_sender_email', 'brevo_recipient_email',
			'telegram_enabled', 'telegram_bot_token', 'telegram_chat_id',
			'discord_enabled', 'discord_webhook_url'
		)
	`)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			result[k] = v
		}
	}
	return result
}

// SendBrevoEmail sends an email alert via Brevo REST API
func SendBrevoEmail(apiKey, senderName, senderEmail, recipientEmail, devName, ip, alertType, detail, timestamp, company, location string) error {
	apiKey = strings.TrimSpace(apiKey)
	senderEmail = strings.TrimSpace(senderEmail)
	recipientEmail = strings.TrimSpace(recipientEmail)

	if apiKey == "" {
		return fmt.Errorf("Brevo API anahtarı boş olamaz")
	}
	if recipientEmail == "" {
		return fmt.Errorf("Alıcı e-posta adresi boş olamaz")
	}
	if senderName == "" {
		senderName = "GNNwatchtower"
	}
	if senderEmail == "" {
		senderEmail = "alerts@gnnwatchtower.local"
	}
	if company == "" {
		company = "Müşteri Altyapısı"
	}
	if location == "" {
		location = "Merkez Sistem Odası"
	}

	escComp := html.EscapeString(company)
	escLoc := html.EscapeString(location)
	escType := html.EscapeString(alertType)
	escDev := html.EscapeString(devName)
	escIP := html.EscapeString(ip)
	escTime := html.EscapeString(timestamp)
	escDetail := html.EscapeString(detail)

	subject := fmt.Sprintf("[%s] [%s] %s (%s)", company, alertType, devName, ip)
	htmlBody := fmt.Sprintf(`
		<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #e2e8f0; border-radius: 6px; background-color: #ffffff;">
			<div style="border-bottom: 2px solid #0284c7; padding-bottom: 10px; margin-bottom: 20px;">
				<h2 style="color: #0f172a; margin: 0; font-size: 20px;">GNNwatchtower Altyapı Bildirimi</h2>
				<p style="color: #64748b; margin: 4px 0 0 0; font-size: 13px;">Altyapı İzleme ve Olay Yönetim Sistemi</p>
			</div>
			<div style="background-color: #f8fafc; padding: 15px; border-radius: 4px; margin-bottom: 20px;">
				<p style="margin: 6px 0; font-size: 14px;"><strong>Müşteri:</strong> <span style="color: #0284c7; font-weight: bold;">%s</span></p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>Lokasyon:</strong> %s</p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>Olay Tipi:</strong> <span style="color: #dc2626; font-weight: bold;">%s</span></p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>Cihaz Adı:</strong> %s</p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>IP Adresi:</strong> %s</p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>Zaman:</strong> %s</p>
				<p style="margin: 6px 0; font-size: 14px;"><strong>Detay:</strong> %s</p>
			</div>
			<p style="color: #94a3b8; font-size: 12px; margin: 0;">Bu e-posta GNNwatchtower tarafından otomatik olarak oluşturulmuştur.</p>
		</div>
	`, escComp, escLoc, escType, escDev, escIP, escTime, escDetail)

	payload := map[string]interface{}{
		"sender": map[string]string{
			"name":  senderName,
			"email": senderEmail,
		},
		"to": []map[string]string{
			{
				"email": recipientEmail,
				"name":  "Sistem Yöneticisi",
			},
		},
		"subject":     subject,
		"htmlContent": htmlBody,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Alert Engine] Brevo email dispatch error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[Alert Engine] Brevo email API error (%d): %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("Brevo API Hatası (%d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[Alert Engine] Brevo email dispatched to %s (Status: %d)", recipientEmail, resp.StatusCode)
	return nil
}

// SendTelegram dispatches a message to a Telegram chat or channel
func SendTelegram(token, chatID, text string) error {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "bot")
	chatID = strings.TrimSpace(chatID)

	if token == "" {
		return fmt.Errorf("Telegram Bot Token boş olamaz")
	}
	if chatID == "" {
		return fmt.Errorf("Telegram Chat ID boş olamaz")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Alert Engine] Telegram HTTP error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Alert Engine] Telegram API error (%d): %s", resp.StatusCode, string(respBody))
		var tgErr struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(respBody, &tgErr)
		if tgErr.Description != "" {
			return fmt.Errorf("Telegram API Hatası (%d): %s", resp.StatusCode, tgErr.Description)
		}
		return fmt.Errorf("Telegram API Hatası: HTTP %d", resp.StatusCode)
	}

	log.Printf("[Alert Engine] Telegram notification delivered successfully to chat %s", chatID)
	return nil
}

// SendDiscord dispatches an alert message to a Discord webhook URL
func SendDiscord(webhookURL, payloadOrText string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("Discord Webhook URL boş olamaz")
	}

	var body []byte
	trimmed := strings.TrimSpace(payloadOrText)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		body = []byte(trimmed)
	} else {
		payload := map[string]string{
			"content": payloadOrText,
		}
		body, _ = json.Marshal(payload)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Alert Engine] Discord HTTP error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[Alert Engine] Discord Webhook error (%d): %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("Discord Webhook Hatası (%d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[Alert Engine] Discord notification delivered successfully")
	return nil
}
