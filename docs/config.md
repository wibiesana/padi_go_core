# ⚙️ Configuration & Environment Guide

`padi_go_core/config` loads `.env` variables and provides typed getters and environment checking helpers.

---

## 📄 Loading Configuration

```go
import "github.com/wibiesana/padi_go_core/config"

// Loads .env from current directory
cfg := config.Load()

// Or access globally loaded singleton
cfg := config.AppConfig
```

---

## 🔍 Typed Environment Getters

```go
// String with fallback
appName := config.GetEnv("APP_NAME", "PADI REST API")

// Integer with fallback
appPort := config.GetEnvInt("APP_PORT", 8080)

// Boolean with fallback
debugMode := config.GetEnvBool("APP_DEBUG", false)

// Float with fallback
rateLimit := config.GetEnvFloat("RATE_LIMIT_RATIO", 1.5)

// Duration with fallback
timeout := config.GetEnvDuration("REQUEST_TIMEOUT", 30*time.Second)
```

---

## 🌍 Environment State Checkers

```go
if config.IsProduction() {
    // Hide debug trace, enable strict security
}

if config.IsDevelopment() {
    // Enable live debug telemetry & verbose query logs
}

if config.IsTesting() {
    // Use test database fixtures
}
```
