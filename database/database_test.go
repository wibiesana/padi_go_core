package database_test

import (
	"testing"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/database"
)

func TestDatabaseConnectAndGetDriver(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:db_test_mem?mode=memory&cache=shared",
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to sqlite in-memory db: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("db ping failed: %v", err)
	}

	drv := database.GetDriver()
	if drv != "sqlite" {
		t.Fatalf("expected driver sqlite, got '%s'", drv)
	}

	activeDB := database.GetDB()
	if activeDB == nil {
		t.Fatalf("expected non-nil active DB")
	}

	// Test unsupported driver
	badCfg := &config.Config{
		DBConnection: "oracle",
	}
	_, err = database.Connect(badCfg)
	if err == nil {
		t.Fatalf("expected error on unsupported driver")
	}
}
