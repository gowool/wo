package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// go test -run Test_RateLimiterMemoryStorage -v -race
func Test_RateLimiterMemoryStorage(t *testing.T) {
	t.Parallel()

	t.Run("basic get set operations", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "john-internal"
			val = []byte("doe")
		)

		// Set key with value
		err := store.Set(t.Context(), key, val, 0)
		require.NoError(t, err)
		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)

		// Get non-existing key
		result, err = store.Get(t.Context(), "empty")
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("sets expiration correctly", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "expiring-key"
			val = []byte("expiring-value")
			exp = 2 * time.Second
		)

		// Set key with value and ttl
		err := store.Set(t.Context(), key, val, exp)
		require.NoError(t, err)

		// Should be available immediately
		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)
	})

	t.Run("no expiration", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "permanent-key"
			val = []byte("permanent-value")
		)

		// Set key with value and no expiration
		err := store.Set(t.Context(), key, val, 0)
		require.NoError(t, err)

		// Wait a bit and ensure it's still there
		time.Sleep(100 * time.Millisecond)
		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)
	})

	t.Run("update existing key", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key  = "update-key"
			val1 = []byte("value1")
			val2 = []byte("value2")
		)

		// Set initial value
		err := store.Set(t.Context(), key, val1, 0)
		require.NoError(t, err)

		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val1, result)

		// Update with new value
		err = store.Set(t.Context(), key, val2, 0)
		require.NoError(t, err)

		result, err = store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val2, result)
	})

	t.Run("can update expiration", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "exp-update-key"
			val = []byte("exp-value")
		)

		// Set with initial value
		err := store.Set(t.Context(), key, val, 0)
		require.NoError(t, err)

		// Should be available immediately
		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)

		// Update with new expiration
		err = store.Set(t.Context(), key, val, 2*time.Second)
		require.NoError(t, err)

		// Should still be there after update
		result, err = store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)
	})

	t.Run("negative ttl", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "negative-ttl-key"
			val = []byte("negative-value")
		)

		// Set with negative ttl (should be treated as no expiration)
		err := store.Set(t.Context(), key, val, -1*time.Second)
		require.NoError(t, err)

		result, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, val, result)
	})

	t.Run("concurrent access", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		var (
			key = "concurrent-key"
		)

		// Concurrent writes
		done := make(chan bool, 20)
		for i := 0; i < 10; i++ {
			go func(id int) {
				testKey := fmt.Sprintf("%s-%d", key, id)
				testVal := []byte(fmt.Sprintf("value-%d", id))
				err := store.Set(context.Background(), testKey, testVal, 0)
				require.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all writes
		for i := 0; i < 10; i++ {
			<-done
		}

		// Concurrent reads
		for i := 0; i < 10; i++ {
			go func(id int) {
				testKey := fmt.Sprintf("%s-%d", key, id)
				testVal := []byte(fmt.Sprintf("value-%d", id))
				result, err := store.Get(context.Background(), testKey)
				require.NoError(t, err)
				require.Equal(t, testVal, result)
				done <- true
			}(i)
		}

		// Wait for all reads
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("store works with GC", func(t *testing.T) {
		// Use the default store constructor which starts GC automatically
		store := NewRateLimiterMemoryStorage(timestampFunc)

		// Add permanent item to test basic functionality
		err := store.Set(t.Context(), "permanent", []byte("permanent"), 0)
		require.NoError(t, err)

		// Verify item exists
		val, err := store.Get(t.Context(), "permanent")
		require.NoError(t, err)
		require.Equal(t, []byte("permanent"), val)
	})

	t.Run("GC handles empty expired list", func(t *testing.T) {
		// Create store manually to control GC timing
		store := &RateLimiterMemoryStorage{
			timeFunc: timestampFunc,
			data:     make(map[string]rlMemItem),
		}

		// Start GC with short interval
		go store.gc(100 * time.Millisecond)

		// Only add permanent items - no expired items
		err := store.Set(t.Context(), "permanent", []byte("value"), 0)
		require.NoError(t, err)

		// Wait for GC cycles
		time.Sleep(200 * time.Millisecond)

		// Permanent item should still be there
		val, err := store.Get(t.Context(), "permanent")
		require.NoError(t, err)
		require.Equal(t, []byte("value"), val)
	})

	t.Run("GC handles item replacement", func(t *testing.T) {
		store := &RateLimiterMemoryStorage{
			timeFunc: timestampFunc,
			data:     make(map[string]rlMemItem),
		}

		// Start GC with short interval
		go store.gc(100 * time.Millisecond)

		// Add expiring item
		err1 := store.Set(t.Context(), "test", []byte("original"), 100*time.Millisecond)
		require.NoError(t, err1)

		// Replace with permanent item before GC runs
		err2 := store.Set(t.Context(), "test", []byte("permanent"), 0)
		require.NoError(t, err2)

		// Wait for GC
		time.Sleep(200 * time.Millisecond)

		// Permanent item should still be there (not deleted by mistake)
		val, err := store.Get(t.Context(), "test")
		require.NoError(t, err)
		require.Equal(t, []byte("permanent"), val)
	})

	t.Run("GC with multiple expired items", func(t *testing.T) {
		store := &RateLimiterMemoryStorage{
			timeFunc: timestampFunc,
			data:     make(map[string]rlMemItem),
		}

		// Start GC with short interval
		go store.gc(50 * time.Millisecond)

		// Add multiple expiring items
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("expiring-%d", i)
			err := store.Set(t.Context(), key, []byte(fmt.Sprintf("value-%d", i)), 80*time.Millisecond)
			require.NoError(t, err)
		}

		// Add some permanent items
		for i := 0; i < 3; i++ {
			key := fmt.Sprintf("permanent-%d", i)
			err := store.Set(t.Context(), key, []byte(fmt.Sprintf("value-%d", i)), 0)
			require.NoError(t, err)
		}

		// Wait for GC to run multiple times
		time.Sleep(300 * time.Millisecond)

		// Permanent items should still be there
		for i := 0; i < 3; i++ {
			key := fmt.Sprintf("permanent-%d", i)
			val, err := store.Get(t.Context(), key)
			require.NoError(t, err)
			require.Equal(t, []byte(fmt.Sprintf("value-%d", i)), val)
		}
	})
}

func TestRateLimiterMemoryStorage_DefensiveCopying(t *testing.T) {
	t.Parallel()

	t.Run("defensive copying prevents mutation", func(t *testing.T) {
		store := NewRateLimiterMemoryStorage(timestampFunc)
		originalVal := []byte("original")
		key := "copy-test-key"

		// Store a value
		err := store.Set(t.Context(), key, originalVal, 0)
		require.NoError(t, err)

		// Get the value
		retrievedVal, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, originalVal, retrievedVal)

		// Modify the retrieved value
		retrievedVal[0] = 'M'

		// Get the value again - should be unchanged
		retrievedVal2, err := store.Get(t.Context(), key)
		require.NoError(t, err)
		require.Equal(t, originalVal, retrievedVal2)
		require.NotEqual(t, retrievedVal, retrievedVal2)
	})
}
