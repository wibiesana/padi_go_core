package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/response"
	"github.com/wibiesana/padi_go_core/router"

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

	// 3. Test Any
	r.Any("/ping-any", func(w http.ResponseWriter, req *http.Request) {
		response.Success(w, "pong")
	})

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/ping-any", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expected Any route to handle %s with 200, got %d", method, rec.Code)
		}
	}

	// 4. Test Query Helpers
	queryReq := httptest.NewRequest(http.MethodGet, "/test?page=2&ratio=1.5&active=true&tags=go,framework,rest", nil)
	if p := router.QueryParamInt(queryReq, "page", 1); p != 2 {
		t.Errorf("QueryParamInt failed, got %d", p)
	}
	if f := router.QueryParamFloat(queryReq, "ratio", 1.0); f != 1.5 {
		t.Errorf("QueryParamFloat failed, got %f", f)
	}
	if b := router.QueryParamBool(queryReq, "active", false); !b {
		t.Errorf("QueryParamBool failed")
	}
	tags := router.QueryParamSlice(queryReq, "tags")
	if len(tags) != 3 || tags[0] != "go" || tags[1] != "framework" {
		t.Errorf("QueryParamSlice failed: %v", tags)
	}
}
