package incident

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPadRight(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		targetWidth int
		wantLen     int // rune length
	}{
		{"ASCII short", "Dev1", 10, 10},
		{"Turkish short", "Müşteri", 10, 10},
		{"Turkish exact", "Geliştirici", 11, 11},
		{"ASCII exact", "1234567890", 10, 10},
		{"Slice long if exceeding", "ÇokUzunMüşteriŞirketAdıAŞ", 10, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := padRight(tc.input, tc.targetWidth)
			gotLen := utf8.RuneCountInString(res)
			if gotLen != tc.wantLen {
				t.Errorf("padRight(%q, %d) rune length = %d; want %d", tc.input, tc.targetWidth, gotLen, tc.wantLen)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	longText := "Akansu Plastik ve Cam San. ve Tic. A.Ş."
	lines := wrapText(longText, 26)
	if len(lines) < 2 {
		t.Fatalf("expected wrapText to split %q into multiple lines, got %v", longText, lines)
	}

	for i, l := range lines {
		runeCount := utf8.RuneCountInString(l)
		if runeCount > 26 {
			t.Errorf("line %d (%s) exceeds maxWidth 26: length %d", i, l, runeCount)
		}
	}

	// Verify no text is lost when joined with spaces
	rejoined := strings.Join(lines, " ")
	if rejoined != longText {
		t.Errorf("wrapText lost content: rejoined = %q, want %q", rejoined, longText)
	}
}

func TestGetAlertSeverity(t *testing.T) {
	tests := []struct {
		alertType    string
		wantSeverity AlertSeverity
		wantEmoji    string
		wantDiscord  int
		wantHex      string
	}{
		{"BAĞLANTI KESİNTİSİ (DOWN)", SeverityDown, "🔴", 0xDC2626, "#DC2626"},
		{"HATA OLUŞTU", SeverityDown, "🔴", 0xDC2626, "#DC2626"},
		{"CİHAZ TEKRAR ÇEVRİMİÇİ (ONLINE)", SeverityRecover, "🟢", 0x16A34A, "#16A34A"},
		{"BAŞARIYLA GÜNCELLENDİ", SeverityRecover, "🟢", 0x16A34A, "#16A34A"},
		{"PLANLI BAKIM", SeverityWarning, "🟡", 0xF59E0B, "#F59E0B"},
		{"TEST BAĞLANTISI", SeverityInfo, "🔵", 0x2563EB, "#2563EB"},
	}

	for _, tc := range tests {
		t.Run(tc.alertType, func(t *testing.T) {
			sev, emoji, dColor, hexColor := GetAlertSeverity(tc.alertType)
			if sev != tc.wantSeverity || emoji != tc.wantEmoji || dColor != tc.wantDiscord || hexColor != tc.wantHex {
				t.Errorf("GetAlertSeverity(%q) = (%v, %v, %x, %v); want (%v, %v, %x, %v)",
					tc.alertType, sev, emoji, dColor, hexColor, tc.wantSeverity, tc.wantEmoji, tc.wantDiscord, tc.wantHex)
			}
		})
	}
}

func TestBuildAsciiTable(t *testing.T) {
	company := "Akansu Plastik ve Cam San. ve Tic. A.Ş."
	location := "Merkez Sistem Odası"
	device := "IBMSRV"
	ip := "10.38.1.155"
	status := "AJAN OTOMATİK BAĞLANDI"
	timestamp := "2026-10-15 18:04:14"
	detail := "Kayıtlı cihaz 'IBMSRV' yeni ajan anahtarı (v0.4.7) ile otomatik tescil edildi."

	table := BuildAsciiTable(company, location, device, ip, status, timestamp, detail)

	lines := strings.Split(table, "\n")
	if len(lines) < 8 {
		t.Fatalf("expected at least 8 lines in ASCII table, got %d", len(lines))
	}

	// Each line in the table should have the exact same visual rune length (42 runes)
	expectedRuneCount := 42
	for i, line := range lines {
		runeCount := utf8.RuneCountInString(line)
		if runeCount != expectedRuneCount {
			t.Errorf("line %d (%s) has rune length %d; want %d", i, line, runeCount, expectedRuneCount)
		}
	}

	// Verify required content is in table without truncation ellipses
	if strings.Contains(table, "...") {
		t.Errorf("table should not contain truncation ellipses: %s", table)
	}
	if !strings.Contains(table, "Parametre") || !strings.Contains(table, "Değer") {
		t.Errorf("table missing header: %s", table)
	}
	if !strings.Contains(table, "Akansu Plastik ve Cam San.") || !strings.Contains(table, "ve Tic. A.Ş.") {
		t.Errorf("table missing wrapped customer name: %s", table)
	}
}

func TestFormatTelegramAlert(t *testing.T) {
	company := "Akansu Plastik ve Cam San. ve Tic. A.Ş."
	location := "Merkez Sistem Odası"
	device := "IBMSRV"
	ip := "10.38.1.155"
	alertType := "BAĞLANTI KESİNTİSİ (DOWN)"
	detail := "Cihaz 3 periyot boyunca ping yanıtı vermedi & zaman aşımına uğradı <timeout>."
	timestamp := "2026-10-15 18:04:14"

	msg := FormatTelegramAlert(company, location, device, ip, alertType, detail, timestamp)

	// Verify title has [ALERT - GNNWATCHTOWER] and red circle emoji
	if !strings.Contains(msg, "<b>🔴 [ALERT - GNNWATCHTOWER] BAĞLANTI KESİNTİSİ (DOWN)</b>") {
		t.Errorf("telegram message missing expected title: %s", msg)
	}

	// Verify no redundant lines outside table
	if strings.Contains(msg, "<b>MÜŞTERİ:") || strings.Contains(msg, "<b>BİLDİRİM:") {
		t.Errorf("telegram message should not have redundant headers outside table: %s", msg)
	}

	// Verify <pre> block for ASCII table exists
	if !strings.Contains(msg, "<pre>") || !strings.Contains(msg, "</pre>") {
		t.Errorf("telegram message missing <pre> table block: %s", msg)
	}

	// Verify HTML escaping for detail inside table (& should be &amp; or text preserved safely)
	if strings.Contains(msg, "<timeout>") {
		t.Errorf("detail text was not properly HTML escaped: %s", msg)
	}
	if !strings.Contains(msg, "&lt;timeout&gt;") {
		t.Errorf("expected &lt;timeout&gt; in escaped table: %s", msg)
	}
}

func TestFormatDiscordAlert(t *testing.T) {
	company := "Boğaziçi Finans A.Ş."
	location := "İstanbul Veri Merkezi"
	device := "Edge-Firewall-01"
	ip := "172.16.0.1"
	alertType := "BAĞLANTI KESİNTİSİ (DOWN)"
	detail := "Firewall erişimi koptu"
	timestamp := "2026-10-15 16:45:00"

	jsonPayload := FormatDiscordAlert(company, location, device, ip, alertType, detail, timestamp)

	var payload struct {
		Content string `json:"content"`
		Embeds  []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Color       int    `json:"color"`
			Fields      []struct {
				Name   string `json:"name"`
				Value  string `json:"value"`
				Inline bool   `json:"inline"`
			} `json:"fields"`
			Footer struct {
				Text string `json:"text"`
			} `json:"footer"`
		} `json:"embeds"`
	}

	if err := json.Unmarshal([]byte(jsonPayload), &payload); err != nil {
		t.Fatalf("FormatDiscordAlert did not produce valid JSON: %v", err)
	}

	if !strings.Contains(payload.Content, "[ALERT - GNNWATCHTOWER]") {
		t.Errorf("content does not contain [ALERT - GNNWATCHTOWER]: %s", payload.Content)
	}

	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]
	// DOWN alert should map to Red color 0xDC2626
	if embed.Color != 0xDC2626 {
		t.Errorf("expected color 0xDC2626 for DOWN alert, got 0x%X", embed.Color)
	}

	// Verify title has [ALERT - GNNWATCHTOWER]
	if !strings.Contains(embed.Title, "🔴 [ALERT - GNNWATCHTOWER]") {
		t.Errorf("embed title missing [ALERT - GNNWATCHTOWER]: %s", embed.Title)
	}

	// Verify ASCII table exists inside description code block
	if !strings.Contains(embed.Description, "```") || !strings.Contains(embed.Description, "┌───────────┬────────────────────────────┐") {
		t.Errorf("embed description does not contain markdown table: %s", embed.Description)
	}

	// Test ONLINE recovery alert maps to Green color 0x16A34A
	greenPayloadJSON := FormatDiscordAlert(company, location, device, ip, "CİHAZ TEKRAR ÇEVRİMİÇİ (ONLINE)", "Erişim sağlandı", timestamp)
	var greenPayload struct {
		Embeds []struct {
			Color int `json:"color"`
		} `json:"embeds"`
	}
	_ = json.Unmarshal([]byte(greenPayloadJSON), &greenPayload)
	if len(greenPayload.Embeds) > 0 && greenPayload.Embeds[0].Color != 0x16A34A {
		t.Errorf("expected color 0x16A34A for ONLINE alert, got 0x%X", greenPayload.Embeds[0].Color)
	}
}

