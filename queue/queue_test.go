package queue_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/queue"
)

func TestQueuePushAndWork(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "file:queue_memdb?mode=memory&cache=shared",
	}
	config.AppConfig = cfg

	_, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	processedName := ""

	queue.RegisterJobHandler("SendWelcomeEmail", func(payload []byte) error {
		var data struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		processedName = data.Email
		wg.Done()
		return nil
	})

	// Push Job
	err = queue.Push("SendWelcomeEmail", map[string]string{"email": "john@example.com"}, "emails")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Run Worker for 1 job in background
	go queue.Work("emails", 1)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if processedName != "john@example.com" {
			t.Fatalf("expected email 'john@example.com', got '%s'", processedName)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("queue worker timed out processing job")
	}

	// 2. Test RegisterTyped and Size
	type NotificationData struct {
		Message string `json:"message"`
	}
	var typedMsg string
	var wgTyped sync.WaitGroup
	wgTyped.Add(1)

	queue.RegisterTyped("SendNotification", func(data NotificationData) error {
		typedMsg = data.Message
		wgTyped.Done()
		return nil
	})

	_ = queue.Push("SendNotification", NotificationData{Message: "System alert"}, "alerts")

	size, err := queue.Size("alerts")
	if err != nil || size != 1 {
		t.Fatalf("expected queue size 1, got %d", size)
	}

	go queue.Work("alerts", 1)
	wgTyped.Wait()

	if typedMsg != "System alert" {
		t.Fatalf("typed handler did not receive expected message: %s", typedMsg)
	}

	// 3. Test Clear
	_ = queue.Push("SendNotification", NotificationData{Message: "Old alert"}, "alerts")
	if err := queue.Clear("alerts"); err != nil {
		t.Fatalf("queue.Clear failed: %v", err)
	}
	sizeAfterClear, _ := queue.Size("alerts")
	if sizeAfterClear != 0 {
		t.Fatalf("expected queue size 0 after clear, got %d", sizeAfterClear)
	}
}
