package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wibiesana/padi_go_core/response"
)

func TestResponseHelpers(t *testing.T) {
	// 1. Test Item
	t.Run("Item Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"id": "1", "name": "Test Item"}
		response.Item(w, data, "Item fetched")

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &rawMap); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify PHP equivalence: success, message, message_code, item. NO "status": 200 in JSON!
		if rawMap["success"] != true {
			t.Fatalf("expected success true, got %v", rawMap["success"])
		}
		if rawMap["message_code"] != "SUCCESS" {
			t.Fatalf("expected message_code SUCCESS, got %v", rawMap["message_code"])
		}
		if _, exists := rawMap["status"]; exists {
			t.Fatalf("expected status field to NOT be present in full JSON envelope, found %v", rawMap["status"])
		}
		if rawMap["item"] == nil {
			t.Fatalf("expected non-nil item")
		}

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode Response struct: %v", err)
		}
		if !res.Success || res.Item == nil || res.MessageCode != "SUCCESS" {
			t.Fatalf("expected success and non-nil item with SUCCESS message code")
		}
	})

	// 2. Test Items
	t.Run("Items Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		items := []string{"item1", "item2"}
		response.Items(w, items, "Items list")

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &rawMap); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if rawMap["success"] != true || rawMap["message_code"] != "SUCCESS" {
			t.Fatalf("expected success true and message_code SUCCESS")
		}
		// In PHP, list/collection is wrapped in "item"
		if rawMap["item"] == nil {
			t.Fatalf("expected list to be wrapped in 'item' matching PHP")
		}

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !res.Success || res.Items == nil {
			t.Fatalf("expected success and non-nil items")
		}
	})

	// 3. Test Paginated
	t.Run("Paginated Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		items := []int{1, 2, 3}
		meta := response.Pagination{
			Total:       10,
			PerPage:     3,
			CurrentPage: 1,
			LastPage:    4,
			From:        1,
			To:          3,
		}
		response.Paginated(w, items, meta, "Paginated list")

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &rawMap); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify standard: "item" contains array directly and "meta" is top-level
		itemSlice, ok := rawMap["item"].([]interface{})
		if !ok || itemSlice == nil {
			t.Fatalf("expected item to be an array of records, got %T", rawMap["item"])
		}
		if len(itemSlice) != 3 {
			t.Fatalf("expected 3 items in item array, got %d", len(itemSlice))
		}
		metaMap, ok := rawMap["meta"].(map[string]interface{})
		if !ok || metaMap == nil {
			t.Fatalf("expected meta to be a top-level object, got %T", rawMap["meta"])
		}
		if metaMap["total"] == nil {
			t.Fatalf("expected meta.total to exist")
		}

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.Meta == nil || res.Meta.Total != 10 {
			t.Fatalf("expected pagination metadata to be populated on Response struct")
		}
		if res.Items == nil {
			t.Fatalf("expected items to be populated on Response struct")
		}
	})

	// 4. Test Created
	t.Run("Created Response", func(t *testing.T) {
		w := httptest.NewRecorder()
		response.Created(w, map[string]string{"id": "42"}, "Created user")
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
		var rawMap map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &rawMap)
		if rawMap["message_code"] != "CREATED" {
			t.Fatalf("expected CREATED message_code, got %v", rawMap["message_code"])
		}
	})

	// 5. Test Error helpers
	t.Run("Error Responses", func(t *testing.T) {
		w1 := httptest.NewRecorder()
		response.NotFound(w1, "Custom not found")
		if w1.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w1.Code)
		}
		var m1 map[string]interface{}
		_ = json.Unmarshal(w1.Body.Bytes(), &m1)
		if m1["message_code"] != "NOT_FOUND" || m1["success"] != false {
			t.Fatalf("expected NOT_FOUND and success false")
		}

		w2 := httptest.NewRecorder()
		response.BadRequest(w2, "Invalid input")
		if w2.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w2.Code)
		}

		w3 := httptest.NewRecorder()
		response.Unauthorized(w3)
		if w3.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w3.Code)
		}

		w4 := httptest.NewRecorder()
		response.Forbidden(w4)
		if w4.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", w4.Code)
		}

		w5 := httptest.NewRecorder()
		response.UnprocessableEntity(w5, map[string]string{"email": "required"})
		if w5.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", w5.Code)
		}
		var m5 map[string]interface{}
		_ = json.Unmarshal(w5.Body.Bytes(), &m5)
		if m5["message_code"] != "VALIDATION_FAILED" || m5["errors"] == nil {
			t.Fatalf("expected VALIDATION_FAILED and errors map")
		}

		w6 := httptest.NewRecorder()
		response.InternalServerError(w6)
		if w6.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w6.Code)
		}

		w7 := httptest.NewRecorder()
		response.NoContent(w7)
		if w7.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w7.Code)
		}

		w8 := httptest.NewRecorder()
		response.Conflict(w8, "Duplicate entry")
		if w8.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", w8.Code)
		}

		w9 := httptest.NewRecorder()
		response.TooManyRequests(w9)
		if w9.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", w9.Code)
		}
	})
}
