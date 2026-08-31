package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wibiesana/padi_go_core/config"

	"github.com/redis/go-redis/v9"
)

// CacheItem represents a single cached entry
type CacheItem struct {
	Value     interface{} `json:"value"`
	ExpiresAt int64       `json:"expires_at"` // Unix timestamp, 0 = forever
}

// CacheManager manages L1 (memory) + L2 (redis | file) caching
type CacheManager struct {
	mu          sync.RWMutex
	driver      string
	memoryCache map[string]CacheItem
	cacheDir    string
	redisClient *redis.Client
	maxMemory   int
}

var defaultManager *CacheManager
var once sync.Once

// GetManager returns the singleton CacheManager
func GetManager() *CacheManager {
	once.Do(func() {
		cfg := config.AppConfig
		if cfg == nil {
			cfg = config.Load()
		}

		driver := config.GetEnv("CACHE_DRIVER", "memory")
		maxMemory := config.GetEnvInt("CACHE_L1_MAX", 1000)
		cacheDir := "storage/cache"
		_ = os.MkdirAll(cacheDir, 0755)

		mgr := &CacheManager{
			driver:      driver,
			memoryCache: make(map[string]CacheItem),
			cacheDir:    cacheDir,
			maxMemory:   maxMemory,
		}

		if driver == "redis" {
			redisHost := config.GetEnv("REDIS_HOST", "127.0.0.1")
			redisPort := config.GetEnv("REDIS_PORT", "6379")
			redisPass := config.GetEnv("REDIS_PASSWORD", "")
			redisDB := config.GetEnvInt("REDIS_DB", 0)

			mgr.redisClient = redis.NewClient(&redis.Options{
				Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
				Password: redisPass,
				DB:       redisDB,
			})
		}

		defaultManager = mgr
	})

	return defaultManager
}

// hashKey generates hashed bucketed file path (256 subdirectories, like PHP)
func (m *CacheManager) hashKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashStr := hex.EncodeToString(hasher.Sum(nil))
	return filepath.Join(m.cacheDir, hashStr[:2], hashStr+".cache")
}

// evictIfNeeded evicts oldest 25% of L1 when it exceeds maxMemory (must be called under write lock)
func (m *CacheManager) evictIfNeeded() {
	if len(m.memoryCache) < m.maxMemory {
		return
	}
	// Evict ~25% of entries (oldest by expiry)
	evictCount := m.maxMemory / 4
	count := 0
	for k := range m.memoryCache {
		if count >= evictCount {
			break
		}
		delete(m.memoryCache, k)
		count++
	}
}

// fileWrite performs atomic write: write to tmp → rename (prevents partial reads under concurrency)
func (m *CacheManager) fileWrite(key string, item CacheItem) error {
	filePath := m.hashKey(key)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	// Write to tmp file first, then atomically rename
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, filePath)
}

// fileRead reads and validates a cache file. Returns (item, found)
func (m *CacheManager) fileRead(key string) (CacheItem, bool) {
	filePath := m.hashKey(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return CacheItem{}, false
	}

	var item CacheItem
	if err := json.Unmarshal(data, &item); err != nil {
		_ = os.Remove(filePath)
		return CacheItem{}, false
	}

	if item.ExpiresAt > 0 && time.Now().Unix() >= item.ExpiresAt {
		_ = os.Remove(filePath)
		return CacheItem{}, false
	}

	return item, true
}

// ─── Public API ──────────────────────────────────────────────────────────────

// Set stores a key-value pair in cache with expiration
func Set(key string, val interface{}, ttl time.Duration) error {
	m := GetManager()

	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}

	item := CacheItem{Value: val, ExpiresAt: expiresAt}

	// L1 Memory
	m.mu.Lock()
	m.evictIfNeeded()
	m.memoryCache[key] = item
	m.mu.Unlock()

	// L2 Redis
	if m.driver == "redis" && m.redisClient != nil {
		bytes, err := json.Marshal(val)
		if err == nil {
			return m.redisClient.Set(context.Background(), key, bytes, ttl).Err()
		}
	}

	// L2 File (atomic write)
	if m.driver == "file" {
		return m.fileWrite(key, item)
	}

	return nil
}

// Get retrieves a value from cache. L1 → L2 lookup order.
func Get(key string, target interface{}) bool {
	m := GetManager()

	// L1 Memory
	m.mu.RLock()
	item, exists := m.memoryCache[key]
	m.mu.RUnlock()

	if exists {
		if item.ExpiresAt == 0 || time.Now().Unix() < item.ExpiresAt {
			data, _ := json.Marshal(item.Value)
			_ = json.Unmarshal(data, target)
			return true
		}
		m.mu.Lock()
		delete(m.memoryCache, key)
		m.mu.Unlock()
	}

	// L2 Redis
	if m.driver == "redis" && m.redisClient != nil {
		val, err := m.redisClient.Get(context.Background(), key).Result()
		if err == nil {
			if err := json.Unmarshal([]byte(val), target); err == nil {
				return true
			}
		}
	}

	// L2 File
	if m.driver == "file" {
		fileItem, found := m.fileRead(key)
		if found {
			itemBytes, _ := json.Marshal(fileItem.Value)
			_ = json.Unmarshal(itemBytes, target)
			// Promote to L1
			m.mu.Lock()
			m.memoryCache[key] = fileItem
			m.mu.Unlock()
			return true
		}
	}

	return false
}

