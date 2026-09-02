# 🗃️ Database Migrations Guide

`padi_go_core/migrator` provides automated schema migrations with batch tracking, multi-driver support, and rollback functionality.

---

## 📝 Defining & Registering Migrations

In `database/migrations/`:

```go
package migrations

import (
	"database/sql"
	"github.com/wibiesana/padi_go_core/migrator"
)

func init() {
	migrator.Register(
		"2026_01_01_000001_create_users_table",
		// Up
		func(db *sql.DB) error {
			sqlScript := `
				CREATE TABLE IF NOT EXISTS users (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name VARCHAR(255) NOT NULL,
					email VARCHAR(255) UNIQUE NOT NULL,
					password VARCHAR(255) NOT NULL,
					role VARCHAR(50) DEFAULT 'user',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);
			`
			return migrator.ExecHelper(db, sqlScript)
		},
		// Down
		func(db *sql.DB) error {
			return migrator.ExecHelper(db, "DROP TABLE IF EXISTS users;")
		},
	)
}
```

---

## 🚀 Migration CLI Commands

```go
db := database.GetDB()

// 1. Run all pending migrations
err := migrator.RunPending(db)

// 2. Rollback the last migration batch
err = migrator.RollbackLast(db)

// 3. Rollback all migrations (Reset)
err = migrator.Reset(db)

// 4. Drop all and re-run all (Fresh)
err = migrator.Fresh(db)

// 5. Inspect migration statuses
statuses, err := migrator.Status(db)
for _, s := range statuses {
    fmt.Printf("[%t] %s (Batch: %d)\n", s.Ran, s.Name, s.Batch)
}
```
