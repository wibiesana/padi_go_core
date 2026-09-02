package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wibiesana/padi_go_core/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	DB     *sql.DB
	driver string
	mu     sync.RWMutex
)

// QueryLog represents an executed SQL query with telemetry
type QueryLog struct {
	Query      string        `json:"query"`
	Bindings   []interface{} `json:"bindings,omitempty"`
	Time       string        `json:"time"`
	DurationMs float64       `json:"duration_ms"`
}

type trackerContextKey string

const QueryTrackerKey trackerContextKey = "padi_query_tracker"

// QueryTracker collects executed SQL queries during a request lifecycle
type QueryTracker struct {
	mu      sync.Mutex
	queries []QueryLog
}

// NewQueryTracker creates a new query collector
func NewQueryTracker() *QueryTracker {
	return &QueryTracker{
		queries: make([]QueryLog, 0),
	}
}

// Add appends an executed query to the tracker
func (t *QueryTracker) Add(queryStr string, bindings []interface{}, duration time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	ms := float64(duration.Microseconds()) / 1000.0
	t.queries = append(t.queries, QueryLog{
		Query:      queryStr,
		Bindings:   bindings,
		Time:       fmt.Sprintf("%.2f ms", ms),
		DurationMs: ms,
	})
}

// Queries returns all recorded query logs
func (t *QueryTracker) Queries() []QueryLog {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	res := make([]QueryLog, len(t.queries))
	copy(res, t.queries)
	return res
}

// TrackQuery records a query into the context's QueryTracker if available
func TrackQuery(ctx interface{}, sqlStr string, args []interface{}, duration time.Duration) {
	if ctx == nil {
		return
	}
	if c, ok := ctx.(interface{ Value(key any) any }); ok {
		if t, ok := c.Value(QueryTrackerKey).(*QueryTracker); ok && t != nil {
			t.Add(sqlStr, args, duration)
		}
	}
}

// Connect initializes the primary database connection using native database/sql
func Connect(cfg *config.Config) (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	drv := strings.ToLower(cfg.DBConnection)
	var dsn string

	switch drv {
	case "mysql":
		driver = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			cfg.DBUsername,
			cfg.DBPassword,
			cfg.DBHost,
			cfg.DBPort,
			cfg.DBDatabase,
		)

	case "pgsql", "postgres", "postgresql":
		driver = "postgres"
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			cfg.DBHost,
			cfg.DBUsername,
			cfg.DBPassword,
			cfg.DBDatabase,
			cfg.DBPort,
		)

	case "sqlite", "sqlite3":
		driver = "sqlite"
		dbPath := cfg.DBDatabase
		if dbPath == "" {
			dbPath = "database/database.sqlite"
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create sqlite dir: %w", err)
		}
		dsn = dbPath

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", drv)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	return db, nil
}

// GetDB returns current active DB instance, initializing from config if not yet connected.
// Safe for concurrent use.
func GetDB() *sql.DB {
	mu.RLock()
	db := DB
	mu.RUnlock()
	if db != nil {
		return db
	}

	log.Println("Initializing database with default config...")
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}
	connected, err := Connect(cfg)
	if err != nil {
		log.Fatalf("Fatal: could not initialize database: %v", err)
	}
	return connected
}

// GetDriver returns the active database driver (sqlite, mysql, postgres)
func GetDriver() string {
	if driver == "" {
		cfg := config.AppConfig
		if cfg != nil {
			return strings.ToLower(cfg.DBConnection)
		}
		return "sqlite"
	}
	return driver
}

// Transaction executes operations within a database transaction with automatic commit/rollback
func Transaction(fn func(tx *sql.Tx) error) error {
	return TransactionContext(context.Background(), fn)
}

// TransactionContext executes operations within a transaction with context and automatic commit/rollback
func TransactionContext(ctx context.Context, fn func(tx *sql.Tx) error) error {
	db := GetDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Exec executes a raw SQL query with query telemetry tracking
func Exec(query string, args ...interface{}) (sql.Result, error) {
	return ExecContext(context.Background(), query, args...)
}

// ExecContext executes a raw SQL query with context and query telemetry tracking
func ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	db := GetDB()
	start := time.Now()
	res, err := db.ExecContext(ctx, query, args...)
	TrackQuery(ctx, query, args, time.Since(start))
	return res, err
}

// Query executes a query returning rows with query telemetry tracking
func Query(query string, args ...interface{}) (*sql.Rows, error) {
	return QueryContext(context.Background(), query, args...)
}

// QueryContext executes a query returning rows with context and query telemetry tracking
func QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	db := GetDB()
	start := time.Now()
	rows, err := db.QueryContext(ctx, query, args...)
	TrackQuery(ctx, query, args, time.Since(start))
	return rows, err
}

// QueryRow executes a query returning a single row with query telemetry tracking
func QueryRow(query string, args ...interface{}) *sql.Row {
	return QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext executes a query returning a single row with context and query telemetry tracking
func QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	db := GetDB()
	start := time.Now()
	row := db.QueryRowContext(ctx, query, args...)
	TrackQuery(ctx, query, args, time.Since(start))
	return row
}

// Ping checks if the database connection is alive
func Ping() error {
	return GetDB().Ping()
}

// Close gracefully closes the primary database connection
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if DB != nil {
		err := DB.Close()
		DB = nil
		return err
	}
	return nil
}

// Stats returns database connection pool statistics
func Stats() sql.DBStats {
	return GetDB().Stats()
}
