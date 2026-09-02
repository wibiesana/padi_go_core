package realtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/realtime"
)

func TestRealtimeSSEAndPublish(t *testing.T) {
	handler := realtime.SubscribeSSE("notifications")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler(w, req)
		close(done)
	}()

	// Allow subscriber to register
	time.Sleep(50 * time.Millisecond)

	// Publish message
	realtime.Publish("notifications", map[string]string{"message": "Hello Realtime!"})
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop SSE stream
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "event: notifications") || !strings.Contains(body, "Hello Realtime!") {
		t.Fatalf("expected SSE stream to contain published event, got:\n%s", body)
	}

	// 2. Test Batch & Broadcast without panics
	realtime.PublishBatch([]realtime.Event{
		{Topic: "chat", Data: "msg1"},
		{Topic: "chat", Data: "msg2"},
	})
	realtime.Broadcast("system announcement")

	_ = realtime.SubscriberCount("chat")
	_ = realtime.Topics()
}
