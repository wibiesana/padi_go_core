package router

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wibiesana/padi-core/config"
	"github.com/wibiesana/padi-core/middleware"

	"github.com/go-chi/chi/v5"
)

type Router struct {
	Mux *chi.Mux
	cfg *config.Config
}

// New creates a new initialized Padi Router with default middlewares
func New(cfg *config.Config) *Router {
	r := chi.NewRouter()

	// Default Middlewares
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CORS(cfg))

	if cfg.RateLimitReqs > 0 {
		r.Use(middleware.RateLimit(cfg.RateLimitReqs, cfg.RateLimitWindow))
	}

	return &Router{
		Mux: r,
		cfg: cfg,
	}
}

// Version creates a versioned route group (e.g. /v1, /v2)
func (r *Router) Version(version string, fn func(r chi.Router)) {
	v := strings.TrimPrefix(strings.ToLower(version), "v")
	r.Mux.Route("/v"+v, fn)
}

// Group creates a nested route group
func (r *Router) Group(pattern string, fn func(r chi.Router)) {
	r.Mux.Route(pattern, fn)
}

// ServeHTTP implements http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Mux.ServeHTTP(w, req)
}

// Param retrieves a URL parameter from request context
func Param(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// ParamUint retrieves an unsigned integer URL parameter
func ParamUint(r *http.Request, key string) (uint, error) {
	val := chi.URLParam(r, key)
	if val == "" {
		return 0, fmt.Errorf("param %s is empty", key)
	}
	n, err := strconv.ParseUint(val, 10, 32)
	return uint(n), err
}

// QueryParam retrieves a query parameter with default fallback
func QueryParam(r *http.Request, key, defaultVal string) string {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// QueryParamInt retrieves integer query parameter with default
func QueryParamInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
