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
	"time"

	"github.com/wibiesana/padi-core/config"

	"github.com/redis/go-redis/v9"
)

type CacheItem struct {
	Value     interface{} `json:"value"`
	ExpiresAt int64       `json:"expires_at"` // Unix timestamp, 0 = forever
}

type CacheManager struct {
	mu          sync.RWMutex
	driver      string
	memoryCache map[string]CacheItem
	cacheDir    string
	redisClient *redis.Client
}

var defaultManager *CacheManager
var once sync.Once

func GetManager() *CacheManager {
	once.Do(func() {
		cfg := config.AppConfig
		if cfg == nil {
			cfg = config.Load()
		}

		driver := config.GetEnv("CACHE_DRIVER", "memory")
		cacheDir := "storage/cache"
		_ = os.MkdirAll(cacheDir, 0755)

		mgr := &CacheManager{
			driver:      driver,
			memoryCache: make(map[string]CacheItem),
			cacheDir:    cacheDir,
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

// hashKey generates hashed path for file driver
func (m *CacheManager) hashKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashStr := hex.EncodeToString(hasher.Sum(nil))
	return filepath.Join(m.cacheDir, hashStr[:2], hashStr+".json")
}

// Set stores a key-value pair in cache with expiration
func Set(key string, val interface{}, ttl time.Duration) error {
	m := GetManager()

	var expiresAt int64 = 0
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}

	item := CacheItem{
		Value:     val,
		ExpiresAt: expiresAt,
	}

	// Always write to L1 Memory
	m.mu.Lock()
	m.memoryCache[key] = item
	m.mu.Unlock()

	// Write to L2 if Redis
	if m.driver == "redis" && m.redisClient != nil {
		bytes, err := json.Marshal(val)
		if err == nil {
			return m.redisClient.Set(context.Background(), key, bytes, ttl).Err()
		}
	}

	// Write to L2 if File
	if m.driver == "file" {
		filePath := m.hashKey(key)
		_ = os.MkdirAll(filepath.Dir(filePath), 0755)
		bytes, err := json.Marshal(item)
		if err == nil {
			return os.WriteFile(filePath, bytes, 0644)
		}
	}

	return nil
}

// Get retrieves a key from cache
func Get(key string, target interface{}) bool {
	m := GetManager()

	// 1. Check Memory (L1)
	m.mu.RLock()
	item, exists := m.memoryCache[key]
	m.mu.RUnlock()

	if exists {
		if item.ExpiresAt == 0 || time.Now().Unix() < item.ExpiresAt {
			data, _ := json.Marshal(item.Value)
			_ = json.Unmarshal(data, target)
			return true
		}
		// Expired in memory
		m.mu.Lock()
		delete(m.memoryCache, key)
		m.mu.Unlock()
	}

	// 2. Check Redis
	if m.driver == "redis" && m.redisClient != nil {
		val, err := m.redisClient.Get(context.Background(), key).Result()
		if err == nil {
			if err := json.Unmarshal([]byte(val), target); err == nil {
				return true
			}
		}
	}

	// 3. Check File
	if m.driver == "file" {
		filePath := m.hashKey(key)
		data, err := os.ReadFile(filePath)
		if err == nil {
			var fileItem CacheItem
			if err := json.Unmarshal(data, &fileItem); err == nil {
				if fileItem.ExpiresAt == 0 || time.Now().Unix() < fileItem.ExpiresAt {
					itemBytes, _ := json.Marshal(fileItem.Value)
					_ = json.Unmarshal(itemBytes, target)

					// Backfill L1 Memory
					m.mu.Lock()
					m.memoryCache[key] = fileItem
					m.mu.Unlock()
					return true
				}
				_ = os.Remove(filePath)
			}
		}
	}

	return false
}

// Delete removes a key from cache
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

// Remember gets cached value or calls fallback closure and caches the result
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

// Flush clears memory and file caches
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
