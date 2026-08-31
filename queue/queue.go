package queue

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/wibiesana/padi_go_core/database"
)

type JobRecord struct {
	ID          uint      `db:"id" json:"id"`
	Queue       string    `db:"queue" json:"queue"`
	Payload     string    `db:"payload" json:"payload"`
	Attempts    int       `db:"attempts" json:"attempts"`
	ReservedAt  *int64    `db:"reserved_at" json:"reserved_at"`
	AvailableAt int64     `db:"available_at" json:"available_at"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

func (JobRecord) TableName() string {
	return "jobs"
}

type JobHandler func(payload []byte) error

var handlersMu sync.RWMutex
var jobHandlers = make(map[string]JobHandler)

// RegisterJobHandler registers a worker handler for a specific job type name
func RegisterJobHandler(name string, handler JobHandler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	jobHandlers[name] = handler
}

func ensureQueueTable(db *sql.DB) error {
	driver := database.GetDriver()
	var createSQL string

	switch driver {
	case "postgres":
		createSQL = `CREATE TABLE IF NOT EXISTS jobs (
			id SERIAL PRIMARY KEY,
			queue VARCHAR(255) NOT NULL,
			payload TEXT NOT NULL,
			attempts INT DEFAULT 0,
			reserved_at BIGINT,
			available_at BIGINT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	case "mysql":
		createSQL = `CREATE TABLE IF NOT EXISTS jobs (
			id INT AUTO_INCREMENT PRIMARY KEY,
			queue VARCHAR(255) NOT NULL,
			payload LONGTEXT NOT NULL,
			attempts INT DEFAULT 0,
			reserved_at BIGINT,
			available_at BIGINT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_queue_avail (queue, available_at, reserved_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	default:
		createSQL = `CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			queue TEXT NOT NULL,
			payload TEXT NOT NULL,
			attempts INTEGER DEFAULT 0,
			reserved_at INTEGER,
			available_at INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	}

	_, err := db.Exec(createSQL)
	return err
}

// Push pushes a new job to the database queue
func Push(jobName string, data interface{}, queueName ...string) error {
	db := database.GetDB()
	_ = ensureQueueTable(db)

	qName := "default"
	if len(queueName) > 0 && queueName[0] != "" {
		qName = queueName[0]
	}

	payloadMap := map[string]interface{}{
		"job":  jobName,
		"data": data,
	}

	bytes, err := json.Marshal(payloadMap)
	if err != nil {
		return err
	}

	driver := database.GetDriver()
	insertSQL := "INSERT INTO jobs (queue, payload, available_at) VALUES (?, ?, ?)"
	if driver == "postgres" {
		insertSQL = "INSERT INTO jobs (queue, payload, available_at) VALUES ($1, $2, $3)"
	}

	_, err = db.Exec(insertSQL, qName, string(bytes), time.Now().Unix())
	return err
}

// Work starts processing jobs in the queue
func Work(queueName string, maxJobs int) {
	db := database.GetDB()
	_ = ensureQueueTable(db)

	if queueName == "" {
		queueName = "default"
	}

	log.Printf("👷 Queue worker listening on queue [%s]...", queueName)
	driver := database.GetDriver()

	processed := 0
	for {
		if maxJobs > 0 && processed >= maxJobs {
			log.Printf("Worker reached max jobs (%d). Stopping.", maxJobs)
			break
		}

		now := time.Now().Unix()
		var job JobRecord

		selectSQL := "SELECT id, queue, payload, attempts FROM jobs WHERE queue = ? AND available_at <= ? AND (reserved_at IS NULL OR reserved_at < ?) ORDER BY id ASC LIMIT 1"
		if driver == "postgres" {
			selectSQL = "SELECT id, queue, payload, attempts FROM jobs WHERE queue = $1 AND available_at <= $2 AND (reserved_at IS NULL OR reserved_at < $3) ORDER BY id ASC LIMIT 1"
		}

		err := db.QueryRow(selectSQL, queueName, now, now-300).Scan(&job.ID, &job.Queue, &job.Payload, &job.Attempts)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		// Reserve Job
		job.Attempts++
		updateSQL := "UPDATE jobs SET reserved_at = ?, attempts = ? WHERE id = ?"
		if driver == "postgres" {
			updateSQL = "UPDATE jobs SET reserved_at = $1, attempts = $2 WHERE id = $3"
		}
		_, _ = db.Exec(updateSQL, now, job.Attempts, job.ID)

		// Execute Job
		log.Printf("⏳ Processing Job #%d [Queue: %s]", job.ID, job.Queue)

		var payload struct {
			Job  string          `json:"job"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			log.Printf("❌ Job #%d payload error: %v", job.ID, err)
			deleteJob(db, job.ID)
			continue
		}

		handlersMu.RLock()
		handler, exists := jobHandlers[payload.Job]
		handlersMu.RUnlock()

		if !exists {
			log.Printf("⚠️ No registered handler for job [%s]. Deleting.", payload.Job)
			deleteJob(db, job.ID)
			continue
		}

		if err := handler(payload.Data); err != nil {
			log.Printf("❌ Job #%d failed: %v", job.ID, err)
			if job.Attempts >= 3 {
				log.Printf("💥 Job #%d exceeded max retries (3). Removing.", job.ID)
				deleteJob(db, job.ID)
			} else {
				retryAt := time.Now().Unix() + 30
				retrySQL := "UPDATE jobs SET reserved_at = NULL, available_at = ? WHERE id = ?"
				if driver == "postgres" {
					retrySQL = "UPDATE jobs SET reserved_at = NULL, available_at = $1 WHERE id = $2"
				}
				_, _ = db.Exec(retrySQL, retryAt, job.ID)
			}
		} else {
			log.Printf("✅ Job #%d completed successfully", job.ID)
			deleteJob(db, job.ID)
		}

		processed++
	}
}

func deleteJob(db *sql.DB, id uint) {
	driver := database.GetDriver()
	delSQL := "DELETE FROM jobs WHERE id = ?"
	if driver == "postgres" {
		delSQL = "DELETE FROM jobs WHERE id = $1"
	}
	_, _ = db.Exec(delSQL, id)
}
