# 📜 Structured Rotating Logger Guide

`padi_go_core/logger` provides dual-output (stdout + daily files in `storage/logs/`) structured logging with 14-day auto-retention and JSON context support.

---

## 📝 Logging Methods

```go
import "github.com/wibiesana/padi_go_core/logger"

// 1. Simple String Messages
logger.Info("Server started on port 8080")
logger.Warn("High memory usage detected")
logger.Error("Database connection lost")

// 2. Structured Messages with JSON Context
logger.Info("User logged in", map[string]interface{}{
    "user_id": 42,
    "ip":      "192.168.1.1",
    "device":  "Chrome/Mac",
})

logger.Error("Payment gateway failed", map[string]interface{}{
    "order_id": 1092,
    "gateway":  "Stripe",
    "error":    "card_declined",
})

// 3. Printf-Style Formatted Logging
logger.Infof("Processing order #%d for customer %s", 102, "Alice")
logger.Warnf("Queue backlog reached %d jobs", 500)
logger.Errorf("Failed to render invoice for user %d: %v", 42, err)
```

---

## 🔄 Daily Log File Rotation

Logs are automatically organized daily:
- `storage/logs/app-YYYY-MM-DD.log` (All levels)
- `storage/logs/error-YYYY-MM-DD.log` (Errors & Panics only)

To trigger log cleanup manually:
```go
// Deletes logs older than 14 days
logger.RotateLogs(14)
```
