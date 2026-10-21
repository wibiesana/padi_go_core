package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wibiesana/padi_go_core/config"
)

// Level represents log severity levels
type Level string

const (
	LevelInfo     Level = "INFO"
	LevelWarning  Level = "WARNING"
	LevelError    Level = "ERROR"
	LevelDebug    Level = "DEBUG"
	LevelCritical Level = "CRITICAL"
)

// Logger handles daily rotating file logging
type Logger struct {
	mu      sync.Mutex
	logDir  string
	appName string
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// GetLogger returns the singleton Logger instance
func GetLogger() *Logger {
	once.Do(func() {
		cfg := config.AppConfig
		if cfg == nil {
			cfg = config.Load()
		}

		appName := "app"
		if cfg != nil && cfg.AppName != "" {
			appName = strings.ToLower(strings.ReplaceAll(cfg.AppName, " ", "_"))
		}

		logDir := "storage/logs"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("[logger] failed to create log directory: %v", err)
		}

		defaultLogger = &Logger{
			logDir:  logDir,
			appName: appName,
		}

		// Occasional log rotation (~1% of process starts)
		if rand.Intn(100) == 0 { //nolint:gosec
			defaultLogger.rotateLogs()
		}
	})
	return defaultLogger
}

// write writes a formatted log line to the daily app log (and error log for ERROR/CRITICAL)
func (l *Logger) write(level Level, message string, context ...map[string]interface{}) {
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("2006-01-02 15:04:05")

	ctxStr := ""
	if len(context) > 0 && len(context[0]) > 0 {
		if b, err := json.Marshal(context[0]); err == nil {
			ctxStr = " " + string(b)
		}
	}

	line := fmt.Sprintf("[%s] %s: %s%s\n", timeStr, level, message, ctxStr)

	// Write to stdout as well for container/cloud environments
	log.Print(strings.TrimRight(line, "\n"))

	l.mu.Lock()
	defer l.mu.Unlock()

	// Daily app log: storage/logs/app-2026-08-31.log
	appLog := filepath.Join(l.logDir, "app-"+dateStr+".log")
	l.appendFile(appLog, line)

	// Also write to error log for ERROR and CRITICAL
	if level == LevelError || level == LevelCritical {
		errLog := filepath.Join(l.logDir, "error-"+dateStr+".log")
		l.appendFile(errLog, line)
	}
}

// appendFile safely appends a line to a log file using a temp file → rename strategy
func (l *Logger) appendFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

// rotateLogs removes log files older than 14 days
func (l *Logger) rotateLogs() {
	cutoff := time.Now().AddDate(0, 0, -14)

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(l.logDir, entry.Name()))
		}
	}
}

// ─── Package-level convenience functions ────────────────────────────────────

// Info logs an informational message
func Info(message string, context ...map[string]interface{}) {
	GetLogger().write(LevelInfo, message, context...)
}

// Infof logs a formatted informational message
func Infof(format string, args ...interface{}) {
	GetLogger().write(LevelInfo, fmt.Sprintf(format, args...))
}

// Warning logs a warning message
func Warning(message string, context ...map[string]interface{}) {
	GetLogger().write(LevelWarning, message, context...)
}

// Warn is an alias for Warning
func Warn(message string, context ...map[string]interface{}) {
	Warning(message, context...)
}

// Warningf logs a formatted warning message
func Warningf(format string, args ...interface{}) {
	GetLogger().write(LevelWarning, fmt.Sprintf(format, args...))
}

// Warnf is an alias for Warningf
func Warnf(format string, args ...interface{}) {
	Warningf(format, args...)
}

// Error logs an error message
func Error(message string, context ...map[string]interface{}) {
	GetLogger().write(LevelError, message, context...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	GetLogger().write(LevelError, fmt.Sprintf(format, args...))
}

// Debug logs a debug message
func Debug(message string, context ...map[string]interface{}) {
	GetLogger().write(LevelDebug, message, context...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	GetLogger().write(LevelDebug, fmt.Sprintf(format, args...))
}

// Critical logs a critical message
func Critical(message string, context ...map[string]interface{}) {
	GetLogger().write(LevelCritical, message, context...)
}

// Criticalf logs a formatted critical message
func Criticalf(format string, args ...interface{}) {
	GetLogger().write(LevelCritical, fmt.Sprintf(format, args...))
}

// RotateLogs triggers log rotation, removing log files older than retentionDays (default: 14)
func RotateLogs(retentionDays ...int) {
	days := 14
	if len(retentionDays) > 0 && retentionDays[0] > 0 {
		days = retentionDays[0]
	}
	l := GetLogger()
	cutoff := time.Now().AddDate(0, 0, -days)

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(l.logDir, entry.Name()))
		}
	}
}
