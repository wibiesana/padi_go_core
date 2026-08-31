package migrator_test

import (
	"database/sql"
	"testing"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
	"github.com/wibiesana/padi-core/migrator"
)

func TestMigratorRunAndRollback(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:migrator_test_mem?mode=memory&cache=shared",
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Register test migration
	migrator.Register(
		"2026_01_01_000001_create_sample_table",
		func(db *sql.DB) error {
			_, err := db.Exec("CREATE TABLE sample_data (id INTEGER PRIMARY KEY, title TEXT);")
			return err
		},
		func(db *sql.DB) error {
			_, err := db.Exec("DROP TABLE IF EXISTS sample_data;")
			return err
		},
	)

	// 1. Run migrations
	if err := migrator.RunPending(db); err != nil {
		t.Fatalf("RunPending failed: %v", err)
	}

	// Verify table exists
	_, err = db.Exec("INSERT INTO sample_data (title) VALUES ('Sample 1');")
	if err != nil {
		t.Fatalf("failed to insert into sample_data after migration: %v", err)
	}

	// 2. Rollback
	if err := migrator.RollbackLast(db); err != nil {
		t.Fatalf("RollbackLast failed: %v", err)
	}

	// Verify table was dropped
	_, err = db.Exec("SELECT COUNT(*) FROM sample_data;")
	if err == nil {
		t.Fatalf("expected sample_data table to be dropped after rollback")
	}
}
