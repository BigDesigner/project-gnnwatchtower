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
		{"Truncate with ellipsis", "ÇokUzunMüşteriŞirketAdıAŞ", 10, 10},
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

func TestBuildAsciiTable(t *testing.T) {
	company := "Örnek Teknoloji A.Ş."
	device := "İç-Switch-01"
	ip := "192.168.1.100"
	status := "KESİNTİ (DOWN)"
	timestamp := "2026-10-15 16:45:00"

	table := BuildAsciiTable(company, device, ip, status, timestamp)

	lines := strings.Split(table, "\n")
	if len(lines) != 9 {
		t.Fatalf("expected 9 lines in ASCII table, got %d", len(lines))
	}

	// Each line in the table should have the exact same visual rune length (36 runes)
	expectedRuneCount := 36
	for i, line := range lines {
		runeCount := utf8.RuneCountInString(line)
		if runeCount != expectedRuneCount {
			t.Errorf("line %d (%s) has rune length %d; want %d", i, line, runeCount, expectedRuneCount)
		}
	}

	// Verify required content is in table
	if !strings.Contains(table, "Parametre") || !strings.Contains(table, "Değer") {
		t.Errorf("table missing header: %s", table)
	}
}

func TestFormatTelegramAlert(t *testing.T) {
	company := "Akdeniz Lojistik Ltd. Şti."
	location := "Antalya Şubesi"
	device := "Router-Core"
	ip := "10.10.1.1"
	alertType := "BAĞLANTI KESİNTİSİ (DOWN)"
	detail := "Cihaz 3 periyot boyunca ping yanıtı vermedi & zaman aşımına uğradı <timeout>."
	timestamp := "2026-10-15 16:45:00"

	msg := FormatTelegramAlert(company, location, device, ip, alertType, detail, timestamp)

	// Verify company name is prominent
	if !strings.Contains(msg, "<b>MÜŞTERİ: Akdeniz Lojistik Ltd. Şti.</b>") {
		t.Errorf("telegram message missing bold customer name: %s", msg)
	}
	if !strings.Contains(msg, "(Antalya Şubesi)") {
		t.Errorf("telegram message missing location: %s", msg)
	}

	// Verify <pre> block for ASCII table exists
	if !strings.Contains(msg, "<pre>") || !strings.Contains(msg, "</pre>") {
		t.Errorf("telegram message missing <pre> table block: %s", msg)
	}

	// Verify HTML escaping for detail text (<timeout> should be escaped to &lt;timeout&gt;)
	if strings.Contains(msg, "<timeout>") {
		t.Errorf("detail text was not properly HTML escaped: %s", msg)
	}
	if !strings.Contains(msg, "&lt;timeout&gt;") {
		t.Errorf("expected &lt;timeout&gt; in detail: %s", msg)
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

	if !strings.Contains(payload.Content, company) {
		t.Errorf("content does not contain company: %s", payload.Content)
	}

	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]
	// DOWN alert should map to Red color 0xDC2626
	if embed.Color != 0xDC2626 {
		t.Errorf("expected color 0xDC2626 for DOWN alert, got 0x%X", embed.Color)
	}

	// Verify ASCII table exists inside description code block
	if !strings.Contains(embed.Description, "```") || !strings.Contains(embed.Description, "┌───────────┬──────────────────────┐") {
		t.Errorf("embed description does not contain markdown table: %s", embed.Description)
	}

	// Verify fields
	foundCompany := false
	for _, f := range embed.Fields {
		if strings.Contains(f.Name, "Müşteri Şirketi") && strings.Contains(f.Value, company) {
			foundCompany = true
			break
		}
	}
	if !foundCompany {
		t.Errorf("fields did not contain customer company: %+v", embed.Fields)
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
