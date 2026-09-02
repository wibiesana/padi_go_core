# 📡 Real-Time Server-Sent Events (SSE) Guide

`padi_go_core/realtime` provides real-time event streaming and topic pub/sub using native Go channels and HTTP Server-Sent Events (SSE) with no third-party services required.

---

## 🌐 Setting Up SSE Endpoint in Router

Mount the SSE stream handler:

```go
r := router.New(config.AppConfig)

// 1. Dynamic topic subscription (client passes ?topic=orders or ?topics=chat,alerts)
r.Get("/events", realtime.SubscribeSSE())

// 2. Fixed topic subscription
r.Get("/notifications/stream", realtime.SubscribeSSE("notifications", "broadcasts"))
```

---

## 📢 Broadcasting & Publishing Events

```go
// 1. Publish to a specific topic
realtime.Publish("orders", map[string]interface{}{
    "order_id": 9921,
    "status":   "shipped",
    "amount":   129.50,
})

// 2. Publish batch of events
realtime.PublishBatch([]realtime.Event{
    {Topic: "chat:room_1", Data: map[string]string{"user": "Alice", "msg": "Hi!"}},
    {Topic: "chat:room_1", Data: map[string]string{"user": "Bob", "msg": "Hello!"}},
})

// 3. Broadcast to all currently active topics
realtime.Broadcast("Server maintenance scheduled in 10 minutes.")
```

---

## 📊 Topic Monitoring

```go
// Get active subscriber count for a topic
count := realtime.SubscriberCount("orders")

// List all active topics with at least one subscriber
topics := realtime.Topics()
```

---

## 💻 Client Integration (JavaScript / Browser)

```javascript
// Connect to SSE stream
const eventSource = new EventSource('/events?topic=orders');

eventSource.addEventListener('orders', (event) => {
    const data = JSON.parse(event.data);
    console.log('Received order update:', data);
});

eventSource.onerror = (err) => {
    console.error('SSE Error, reconnecting...', err);
};
```
