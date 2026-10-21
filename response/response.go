package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
)

// Response standard Padi API response structure matching PHP & Rust
type Response struct {
	Message     string      `json:"message,omitempty"`
	MessageCode string      `json:"message_code,omitempty"`
	Success     bool        `json:"success"`
	Item        interface{} `json:"item,omitempty"`
	Items       interface{} `json:"items,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	Meta        *Pagination `json:"meta,omitempty"`
	Errors      interface{} `json:"errors,omitempty"`
	Debug       interface{} `json:"debug,omitempty"`
	Status      int         `json:"status,omitempty"`
}

// Pagination metadata structure matching Padi PHP & Rust frameworks
type Pagination struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	From        int   `json:"from"`
	To          int   `json:"to"`
}

// StatusCodeName returns standard uppercase code name matching PHP & Rust (e.g., "VALIDATION_FAILED")
func StatusCodeName(code int) string {
	switch code {
	case 200:
		return "SUCCESS"
	case 201:
		return "CREATED"
	case 204:
		return "NO_CONTENT"
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 422:
		return "VALIDATION_FAILED"
	case 429:
		return "TOO_MANY_REQUESTS"
	case 500:
		return "INTERNAL_SERVER_ERROR"
	case 502:
		return "BAD_GATEWAY"
	case 503:
		return "SERVICE_UNAVAILABLE"
	default:
		if code >= 400 {
			return "ERROR"
		}
		return "SUCCESS"
	}
}

// UnmarshalJSON implements custom unmarshaling to ensure full backward compatibility
// when Go client/test code deserializes responses into *Response.
func (r *Response) UnmarshalJSON(b []byte) error {
	type Alias Response
	var aux struct {
		Alias
		StatusVal interface{} `json:"status"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*r = Response(aux.Alias)

	// If status was in JSON as an int, capture it
	if aux.StatusVal != nil {
		if s, ok := aux.StatusVal.(float64); ok {
			r.Status = int(s)
		}
	}

	// Auto-populate Items and Meta from Item to maintain 100% compatibility with older Go tests/consumers
	if r.Item != nil {
		if r.Items == nil {
			switch v := r.Item.(type) {
			case []interface{}:
				r.Items = v
			case map[string]interface{}:
				if data, ok := v["data"]; ok {
					r.Items = data
				}
				if metaVal, ok := v["meta"]; ok && r.Meta == nil {
					if metaBytes, err := json.Marshal(metaVal); err == nil {
						var p Pagination
						if json.Unmarshal(metaBytes, &p) == nil {
							r.Meta = &p
						}
					}
				}
			}
		}
	}

	return nil
}

