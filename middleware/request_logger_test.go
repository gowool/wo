package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

// newTestEvent creates a test event for testing purposes
func newTestEvent() *wo.Event {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// newTestEventWithMethod creates a test event with specified HTTP method
func newTestEventWithMethod(method, url string) *wo.Event {
	req := httptest.NewRequest(method, url, nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// testEvent wraps an event to simulate handler behavior for testing
type testEvent struct {
	*wo.Event
	status int
	err    error
}

func (h *testEvent) Next() error {
	if h.status > 0 {
		h.Response().WriteHeader(h.status)
	}
	return h.err
}

// newTestHandlerEvent creates a test event that returns a specific status
func newTestHandlerEvent(status int) *testEvent {
	e := newTestEvent()
	return &testEvent{Event: e, status: status}
}

// newTestErrorEvent creates a test event that returns a specific error
func newTestErrorEvent(err error) *testEvent {
	e := newTestEvent()
	return &testEvent{Event: e, err: err}
}

// newTestHandlerEventWithMethod creates a test event with specific method and status
func newTestHandlerEventWithMethod(method, url string, status int) *testEvent {
	e := newTestEventWithMethod(method, url)
	return &testEvent{Event: e, status: status}
}

// parseLogEntries parses slog JSON output from a bytes buffer
func parseLogEntries(logBuffer *bytes.Buffer) ([]map[string]interface{}, error) {
	var entries []map[string]interface{}

	lines := bytes.Split(logBuffer.Bytes(), []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// findAttribute finds a key in a log entry map
func findAttribute(entry map[string]interface{}, key string) (interface{}, bool) {
	val, ok := entry[key]
	return val, ok
}

// TestRequestLoggerNilLogger tests that the middleware panics when logger is nil
func TestRequestLoggerNilLogger(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r, "Expected panic when logger is nil")
		assert.Contains(t, r.(string), "logger is nil")
	}()

	RequestLogger[*testEvent](nil, nil)
}

// TestRequestLoggerDefaultAttrFunc tests that the middleware uses default attr function when none provided
func TestRequestLoggerDefaultAttrFunc(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	middleware := RequestLogger[*testEvent](logger, nil)

	handler := newTestHandlerEvent(http.StatusOK)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	assert.True(t, len(entries) > 0, "Expected logger entry to be created")
	assert.Equal(t, "incoming request", entries[0]["msg"])
}

// TestRequestLoggerSkip tests that the middleware respects skip function
func TestRequestLoggerSkip(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	skipFunc := func(e *testEvent) bool {
		return true
	}
	middleware := RequestLogger[*testEvent](logger, nil, skipFunc)

	handler := newTestHandlerEvent(http.StatusOK)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	assert.Equal(t, 0, len(entries), "Expected no logger entries when skipped")
}

// TestRequestLoggerSuccessStatus tests logging for successful requests
func TestRequestLoggerSuccessStatus(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.String("path", e.Request().URL.Path),
			slog.Int("status", status),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	handler := newTestHandlerEvent(http.StatusOK)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	require.Equal(t, 1, len(entries), "Expected one logger entry")
	entry := entries[0]
	assert.Equal(t, "INFO", entry["level"]) // Default level for success
	assert.Equal(t, "incoming request", entry["msg"])

	_, hasMethod := findAttribute(entry, "method")
	_, hasPath := findAttribute(entry, "path")
	_, hasStatus := findAttribute(entry, "status")
	assert.True(t, hasMethod, "Expected method attribute")
	assert.True(t, hasPath, "Expected path attribute")
	assert.True(t, hasStatus, "Expected status attribute")
}

// TestRequestLoggerClientErrorStatus tests logging for 4xx status codes
func TestRequestLoggerClientErrorStatus(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.Int("status", status),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	handler := newTestHandlerEventWithMethod("POST", "http://example.com/test", http.StatusBadRequest)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	require.Equal(t, 1, len(entries), "Expected one logger entry")
	entry := entries[0]
	assert.Equal(t, "WARN", entry["level"], "Expected warning level for 4xx status")
	assert.Equal(t, "incoming request", entry["msg"])
}

// TestRequestLoggerServerErrorStatus tests logging for 5xx status codes
func TestRequestLoggerServerErrorStatus(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.Int("status", status),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	handler := newTestHandlerEvent(http.StatusInternalServerError)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	require.Equal(t, 1, len(entries), "Expected one logger entry")
	entry := entries[0]
	assert.Equal(t, "ERROR", entry["level"], "Expected error level for 5xx status")
	assert.Equal(t, "incoming request", entry["msg"])
}

// TestRequestLoggerHandlerError tests logging when handler returns an error
func TestRequestLoggerHandlerError(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		var errMsg string
		if err != nil {
			errMsg = err.Error()
		}
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.Int("status", status),
			slog.String("error", errMsg),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	handlerErr := errors.New("handler failed")
	handler := newTestErrorEvent(handlerErr)

	err := middleware(handler)
	assert.Error(t, err)
	assert.Equal(t, handlerErr, err)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	require.Equal(t, 1, len(entries), "Expected one logger entry")
	entry := entries[0]
	assert.Equal(t, "ERROR", entry["level"], "Expected error level when handler returns error")
	assert.Equal(t, "incoming request", entry["msg"])
}

// TestRequestLoggerSuccessWithError tests that successful request with no error uses default level
func TestRequestLoggerSuccessWithError(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.Int("status", status),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	handler := newTestHandlerEvent(http.StatusAccepted)

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, wo.MustUnwrapResponse(handler.Response()).Status)

	entries, parseErr := parseLogEntries(&logBuffer)
	require.NoError(t, parseErr, "Should be able to parse log entries")
	require.Equal(t, 1, len(entries), "Expected one logger entry")
	entry := entries[0]
	assert.Equal(t, "INFO", entry["level"], "Expected default level for successful request")
	assert.Equal(t, "incoming request", entry["msg"])
}

// TestRequestLoggerContextUpdate tests that request context is updated with logged flag
func TestRequestLoggerContextUpdate(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	middleware := RequestLogger[*testEvent](logger, nil)

	handler := newTestHandlerEvent(http.StatusOK)

	// First, check that context is not marked as logged before middleware
	logged := wo.RequestLogged(handler.Request().Context())
	assert.False(t, logged, "Context should not be marked as logged before middleware completes")

	err := middleware(handler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, wo.MustUnwrapResponse(handler.Response()).Status)

	// After middleware completes, context should be updated
	logged = wo.RequestLogged(handler.Request().Context())
	assert.True(t, logged, "Context should be marked as logged after middleware completes")
}

// TestRequestLoggerMultipleSkippers tests that the middleware respects multiple skip functions
func TestRequestLoggerMultipleSkippers(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		expectSkipped bool
	}{
		{
			name:          "No skip headers",
			headers:       map[string]string{},
			expectSkipped: false,
		},
		{
			name: "First skip header",
			headers: map[string]string{
				"X-Skip-1": "true",
			},
			expectSkipped: true,
		},
		{
			name: "Second skip header",
			headers: map[string]string{
				"X-Skip-2": "true",
			},
			expectSkipped: true,
		},
		{
			name: "Both skip headers",
			headers: map[string]string{
				"X-Skip-1": "true",
				"X-Skip-2": "true",
			},
			expectSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
			skipFunc1 := func(e *testEvent) bool {
				return e.Request().Header.Get("X-Skip-1") == "true"
			}
			skipFunc2 := func(e *testEvent) bool {
				return e.Request().Header.Get("X-Skip-2") == "true"
			}
			middleware := RequestLogger[*testEvent](logger, nil, skipFunc1, skipFunc2)

			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			e := new(wo.Event)
			e.Reset(rec, req)
			handler := &testEvent{Event: e, status: http.StatusOK}

			err := middleware(handler)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, wo.MustUnwrapResponse(handler.Response()).Status)

			entries, parseErr := parseLogEntries(&logBuffer)
			require.NoError(t, parseErr, "Should be able to parse log entries")
			if tt.expectSkipped {
				assert.Equal(t, 0, len(entries), "Expected no logger entries when skipped")
			} else {
				assert.Equal(t, 1, len(entries), "Expected logger entry when not skipped")
			}
		})
	}
}

