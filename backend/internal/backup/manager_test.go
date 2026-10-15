package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
)

func TestEnsureBackupPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gnn-backup-perm-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backupDir := filepath.Join(tmpDir, "backups")
	// Create with permissive 0755 directory mode
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("failed creating backup dir: %v", err)
	}
	_ = os.Chmod(backupDir, 0755)

	// Create a dummy archive file with permissive 0644 file mode
	dummyArchive := filepath.Join(backupDir, "archive.json")
	if err := os.WriteFile(dummyArchive, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed writing dummy archive: %v", err)
	}
	_ = os.Chmod(dummyArchive, 0644)

	// Execute migration
	if err := EnsureBackupPermissions(backupDir); err != nil {
		t.Fatalf("EnsureBackupPermissions failed: %v", err)
	}

	// Verify directory mode migrated to 0700
	dirFi, err := os.Stat(backupDir)
	if err != nil {
		t.Fatalf("failed to stat backup dir: %v", err)
	}
	if dirFi.Mode().Perm() != 0700 {
		t.Fatalf("expected directory perm 0700, got %o", dirFi.Mode().Perm())
	}

	// Verify file mode migrated to 0600
	fileFi, err := os.Stat(dummyArchive)
	if err != nil {
		t.Fatalf("failed to stat archive file: %v", err)
	}
	if fileFi.Mode().Perm() != 0600 {
		t.Fatalf("expected archive perm 0600, got %o", fileFi.Mode().Perm())
	}
}

func TestImportAllRejectsUnsafeEnrollmentKey(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Malicious bundle containing shell subexpression in agent_enrollment_key
	maliciousBundle := BackupBundle{
		Version: "0.4.3",
		Settings: []models.SystemSetting{
			{
				Key:         "agent_enrollment_key",
				Value:       "malicious-key; $(rm -rf /)",
				Description: "Injected key",
			},
		},
	}

	data, err := json.Marshal(maliciousBundle)
	if err != nil {
		t.Fatalf("failed to marshal bundle: %v", err)
	}

	err = ImportAll(data)
	if err == nil {
		t.Fatalf("expected ImportAll to reject unsafe enrollment key, but it succeeded")
	}

	if !strings.Contains(err.Error(), "güvenlik ihlali") {
		t.Fatalf("expected security error, got %v", err)
	}
}
