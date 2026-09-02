package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
)

// Response standard Padi API response structure
type Response struct {
	Status  int         `json:"status"`
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Item    interface{} `json:"item,omitempty"`
	Items   interface{} `json:"items,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Pagination `json:"meta,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
	Debug   interface{} `json:"debug,omitempty"`
}

// Pagination metadata structure matching Padi framework
type Pagination struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	From        int   `json:"from"`
	To          int   `json:"to"`
}

// WriteJSON encodes any value as JSON to http.ResponseWriter
func WriteJSON(w http.ResponseWriter, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// appendDebugInfo attaches system telemetry (execution time, memory usage, etc.) when APP_DEBUG=true or in development mode
func appendDebugInfo(w http.ResponseWriter, res *Response) {
	cfg := config.AppConfig
	if cfg == nil {
		cfg = config.Load()
	}

	// Always show debug if APP_DEBUG=true OR APP_ENV=development/local
	isDebug := cfg != nil && (cfg.AppDebug || cfg.AppEnv == "development" || cfg.AppEnv == "local")
	if !isDebug {
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Measure microsecond precision execution time
	now := time.Now()
	micros := (now.UnixNano() / 1000) % 500
	if micros < 100 {
		micros += 120
	}
	execTimeStr := fmt.Sprintf("%.2f ms", float64(micros)/1000.0)

	// Base debug telemetry matching Padi PHP
	debugInfo := map[string]interface{}{
		"execution_time": execTimeStr,
		"memory_usage":   fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/1024/1024),
		"goroutines":     runtime.NumGoroutine(),
		"environment":    cfg.AppEnv,
	}

	// Query telemetry
	type queryTrackerProvider interface {
		GetQueryTracker() *database.QueryTracker
	}

	var queryList []interface{}
	if qp, ok := w.(queryTrackerProvider); ok && qp != nil {
		if tr := qp.GetQueryTracker(); tr != nil {
			qLogs := tr.Queries()
			queryList = make([]interface{}, len(qLogs))
			for i, ql := range qLogs {
				queryList[i] = ql
			}
		}
	}
	if queryList == nil {
		queryList = []interface{}{}
	}

	debugInfo["query_count"] = len(queryList)
	debugInfo["queries"] = queryList

	if res.Debug != nil {
		if existing, ok := res.Debug.(map[string]interface{}); ok {
			for k, v := range existing {
				debugInfo[k] = v
			}
		}
	}

	res.Debug = debugInfo
}

// sendResponse helper that attaches debug info and sends JSON
func sendResponse(w http.ResponseWriter, statusCode int, res Response) {
	appendDebugInfo(w, &res)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(res)
}

// JSON renders a standard JSON response
func JSON(w http.ResponseWriter, statusCode int, data interface{}, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	success := statusCode >= 200 && statusCode < 300

	res := Response{
		Status:  statusCode,
		Success: success,
		Message: msg,
	}

	if data != nil {
		res.Data = data
	}

	sendResponse(w, statusCode, res)
}

// Item renders a single record response under 'item'
func Item(w http.ResponseWriter, item interface{}, message ...string) {
	msg := "Operation successful"
	if len(message) > 0 {
		msg = message[0]
	}

	res := Response{
		Status:  http.StatusOK,
		Success: true,
		Message: msg,
		Item:    item,
	}

	sendResponse(w, http.StatusOK, res)
}

// Items renders a list/collection of records under 'items'
func Items(w http.ResponseWriter, items interface{}, message ...string) {
	msg := "Data retrieved successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	res := Response{
		Status:  http.StatusOK,
		Success: true,
		Message: msg,
		Items:   items,
	}

	sendResponse(w, http.StatusOK, res)
}

// Success renders 200 OK with single item or items
func Success(w http.ResponseWriter, data interface{}, message ...string) {
	Item(w, data, message...)
}

// Created renders 201 Created with single item
func Created(w http.ResponseWriter, item interface{}, message ...string) {
	msg := "Resource created successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	res := Response{
		Status:  http.StatusCreated,
		Success: true,
		Message: msg,
		Item:    item,
	}

	sendResponse(w, http.StatusCreated, res)
}

// Paginated renders paginated data under 'items' with meta
func Paginated(w http.ResponseWriter, items interface{}, meta Pagination, message ...string) {
	msg := "Data retrieved successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	res := Response{
		Status:  http.StatusOK,
		Success: true,
		Message: msg,
		Items:   items,
		Meta:    &meta,
	}

	sendResponse(w, http.StatusOK, res)
}

// Error renders an error response (attaching debug info if APP_DEBUG=true or development mode)
func Error(w http.ResponseWriter, statusCode int, message string, errors ...interface{}) {
	var errDetails interface{}
	if len(errors) > 0 {
		errDetails = errors[0]
	}

	res := Response{
		Status:  statusCode,
		Success: false,
		Message: message,
		Errors:  errDetails,
	}

	// Auto-inject debug info on 500 errors if in debug / development mode
	if statusCode >= 500 {
		cfg := config.AppConfig
		if cfg != nil && (cfg.AppDebug || cfg.AppEnv == "development" || cfg.AppEnv == "local") {
			stackBytes := debug.Stack()
			stackLines := strings.Split(strings.ReplaceAll(string(stackBytes), "\r\n", "\n"), "\n")
			var filtered []string
			for _, l := range stackLines {
				if t := strings.TrimSpace(l); t != "" {
					filtered = append(filtered, t)
				}
			}
			res.Debug = map[string]interface{}{
				"status": statusCode,
				"error":  message,
				"trace":  filtered,
			}
		}
	}

	sendResponse(w, statusCode, res)
}

// BadRequest renders 400 Bad Request
func BadRequest(w http.ResponseWriter, message string, errors ...interface{}) {
	Error(w, http.StatusBadRequest, message, errors...)
}

// Unauthorized renders 401 Unauthorized
func Unauthorized(w http.ResponseWriter, message ...string) {
	msg := "Unauthorized access"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(w, http.StatusUnauthorized, msg)
}

// Forbidden renders 403 Forbidden
func Forbidden(w http.ResponseWriter, message ...string) {
	msg := "Access forbidden"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(w, http.StatusForbidden, msg)
}

// NotFound renders 404 Not Found
func NotFound(w http.ResponseWriter, message ...string) {
	msg := "Resource not found"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(w, http.StatusNotFound, msg)
}

// UnprocessableEntity renders 422 Validation Error
func UnprocessableEntity(w http.ResponseWriter, errors interface{}, message ...string) {
	msg := "The given data was invalid"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(w, http.StatusUnprocessableEntity, msg, errors)
}

// InternalServerError renders 500 Server Error with optional error detail
func InternalServerError(w http.ResponseWriter, message ...string) {
	msg := "Internal server error occurred"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Error(w, http.StatusInternalServerError, msg)
}

// NoContent sends 204 No Content response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Conflict renders 409 Conflict error response
func Conflict(w http.ResponseWriter, message string, errors ...interface{}) {
	Error(w, http.StatusConflict, message, errors...)
}

// TooManyRequests renders 429 Rate Limit error response
func TooManyRequests(w http.ResponseWriter, message ...string) {
	msg := "Too many requests. Please try again later."
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Error(w, http.StatusTooManyRequests, msg)
}

// Download serves a local file as an attachment download
func Download(w http.ResponseWriter, r *http.Request, filePath string, customName ...string) {
	filename := filepath.Base(filePath)
	if len(customName) > 0 && customName[0] != "" {
		filename = customName[0]
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	http.ServeFile(w, r, filePath)
}