// TestRequestLoggerEdgeCases tests edge cases and boundary conditions
func TestRequestLoggerEdgeCases(t *testing.T) {
	t.Run("Empty request path", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
		middleware := RequestLogger[*testEvent](logger, nil)

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		e := new(wo.Event)
		e.Reset(rec, req)
		handler := &testEvent{Event: e, status: http.StatusOK}

		err := middleware(handler)
		assert.NoError(t, err)
		entries, parseErr := parseLogEntries(&logBuffer)
		require.NoError(t, parseErr, "Should be able to parse log entries")
		assert.Equal(t, 1, len(entries), "Expected logger entry for root path")
	})

	t.Run("Multiple calls to same event", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
		middleware := RequestLogger[*testEvent](logger, nil)

		handler := newTestHandlerEvent(http.StatusOK)

		// Call middleware multiple times on same event
		for i := 0; i < 3; i++ {
			err := middleware(handler)
			assert.NoError(t, err)
		}

		entries, parseErr := parseLogEntries(&logBuffer)
		require.NoError(t, parseErr, "Should be able to parse log entries")
		assert.Equal(t, 3, len(entries), "Expected logger entry for each middleware call")
	})

	t.Run("Request with query parameters", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
		attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
			return []slog.Attr{
				slog.String("url", e.Request().URL.String()),
				slog.String("path", e.Request().URL.Path),
				slog.String("query", e.Request().URL.RawQuery),
			}
		}
		middleware := RequestLogger[*testEvent](logger, attrFunc)

		req := httptest.NewRequest("GET", "http://example.com/test?param1=value1&param2=value2", nil)
		rec := httptest.NewRecorder()
		e := new(wo.Event)
		e.Reset(rec, req)
		handler := &testEvent{Event: e, status: http.StatusOK}

		err := middleware(handler)
		assert.NoError(t, err)
		entries, parseErr := parseLogEntries(&logBuffer)
		require.NoError(t, parseErr, "Should be able to parse log entries")
		require.Equal(t, 1, len(entries), "Expected one logger entry")

		entry := entries[0]
		if urlAttr, found := findAttribute(entry, "url"); found {
			assert.Contains(t, urlAttr.(string), "param1=value1")
		}
		if queryAttr, found := findAttribute(entry, "query"); found {
			assert.Equal(t, "param1=value1&param2=value2", queryAttr.(string))
		}
	})

	t.Run("Status 100 (Continue) - default level", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
		middleware := RequestLogger[*testEvent](logger, nil)

		handler := newTestHandlerEvent(http.StatusContinue)

		err := middleware(handler)
		assert.NoError(t, err)
		entries, parseErr := parseLogEntries(&logBuffer)
		require.NoError(t, parseErr, "Should be able to parse log entries")
		require.Equal(t, 1, len(entries), "Expected one logger entry")
		assert.Equal(t, "INFO", entries[0]["level"], "Expected default level for 1xx status")
	})

	t.Run("Status 300 (Multiple Choices) - default level", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
		middleware := RequestLogger[*testEvent](logger, nil)

		handler := newTestHandlerEvent(http.StatusMultipleChoices)

		err := middleware(handler)
		assert.NoError(t, err)
		entries, parseErr := parseLogEntries(&logBuffer)
		require.NoError(t, parseErr, "Should be able to parse log entries")
		require.Equal(t, 1, len(entries), "Expected one logger entry")
		assert.Equal(t, "INFO", entries[0]["level"], "Expected default level for 3xx status")
	})
}

