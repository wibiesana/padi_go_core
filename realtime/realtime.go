package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// PublishBatch broadcasts multiple events
func PublishBatch(events []Event) {
	for _, ev := range events {
		Publish(ev.Topic, ev.Data)
	}
}

// Broadcast sends an event data payload to all registered topics
func Broadcast(data interface{}) {
	globalHub.mu.RLock()
	var topics []string
	for t := range globalHub.subscribers {
		topics = append(topics, t)
	}
	globalHub.mu.RUnlock()

	for _, t := range topics {
		Publish(t, data)
	}
}

// SubscriberCount returns the number of active clients subscribed to a topic
func SubscriberCount(topic string) int {
	globalHub.mu.RLock()
	defer globalHub.mu.RUnlock()
	if subMap, exists := globalHub.subscribers[topic]; exists {
		return len(subMap)
	}
	return 0
}

// Topics returns a list of all currently active topics with at least one subscriber
func Topics() []string {
	globalHub.mu.RLock()
	defer globalHub.mu.RUnlock()
	var list []string
	for t := range globalHub.subscribers {
		list = append(list, t)
	}
	return list
}

// SubscribeSSE provides an HTTP SSE endpoint handler for clients.
// If topics are passed, it subscribes to them; otherwise it reads from ?topic= or ?topics= query params.
func SubscribeSSE(topics ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		activeTopics := topics
		if len(activeTopics) == 0 {
			if qTopics := r.URL.Query().Get("topics"); qTopics != "" {
				for _, t := range strings.Split(qTopics, ",") {
					tTrimmed := strings.TrimSpace(t)
					if tTrimmed != "" {
						activeTopics = append(activeTopics, tTrimmed)
					}
				}
			} else if qTopic := r.URL.Query().Get("topic"); qTopic != "" {
				activeTopics = append(activeTopics, strings.TrimSpace(qTopic))
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		eventChan := make(chan Event, 20)

		// Register subscriber
		globalHub.mu.Lock()
		for _, topic := range activeTopics {
			if _, exists := globalHub.subscribers[topic]; !exists {
				globalHub.subscribers[topic] = make(map[chan Event]bool)
			}
			globalHub.subscribers[topic][eventChan] = true
		}
		globalHub.mu.Unlock()

		defer func() {
			globalHub.mu.Lock()
			for _, topic := range activeTopics {
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
