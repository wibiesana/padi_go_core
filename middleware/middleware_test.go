package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/auth"
	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/middleware"
	"github.com/wibiesana/padi_go_core/response"
)

func TestMiddlewareStack(t *testing.T) {
	// 1. Test Logger
	t.Run("Logger", func(t *testing.T) {
		handler := middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	// 1b. Test Logger with QueryTracker debug info
	t.Run("Logger With QueryTracker", func(t *testing.T) {
		config.AppConfig = &config.Config{
			AppDebug: true,
			AppEnv:   "development",
		}
		handler := middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			database.TrackQuery(r.Context(), "SELECT * FROM users WHERE id = ?", []interface{}{1}, 2*time.Millisecond)
			database.TrackQuery(r.Context(), "SELECT COUNT(*) FROM posts", nil, 1*time.Millisecond)
			response.Item(w, map[string]string{"name": "test"}, "Success")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test-query", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse json response: %v", err)
		}
		if res.Debug == nil {
			t.Fatalf("expected debug info in response")
		}
		dbg, ok := res.Debug.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map debug info, got %T", res.Debug)
		}
		if qCount, ok := dbg["query_count"].(float64); !ok || int(qCount) != 2 {
			t.Fatalf("expected query_count = 2, got %v", dbg["query_count"])
		}
		queries, ok := dbg["queries"].([]interface{})
		if !ok || len(queries) != 2 {
			t.Fatalf("expected 2 queries logged, got %v", dbg["queries"])
		}
	})

	// 2. Test Recoverer
	t.Run("Recoverer", func(t *testing.T) {
		handler := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went wrong")
		}))
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on panic recovery, got %d", w.Code)
		}
	})

	// 3. Test CORS
	t.Run("CORS", func(t *testing.T) {
		cfg := &config.Config{CorsOrigins: []string{"http://localhost:3000"}}
		corsMiddleware := middleware.CORS(cfg)
		handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatalf("expected CORS origin header")
		}
	})

	// 4. Test AuthRequired
	t.Run("AuthRequired", func(t *testing.T) {
		config.AppConfig = &config.Config{
			JWTSecret:     "test-secret-key-32-chars-long-abc",
			JWTExpiration: 1,
		}

		handler := middleware.AuthRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(middleware.UserContextKey).(*auth.JWTClaims)
			if !ok || claims.UserID != 42 {
				http.Error(w, "invalid context", http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))

		// Missing header
		reqNoAuth := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, reqNoAuth)
		if w1.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on missing auth, got %d", w1.Code)
		}

		// Valid token
		token, _ := auth.GenerateToken(42, "test@example.com", "admin")
		reqAuth := httptest.NewRequest(http.MethodGet, "/protected", nil)
		reqAuth.Header.Set("Authorization", "Bearer "+token)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, reqAuth)
		if w2.Code != http.StatusOK {
			t.Fatalf("expected 200 with valid token, got %d", w2.Code)
		}
	})

	// 5. Test RateLimit
	t.Run("RateLimit", func(t *testing.T) {
		limiter := middleware.RateLimit(2, 60)
		handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/rate-limited", nil)
		req.RemoteAddr = "192.0.2.1:1234"

		// 1st request - ok
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req)
		if w1.Code != http.StatusOK {
			t.Fatalf("expected 200 on 1st request, got %d", w1.Code)
		}

		// 2nd request - ok
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req)
		if w2.Code != http.StatusOK {
			t.Fatalf("expected 200 on 2nd request, got %d", w2.Code)
		}

		// 3rd request - blocked (429)
		w3 := httptest.NewRecorder()
		handler.ServeHTTP(w3, req)
		if w3.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on 3rd request, got %d", w3.Code)
		}
	})

	// 6. Test RequireRole
	t.Run("RequireRole", func(t *testing.T) {
		roleMW := middleware.RequireRole("admin")
		handler := middleware.Auth(roleMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))

		// Token with user role -> 403 Forbidden
		userToken, _ := auth.GenerateToken(1, "user@example.com", "user")
		reqUser := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		reqUser.Header.Set("Authorization", "Bearer "+userToken)
		wUser := httptest.NewRecorder()
		handler.ServeHTTP(wUser, reqUser)
		if wUser.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for user role, got %d", wUser.Code)
		}

		// Token with admin role -> 200 OK
		adminToken, _ := auth.GenerateToken(2, "admin@example.com", "admin")
		reqAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
		wAdmin := httptest.NewRecorder()
		handler.ServeHTTP(wAdmin, reqAdmin)
		if wAdmin.Code != http.StatusOK {
			t.Fatalf("expected 200 for admin role, got %d", wAdmin.Code)
		}
	})

	// 7. Test SecurityHeaders
	t.Run("SecurityHeaders", func(t *testing.T) {
		handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("expected X-Frame-Options DENY")
		}
	})
}