// Has checks if a key exists and is not expired
func Has(key string) bool {
	m := GetManager()

	// L1
	m.mu.RLock()
	item, exists := m.memoryCache[key]
	m.mu.RUnlock()
	if exists && (item.ExpiresAt == 0 || time.Now().Unix() < item.ExpiresAt) {
		return true
	}

	// L2 Redis
	if m.driver == "redis" && m.redisClient != nil {
		n, err := m.redisClient.Exists(context.Background(), key).Result()
		return err == nil && n > 0
	}

	// L2 File
	if m.driver == "file" {
		_, found := m.fileRead(key)
		return found
	}

	return false
}

// Delete removes a key from cache (L1 + L2)
func Delete(key string) error {
	m := GetManager()

	m.mu.Lock()
	delete(m.memoryCache, key)
	m.mu.Unlock()

	if m.driver == "redis" && m.redisClient != nil {
		_ = m.redisClient.Del(context.Background(), key).Err()
	}

	if m.driver == "file" {
		_ = os.Remove(m.hashKey(key))
	}

	return nil
}

// DeleteMany removes multiple keys at once
func DeleteMany(keys []string) int {
	m := GetManager()

	m.mu.Lock()
	for _, k := range keys {
		delete(m.memoryCache, k)
	}
	m.mu.Unlock()

	if m.driver == "redis" && m.redisClient != nil {
		n, err := m.redisClient.Del(context.Background(), keys...).Result()
		if err == nil {
			return int(n)
		}
	}

	deleted := 0
	if m.driver == "file" {
		for _, k := range keys {
			if err := os.Remove(m.hashKey(k)); err == nil {
				deleted++
			}
		}
	}

	return deleted
}

// Increment atomically increments a numeric cached value by step (default 1)
func Increment(key string, step ...int64) (int64, error) {
	m := GetManager()
	delta := int64(1)
	if len(step) > 0 {
		delta = step[0]
	}

	// Redis: INCRBY is natively atomic
	if m.driver == "redis" && m.redisClient != nil {
		n, err := m.redisClient.IncrBy(context.Background(), key, delta).Result()
		if err != nil {
			return 0, err
		}
		// Backfill L1
		m.mu.Lock()
		m.memoryCache[key] = CacheItem{Value: n, ExpiresAt: 0}
		m.mu.Unlock()
		return n, nil
	}

	// Memory: use atomic counter via mutex
	m.mu.Lock()
	defer m.mu.Unlock()

	item := m.memoryCache[key]
	current := int64(0)
	if item.Value != nil {
		switch v := item.Value.(type) {
		case float64:
			current = int64(v)
		case int64:
			current = v
		case int:
			current = int64(v)
		}
	}

	newVal := current + delta
	m.memoryCache[key] = CacheItem{Value: newVal, ExpiresAt: item.ExpiresAt}

	// Persist to file if file driver
	if m.driver == "file" {
		_ = m.fileWrite(key, CacheItem{Value: newVal, ExpiresAt: item.ExpiresAt})
	}

	return newVal, nil
}

// Decrement atomically decrements a numeric cached value
func Decrement(key string, step ...int64) (int64, error) {
	delta := int64(1)
	if len(step) > 0 {
		delta = step[0]
	}
	return Increment(key, -delta)
}

// Remember gets cached value or computes and caches it
func Remember(key string, ttl time.Duration, target interface{}, fallback func() (interface{}, error)) error {
	if Get(key, target) {
		return nil
	}

	val, err := fallback()
	if err != nil {
		return err
	}

	if err := Set(key, val, ttl); err != nil {
		return err
	}

	data, _ := json.Marshal(val)
	return json.Unmarshal(data, target)
}

// Flush clears all cache entries (L1 + L2)
func Flush() error {
	m := GetManager()

	m.mu.Lock()
	m.memoryCache = make(map[string]CacheItem)
	m.mu.Unlock()

	if m.driver == "redis" && m.redisClient != nil {
		_ = m.redisClient.FlushDB(context.Background()).Err()
	}

	if m.driver == "file" {
		_ = os.RemoveAll(m.cacheDir)
		_ = os.MkdirAll(m.cacheDir, 0755)
	}

	return nil
}

// Cleanup removes expired file cache entries. Returns count of deleted files.
func Cleanup() int {
	m := GetManager()
	if m.driver != "file" {
		return 0 // Redis handles TTL natively
	}

	var deleted int32
	now := time.Now().Unix()

	_ = filepath.WalkDir(m.cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".cache" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			_ = os.Remove(path)
			atomic.AddInt32(&deleted, 1)
			return nil
		}

		var item CacheItem
		if err := json.Unmarshal(data, &item); err != nil {
			_ = os.Remove(path)
			atomic.AddInt32(&deleted, 1)
			return nil
		}

		if item.ExpiresAt > 0 && now >= item.ExpiresAt {
			_ = os.Remove(path)
			atomic.AddInt32(&deleted, 1)
		}

		return nil
	})

	return int(atomic.LoadInt32(&deleted))
}

// GetMemorySize returns current L1 cache entry count (for monitoring)
func GetMemorySize() int {
	m := GetManager()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.memoryCache)
}
