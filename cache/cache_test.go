package cache_test

import (
	"testing"
	"time"

	"github.com/wibiesana/padi_go_core/cache"
)

func TestCacheSetGetRemember(t *testing.T) {
	key := "test_key_user"
	expectedVal := map[string]string{"name": "Alice"}

	// 1. Test Set
	err := cache.Set(key, expectedVal, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// 2. Test Get
	var retrieved map[string]string
	found := cache.Get(key, &retrieved)
	if !found {
		t.Fatalf("Expected key %s to be found in cache", key)
	}
	if retrieved["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got %s", retrieved["name"])
	}

	// 3. Test Remember
	var rememberResult string
	err = cache.Remember("remember_key", 5*time.Second, &rememberResult, func() (interface{}, error) {
		return "cached_computation", nil
	})
	if err != nil {
		t.Fatalf("Failed to remember: %v", err)
	}
	if rememberResult != "cached_computation" {
		t.Errorf("Expected 'cached_computation', got %s", rememberResult)
	}

	// 4. Test Delete
	_ = cache.Delete(key)
	if cache.Get(key, &retrieved) {
		t.Errorf("Expected key %s to be deleted", key)
	}
}
