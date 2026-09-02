# 📦 HTTP Response Guide

`padi_go_core/response` standardizes JSON API response envelopes, pagination metadata, file download helpers, and automatic telemetry debugging injection.

---

## 🎯 Standard Response Envelopes

### 1. Single Record (`Item` / `Success`)
```go
user, _ := (User{}).Find(1)
response.Item(w, user, "User profile retrieved")
// Response JSON:
// { "status": 200, "success": true, "message": "User profile retrieved", "item": { ... } }
```

### 2. Collection (`Items`)
```go
users, _ := activerecord.NewModelQuery[User]().Get()
response.Items(w, users, "List of users")
// Response JSON:
// { "status": 200, "success": true, "message": "List of users", "items": [ ... ] }
```

### 3. Resource Created (`Created`)
```go
response.Created(w, newProduct, "Product created successfully")
// HTTP 201 Created
```

### 4. Paginated List (`Paginated`)
```go
opts := query.ParseOptions(r)
var posts []Post
meta, _ := query.New("posts").Paginate(opts, []string{"title"}, &posts)
response.Paginated(w, posts, meta)
```

### 5. No Content (`NoContent`)
```go
response.NoContent(w)
// HTTP 204 No Content
```

---

## ⚠️ Error Responses

```go
// 400 Bad Request
response.BadRequest(w, "Invalid payment method")

// 401 Unauthorized
response.Unauthorized(w, "Session expired, please log in")

// 403 Forbidden
response.Forbidden(w, "Insufficient permissions")

// 404 Not Found
response.NotFound(w, "Order not found")

// 409 Conflict
response.Conflict(w, "Email already registered")

// 422 Unprocessable Entity (Validation error)
response.UnprocessableEntity(w, validationErrorMap)

// 429 Too Many Requests
response.TooManyRequests(w)

// 500 Internal Server Error (Includes stack trace when APP_DEBUG=true)
response.InternalServerError(w, "Payment processing failed")
```

---

## 📥 File Downloads

```go
func (c *ReportController) Export(w http.ResponseWriter, r *http.Request) {
    filePath := "storage/exports/sales_2026.xlsx"
    response.Download(w, r, filePath, "Annual_Sales_Report.xlsx")
}
```