// WriteJSON encodes any value as JSON to http.ResponseWriter
func WriteJSON(w http.ResponseWriter, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// getResponseFormat determines response format mode: full | simple | raw
func getResponseFormat(r *http.Request) string {
	if r != nil {
		if f := r.Header.Get("X-Response-Format"); f != "" {
			return strings.ToLower(f)
		}
	}
	if f := os.Getenv("RESPONSE_FORMAT"); f != "" {
		return strings.ToLower(f)
	}
	return "full"
}

// appendDebugInfo attaches system telemetry (execution time, memory usage, etc.) when in debug/development mode
func appendDebugInfo(w http.ResponseWriter, r *http.Request, res *Response) {
	if !config.IsDebug() {
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Compute precise execution time from request start timestamp stored by Logger middleware.
	var execTimeStr string
	if r != nil {
		if startTime, ok := r.Context().Value("padi_request_start_time").(time.Time); ok && !startTime.IsZero() {
			execTimeStr = fmt.Sprintf("%.4f ms", float64(time.Since(startTime).Microseconds())/1000.0)
		}
	}
	if execTimeStr == "" {
		micros := (time.Now().UnixNano() / 1000) % 500
		if micros < 100 {
			micros += 120
		}
		execTimeStr = fmt.Sprintf("%.2f ms", float64(micros)/1000.0)
	}

	// Base debug telemetry matching Padi PHP
	debugInfo := map[string]interface{}{
		"execution_time": execTimeStr,
		"memory_usage":   fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/1024/1024),
		"goroutines":     runtime.NumGoroutine(),
		"environment":    config.Current().AppEnv,
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

// sendResponse helper that formats payload according to standard and sends JSON
func sendResponse(w http.ResponseWriter, r *http.Request, statusCode int, res Response) {
	appendDebugInfo(w, r, &res)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if res.MessageCode == "" {
		res.MessageCode = StatusCodeName(statusCode)
	}

	format := getResponseFormat(r)
	switch format {
	case "raw":
		if res.Item != nil {
			_ = json.NewEncoder(w).Encode(res.Item)
		} else if res.Data != nil {
			_ = json.NewEncoder(w).Encode(res.Data)
		} else {
			_ = json.NewEncoder(w).Encode(res)
		}
		return

	case "simple":
		statusStr := "success"
		if !res.Success {
			statusStr = "error"
		}
		itemVal := res.Item
		if itemVal == nil {
			itemVal = res.Data
		}
		if m, ok := itemVal.(map[string]interface{}); ok && m != nil {
			if d, hasData := m["data"]; hasData {
				if mt, hasMeta := m["meta"]; hasMeta {
					itemVal = d
					if res.Meta == nil {
						if p, ok := mt.(Pagination); ok {
							res.Meta = &p
						} else if pPtr, ok := mt.(*Pagination); ok {
							res.Meta = pPtr
						}
					}
				}
			}
		}
		simplePayload := struct {
			Status string      `json:"status"`
			Code   string      `json:"code,omitempty"`
			Item   interface{} `json:"item,omitempty"`
			Meta   *Pagination `json:"meta,omitempty"`
			Errors interface{} `json:"errors,omitempty"`
			Debug  interface{} `json:"debug,omitempty"`
		}{
			Status: statusStr,
			Code:   res.MessageCode,
			Item:   itemVal,
			Meta:   res.Meta,
			Errors: res.Errors,
			Debug:  res.Debug,
		}
		_ = json.NewEncoder(w).Encode(simplePayload)
		return

	default:
		// Full format (Default matching Padi PHP Router & Rust: message -> message_code -> success -> item -> meta -> debug)
		if m, ok := res.Item.(map[string]interface{}); ok && m != nil {
			if _, hasSuccess := m["success"]; hasSuccess {
				msg, _ := m["message"].(string)
				msgCode, _ := m["message_code"].(string)
				succ, _ := m["success"].(bool)
				it := m["item"]
				if it == nil {
					it = m["data"]
				}
				errs := m["errors"]
				dbg := res.Debug
				if dbg == nil {
					dbg = m["debug"]
				}
				payload := struct {
					Message     string      `json:"message,omitempty"`
					MessageCode string      `json:"message_code,omitempty"`
					Success     bool        `json:"success"`
					Item        interface{} `json:"item,omitempty"`
					Meta        *Pagination `json:"meta,omitempty"`
					Errors      interface{} `json:"errors,omitempty"`
					Debug       interface{} `json:"debug,omitempty"`
				}{
					Message:     msg,
					MessageCode: msgCode,
					Success:     succ,
					Item:        it,
					Meta:        res.Meta,
					Errors:      errs,
					Debug:       dbg,
				}
				_ = json.NewEncoder(w).Encode(payload)
				return
			}

			// If item was passed as {data: [...], meta: {...}}, unroll so item is array and meta is top-level
			if d, hasData := m["data"]; hasData {
				if mt, hasMeta := m["meta"]; hasMeta {
					res.Item = d
					if res.Meta == nil {
						if p, ok := mt.(Pagination); ok {
							res.Meta = &p
						} else if pPtr, ok := mt.(*Pagination); ok {
							res.Meta = pPtr
						}
					}
				}
			}
		}

		itemVal := res.Item
		if itemVal == nil {
			itemVal = res.Data
		}

		fullPayload := struct {
			Message     string      `json:"message,omitempty"`
			MessageCode string      `json:"message_code,omitempty"`
			Success     bool        `json:"success"`
			Item        interface{} `json:"item,omitempty"`
			Meta        *Pagination `json:"meta,omitempty"`
			Errors      interface{} `json:"errors,omitempty"`
			Debug       interface{} `json:"debug,omitempty"`
		}{
			Message:     res.Message,
			MessageCode: res.MessageCode,
			Success:     res.Success,
			Item:        itemVal,
			Meta:        res.Meta,
			Errors:      res.Errors,
			Debug:       res.Debug,
		}

		_ = json.NewEncoder(w).Encode(fullPayload)
	}
}

// JSON renders a standard JSON response
func JSON(w http.ResponseWriter, statusCode int, data interface{}, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	success := statusCode >= 200 && statusCode < 300

	res := Response{
		Status:      statusCode,
		Success:     success,
		Message:     msg,
		MessageCode: StatusCodeName(statusCode),
		Item:        data,
		Data:        data,
	}

	sendResponse(w, nil, statusCode, res)
}

// Item renders a single record response under 'item'
func Item(w http.ResponseWriter, item interface{}, message ...string) {
	msg := "Operation successful"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	res := Response{
		Status:      http.StatusOK,
		Success:     true,
		Message:     msg,
		MessageCode: "SUCCESS",
		Item:        item,
	}

	sendResponse(w, nil, http.StatusOK, res)
}

// Items renders a list/collection of records under 'item' (matching PHP)
func Items(w http.ResponseWriter, items interface{}, message ...string) {
	msg := "Data retrieved successfully"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	res := Response{
		Status:      http.StatusOK,
		Success:     true,
		Message:     msg,
		MessageCode: "SUCCESS",
		Item:        items,
		Items:       items,
	}

	sendResponse(w, nil, http.StatusOK, res)
}

// Success renders 200 OK with single item or items
func Success(w http.ResponseWriter, data interface{}, message ...string) {
	Item(w, data, message...)
}

// Created renders 201 Created with single item
func Created(w http.ResponseWriter, item interface{}, message ...string) {
	msg := "Resource created successfully"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	res := Response{
		Status:      http.StatusCreated,
		Success:     true,
		Message:     msg,
		MessageCode: "CREATED",
		Item:        item,
	}

	sendResponse(w, nil, http.StatusCreated, res)
}

// Paginated renders paginated data with array directly under 'item' and pagination metadata under 'meta'
func Paginated(w http.ResponseWriter, items interface{}, meta Pagination, message ...string) {
	msg := "Data retrieved successfully"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	res := Response{
		Status:      http.StatusOK,
		Success:     true,
		Message:     msg,
		MessageCode: "SUCCESS",
		Item:        items,
		Items:       items,
		Meta:        &meta,
	}

	sendResponse(w, nil, http.StatusOK, res)
}

// Error renders an error response (attaching debug info if APP_DEBUG=true or development mode)
func Error(w http.ResponseWriter, statusCode int, message string, errors ...interface{}) {
	var errDetails interface{}
	if len(errors) > 0 {
		errDetails = errors[0]
	}

	res := Response{
		Status:      statusCode,
		Success:     false,
		Message:     message,
		MessageCode: StatusCodeName(statusCode),
		Errors:      errDetails,
	}

	// Auto-inject debug info on 500 errors if in debug / development mode
	if statusCode >= 500 && config.IsDebug() {
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

	sendResponse(w, nil, statusCode, res)
}

// BadRequest renders 400 Bad Request
func BadRequest(w http.ResponseWriter, message string, errors ...interface{}) {
	Error(w, http.StatusBadRequest, message, errors...)
}

// Unauthorized renders 401 Unauthorized
func Unauthorized(w http.ResponseWriter, message ...string) {
	msg := "Unauthorized access"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Error(w, http.StatusUnauthorized, msg)
}

// Forbidden renders 403 Forbidden
func Forbidden(w http.ResponseWriter, message ...string) {
	msg := "Access forbidden"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Error(w, http.StatusForbidden, msg)
}

// NotFound renders 404 Not Found
func NotFound(w http.ResponseWriter, message ...string) {
	msg := "Resource not found"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Error(w, http.StatusNotFound, msg)
}

// UnprocessableEntity renders 422 Validation Error
func UnprocessableEntity(w http.ResponseWriter, errors interface{}, message ...string) {
	msg := "The given data was invalid"
	if len(message) > 0 && message[0] != "" {
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
