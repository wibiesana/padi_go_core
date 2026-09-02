# ⚡ Caching Guide

`padi_go_core/cache` provides multi-tier L1 (In-Memory) + L2 (Redis / File) caching with generic type-safety, atomic operations, and auto-fallback loaders.

---

## 🎯 Type-Safe Generic Caching

### 1. `cache.RememberTyped[T]`
Loads value from cache; if absent, executes the callback, stores the result with TTL, and returns it directly as type `T`:

```go
user, err := cache.RememberTyped[User]("user:42", 15*time.Minute, func() (User, error) {
    var u User
    err := query.New("users").Where("id", 42).First(&u)
    return u, err
})
```

### 2. `cache.GetTyped[T]`
```go
topPosts, found, err := cache.GetTyped[[]Post]("top_posts")
if found {
    fmt.Printf("Loaded %d posts from cache\n", len(topPosts))
}
```

---

## 💾 Basic Cache Operations

```go
// Store item
err := cache.Set("system:status", "online", 1*time.Hour)

// Check existence
exists := cache.Has("system:status")

// Delete single key
err = cache.Delete("system:status")

// Delete multiple keys
err = cache.DeleteMany("key1", "key2", "key3")

// Invalidate entire cache
err = cache.Clear()
```

---

## 🔢 Atomic Operations

```go
// Atomic Increment
newCount, err := cache.Increment("page_views:home", 1)

// Atomic Decrement
remainingStock, err := cache.Decrement("stock:item_99", 1)
```
