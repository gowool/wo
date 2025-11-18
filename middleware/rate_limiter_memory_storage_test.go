package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// go test -run Test_Memory -v -race
func Test_RateLimiterMemoryStorage(t *testing.T) {
	t.Parallel()
	store := NewRateLimiterMemoryStorage(timestampFunc)
	var (
		key = "john-internal"
		val = []byte("doe")
		exp = 1 * time.Second
	)

	// Set key with value
	_ = store.Set(t.Context(), key, val, 0)
	result, _ := store.Get(t.Context(), key)
	require.Equal(t, val, result)

	// Get non-existing key
	result, _ = store.Get(t.Context(), "empty")
	require.Nil(t, result)

	// Set key with value and ttl
	_ = store.Set(t.Context(), key, val, exp)
	time.Sleep(1100 * time.Millisecond)
	result, _ = store.Get(t.Context(), key)
	require.Nil(t, result)

	// Set key with value and no expiration
	_ = store.Set(t.Context(), key, val, 0)
	result, _ = store.Get(t.Context(), key)
	require.Equal(t, val, result)
}
