<div align="center">

# 🌾 Padi Core Engine (Go)

**The pure Go, zero-bloat, high-performance core engine for Padi REST API Framework**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/wibiesana/padi_go_core.svg)](https://pkg.go.dev/github.com/wibiesana/padi_go_core)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](#)

> *"High-throughput, type-safe API engine with zero external ORM bloat."*

</div>

---

## ⚡ Overview

`padi_go_core` is the foundational engine powering the **Padi REST API Go Framework**. Built with raw Go performance and modern generics (`Go 1.22+`), it delivers a cohesive suite of modular packages designed for high-concurrency, enterprise-grade cloud backends.

- 🚀 **Zero External ORM Overhead**: Native ActiveRecord and Fluent Query Builder powered by standard `database/sql`.
- 🌾 **Out-of-the-Box Eager Loading**: Multi-level relations (`.With("author", "comments.author")`) without N+1 query overhead.
- 🗄️ **Multi-Driver Database**: Pure-Go SQLite (100% CGO-free), PostgreSQL, and MySQL/MariaDB.
- 🔐 **Built-in Security**: Stateless JWT authentication, Bcrypt password hashing, and IP rate limiting.
- 📬 **Async Queue & Workers**: Database-backed job queues with delayed execution (`PushLater`) and type-safe handlers.
- 📡 **Native Real-Time**: Server-Sent Events (SSE) topic broadcaster and pub/sub engine.
- 📜 **Observability**: Daily rotating structured file logger (`storage/logs/`) with sub-millisecond query telemetry.

---

## 📦 Installation

```bash
go get github.com/wibiesana/padi_go_core@latest
```

---

## 💡 Quick Taste

```go
package main

import (
	"net/http"
	"github.com/wibiesana/padi_go_core/activerecord"
	"github.com/wibiesana/padi_go_core/response"
	"github.com/wibiesana/padi_go_core/validator"
)

type CreateArticleRequest struct {
	Title   string `json:"title" validate:"required,min=5,max=255"`
	Content string `json:"content" validate:"required"`
}

func CreateArticle(w http.ResponseWriter, r *http.Request) {
	// 1. One-line type-safe binding & validation
	req, errs, err := validator.Bind[CreateArticleRequest](r)
	if err != nil {
		response.UnprocessableEntity(w, errs)
		return
	}

	// 2. Context-aware ActiveRecord with auto-audit trail
	article := Article{Title: req.Title, Content: req.Content}
	if err := article.Save(r.Context()); err != nil {
		response.InternalServerError(w, err.Error())
		return
	}

	// 3. Deterministic JSON API response
	response.Created(w, article, "Article published successfully")
}
```

---

## 📦 Core Packages

| Package | Role |
| :--- | :--- |
| **`activerecord`** | Generic ActiveRecord ORM with recursive eager loading (`.With()`) and audit trail |
| **`query`** | Parameterized fluent SQL query builder with generic scanners (`GetAll[T]`, `GetFirst[T]`) |
| **`database`** | Thread-safe connection pooling, auto-commit/rollback transactions, and query telemetry |
| **`auth`** | Stateless JWT tokens, password hashing, and request user context helpers |
| **`router`** | Chi router wrapper with semantic versioning (`/v1`), static files, and parameter extractors |
| **`middleware`** | `AuthRequired`, `RequireRole`, `SecurityHeaders`, `CORS`, `Logger`, `RateLimit`, `Recoverer` |
| **`validator`** | One-line generic struct binder (`Bind[T]`) and procedural `FormValidator` |
| **`response`** | Standardized JSON response envelopes (`Item`, `Items`, `Paginated`, `Download`) |
| **`queue`** | Asynchronous background job queue with delayed dispatch and type-safe workers |
| **`realtime`** | Real-time Server-Sent Events (SSE) broadcaster and pub/sub hub |
| **`cache`** | Multi-tier L1 (Memory) + L2 (Redis / File) caching with type-safe `RememberTyped[T]` |
| **`logger`** | Daily rotating structured logger (`app-*.log`, `error-*.log`) with JSON context |
| **`file` / `storage`** | Secure multipart file uploads with MIME binary sniffing and filesystem helpers |
| **`email`** | SMTP mailer with HTML/plain-text, attachments, and Go template rendering |
| **`migrator`** | Database migration runner with batch tracking, rollback, and `Fresh` / `Reset` |
| **`generator`** | Signature CRUD scaffolding for models, controllers, resources, and API collections |

---

## 📖 Complete Documentation

Looking for detailed guides, architecture deep-dives, and comprehensive code recipes?

👉 **[Explore Full Documentation & Examples](https://github.com/wibiesana/padi_go)** *(or visit the documentation website)*

---

## 🧪 Running Tests

```bash
go test -v ./...
```

---

## 📄 License

MIT License © 2026 Padi Framework.
