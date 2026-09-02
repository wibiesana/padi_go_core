# 🔐 Auth & JWT Guide

`padi_go_core/auth` provides JWT authentication, password hashing with Bcrypt, and convenient HTTP request/context user extractors.

---

## 🔑 Token Management

### Generate JWT Token
```go
// Default token expiry (24h or JWT_EXPIRY_HOURS in .env)
token, err := auth.GenerateToken(user.ID, user.Email, user.Role)

// Custom token expiry duration
token, err := auth.GenerateTokenWithExpiry(user.ID, user.Email, user.Role, 7*24*time.Hour)
```

### Validate JWT Token
```go
claims, err := auth.ValidateToken(tokenString)
if err != nil {
    // Token is invalid or expired
}
fmt.Println(claims.UserID, claims.Email, claims.Role)
```

---

## 👤 Request User Context Helpers

Inside HTTP controllers/handlers:

```go
func (c *UserController) Profile(w http.ResponseWriter, r *http.Request) {
    // 1. Check if authenticated
    if !auth.Check(r) {
        response.Unauthorized(w)
        return
    }

    // 2. Get User ID
    userID := auth.UserID(r)

    // 3. Get full Claims (*auth.JWTClaims)
    userClaims := auth.User(r)

    // 4. Role checking
    if auth.HasRole(r, "admin", "superadmin") {
        // Elevated privileges
    }

    // 5. Extract raw token string
    rawToken := auth.ExtractToken(r)
    _ = rawToken
}
```

---

## 🔒 Password Hashing (Bcrypt)

```go
// Hash password
hash, err := auth.HashPassword("my_secret_password")

// Verify password
if auth.VerifyPassword("my_secret_password", hash) {
    // Password matched
}
```
