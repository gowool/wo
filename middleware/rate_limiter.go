package middleware

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gowool/wo"
)

// ErrRateLimitExceeded denotes an error raised when rate limit is exceeded
var ErrRateLimitExceeded = wo.ErrTooManyRequests.WithMessage("rate limit exceeded")

// ErrExtractorError denotes an error raised when extractor function is unsuccessful
var ErrExtractorError = wo.ErrForbidden.WithMessage("error while extracting identifier")

type RateLimiterStorage interface {
	// Get gets the value for the given key with a context.
	// `nil, nil` is returned when the key does not exist
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores the given value for the given key with an expiration value.
	Set(ctx context.Context, key string, value []byte, exp time.Duration) error
}

type RateLimiterConfig[T wo.Resolver] struct {
	// Storage is used to store the state of the middleware
	//
	// Default: in memory storage
	Storage RateLimiterStorage `json:"-" yaml:"-"`

	// TimestampFunc return current unix timestamp (seconds)
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	//
	// Default: func() uint32 {
	//   return uint32(time.Now().Unix())
	// }
	TimestampFunc func() uint32 `json:"-" yaml:"-"`

	// IdentifierExtractor uses T wo.Resolver to extract the identifier, by default c.Request().RemoteAddr is used
	//
	// Default: func(c T) string {
	//   return c.Request().RemoteAddr
	// }
	IdentifierExtractor func(T) (string, error) `json:"-" yaml:"-"`

	// Max number of recent connections during `Expiration` seconds before sending a 429 response
	//
	// Default: 5
	Max uint `env:"MAX" json:"max,omitempty" yaml:"max,omitempty"`

	// MaxFunc a function to dynamically calculate the max requests supported by the rate limiter middleware
	//
	// Default: func(T) int {
	//   return c.Max
	// }
	MaxFunc func(T) uint `json:"-" yaml:"-"`

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration `env:"EXPIRATION" json:"expiration,omitempty" yaml:"expiration,omitempty"`

	// ExpirationFunc a function to dynamically calculate the expiration supported by the rate limiter middleware
	//
	// Default: func(T) time.Duration {
	//   return c.Expiration
	// }
	ExpirationFunc func(T) time.Duration `json:"-" yaml:"-"`

	// When set to true, the middleware will not include the rate limit headers (X-RateLimit-* and Retry-After) in the response.
	//
	// Default: false
	DisableHeaders bool `env:"DISABLE_HEADERS" json:"disableHeaders,omitempty" yaml:"disableHeaders,omitempty"`

	// DisableValueRedaction turns off masking limiter keys in logs and error messages when set to true.
	//
	// Default: false
	DisableValueRedaction bool `env:"DISABLE_VALUE_REDACTION" json:"disableValueRedaction,omitempty" yaml:"disableValueRedaction,omitempty"`
}

func (c *RateLimiterConfig[T]) SetDefaults() {
	if c.TimestampFunc == nil {
		c.TimestampFunc = timestampFunc
	}

	if c.Storage == nil {
		c.Storage = NewRateLimiterMemoryStorage(c.TimestampFunc)
	}

	if c.IdentifierExtractor == nil {
		c.IdentifierExtractor = func(t T) (string, error) {
			return t.Request().RemoteAddr, nil
		}
	}

	if c.Max == 0 {
		c.Max = 5
	}
	if c.MaxFunc == nil {
		c.MaxFunc = func(_ T) uint {
			return c.Max
		}
	}

	if c.Expiration == 0 {
		c.Expiration = 1 * time.Minute
	}
	if c.ExpirationFunc == nil {
		c.ExpirationFunc = func(_ T) time.Duration {
			return c.Expiration
		}
	}
}