func TestFormatSlackAlert(t *testing.T) {
	company := "Boğaziçi Finans A.Ş."
	location := "İstanbul Veri Merkezi"
	device := "Edge-Firewall-01"
	ip := "172.16.0.1"
	alertType := "BAĞLANTI KESİNTİSİ (DOWN)"
	detail := "Firewall erişimi koptu"
	timestamp := "2026-10-15 16:45:00"

	jsonPayload := FormatSlackAlert(company, location, device, ip, alertType, detail, timestamp)

	var payload struct {
		Attachments []struct {
			Color  string `json:"color"`
			Title  string `json:"title"`
			Text   string `json:"text"`
			Fields []struct {
				Title string `json:"title"`
				Value string `json:"value"`
			} `json:"fields"`
			Footer string `json:"footer"`
		} `json:"attachments"`
	}

	if err := json.Unmarshal([]byte(jsonPayload), &payload); err != nil {
		t.Fatalf("FormatSlackAlert did not produce valid JSON: %v", err)
	}

	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}

	att := payload.Attachments[0]
	if att.Color != "#DC2626" {
		t.Errorf("expected color #DC2626 for Slack DOWN alert, got %s", att.Color)
	}
	if !strings.Contains(att.Title, "🔴 [ALERT - GNNWATCHTOWER]") {
		t.Errorf("attachment title missing [ALERT - GNNWATCHTOWER]: %s", att.Title)
	}
	if !strings.Contains(att.Text, "```") || !strings.Contains(att.Text, "┌───────────┬────────────────────────────┐") {
		t.Errorf("attachment text missing ASCII table: %s", att.Text)
	}
}
