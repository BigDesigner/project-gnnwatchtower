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
	"gnnwatchtower/internal/version"
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
	if len(runes) >= targetWidth {
		return string(runes[:targetWidth])
	}
	return s + strings.Repeat(" ", targetWidth-len(runes))
}

// wrapText splits text into lines of at most maxWidth runes without breaking words when possible
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	var result []string
	paragraphs := strings.Split(text, "\n")
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		words := strings.Fields(para)
		if len(words) == 0 {
			continue
		}

		currentLine := ""
		for _, word := range words {
			wRunes := []rune(word)
			if currentLine == "" {
				if len(wRunes) <= maxWidth {
					currentLine = word
				} else {
					for len(wRunes) > maxWidth {
						result = append(result, string(wRunes[:maxWidth]))
						wRunes = wRunes[maxWidth:]
					}
					currentLine = string(wRunes)
				}
			} else {
				cRunes := []rune(currentLine)
				if len(cRunes)+1+len(wRunes) <= maxWidth {
					currentLine += " " + word
				} else {
					result = append(result, currentLine)
					if len(wRunes) <= maxWidth {
						currentLine = word
					} else {
						for len(wRunes) > maxWidth {
							result = append(result, string(wRunes[:maxWidth]))
							wRunes = wRunes[maxWidth:]
						}
						currentLine = string(wRunes)
					}
				}
			}
		}
		if currentLine != "" {
			result = append(result, currentLine)
		}
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

// AlertSeverity represents alert status severity level
type AlertSeverity string

const (
	SeverityDown    AlertSeverity = "DOWN"
	SeverityRecover AlertSeverity = "RECOVER"
	SeverityWarning AlertSeverity = "WARNING"
	SeverityInfo    AlertSeverity = "INFO"
)

// GetAlertSeverity returns severity category, emoji, Discord int color, and hex color
func GetAlertSeverity(alertType string) (AlertSeverity, string, int, string) {
	upper := strings.ToUpper(alertType)
	if strings.Contains(upper, "DOWN") ||
		strings.Contains(upper, "KESİNTİ") ||
		strings.Contains(upper, "HATA") ||
		strings.Contains(upper, "CRASH") ||
		strings.Contains(upper, "OFFLINE") ||
		strings.Contains(upper, "KAPANDI") ||
		strings.Contains(upper, "BAŞARISIZ") {
		return SeverityDown, "🔴", 0xDC2626, "#DC2626"
	}
	if strings.Contains(upper, "ONLINE") ||
		strings.Contains(upper, "RECOVER") ||
		strings.Contains(upper, "ÇÖZÜLDÜ") ||
		strings.Contains(upper, "DÜZELDİ") ||
		strings.Contains(upper, "BAŞARI") ||
		strings.Contains(upper, "AKTİF") {
		return SeverityRecover, "🟢", 0x16A34A, "#16A34A"
	}
	if strings.Contains(upper, "BAKIM") ||
		strings.Contains(upper, "PLANLI") ||
		strings.Contains(upper, "UYARI") {
		return SeverityWarning, "🟡", 0xF59E0B, "#F59E0B"
	}
	return SeverityInfo, "🔵", 0x2563EB, "#2563EB"
}

