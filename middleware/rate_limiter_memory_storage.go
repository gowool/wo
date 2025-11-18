package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gowool/wo/internal/arr"
)

var _ RateLimiterStorage = (*RateLimiterMemoryStorage)(nil)

type rlMemItem struct {
	v []byte // val
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	e uint32 // exp
}

type RateLimiterMemoryStorage struct {
	timeFunc func() uint32
	data     map[string]rlMemItem // data
	mu       sync.RWMutex
}

func NewRateLimiterMemoryStorage(timestampFunc func() uint32) *RateLimiterMemoryStorage {
	store := &RateLimiterMemoryStorage{
		timeFunc: timestampFunc,
		data:     make(map[string]rlMemItem),
	}
	go store.gc(1 * time.Second)
	return store
}

// Get retrieves the value stored under key, returning nil when the entry does
// not exist or has expired.
//
// For []byte values, this returns a defensive copy to prevent callers from
// mutating the stored data. Other types are returned as-is.
func (s *RateLimiterMemoryStorage) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	if !ok || v.e != 0 && v.e <= s.timeFunc() {
		return nil, nil
	}

	return arr.Copy(v.v), nil
}

// Set stores val under key and applies the optional ttl before expiring the
// entry. A non-positive ttl keeps the item forever.
//
// String keys are defensively copied to prevent corruption from pooled buffers.
// []byte values are also copied to prevent external mutation of stored data.
// Other types are stored as-is (structs are copied by value automatically).
func (s *RateLimiterMemoryStorage) Set(_ context.Context, key string, val []byte, ttl time.Duration) error {
	var exp uint32
	if ttl > 0 {
		exp = uint32(ttl.Seconds()) + s.timeFunc()
	}

	i := rlMemItem{e: exp, v: arr.Copy(val)}
	s.mu.Lock()
	s.data[key] = i
	s.mu.Unlock()

	return nil
}

func (s *RateLimiterMemoryStorage) gc(sleep time.Duration) {
	ticker := time.NewTicker(sleep)
	defer ticker.Stop()
	var expired []string

	for range ticker.C {
		ts := s.timeFunc()
		expired = expired[:0]
		s.mu.RLock()
		for key, v := range s.data {
			if v.e != 0 && v.e <= ts {
				expired = append(expired, key)
			}
		}
		s.mu.RUnlock()

		if len(expired) == 0 {
			// avoid locking if nothing to delete
			continue
		}

		s.mu.Lock()
		// Double-checked locking.
		// We might have replaced the item in the meantime.
		for i := range expired {
			v := s.data[expired[i]]
			if v.e != 0 && v.e <= ts {
				delete(s.data, expired[i])
			}
		}
		s.mu.Unlock()
	}
}
