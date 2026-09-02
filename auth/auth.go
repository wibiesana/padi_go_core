package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wibiesana/padi_go_core/config"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ContextKey type for context value lookups
type ContextKey string

const (
	UserContextKey   ContextKey = "padi_user"
	UserIDContextKey ContextKey = "padi_user_id"
)

// JWTClaims custom JWT payload claims
type JWTClaims struct {
	UserID   uint                   `json:"user_id"`
	Email    string                 `json:"email"`
	Role     string                 `json:"role,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

// HashPassword hashes plain password with bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares password with hashed string
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// VerifyPassword verifies password against hash (supports both argument orders)
func VerifyPassword(p1, p2 string) bool {
	if strings.HasPrefix(p1, "$2a$") || strings.HasPrefix(p1, "$2b$") || strings.HasPrefix(p1, "$2y$") {
		return CheckPasswordHash(p2, p1)
	}
	return CheckPasswordHash(p1, p2)
}

// GenerateToken generates signed JWT string for given user with default expiration
func GenerateToken(userID uint, email string, role string, meta ...map[string]interface{}) (string, error) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	expirationHours := cfg.JWTExpiration
	if expirationHours <= 0 {
		expirationHours = 24
	}

	return GenerateTokenWithExpiry(userID, email, role, time.Hour*time.Duration(expirationHours), meta...)
}

// GenerateTokenWithExpiry generates signed JWT string for given user with custom expiry duration
func GenerateTokenWithExpiry(userID uint, email string, role string, expiry time.Duration, meta ...map[string]interface{}) (string, error) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	var metadata map[string]interface{}
	if len(meta) > 0 {
		metadata = meta[0]
	}

	if expiry <= 0 {
		expiry = 24 * time.Hour
	}

	claims := JWTClaims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		Metadata: metadata,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.AppName,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateToken parses and validates a JWT token string
func ValidateToken(tokenString string) (*JWTClaims, error) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid or expired token")
}

// VerifyToken is an alias for ValidateToken matching PHP Auth::verifyToken
func VerifyToken(tokenString string) (*JWTClaims, error) {
	return ValidateToken(tokenString)
}

// ExtractToken extracts Bearer token from request Authorization header, query string, or cookie
func ExtractToken(r *http.Request) string {
	if r == nil {
		return ""
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		authHeader = r.Header.Get("authorization")
	}
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if qToken := r.URL.Query().Get("token"); qToken != "" {
		return qToken
	}

	if cookie, err := r.Cookie("token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// User extracts authenticated JWT claims from request context (or decodes token from headers)
func User(r *http.Request) *JWTClaims {
	if r == nil {
		return nil
	}
	if claims := UserFromContext(r.Context()); claims != nil {
		return claims
	}
	if token := ExtractToken(r); token != "" {
		if claims, err := ValidateToken(token); err == nil {
			return claims
		}
	}
	return nil
}

// UserID returns authenticated User ID from request context (or 0 if unauthenticated)
func UserID(r *http.Request) uint {
	if u := User(r); u != nil {
		return u.UserID
	}
	return 0
}

// UserFromContext extracts authenticated JWT claims from context
func UserFromContext(ctx context.Context) *JWTClaims {
	if ctx == nil {
		return nil
	}
	// Check auth.UserContextKey
	if claims, ok := ctx.Value(UserContextKey).(*JWTClaims); ok && claims != nil {
		return claims
	}
	// Check generic string key "padi_user"
	if claims, ok := ctx.Value("padi_user").(*JWTClaims); ok && claims != nil {
		return claims
	}
	return nil
}

// UserIDFromContext returns authenticated User ID from context
func UserIDFromContext(ctx context.Context) uint {
	if u := UserFromContext(ctx); u != nil {
		return u.UserID
	}
	if ctx != nil {
		if id, ok := ctx.Value(UserIDContextKey).(uint); ok {
			return id
		}
		if id, ok := ctx.Value("padi_user_id").(uint); ok {
			return id
		}
	}
	return 0
}

// Check verifies if the request contains valid authentication
func Check(r *http.Request) bool {
	return User(r) != nil
}

// HasRole checks if the authenticated user has any of the specified roles
func HasRole(r *http.Request, roles ...string) bool {
	u := User(r)
	if u == nil || u.Role == "" {
		return false
	}
	for _, role := range roles {
		if strings.EqualFold(u.Role, role) {
			return true
		}
	}
	return false
}

// GenerateSecureRandomString generates cryptographically secure random alphanumeric string
func GenerateSecureRandomString(length int) string {
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}
