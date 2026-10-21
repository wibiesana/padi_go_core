package database_test

import (
	"database/sql"
	"testing"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
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

	// Test Exec, Query, QueryRow, Transaction
	_, err = database.Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Exec CREATE TABLE failed: %v", err)
	}

	_, err = database.Exec("INSERT INTO test_items (id, name) VALUES (?, ?)", 1, "First Item")
	if err != nil {
		t.Fatalf("Exec INSERT failed: %v", err)
	}

	var name string
	err = database.QueryRow("SELECT name FROM test_items WHERE id = ?", 1).Scan(&name)
	if err != nil || name != "First Item" {
		t.Fatalf("QueryRow failed: name=%s, err=%v", name, err)
	}

	rows, err := database.Query("SELECT id, name FROM test_items")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	rows.Close()

	// Test Transaction
	err = database.Transaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_items (id, name) VALUES (?, ?)", 2, "Second Item")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Test Ping and Stats
	if err := database.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	stats := database.Stats()
	if stats.OpenConnections < 0 {
		t.Fatalf("invalid stats")
	}
}
