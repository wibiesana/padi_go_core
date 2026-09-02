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

| Package | Description | Documentation |
| :--- | :--- | :--- |
| **`activerecord`** | Reflection-powered native ActiveRecord engine with generic `ModelQuery[T]`, eager loading (`.With()`), nested relations, and lifecycle hooks. | [Guide](docs/activerecord.md) |
| **`auth`** | JWT Token management (generation & verification), Bcrypt hashing, and HTTP request user context extractors. | [Guide](docs/auth.md) |
| **`cache`** | Multi-tier L1 (Memory) + L2 (Redis / File) caching with type-safe `GetTyped[T]` and `RememberTyped[T]`. | [Guide](docs/cache.md) |
| **`config`** | `.env`-driven configuration loader, environment checkers (`IsProduction`), and typed getters. | [Guide](docs/config.md) |
| **`database`** | Multi-driver connection pooling (`sqlite` pure-Go, `mysql`, `postgres`), transaction managers, and query telemetry. | [Guide](docs/database.md) |
| **`email`** | SMTP mailer with HTML/plain-text, multipart base64 attachments, template rendering, and async dispatching. | [Guide](docs/email.md) |
| **`file` / `storage`** | Secure file uploads with MIME content sniffing, path traversal defense, and `URLOrNull` helpers. | [Guide](docs/file_storage.md) |
| **`generator`** | Signature scaffolding generator for Base Models, Custom Models, Resources, Controllers, Routes, and Postman Collections. | Auto-scaffolding |
| **`logger`** | Daily rotating structured file logger (`storage/logs/app-*.log`, `error-*.log`) with 14-day retention, printf methods, and JSON context. | [Guide](docs/logger.md) |
| **`middleware`** | HTTP middleware suite: `AuthRequired`, `RequireRole`, `SecurityHeaders`, `CORS`, `Logger`, `RateLimit`, and `Recoverer`. | [Guide](docs/middleware.md) |
| **`migrator`** | Migration registry, schema runner, batch rollback, status inspection, and `Fresh` / `Reset` helpers. | [Guide](docs/migrator.md) |
| **`model`** | Base model interfaces, schema definitions, and ActiveRecord lifecycle hooks. | [Guide](docs/activerecord.md) |
| **`query`** | Native Fluent Query Builder (`Where`, `Join`, `OrderBy`, `GroupBy`, `Paginate`, etc.) with generic `GetAll[T]` and `GetFirst[T]`. | [Guide](docs/query.md) |
| **`queue`** | Database-backed background job queue with delayed dispatch (`PushLater`), retries, and type-safe `RegisterTyped[T]` handlers. | [Guide](docs/queue.md) |
| **`realtime`** | Native Server-Sent Events (SSE) broadcaster, topic pub/sub hub, and batch publisher. | [Guide](docs/realtime.md) |
| **`response`** | Standardized JSON response envelopes (`Success`, `Created`, `BadRequest`, `NotFound`, `Paginated`, `Download`, etc.). | [Guide](docs/response.md) |
| **`router`** | Chi router wrapper with semantic group versioning (`/v1`), static file server, and URL/query parameter extractors. | [Guide](docs/router.md) |
| **`validator`** | Tag-based struct validation, generic one-line `Bind[T]`, and procedural `FormValidator` builder. | [Guide](docs/validator.md) |
| **`wizard`** | Interactive CLI setup wizard for project configuration and `.env` generation. | CLI Wizard |

---

## 💡 Quick Examples

### 1. ActiveRecord Out-of-the-Box Eager Loading & Generic Querying
```go
import "github.com/wibiesana/padi_go_core/activerecord"

// Query with nested eager loading and column filtering
posts, meta, err := activerecord.NewModelQuery[Post]().
    With("author:id,name,email", "comments.author").
    Where("status", "published").
    OrderBy("created_at", "DESC").
    Paginate(opts, []string{"title", "content"})
```

### 2. Type-Safe Request Binding & Validation
```go
type CreatePostRequest struct {
    Title   string `json:"title" validate:"required,min=5,max=255"`
    Content string `json:"content" validate:"required"`
}

func (c *PostController) Create(w http.ResponseWriter, r *http.Request) {
    req, errs, err := validator.Bind[CreatePostRequest](r)
    if err != nil {
        response.UnprocessableEntity(w, errs)
        return
    }
    
    // req is *CreatePostRequest, fully validated
    post := Post{Title: req.Title, Content: req.Content}
    post.Save(r.Context())
    response.Created(w, post)
}
```

### 3. Background Queue with Type-Safe Handlers
```go
type EmailJob struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

// Register type-safe worker handler
queue.RegisterTyped("SendEmail", func(job EmailJob) error {
    return email.SendHTML(job.To, job.Subject, job.Body)
})

// Dispatch immediately or with delay
queue.Push("SendEmail", EmailJob{To: "user@example.com", Subject: "Welcome", Body: "<h1>Hi!</h1>"})
queue.Later(10*time.Minute, "SendEmail", EmailJob{To: "user@example.com", Subject: "Follow-up", Body: "<p>Check this out</p>"})
```

### 4. Real-Time Server-Sent Events (SSE)
```go
// 1. Mount SSE endpoint in router
r.Get("/events", realtime.SubscribeSSE()) // Listens to ?topic=orders or ?topics=chat,alerts

// 2. Broadcast from controller
realtime.Publish("orders", map[string]interface{}{
    "order_id": 1029,
    "status":   "paid",
})
```

### 5. Daily Rotating Logger
```go
import "github.com/wibiesana/padi_go_core/logger"

// Log structured entries with optional JSON context
logger.Info("User logged in", map[string]interface{}{"user_id": 42, "ip": "127.0.0.1"})
logger.Infof("Order #%d processed in %v", orderID, duration)
logger.Error("Payment gateway timeout", map[string]interface{}{"gateway": "Midtrans"})
```

---

## 🧪 Testing

Run all unit tests across the entire core suite:

```bash
go test -v ./...
```

---

## 📜 Changelog

See [CHANGELOG.md](CHANGELOG.md) for full history and unreleased updates.

---

## 📄 License

MIT License © 2026 Padi Framework.

