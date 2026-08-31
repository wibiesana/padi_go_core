package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Topic string      `json:"topic"`
	Data  interface{} `json:"data"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]bool
}

var globalHub = &Hub{
	subscribers: make(map[string]map[chan Event]bool),
}

// Publish broadcasts an event to all subscribers listening to topic
func Publish(topic string, data interface{}) {
	globalHub.mu.RLock()
	defer globalHub.mu.RUnlock()

	channels, exists := globalHub.subscribers[topic]
	if !exists {
		return
	}

	event := Event{Topic: topic, Data: data}
	for ch := range channels {
		select {
		case ch <- event:
		default:
			// avoid blocking if client channel is full
		}
	}
}

// SubscribeSSE provides an HTTP SSE endpoint handler for clients
func SubscribeSSE(topics ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		eventChan := make(chan Event, 20)

		// Register subscriber
		globalHub.mu.Lock()
		for _, topic := range topics {
			if _, exists := globalHub.subscribers[topic]; !exists {
				globalHub.subscribers[topic] = make(map[chan Event]bool)
			}
			globalHub.subscribers[topic][eventChan] = true
		}
		globalHub.mu.Unlock()

		defer func() {
			globalHub.mu.Lock()
			for _, topic := range topics {
				if subMap, exists := globalHub.subscribers[topic]; exists {
					delete(subMap, eventChan)
					if len(subMap) == 0 {
						delete(globalHub.subscribers, topic)
					}
				}
			}
			globalHub.mu.Unlock()
			close(eventChan)
		}()

		// Send initial keepalive
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-eventChan:
				payload, _ := json.Marshal(event.Data)
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Topic, string(payload))
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}
}
