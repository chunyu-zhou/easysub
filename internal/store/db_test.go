package store

import (
	"os"
	"path/filepath"
	"testing"

	"easysub/internal/config"
)

func TestEmbeddedSQLiteConfigAndOpen(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	dataDir := t.TempDir()
	config.C = &config.Config{DataDir: dataDir, SQLitePath: "easysub.db"}

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if c.Path != "easysub.db" {
		t.Fatalf("default path = %q, want easysub.db", c.Path)
	}

	g, err := Open(c)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	sqlDB, _ := g.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec("CREATE TABLE test_embedded_sqlite (id integer primary key)"); err != nil {
		t.Fatalf("write embedded database: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "easysub.db")); err != nil {
		t.Fatalf("database file was not created in DATA_DIR: %v", err)
	}
}

func TestLegacyRemoteConfigFallsBackToSQLite(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	dataDir := t.TempDir()
	config.C = &config.Config{DataDir: dataDir, SQLitePath: "local.db"}
	if err := os.WriteFile(filepath.Join(dataDir, "db_config.json"), []byte(`{"driver":"mysql","host":"remote"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if c.Path != "local.db" {
		t.Fatalf("legacy config path = %q, want local.db", c.Path)
	}
}
