# 🌐 HTTP Router Guide

`padi_go_core/router` provides a high-performance HTTP routing engine with automatic default middlewares, API versioning (`/v1`), route groups, static file serving, and parameter extractors.

---

## 🚦 Router Initialization & Routes

```go
r := router.New(config.AppConfig)

// Basic Routes
r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
    response.Success(w, map[string]string{"status": "ok"})
})
r.Post("/orders", orderController.Create)
r.Put("/orders/{id}", orderController.Update)
r.Delete("/orders/{id}", orderController.Destroy)

// Any HTTP Method
r.Any("/webhook/stripe", stripeWebhookHandler)

// Static File Server
r.Static("/storage", "storage")
```

---

## 🏷️ API Versioning & Route Groups

```go
// Versioned /v1 group
r.Version("v1", func(v1 router.Route) {
    // Public routes
    v1.Post("/login", authController.Login)
    v1.Post("/register", authController.Register)

    // Authenticated routes
    v1.Group(func(authRoute router.Route) {
        authRoute.Use(middleware.Auth)

        authRoute.Get("/profile", userController.Profile)
        authRoute.Get("/posts", postController.Index)
    })
})
```

---

## 🔍 Request Parameter Extractors

```go
func (c *PostController) Show(w http.ResponseWriter, r *http.Request) {
    // 1. URL Path Parameters
    id := router.Param(r, "id")           // string
    idInt := router.ParamInt(r, "id", 0)  // int with default fallback
    idUint, _ := router.ParamUint(r, "id") // uint

    // 2. Query String Parameters
    page := router.QueryParamInt(r, "page", 1)
    sort := router.QueryParam(r, "sort", "created_at")
    active := router.QueryParamBool(r, "active", true)
    threshold := router.QueryParamFloat(r, "threshold", 0.0)
    tags := router.QueryParamSlice(r, "tags") // comma-separated query param to []string

    _ = id
    _ = idInt
    _ = idUint
    _ = page
    _ = sort
    _ = active
    _ = threshold
    _ = tags
}
```
