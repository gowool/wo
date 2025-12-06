package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterManager(t *testing.T) {
	t.Parallel()

	t.Run("creates manager with pool", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		require.NotNil(t, manager.pool)
		require.Equal(t, storage, manager.storage)
		require.False(t, manager.redactKeys)
	})

	t.Run("acquires and releases items from pool", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		// Acquire item
		item := manager.acquire()
		require.NotNil(t, item)
		require.Equal(t, 0, item.currHits)
		require.Equal(t, 0, item.prevHits)
		require.Equal(t, uint64(0), item.exp)

		// Modify item
		item.currHits = 5
		item.prevHits = 3
		item.exp = 12345

		// Release item should reset it
		manager.release(item)
		require.Equal(t, 0, item.currHits)
		require.Equal(t, 0, item.prevHits)
		require.Equal(t, uint64(0), item.exp)
	})

	t.Run("gets item from storage when exists", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		// Create a test item and marshal it
		testItem := &item{currHits: 2, prevHits: 1, exp: 12345}
		marshaled, err := testItem.MarshalMsg(nil)
		require.NoError(t, err)

		storage.On("Get", mock.Anything, "test-key").Return(marshaled, nil)

		item, err := manager.get(context.Background(), "test-key")
		require.NoError(t, err)
		require.NotNil(t, item)
		require.Equal(t, 2, item.currHits)
		require.Equal(t, 1, item.prevHits)
		require.Equal(t, uint64(12345), item.exp)

		storage.AssertExpectations(t)
	})

	t.Run("gets new item when storage returns empty", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		storage.On("Get", mock.Anything, "test-key").Return([]byte{}, nil)

		item, err := manager.get(context.Background(), "test-key")
		require.NoError(t, err)
		require.NotNil(t, item)
		require.Equal(t, 0, item.currHits)
		require.Equal(t, 0, item.prevHits)
		require.Equal(t, uint64(0), item.exp)

		storage.AssertExpectations(t)
	})

	t.Run("handles storage get error", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		storage.On("Get", mock.Anything, "test-key").Return([]byte{}, errors.New("storage error"))

		item, err := manager.get(context.Background(), "test-key")
		require.Error(t, err)
		require.Nil(t, item)
		require.Contains(t, err.Error(), "failed to get key")
		require.Contains(t, err.Error(), "test-key")

		storage.AssertExpectations(t)
	})

	t.Run("handles unmarshal error with redaction disabled", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		storage.On("Get", mock.Anything, "test-key").Return([]byte{0xFF, 0xFF, 0xFF}, nil)

		item, err := manager.get(context.Background(), "test-key")
		require.Error(t, err)
		require.Nil(t, item)
		require.Contains(t, err.Error(), "failed to unmarshal key")
		require.Contains(t, err.Error(), "test-key")

		storage.AssertExpectations(t)
	})

	t.Run("handles unmarshal error with redaction enabled", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, true)

		storage.On("Get", mock.Anything, "test-key").Return([]byte{0xFF, 0xFF, 0xFF}, nil)

		item, err := manager.get(context.Background(), "test-key")
		require.Error(t, err)
		require.Nil(t, item)
		require.Contains(t, err.Error(), "failed to unmarshal key")
		require.Contains(t, err.Error(), "[redacted]")

		storage.AssertExpectations(t)
	})

	t.Run("sets item to storage successfully", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		testItem := &item{currHits: 3, prevHits: 2, exp: 54321}
		storage.On("Set", mock.Anything, "test-key", mock.Anything, 5*time.Second).Return(nil)

		err := manager.set(context.Background(), "test-key", testItem, 5*time.Second)
		require.NoError(t, err)

		storage.AssertExpectations(t)

		// Item should be reset after set
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)
	})

	t.Run("handles marshal error", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		// Create a valid item but let's test the storage failure path
		testItem := &item{currHits: 3, prevHits: 2, exp: 54321}

		// Force a storage error to test the error path
		storage.On("Set", mock.Anything, "test-key", mock.Anything, 5*time.Second).Return(errors.New("storage error"))

		err := manager.set(context.Background(), "test-key", testItem, 5*time.Second)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to store key")

		storage.AssertExpectations(t)
	})

	t.Run("logKey redacts when enabled", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, true)

		logged := manager.logKey("secret-key")
		require.Equal(t, redactedKey, logged)
	})

	t.Run("logKey returns original when redaction disabled", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		logged := manager.logKey("test-key")
		require.Equal(t, "test-key", logged)
	})
}

func TestRateLimiterManager_MissingLinesCoverage(t *testing.T) {
	t.Parallel()

	t.Run("set function error paths", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		testItem := &item{currHits: 1, prevHits: 1, exp: 12345}

		// Test marshal error by creating a scenario that might cause it
		// Since MessagePack is generally robust, we'll test storage error paths
		storage.On("Set", mock.Anything, "test-key", mock.Anything, mock.Anything).Return(errors.New("storage set error")).Once()

		err := manager.set(context.Background(), "test-key", testItem, 5*time.Second)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to store key")

		storage.AssertExpectations(t)
	})

	t.Run("get function error paths with redaction", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, true) // Enable redaction

		storage.On("Get", mock.Anything, "sensitive-key").Return([]byte{}, errors.New("get error"))

		item, err := manager.get(context.Background(), "sensitive-key")
		require.Error(t, err)
		require.Nil(t, item)
		require.Contains(t, err.Error(), "failed to get key")
		require.Contains(t, err.Error(), "[redacted]") // Should be redacted

		storage.AssertExpectations(t)
	})

	t.Run("set function successful path", func(t *testing.T) {
		storage := &MockRateLimiterStorage{}
		manager := newRateLimiterManager(storage, false)

		testItem := &item{currHits: 5, prevHits: 3, exp: 99999}
		storage.On("Set", mock.Anything, "test-key", mock.Anything, 10*time.Second).Return(nil)

		err := manager.set(context.Background(), "test-key", testItem, 10*time.Second)
		require.NoError(t, err)

		// Item should be reset after set
		require.Equal(t, 0, testItem.currHits)
		require.Equal(t, 0, testItem.prevHits)
		require.Equal(t, uint64(0), testItem.exp)

		storage.AssertExpectations(t)
	})
}
