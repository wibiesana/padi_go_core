package response

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/wibiesana/padi_go_core/config"
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(res)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(res)
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