// RateLimiter middleware implements the sliding-window rate limiting strategy
func RateLimiter[T wo.Resolver](cfg RateLimiterConfig[T], skippers ...Skipper[T]) func(T) error {
	cfg.SetDefaults()

	skip := ChainSkipper[T](skippers...)

	maxFunc := func(t T) int {
		if m := cfg.MaxFunc(t); m > 0 {
			return int(m)
		}
		return int(cfg.Max)
	}

	expirationFunc := func(t T) uint64 {
		if exp := cfg.ExpirationFunc(t); exp > 0 {
			return uint64(exp.Seconds())
		}
		return uint64(cfg.Expiration.Seconds())
	}

	manager := newRateLimiterManager(cfg.Storage, !cfg.DisableValueRedaction)

	mux := new(sync.RWMutex)

	return func(e T) error {
		if skip(e) {
			return e.Next()
		}

		key, err := cfg.IdentifierExtractor(e)
		if err != nil {
			return ErrExtractorError.WithInternal(fmt.Errorf("rate_limiter: failed to extract identifier: %w", err))
		}

		maxRequests := maxFunc(e)
		expiration := expirationFunc(e)

		// Lock entry
		mux.Lock()

		reqCtx := e.Request().Context()

		// Get entry from pool and release when finished
		entry, err := manager.get(reqCtx, key)
		if err != nil {
			mux.Unlock()
			return err
		}

		// Get timestamp
		ts := uint64(cfg.TimestampFunc())

		// Set expiration if entry does not exist
		if entry.exp == 0 {
			entry.exp = ts + expiration
		} else if ts >= entry.exp {
			// The entry has expired, handle the expiration.
			// Set the prevHits to the current hits and reset the hits to 0.
			entry.prevHits = entry.currHits

			// Reset the current hits to 0.
			entry.currHits = 0

			// Check how much into the current window it currently is and sets the
			// expiry based on that; otherwise, this would only reset on
			// the next request and not show the correct expiry.
			elapsed := ts - entry.exp
			if elapsed >= expiration {
				entry.exp = ts + expiration
			} else {
				entry.exp = ts + expiration - elapsed
			}
		}

		// Increment hits
		entry.currHits++

		// Calculate when it resets in seconds
		resetInSec := entry.exp - ts

		// weight = time until current window reset / total window length
		weight := float64(resetInSec) / float64(expiration)

		// rate = request count in previous window - weight + request count in current window
		rate := int(float64(entry.prevHits)*weight) + entry.currHits

		// Calculate how many hits can be made based on the current rate
		remaining := maxRequests - rate

		// Update storage. Garbage collect when the next window ends.
		// |--------------------------|--------------------------|
		//               ^            ^               ^          ^
		//              ts         e.exp   End sample window   End next window
		//               <------------>
		// 				   Reset In Sec
		// resetInSec = e.exp - ts - time until end of current window.
		// duration + expiration = end of next window.
		// Because we don't want to garbage collect in the middle of a window
		// we add the expiration to the duration.
		// Otherwise, after the end of "sample window", attackers could launch
		// a new request with the full window length.
		if setErr := manager.set(reqCtx, key, entry, time.Duration(resetInSec+expiration)*time.Second); setErr != nil { //nolint:gosec // Not a concern
			mux.Unlock()
			return fmt.Errorf("rate_limiter: failed to persist state: %w", setErr)
		}

		// Unlock entry
		mux.Unlock()

		// Check if hits exceed the cfg.Max
		if remaining < 0 {
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			if !cfg.DisableHeaders {
				e.Response().Header().Set(wo.HeaderRetryAfter, strconv.FormatUint(resetInSec, 10))
			}
			return ErrRateLimitExceeded
		}

		if !cfg.DisableHeaders {
			e.Response().Header().Set(wo.HeaderXRateLimitLimit, strconv.Itoa(maxRequests))
			e.Response().Header().Set(wo.HeaderXRateLimitRemaining, strconv.Itoa(remaining))
			e.Response().Header().Set(wo.HeaderXRateLimitReset, strconv.FormatUint(resetInSec, 10))
		}

		return e.Next()
	}
}

func timestampFunc() uint32 {
	return uint32(time.Now().Unix())
}