// BenchmarkRequestLogger benchmarks the RequestLogger middleware
func BenchmarkRequestLogger(b *testing.B) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	middleware := RequestLogger[*testEvent](logger, nil)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logBuffer.Reset()
		e := new(wo.Event)
		e.Reset(rec, req.Clone(req.Context()))
		handler := &testEvent{Event: e, status: http.StatusOK}

		_ = middleware(handler)
	}
}

// BenchmarkRequestLoggerWithSkippers benchmarks the RequestLogger middleware with skip functions
func BenchmarkRequestLoggerWithSkippers(b *testing.B) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	skipFunc := func(e *testEvent) bool {
		return false // Never skip for benchmark
	}
	middleware := RequestLogger[*testEvent](logger, nil, skipFunc)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logBuffer.Reset()
		e := new(wo.Event)
		e.Reset(rec, req.Clone(req.Context()))
		handler := &testEvent{Event: e, status: http.StatusOK}

		_ = middleware(handler)
	}
}

// BenchmarkRequestLoggerWithAttrFunc benchmarks the RequestLogger middleware with custom attribute function
func BenchmarkRequestLoggerWithAttrFunc(b *testing.B) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	attrFunc := func(e *testEvent, status int, err error) []slog.Attr {
		return []slog.Attr{
			slog.String("method", e.Request().Method),
			slog.String("path", e.Request().URL.Path),
			slog.String("user_agent", e.Request().UserAgent()),
			slog.String("remote_addr", e.Request().RemoteAddr),
			slog.Int("status", status),
		}
	}
	middleware := RequestLogger[*testEvent](logger, attrFunc)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logBuffer.Reset()
		e := new(wo.Event)
		e.Reset(rec, req.Clone(req.Context()))
		handler := &testEvent{Event: e, status: http.StatusOK}

		_ = middleware(handler)
	}
}
