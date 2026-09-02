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

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !res.Success || res.Item == nil {
			t.Fatalf("expected success and non-nil item")
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

		var res response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.Meta == nil || res.Meta.Total != 10 {
			t.Fatalf("expected pagination metadata")
		}
	})

	// 4. Test Error helpers
	t.Run("Error Responses", func(t *testing.T) {
		w1 := httptest.NewRecorder()
		response.NotFound(w1, "Custom not found")
		if w1.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w1.Code)
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
