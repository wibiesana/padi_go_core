package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/response"
	"github.com/wibiesana/padi-core/router"

	"github.com/go-chi/chi/v5"
)

func TestRouterVersioningAndEndpoints(t *testing.T) {
	cfg := &config.Config{
		AppName: "Test App",
		CorsOrigins: []string{"*"},
	}

	r := router.New(cfg)

	r.Mux.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		response.Success(w, map[string]string{"status": "ok"}, "Healthy")
	})

	r.Version("v1", func(v1 chi.Router) {
		v1.Get("/items/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := router.Param(req, "id")
			response.Success(w, map[string]string{"item_id": id}, "Item retrieved")
		})
	})

	// 1. Test Health
	req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 on /health, got %d", w1.Code)
	}

	// 2. Test Versioned Route & Param
	req2 := httptest.NewRequest(http.MethodGet, "/v1/items/99", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 on /v1/items/99, got %d", w2.Code)
	}
}
