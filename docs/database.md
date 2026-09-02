# 🗄️ Database & Transactions Guide

`padi_go_core/database` provides multi-driver connection pool management (MySQL, PostgreSQL, pure-Go SQLite), transactions with automatic commit/rollback, raw SQL query execution, and query telemetry tracking.

---

## 🔌 Connection Setup

```go
cfg := config.Load()
db, err := database.Connect(cfg)
if err != nil {
    log.Fatalf("Database connection failed: %v", err)
}
```

---

## 🔄 Automatic Transactions (`Transaction`)

Executes database operations inside a transaction.
- If the callback returns `nil`, the transaction **automatically commits**.
- If the callback returns an error or encounters a panic, it **automatically rolls back**.

```go
err := database.Transaction(func(tx *sql.Tx) error {
    _, err := tx.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", 500, senderID)
    if err != nil {
        return err // Auto rollback
    }

    _, err = tx.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", 500, receiverID)
    if err != nil {
        return err // Auto rollback
    }

    return nil // Auto commit
})
```

---

## ⚡ Raw SQL Execution with Query Telemetry

Every query executed via these functions is automatically tracked in the request's `QueryTracker` for debug/latency logs:

```go
// 1. Exec
res, err := database.Exec("UPDATE users SET status = ? WHERE last_login < ?", "inactive", cutoffDate)
affected, _ := res.RowsAffected()

// 2. Query multiple rows
rows, err := database.Query("SELECT id, name FROM users WHERE role = ?", "admin")
if err == nil {
    defer rows.Close()
    for rows.Next() {
        var id uint
        var name string
        _ = rows.Scan(&id, &name)
    }
}

// 3. Query single row
var totalUsers int64
err := database.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
```

---

## 🩺 Health Check & Monitoring

```go
// Ping database
if err := database.Ping(); err == nil {
    fmt.Println("Database is reachable")
}

// Connection Pool Statistics
stats := database.Stats()
fmt.Printf("Active Connections: %d, Idle: %d, MaxOpen: %d\n", stats.InUse, stats.Idle, stats.MaxOpenConnections)
```
