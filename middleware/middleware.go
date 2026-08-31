package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/wibiesana/padi_go_core/auth"
	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/response"

	"github.com/go-chi/cors"
)

type contextKey string

const (
	UserContextKey   contextKey = "padi_user"
	UserIDContextKey contextKey = "padi_user_id"
)

// Logger logs each incoming HTTP request with latency
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s - %d (%v)", r.Method, r.RequestURI, r.RemoteAddr, ww.statusCode, duration)
	})
}

// Recoverer catches panics and returns 500 JSON response
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC RECOVERED] %v\n%s", rec, string(debug.Stack()))
				response.InternalServerError(w, "An unexpected server error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS returns configured CORS handler
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	allowedOrigins := cfg.CorsOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "Content-Length"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any major browser
	})
}

// AuthRequired ensures request has a valid Bearer JWT token
func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Unauthorized(w, "Authorization token is missing")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Unauthorized(w, "Invalid authorization header format. Expected 'Bearer <token>'")
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			response.Unauthorized(w, fmt.Sprintf("Authentication failed: %v", err.Error()))
			return
		}

		// Inject into context
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		ctx = context.WithValue(ctx, UserIDContextKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Simple in-memory rate limiter
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientBucket
}

type clientBucket struct {
	count     int
	resetTime time.Time
}

var globalLimiter = &rateLimiter{
	clients: make(map[string]*clientBucket),
}

// RateLimit limits requests per IP
func RateLimit(limit int, windowSeconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = strings.Split(forwarded, ",")[0]
			}

			globalLimiter.mu.Lock()
			now := time.Now()
			bucket, exists := globalLimiter.clients[ip]

			if !exists || now.After(bucket.resetTime) {
				globalLimiter.clients[ip] = &clientBucket{
					count:     1,
					resetTime: now.Add(time.Duration(windowSeconds) * time.Second),
				}
				globalLimiter.mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			if bucket.count >= limit {
				globalLimiter.mu.Unlock()
				response.Error(w, http.StatusTooManyRequests, "Too many requests. Please slow down.")
				return
			}

			bucket.count++
			globalLimiter.mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
