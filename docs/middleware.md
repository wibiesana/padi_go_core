# 🛡️ HTTP Middleware Suite Guide

`padi_go_core/middleware` provides standard HTTP middlewares for authentication, role authorization, access logging, panic recovery, CORS, rate limiting, and security headers.

---

## 🔒 Authentication & Role Authorization

```go
// 1. Require Valid JWT Bearer Token
r.Use(middleware.Auth) // or middleware.AuthRequired

// 2. Restrict to Specific Roles
r.Group(func(adminRoute router.Route) {
    adminRoute.Use(middleware.Auth)
    adminRoute.Use(middleware.RequireRole("admin", "superadmin"))

    adminRoute.Get("/admin/users", adminController.ListUsers)
    adminRoute.Delete("/admin/users/{id}", adminController.DeleteUser)
})
```

---

## 🚦 Rate Limiting (`RateLimit`)

Protects sensitive endpoints by IP:

```go
// Allow maximum 5 requests per 60-second window per IP
r.Group(func(authRoute router.Route) {
    authRoute.Use(middleware.RateLimit(5, 60))

    authRoute.Post("/auth/login", authController.Login)
    authRoute.Post("/auth/forgot-password", authController.ForgotPassword)
})
```

---

## 🛡️ Security Headers (`SecurityHeaders`)

Applies recommended security headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`

```go
r.Use(middleware.SecurityHeaders)
```

---

## 📊 Logging & Latency (`Logger`)

Logs request method, path, client IP, response code, and latency in ms:

```go
r.Use(middleware.Logger)
```

---

## 💥 Panic Recovery (`Recoverer`)

Catches panics, writes debug stack trace to JSON (when `APP_DEBUG=true`), and keeps the server alive without crashing:

```go
r.Use(middleware.Recoverer)
```
