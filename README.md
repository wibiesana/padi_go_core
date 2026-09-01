<div align="center">

# 🌾 Padi Core Engine (Go)

**The pure Go, zero-bloat, high-performance core engine for Padi REST API Framework**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/wibiesana/padi_go_core.svg)](https://pkg.go.dev/github.com/wibiesana/padi_go_core)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](#)

</div>

`padi_go_core` is an independent, zero-external-ORM core engine that provides a fast, resilient, and structured architecture for building enterprise-grade REST APIs in Go.

---

## 🚀 Installation

To use `padi_go_core` in your Go project:

```bash
go get github.com/wibiesana/padi_go_core@latest
```

---

## 📦 Core Packages

The engine provides a cohesive suite of packages:

| Package | Description |
| :--- | :--- |
| **`activerecord`** | Reflection-powered native ActiveRecord engine for CRUD operations (`Find`, `Save`, `Delete`, `First`). |
| **`auth`** | JWT Token management (generation & verification) and Bcrypt password hashing. |
| **`cache`** | Multi-tier L1 (Memory) + L2 (Redis / File) caching with atomic writes, numeric incr/decr, `Has`, `DeleteMany`, and `Remember`. |
| **`config`** | `.env`-driven configuration loader and typed environment variable helpers. |
| **`database`** | Multi-driver connection pooling (`sqlite` pure-Go, `mysql`, `postgres`). |
| **`email`** | SMTP mailer with SSL/TLS support and RFC-compliant validation. |
| **`file` / `storage`** | Secure file uploads with MIME content sniffing, path traversal defense, and `URLOrNull` helpers. |
| **`generator`** | Signature scaffolding generator for Base Models, Custom Models, Resources, Controllers, Routes, and Postman Collections. |
| **`logger`** | Daily rotating structured file logger (`storage/logs/app-*.log`, `error-*.log`) with 14-day retention and JSON context. |
| **`middleware`** | HTTP middleware suite: `AuthRequired`, `CORS`, `Logger`, `RateLimiter`, and `Recoverer`. |
| **`migrator`** | Migration registry, schema runner, and batch rollback system. |
| **`model`** | Base model interfaces and ActiveRecord lifecycle hooks. |
| **`query`** | Native Fluent Query Builder (`Where`, `Join`, `OrderBy`, `GroupBy`, `Paginate`, etc.). |
| **`queue`** | Database-backed asynchronous background job queue and worker runner (`queue.Push`, `queue.Work`). |
| **`realtime`** | Native Server-Sent Events (SSE) broadcaster and pub/sub hub. |
| **`response`** | Standardized JSON response envelopes (`Success`, `Created`, `BadRequest`, `NotFound`, `Paginated`, etc.). |
| **`router`** | Chi router wrapper with semantic group versioning and URL parameter extractors. |
| **`validator`** | Struct validation and JSON request payload binding. |
| **`wizard`** | Interactive CLI setup wizard for project configuration. |

---

## 💡 Quick Examples

### 1. Daily Rotating Logger
```go
import "github.com/wibiesana/padi_go_core/logger"

// Log structured entries with optional JSON context
logger.Info("User logged in", map[string]interface{}{"user_id": 42, "ip": "127.0.0.1"})
logger.Error("Database connection timed out", map[string]interface{}{"retry_count": 3})
```

### 2. ActiveRecord with Context-Aware Audit Trail & Timestamps
```go
import (
    "net/http"
    "github.com/wibiesana/padi_go_core/activerecord"
)

func CreateArticle(w http.ResponseWriter, r *http.Request) {
    article := Article{Title: "Padi Go Framework"}

    // Automatically populates created_at, updated_at, created_by, and updated_by
    if err := activerecord.Save(&article, r.Context()); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}
```

### 3. Fluent Query Builder & Caching
```go
import (
    "time"
    "github.com/wibiesana/padi_go_core/cache"
    "github.com/wibiesana/padi_go_core/query"
)

// Cached computation with fallback callback
var user map[string]interface{}
err := cache.Remember("user:42", 10*time.Minute, &user, func() (interface{}, error) {
    return query.New("users").Where("id", "=", 42).First()
})
```

### 3. File Upload with MIME Content Sniffing
```go
import (
    "net/http"
    "github.com/wibiesana/padi_go_core/file"
)

func UploadAvatar(w http.ResponseWriter, r *http.Request) {
    // Validates extension and inspects the first 512 bytes of binary content
    path, err := file.Upload(r, "avatar", file.UploadOptions{
        SubDir:      "avatars",
        AllowedExts: []string{"jpg", "png", "webp"},
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    avatarURL := file.URLOrNull(r, path)
    _ = avatarURL
}
```

---

## 🧪 Testing

Run all unit tests across the entire core suite:

```bash
go test -v ./...
```

---

## 📄 License

MIT License © 2026 Padi Framework.