// BuildAsciiTable formats a 42-character wide UTF-8 box table for Telegram, Discord, and Slack code blocks
func BuildAsciiTable(company, location, device, ip, status, timestamp, detail string) string {
	col1Width := 9
	col2Width := 26

	col1Bar := strings.Repeat("─", col1Width+2)
	col2Bar := strings.Repeat("─", col2Width+2)

	top := fmt.Sprintf("┌%s┬%s┐", col1Bar, col2Bar)
	header := fmt.Sprintf("│ %s │ %s │", padRight("Parametre", col1Width), padRight("Değer", col2Width))
	divider := fmt.Sprintf("├%s┼%s┤", col1Bar, col2Bar)
	bottom := fmt.Sprintf("└%s┴%s┘", col1Bar, col2Bar)

	var lines []string
	lines = append(lines, top, header, divider)

	addField := func(param, val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		wrapped := wrapText(val, col2Width)
		for idx, line := range wrapped {
			p := ""
			if idx == 0 {
				p = param
			}
			lines = append(lines, fmt.Sprintf("│ %s │ %s │", padRight(p, col1Width), padRight(line, col2Width)))
		}
	}

	addField("Müşteri", company)
	if location != "" {
		addField("Lokasyon", location)
	}
	addField("Cihaz", device)
	addField("IP", ip)
	addField("Durum", status)
	addField("Zaman", timestamp)
	if detail != "" {
		addField("Detay", detail)
	}

	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

// FormatTelegramAlert constructs a clean, non-redundant HTML notification with ASCII table and emoji severity
func FormatTelegramAlert(company, location, device, ip, alertType, detail, timestamp string) string {
	_, emoji, _, _ := GetAlertSeverity(alertType)
	escapedType := html.EscapeString(alertType)

	table := BuildAsciiTable(company, location, device, ip, alertType, timestamp, detail)
	escapedTable := html.EscapeString(table)

	return fmt.Sprintf(
		`<b>%s [ALERT - GNNWATCHTOWER] %s</b>

<pre>
%s
</pre>
────────────────────────────────────
<i>GNNwatchtower Enterprise Appliance v%s</i>`,
		emoji, escapedType, escapedTable, version.Get(),
	)
}

// FormatDiscordAlert constructs a Discord webhook payload with color-coded embed, ASCII table and fields
func FormatDiscordAlert(company, location, device, ip, alertType, detail, timestamp string) string {
	table := BuildAsciiTable(company, location, device, ip, alertType, timestamp, detail)
	_, emoji, color, _ := GetAlertSeverity(alertType)

	locDisplay := location
	if locDisplay == "" {
		locDisplay = "Merkez Sistem Odası"
	}

	payload := map[string]interface{}{
		"content": fmt.Sprintf("**%s [ALERT - GNNWATCHTOWER]** — **%s**", emoji, company),
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("%s [ALERT - GNNWATCHTOWER] %s", emoji, alertType),
				"description": fmt.Sprintf("```\n%s\n```", table),
				"color":       color,
				"fields": []map[string]interface{}{
					{"name": "Müşteri Şirketi", "value": fmt.Sprintf("**%s**", company), "inline": true},
					{"name": "Lokasyon / Şube", "value": locDisplay, "inline": true},
					{"name": "Olay Türü", "value": fmt.Sprintf("`%s`", alertType), "inline": true},
					{"name": "Cihaz Adı", "value": fmt.Sprintf("`%s`", device), "inline": true},
					{"name": "IP Adresi", "value": fmt.Sprintf("`%s`", ip), "inline": true},
					{"name": "Zaman", "value": timestamp, "inline": true},
				},
				"footer": map[string]string{
					"text": fmt.Sprintf("GNNwatchtower Enterprise Appliance v%s • %s", version.Get(), company),
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	return string(jsonBytes)
}

// FormatSlackAlert constructs a Slack webhook payload with color attachments and ASCII table
func FormatSlackAlert(company, location, device, ip, alertType, detail, timestamp string) string {
	table := BuildAsciiTable(company, location, device, ip, alertType, timestamp, detail)
	_, emoji, _, hexColor := GetAlertSeverity(alertType)

	locDisplay := location
	if locDisplay == "" {
		locDisplay = "Merkez Sistem Odası"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": hexColor,
				"title": fmt.Sprintf("%s [ALERT - GNNWATCHTOWER] %s", emoji, alertType),
				"text":  fmt.Sprintf("```\n%s\n```", table),
				"fields": []map[string]interface{}{
					{"title": "Müşteri", "value": company, "short": true},
					{"title": "Lokasyon", "value": locDisplay, "short": true},
					{"title": "Cihaz Adı", "value": device, "short": true},
					{"title": "IP Adresi", "value": ip, "short": true},
					{"title": "Durum", "value": alertType, "short": true},
					{"title": "Zaman", "value": timestamp, "short": true},
				},
				"footer": fmt.Sprintf("GNNwatchtower Enterprise Appliance v%s • %s", version.Get(), company),
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

	if !permitAlert(ip + "\x00" + alertType) {
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

	tgEnabled := isChannelEnabled(settings["telegram_enabled"])
	discEnabled := isChannelEnabled(settings["discord_enabled"])
	brevoEnabled := isChannelEnabled(settings["brevo_enabled"])

	log.Printf("[Alert Engine] Tetiklendi: [%s] Cihaz: %s (%s) | Kanallar: Telegram=%v, Discord=%v, Brevo=%v",
		alertType, deviceName, ip, tgEnabled, discEnabled, brevoEnabled)

	// 1. Brevo Email Integration
	if brevoEnabled && settings["brevo_api_key"] != "" && settings["brevo_recipient_email"] != "" {
		enqueueAlert(func() {
			if err := SendBrevoEmail(
				settings["brevo_api_key"],
				settings["brevo_sender_name"],
				settings["brevo_sender_email"],
				settings["brevo_recipient_email"],
				deviceName, ip, alertType, detail, timestamp,
				companyName, location,
			); err != nil {
				log.Printf("[Alert Engine] Brevo bildirim gönderim hatası: %v", err)
			}
		})
	}

	// 2. Telegram Integration
	if tgEnabled && settings["telegram_bot_token"] != "" && settings["telegram_chat_id"] != "" {
		tgMsg := FormatTelegramAlert(companyName, location, deviceName, ip, alertType, detail, timestamp)
		enqueueAlert(func() {
			if err := SendTelegram(settings["telegram_bot_token"], settings["telegram_chat_id"], tgMsg); err != nil {
				log.Printf("[Alert Engine] Telegram bildirim gönderim hatası (%s / %s): %v", deviceName, ip, err)
			}
		})
	} else if settings["telegram_bot_token"] != "" && settings["telegram_chat_id"] != "" && !tgEnabled {
		log.Printf("[Alert Engine] Telegram atlandı: Bot token ve Chat ID mevcut fakat 'telegram_enabled' ayarı kapalı (false).")
	}

	// 3. Discord / Slack Webhook Integration
	if discEnabled && settings["discord_webhook_url"] != "" {
		webhookURL := settings["discord_webhook_url"]
		var webhookMsg string
		if strings.Contains(webhookURL, "hooks.slack.com") {
			webhookMsg = FormatSlackAlert(companyName, location, deviceName, ip, alertType, detail, timestamp)
		} else {
			webhookMsg = FormatDiscordAlert(companyName, location, deviceName, ip, alertType, detail, timestamp)
		}
		enqueueAlert(func() {
			if err := SendDiscord(webhookURL, webhookMsg); err != nil {
				log.Printf("[Alert Engine] Webhook bildirim gönderim hatası (%s / %s): %v", deviceName, ip, err)
			}
		})
	} else if settings["discord_webhook_url"] != "" && !discEnabled {
		log.Printf("[Alert Engine] Discord / Slack atlandı: Webhook URL mevcut fakat 'discord_enabled' ayarı kapalı (false).")
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

	_, emoji, _, hexColor := GetAlertSeverity(alertType)

	escComp := html.EscapeString(company)
	escLoc := html.EscapeString(location)
	escType := html.EscapeString(alertType)
	escDev := html.EscapeString(devName)
	escIP := html.EscapeString(ip)
	escTime := html.EscapeString(timestamp)
	escDetail := html.EscapeString(detail)

	subject := fmt.Sprintf("%s [ALERT - GNNWATCHTOWER] %s — %s (%s)", emoji, alertType, devName, ip)

	detailRow := ""
	if escDetail != "" {
		detailRow = fmt.Sprintf(`
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0; vertical-align: top; width: 120px;">Detay</td>
					<td style="padding: 10px 14px; color: #0f172a; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>`, escDetail)
	}

	htmlBody := fmt.Sprintf(`
		<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; border: 1px solid #e2e8f0; border-top: 4px solid %s; border-radius: 8px; background-color: #ffffff;">
			<div style="border-bottom: 1px solid #e2e8f0; padding-bottom: 14px; margin-bottom: 18px;">
				<h2 style="color: #0f172a; margin: 0; font-size: 19px; font-weight: 700;">%s [ALERT - GNNWATCHTOWER]</h2>
				<p style="color: #64748b; margin: 4px 0 0 0; font-size: 13px;">Altyapı ve Sistem Olay Bildirimi</p>
			</div>
			<div style="margin-bottom: 16px;">
				<span style="display: inline-block; background-color: %s; color: #ffffff; padding: 4px 12px; border-radius: 9999px; font-weight: 600; font-size: 13px;">%s %s</span>
			</div>
			<table style="width: 100%%; border-collapse: collapse; font-size: 14px; margin-bottom: 20px; border: 1px solid #e2e8f0; border-radius: 6px; overflow: hidden;">
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0; width: 120px;">Müşteri</td>
					<td style="padding: 10px 14px; color: #0f172a; font-weight: 600; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0;">Lokasyon</td>
					<td style="padding: 10px 14px; color: #0f172a; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0;">Cihaz Adı</td>
					<td style="padding: 10px 14px; color: #0f172a; font-weight: 600; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0;">IP Adresi</td>
					<td style="padding: 10px 14px; color: #0f172a; font-family: monospace; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0;">Durum</td>
					<td style="padding: 10px 14px; color: %s; font-weight: 700; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 10px 14px; font-weight: 600; color: #475569; background-color: #f8fafc; border-bottom: 1px solid #e2e8f0;">Zaman</td>
					<td style="padding: 10px 14px; color: #0f172a; border-bottom: 1px solid #e2e8f0;">%s</td>
				</tr>
				%s
			</table>
			<p style="color: #94a3b8; font-size: 12px; margin: 0;">Bu e-posta GNNwatchtower Enterprise Appliance v%s tarafından otomatik olarak oluşturulmuştur.</p>
		</div>
	`, hexColor, emoji, hexColor, emoji, escType, escComp, escLoc, escDev, escIP, hexColor, escType, escTime, detailRow, version.Get())

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

	client := notificationClient()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Alert Engine] Brevo email dispatch error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
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
	client := notificationClient()
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Alert Engine] Telegram HTTP error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
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

	client := notificationClient()
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Alert Engine] Discord HTTP error: %v", err)
		return fmt.Errorf("Ağ bağlantı hatası: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		log.Printf("[Alert Engine] Discord Webhook error (%d): %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("Discord Webhook Hatası (%d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[Alert Engine] Discord notification delivered successfully")
	return nil
}
