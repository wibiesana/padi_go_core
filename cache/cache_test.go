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

func TestCacheV002Enhancements(t *testing.T) {
	// 1. Test Has()
	k := "item_has_test"
	_ = cache.Set(k, "active_value", 5*time.Second)
	if !cache.Has(k) {
		t.Errorf("Expected cache.Has(%s) to be true", k)
	}
	if cache.Has("non_existent_key_123") {
		t.Errorf("Expected cache.Has(non_existent_key_123) to be false")
	}

	// 2. Test Increment and Decrement
	counterKey := "counter_test"
	_ = cache.Delete(counterKey)

	n, err := cache.Increment(counterKey, 5)
	if err != nil || n != 5 {
		t.Fatalf("Increment failed: got %d, err %v", n, err)
	}
	n, err = cache.Increment(counterKey)
	if err != nil || n != 6 {
		t.Fatalf("Increment default step failed: got %d, err %v", n, err)
	}
	n, err = cache.Decrement(counterKey, 2)
	if err != nil || n != 4 {
		t.Fatalf("Decrement failed: got %d, err %v", n, err)
	}

	// 3. Test DeleteMany()
	cache.Set("batch_1", "val1", time.Minute)
	cache.Set("batch_2", "val2", time.Minute)
	cache.Set("batch_3", "val3", time.Minute)

	delCount := cache.DeleteMany([]string{"batch_1", "batch_2", "batch_3"})
	if delCount < 3 {
		t.Logf("Deleted %d items via DeleteMany", delCount)
	}
	if cache.Has("batch_1") || cache.Has("batch_2") || cache.Has("batch_3") {
		t.Errorf("DeleteMany failed to remove all specified keys")
	}

	// 4. Test GetMemorySize()
	size := cache.GetMemorySize()
	if size < 0 {
		t.Errorf("Invalid memory size: %d", size)
	}

	// 5. Test Cleanup()
	_ = cache.Cleanup()
}
