package database

import (
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

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	return db, nil
}

// GetDB returns current active DB instance
func GetDB() *sql.DB {
	mu.RLock()
	if DB != nil {
		defer mu.RUnlock()
		return DB
	}
	mu.RUnlock()

	log.Println("Initializing database with default config...")
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}
	db, err := Connect(cfg)
	if err != nil {
		log.Fatalf("Fatal: could not initialize database: %v", err)
	}
	return db
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
