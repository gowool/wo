package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

func newRLEvent() *wo.Event {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1"

	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(wo.NewResponse(rec), req)

	return e
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	rl := RateLimiter(RateLimiterConfig[*wo.Event]{
		Max:        2,
		Expiration: 2 * time.Second,
	})

	e := newRLEvent()
	if err := rl(e); err != nil {
		t.Fatalf("rate limiter 1: %v", err)
	}

	require.Equal(t, "2", e.Response().Header().Get(wo.HeaderXRateLimitLimit))
	require.Equal(t, "1", e.Response().Header().Get(wo.HeaderXRateLimitRemaining))
	require.Equal(t, "2", e.Response().Header().Get(wo.HeaderXRateLimitReset))

	time.Sleep(time.Second + 500*time.Millisecond)

	e = newRLEvent()
	if err := rl(e); err != nil {
		t.Fatalf("rate limiter 2: %v", err)
	}

	require.Equal(t, "2", e.Response().Header().Get(wo.HeaderXRateLimitLimit))
	require.Equal(t, "0", e.Response().Header().Get(wo.HeaderXRateLimitRemaining))
	require.Equal(t, "2", e.Response().Header().Get(wo.HeaderXRateLimitReset))

	time.Sleep(time.Second)

	e = newRLEvent()
	if err := rl(e); err != nil {
		fmt.Println(e.Response().Header().Get(wo.HeaderRetryAfter))
		t.Fatalf("rate limiter 3: %v", err)
	}

	require.Equal(t, "2", e.Response().Header().Get(wo.HeaderXRateLimitLimit))
	require.Equal(t, "0", e.Response().Header().Get(wo.HeaderXRateLimitRemaining))
	require.Equal(t, "1", e.Response().Header().Get(wo.HeaderXRateLimitReset))

	e = newRLEvent()
	if err := rl(e); err == nil {
		t.Fatalf("rate limiter 4")
	}

	require.Equal(t, "1", e.Response().Header().Get(wo.HeaderRetryAfter))
}
