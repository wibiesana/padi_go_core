package logger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/logger"
)

func TestLoggerWritingAndFiles(t *testing.T) {
	logDir := "storage/logs"
	defer os.RemoveAll(logDir)

	dateStr := time.Now().Format("2006-01-02")
	appLogFile := filepath.Join(logDir, "app-"+dateStr+".log")
	errorLogFile := filepath.Join(logDir, "error-"+dateStr+".log")

	// 1. Test Info Log
	testInfoMsg := "Test Info message for v0.0.2 testing"
	logger.Info(testInfoMsg, map[string]interface{}{"user_id": 42, "role": "admin"})

	// Check app log exists and has message
	content, err := os.ReadFile(appLogFile)
	if err != nil {
		t.Fatalf("Failed to read app log: %v", err)
	}
	if !strings.Contains(string(content), testInfoMsg) {
		t.Errorf("App log does not contain info message: %s", string(content))
	}
	if !strings.Contains(string(content), `"user_id":42`) {
		t.Errorf("App log does not contain structured context: %s", string(content))
	}

	// Error log should not exist yet or not have info message
	if _, err := os.Stat(errorLogFile); err == nil {
		errContent, _ := os.ReadFile(errorLogFile)
		if strings.Contains(string(errContent), testInfoMsg) {
			t.Errorf("Error log should NOT contain info message")
		}
	}

	// 2. Test Warning & Debug
	logger.Warning("Test Warning message")
	logger.Debug("Test Debug message")

	// 3. Test Error & Critical
	testErrMsg := "Test Error message occurred"
	testCritMsg := "Test Critical fatal incident"
	logger.Error(testErrMsg, map[string]interface{}{"error_code": 500})
	logger.Critical(testCritMsg)

	// Check error log
	errContent, err := os.ReadFile(errorLogFile)
	if err != nil {
		t.Fatalf("Failed to read error log file: %v", err)
	}
	if !strings.Contains(string(errContent), testErrMsg) {
		t.Errorf("Error log missing error message: %s", string(errContent))
	}
	if !strings.Contains(string(errContent), testCritMsg) {
		t.Errorf("Error log missing critical message: %s", string(errContent))
	}
	if !strings.Contains(string(errContent), `"error_code":500`) {
		t.Errorf("Error log missing structured context: %s", string(errContent))
	}

	// 4. Test formatted loggers and aliases
	logger.Infof("Formatted info: %d users", 100)
	logger.Warn("Warning alias test")
	logger.Warnf("Formatted warning: %s", "high memory")
	logger.Errorf("Formatted error: %v", "timeout")
	logger.Debugf("Formatted debug: %t", true)
	logger.Criticalf("Formatted critical: %d", 999)

	// 5. Test RotateLogs
	logger.RotateLogs(14)
}
