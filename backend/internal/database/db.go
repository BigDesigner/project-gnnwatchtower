package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB initializes SQLite database with WAL mode and runs table migrations
func InitDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		dbPath = getEnv("DB_PATH", "./data/gnnwatchtower.db")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Explicitly configure SQLite PRAGMAs for high-concurrency WAL mode
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 10000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = -20000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("[GNNwatchtower DB] Pragma warning (%s): %v", pragma, err)
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// WAL mode enables concurrent readers while writers wait with busy_timeout.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	DB = db
	log.Printf("[GNNwatchtower DB] SQLite initialized at %s (High-Concurrency WAL Mode)", dbPath)

	if err := migrateSchema(db); err != nil {
		return nil, fmt.Errorf("failed running schema migration: %w", err)
	}

	return db, nil
}

func migrateSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT DEFAULT 'admin',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		ip_address TEXT UNIQUE NOT NULL,
		mac_address TEXT,
		device_type TEXT DEFAULT 'other',
		os_version TEXT,
		purpose TEXT,
		location TEXT,
		operator_notes TEXT,
		is_agent_installed INTEGER DEFAULT 0,
		agent_token TEXT UNIQUE,
		agent_version TEXT DEFAULT '0.4.0',
		agent_upgrade_status TEXT DEFAULT 'none',
		agent_upgrade_target TEXT DEFAULT '',
		agent_upgrade_error TEXT DEFAULT '',
		agent_last_upgrade_at DATETIME,
		machine_uuid TEXT DEFAULT '',
		monitoring_profile TEXT DEFAULT 'active',
		check_interval_seconds INTEGER DEFAULT 15,
		status TEXT DEFAULT 'unknown',
		last_seen_at DATETIME,
		last_status_change_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		response_time_ms INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS services_monitored (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		check_type TEXT DEFAULT 'tcp_port',
		target_port INTEGER,
		target_path TEXT,
		expected_status_code INTEGER DEFAULT 200,
		status TEXT DEFAULT 'unknown',
		response_time_ms INTEGER DEFAULT 0,
		last_check_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS downtime_incidents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		started_at DATETIME NOT NULL,
		resolved_at DATETIME,
		duration_seconds INTEGER,
		classified_cause TEXT DEFAULT 'unclassified',
		auto_detection_hypothesis TEXT,
		operator_confirmed INTEGER DEFAULT 0,
		operator_notes TEXT,
		classified_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		cpu_usage_pct REAL,
		ram_usage_pct REAL,
		disk_usage_pct REAL,
		temperature_celsius REAL,
		net_rx_kbps REAL DEFAULT 0,
		net_tx_kbps REAL DEFAULT 0,
		recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS alert_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		config_json TEXT NOT NULL DEFAULT '{}',
		is_enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		description TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Default configuration values for MSP deployment
	INSERT OR IGNORE INTO system_settings (key, value, description) VALUES
	('client_company_name', 'Müşteri Altyapısı', 'Müşteri şirket adı'),
	('client_location', 'Merkez Sistem Odası', 'Müşteri lokasyonu veya şube'),
	('scan_subnet', '192.168.1.0/24', 'Varsayılan yerel ağ tarama aralığı'),
	('scan_interval_seconds', '15', 'Aktif cihazlar için genel izleme periyodu (saniye)'),
	('server_port', '3800', 'GNNwatchtower web ve servis portu'),
	('power_outage_threshold_count', '3', 'Toplu kesintide elektrik kesintisi varsayılacak eşik cihaz sayısı'),
	('telegram_bot_token', '', 'Telegram bildirim bot anahtarı'),
	('telegram_chat_id', '', 'Telegram bildirim sohbet/kanal ID'),
	('telegram_enabled', 'false', 'Telegram bildirimleri aktif mi'),
	('discord_webhook_url', '', 'Discord kanal webhook bağlantısı'),
	('discord_enabled', 'false', 'Discord bildirimleri aktif mi'),
	('brevo_api_key', '', 'Brevo (Sendinblue) API Anahtarı'),
	('brevo_sender_email', 'alerts@gnnwatchtower.local', 'Brevo onaylı gönderici e-posta adresi'),
	('brevo_sender_name', 'GNNwatchtower Altyapı Bildirimi', 'Brevo gönderici adı'),
	('brevo_recipient_email', '', 'Bildirimlerin gönderileceği e-posta adresi'),
	('brevo_enabled', 'false', 'E-posta bildirimleri aktif mi'),
	('agent_latest_version', '0.4.0', 'En son derlenmiş resmi GNNwatchtower ajan sürümü'),
	('agent_sha256_windows', '', 'Windows ajan ikili dosyasının resmi SHA256 özeti'),
	('agent_sha256_linux', '', 'Linux ajan ikili dosyasının resmi SHA256 özeti');
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Idempotent column migrations for existing databases
	_, _ = db.Exec("ALTER TABLE system_metrics ADD COLUMN net_rx_kbps REAL DEFAULT 0")
	_, _ = db.Exec("ALTER TABLE system_metrics ADD COLUMN net_tx_kbps REAL DEFAULT 0")

	// Idempotent column migrations for devices
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN agent_version TEXT DEFAULT '0.4.0'")
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN agent_upgrade_status TEXT DEFAULT 'none'")
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN agent_upgrade_target TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN agent_upgrade_error TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN agent_last_upgrade_at DATETIME")
	_, _ = db.Exec("ALTER TABLE devices ADD COLUMN machine_uuid TEXT DEFAULT ''")

	// Create default admin user if no users exist (admin / admin)
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		hash := HashPassword("admin")
		_, _ = db.Exec("INSERT INTO users (username, password_hash, role) VALUES ('admin', $1, 'admin')", hash)
		log.Printf("[GNNwatchtower Auth] Default admin account created (username: admin, password: admin)")
	}

	return nil
}

// HashPassword calculates SHA-256 hash
func HashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password + "_gnnwatchtower_salt_2026"))
	return hex.EncodeToString(h.Sum(nil))
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
