package migrator

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/wibiesana/padi_go_core/database"
)

type Migration struct {
	Name string
	Up   func(db *sql.DB) error
	Down func(db *sql.DB) error
}

var registry []Migration

// Register registers a new migration definition
func Register(name string, up func(db *sql.DB) error, down func(db *sql.DB) error) {
	registry = append(registry, Migration{
		Name: name,
		Up:   up,
		Down: down,
	})
}

// EnsureTable ensures migrations tracking table exists
func EnsureTable(db *sql.DB) error {
	driver := database.GetDriver()
	var createSQL string

	if driver == "postgres" {
		createSQL = `CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			migration VARCHAR(255) UNIQUE NOT NULL,
			batch INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else if driver == "mysql" {
		createSQL = `CREATE TABLE IF NOT EXISTS migrations (
			id INT AUTO_INCREMENT PRIMARY KEY,
			migration VARCHAR(255) UNIQUE NOT NULL,
			batch INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		createSQL = `CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			migration TEXT UNIQUE NOT NULL,
			batch INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	}

	_, err := db.Exec(createSQL)
	return err
}

// RunPending executes all pending migrations
func RunPending(db *sql.DB) error {
	if err := EnsureTable(db); err != nil {
		return err
	}

	rows, err := db.Query("SELECT migration, batch FROM migrations")
	if err != nil {
		return err
	}
	defer rows.Close()

	executedMap := make(map[string]bool)
	maxBatch := 0
	for rows.Next() {
		var migName string
		var batch int
		if err := rows.Scan(&migName, &batch); err == nil {
			executedMap[migName] = true
			if batch > maxBatch {
				maxBatch = batch
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	nextBatch := maxBatch + 1
	ranCount := 0
	driver := database.GetDriver()

	for _, mig := range registry {
		if !executedMap[mig.Name] {
			log.Printf("Migrating: %s", mig.Name)
			if err := mig.Up(db); err != nil {
				return fmt.Errorf("migration %s failed: %w", mig.Name, err)
			}

			insertSQL := "INSERT INTO migrations (migration, batch) VALUES (?, ?)"
			if driver == "postgres" {
				insertSQL = "INSERT INTO migrations (migration, batch) VALUES ($1, $2)"
			}

			if _, err := db.Exec(insertSQL, mig.Name, nextBatch); err != nil {
				return fmt.Errorf("failed to record migration %s: %w", mig.Name, err)
			}
			log.Printf("Migrated:  %s", mig.Name)
			ranCount++
		}
	}

	if ranCount == 0 {
		log.Println("Nothing to migrate.")
	} else {
		log.Printf("Successfully executed %d migration(s).", ranCount)
	}

	return nil
}

// RollbackLast rolls back the latest batch of migrations
func RollbackLast(db *sql.DB) error {
	if err := EnsureTable(db); err != nil {
		return err
	}

	var targetBatch int
	err := db.QueryRow("SELECT COALESCE(MAX(batch), 0) FROM migrations").Scan(&targetBatch)
	if err != nil || targetBatch == 0 {
		log.Println("No migrations to rollback.")
		return nil
	}

	driver := database.GetDriver()
	selectSQL := "SELECT migration FROM migrations WHERE batch = ? ORDER BY id DESC"
	deleteSQL := "DELETE FROM migrations WHERE migration = ?"

	if driver == "postgres" {
		selectSQL = "SELECT migration FROM migrations WHERE batch = $1 ORDER BY id DESC"
		deleteSQL = "DELETE FROM migrations WHERE migration = $1"
	}

	rows, err := db.Query(selectSQL, targetBatch)
	if err != nil {
		return err
	}
	defer rows.Close()

	var batchMigs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			batchMigs = append(batchMigs, name)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	migLookup := make(map[string]Migration)
	for _, m := range registry {
		migLookup[m.Name] = m
	}

	for _, migName := range batchMigs {
		if mig, exists := migLookup[migName]; exists && mig.Down != nil {
			log.Printf("Rolling back: %s", migName)
			if err := mig.Down(db); err != nil {
				return fmt.Errorf("rollback of %s failed: %w", migName, err)
			}
			if _, err := db.Exec(deleteSQL, migName); err != nil {
				return fmt.Errorf("failed to delete migration record %s: %w", migName, err)
			}
			log.Printf("Rolled back:  %s", migName)
		}
	}

	return nil
}

// ExecHelper executes multiple SQL statements separated by semicolon
func ExecHelper(db *sql.DB, sqlStatements string) error {
	queries := strings.Split(sqlStatements, ";")
	for _, q := range queries {
		trimmed := strings.TrimSpace(q)
		if trimmed != "" {
			if _, err := db.Exec(trimmed); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrationStatus describes the execution state of a registered migration
type MigrationStatus struct {
	Name     string
	Ran      bool
	Batch    int
}

// Status returns the current status of all registered migrations
func Status(db *sql.DB) ([]MigrationStatus, error) {
	if err := EnsureTable(db); err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT migration, batch FROM migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executedMap := make(map[string]int)
	for rows.Next() {
		var migName string
		var batch int
		if err := rows.Scan(&migName, &batch); err == nil {
			executedMap[migName] = batch
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var statuses []MigrationStatus
	for _, mig := range registry {
		batch, ran := executedMap[mig.Name]
		statuses = append(statuses, MigrationStatus{
			Name:  mig.Name,
			Ran:   ran,
			Batch: batch,
		})
	}
	return statuses, nil
}

// Reset rolls back all executed migrations
func Reset(db *sql.DB) error {
	for {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
		if err != nil || count == 0 {
			break
		}
		if err := RollbackLast(db); err != nil {
			return err
		}
	}
	return nil
}

// Fresh resets all migrations and re-runs them from the beginning
func Fresh(db *sql.DB) error {
	if err := Reset(db); err != nil {
		return err
	}
	return RunPending(db)
}

// ClearRegistry clears in-memory migration registry (useful for testing)
func ClearRegistry() {
	registry = nil
}
