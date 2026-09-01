package router

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/middleware"

	"github.com/go-chi/chi/v5"
)

// Route is an alias for chi.Router to use in sub-routes and group callbacks without importing chi
type Route = chi.Router

// SubRouter is an alias for chi.Router
type SubRouter = chi.Router

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

// Use appends middlewares to the router
func (r *Router) Use(middlewares ...func(http.Handler) http.Handler) {
	r.Mux.Use(middlewares...)
}

// Get adds a GET route
func (r *Router) Get(pattern string, handler http.HandlerFunc) {
	r.Mux.Get(pattern, handler)
}

// Post adds a POST route
func (r *Router) Post(pattern string, handler http.HandlerFunc) {
	r.Mux.Post(pattern, handler)
}

// Put adds a PUT route
func (r *Router) Put(pattern string, handler http.HandlerFunc) {
	r.Mux.Put(pattern, handler)
}

// Patch adds a PATCH route
func (r *Router) Patch(pattern string, handler http.HandlerFunc) {
	r.Mux.Patch(pattern, handler)
}

// Delete adds a DELETE route
func (r *Router) Delete(pattern string, handler http.HandlerFunc) {
	r.Mux.Delete(pattern, handler)
}

// Options adds an OPTIONS route
func (r *Router) Options(pattern string, handler http.HandlerFunc) {
	r.Mux.Options(pattern, handler)
}

// Head adds a HEAD route
func (r *Router) Head(pattern string, handler http.HandlerFunc) {
	r.Mux.Head(pattern, handler)
}

// Handle adds an http.Handler for pattern
func (r *Router) Handle(pattern string, handler http.Handler) {
	r.Mux.Handle(pattern, handler)
}

// HandleFunc adds an http.HandlerFunc for pattern
func (r *Router) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.Mux.HandleFunc(pattern, handler)
}

// Route mounts a sub-router on a pattern
func (r *Router) Route(pattern string, fn func(r Route)) {
	r.Mux.Route(pattern, fn)
}

// Group creates a nested route group
func (r *Router) Group(fn func(r Route)) {
	r.Mux.Group(fn)
}

// Version creates a versioned route group (e.g. /v1, /v2)
func (r *Router) Version(version string, fn func(r Route)) {
	v := strings.TrimPrefix(strings.ToLower(version), "v")
	r.Mux.Route("/v"+v, fn)
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
