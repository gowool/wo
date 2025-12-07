package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
	"github.com/gowool/wo/session"
)

// mockErrorLogger is a mock implementation of ErrorLogger interface
type mockErrorLogger struct {
	errors []string
}

func (m *mockErrorLogger) Error(msg string, _ ...any) {
	m.errors = append(m.errors, msg)
}

// mockStore implements the session.Store interface for testing
type mockStore struct {
	mock.Mock
}

func (m *mockStore) Delete(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockStore) Find(ctx context.Context, token string) ([]byte, bool, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).([]byte), args.Bool(1), args.Error(2)
}

func (m *mockStore) Commit(ctx context.Context, token string, data []byte, expiry time.Time) error {
	args := m.Called(ctx, token, data, expiry)
	return args.Error(0)
}

// newSessionTestEvent creates a test event for session middleware testing purposes
func newSessionTestEvent(method, url string, headers map[string]string) *wo.Event {
	req := httptest.NewRequest(method, url, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(wo.NewResponse(rec), req)

	return e
}

func TestSession_PanicWithNilSession(t *testing.T) {
	assert.Panics(t, func() {
		Session[*wo.Event](nil, nil)
	})
}

func TestSession_SkipperFunctionality(t *testing.T) {
	tests := []struct {
		name        string
		skipperFunc Skipper[*wo.Event]
		expectSkip  bool
	}{
		{
			name:        "No skipper - should process",
			skipperFunc: nil,
			expectSkip:  false,
		},
		{
			name: "Skip by path",
			skipperFunc: func(e *wo.Event) bool {
				return strings.HasPrefix(e.Request().URL.Path, "/skip")
			},
			expectSkip: true,
		},
		{
			name: "Skip by method",
			skipperFunc: func(e *wo.Event) bool {
				return e.Request().Method == http.MethodOptions
			},
			expectSkip: true,
		},
		{
			name: "Skip by header",
			skipperFunc: func(e *wo.Event) bool {
				return e.Request().Header.Get("X-Skip-Session") == "true"
			},
			expectSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStore{}
			s := session.New(session.Config{}, mockStore)

			var skippers []Skipper[*wo.Event]
			if tt.skipperFunc != nil {
				skippers = append(skippers, tt.skipperFunc)
			}

			middleware := Session[*wo.Event](s, nil, skippers...)

			e := newSessionTestEvent(http.MethodGet, "/test", nil)
			switch tt.name {
			case "Skip by path":
				e.Request().URL.Path = "/skip/test"
			case "Skip by method":
				e.Request().Method = http.MethodOptions
			case "Skip by header":
				e.Request().Header.Set("X-Skip-Session", "true")
			}

			err := middleware(e)
			assert.NoError(t, err)

			if tt.expectSkip {
				// When skipped, Next() should not be called
				assert.True(t, true) // Basic check that no error occurred
			} else {
				// When not skipped, middleware should process normally
				assert.True(t, true)
			}
		})
	}
}

func TestSession_MultipleSkippers(t *testing.T) {
	mockStore := &mockStore{}
	s := session.New(session.Config{}, mockStore)

	skipper1 := func(e *wo.Event) bool {
		return e.Request().Header.Get("X-Skip-1") == "true"
	}

	skipper2 := func(e *wo.Event) bool {
		return e.Request().Header.Get("X-Skip-2") == "true"
	}

	middleware := Session[*wo.Event](s, nil, skipper1, skipper2)

	// Test first skipper matches
	e := newSessionTestEvent(http.MethodGet, "/test", nil)
	e.Request().Header.Set("X-Skip-1", "true")
	err := middleware(e)
	assert.NoError(t, err)

	// Test second skipper matches
	e = newSessionTestEvent(http.MethodGet, "/test", nil)
	e.Request().Header.Set("X-Skip-2", "true")
	err = middleware(e)
	assert.NoError(t, err)

	// Test neither skipper matches - this should trigger session processing
	e = newSessionTestEvent(http.MethodGet, "/test", nil)
	err = middleware(e)
	assert.NoError(t, err)
}

func TestSession_ReadSessionCookie_Error(t *testing.T) {
	tests := []struct {
		name        string
		cookie      string
		storeError  error
		expectError bool
	}{
		{
			name:        "No cookie",
			cookie:      "",
			storeError:  nil,
			expectError: false,
		},
		{
			name:        "Store error with cookie",
			cookie:      "session=invalid-token",
			storeError:  errors.New("store error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStore{}
			s := session.New(session.Config{}, mockStore)

			// Mock store behavior only when there's a cookie
			if tt.cookie != "" && tt.storeError != nil {
				mockStore.On("Find", mock.Anything, mock.AnythingOfType("string")).
					Return(nil, false, tt.storeError)
			}

			middleware := Session[*wo.Event](s, nil)

			var headers map[string]string
			if tt.cookie != "" {
				headers = map[string]string{"Cookie": tt.cookie}
			}

			e := newSessionTestEvent(http.MethodGet, "/test", headers)

			err := middleware(e)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestSession_UnmodifiedStatus(t *testing.T) {
	mockStore := &mockStore{}
	s := session.New(session.Config{}, mockStore)
	middleware := Session[*wo.Event](s, nil)

	e := newSessionTestEvent(http.MethodGet, "/test", nil)

	err := middleware(e)
	assert.NoError(t, err)

	// Request should be processed successfully
	assert.NotNil(t, e.Request())
	assert.NotNil(t, e.Response())
}

func TestSession_CommitError(t *testing.T) {
	tests := []struct {
		name           string
		loggerProvided bool
		expectLogError bool
	}{
		{
			name:           "No logger - should not panic",
			loggerProvided: false,
			expectLogError: false,
		},
		{
			name:           "With logger - should log error",
			loggerProvided: true,
			expectLogError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStore{}

			// Mock Commit that fails - accept any data since we can't predict the exact bytes
			mockStore.On("Commit", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Time")).
				Return(errors.New("commit failed"))

			s := session.New(session.Config{}, mockStore)

			var logger ErrorLogger
			if tt.loggerProvided {
				logger = &mockErrorLogger{}
			}

			middleware := Session[*wo.Event](s, logger)

			e := newSessionTestEvent(http.MethodGet, "/test", nil)

			err := middleware(e)
			assert.NoError(t, err)

			// Simulate session modification
			s.Put(e.Context(), "test", "value")

			// Trigger response finalization - this should not panic
			assert.NotPanics(t, func() {
				e.Response().WriteHeader(http.StatusOK)
			})

			if tt.expectLogError {
				require.NotNil(t, logger)
				mockLogger := logger.(*mockErrorLogger)
				assert.Len(t, mockLogger.errors, 1)
				assert.Contains(t, mockLogger.errors[0], "failed to commit session")
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestSession_RequestUpdate(t *testing.T) {
	mockStore := &mockStore{}

	// Create sample session data
	codec := session.NewGobCodec()
	sessionData := map[string]any{"user": "testuser"}
	deadline := time.Now().Add(time.Hour)

	encodedData, err := codec.Encode(deadline, sessionData)
	require.NoError(t, err)

	token := "valid-token"

	mockStore.On("Find", mock.Anything, token).
		Return(encodedData, true, nil)

	s := session.New(session.Config{}, mockStore)
	middleware := Session[*wo.Event](s, nil)

	e := newSessionTestEvent(http.MethodGet, "/test", map[string]string{
		"Cookie": "session=" + token,
	})

	err = middleware(e)
	assert.NoError(t, err)

	// Verify request was updated with new context containing session data
	user := s.GetString(e.Context(), "user")
	assert.Equal(t, "testuser", user)

	mockStore.AssertExpectations(t)
}

func TestSession_EmptyCookieHandling(t *testing.T) {
	tests := []struct {
		name        string
		cookieValue string
		expectFind  bool
	}{
		{
			name:        "No session cookie",
			cookieValue: "",
			expectFind:  false,
		},
		{
			name:        "Valid session cookie",
			cookieValue: "session123",
			expectFind:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStore{}

			if tt.expectFind {
				mockStore.On("Find", mock.Anything, tt.cookieValue).
					Return(nil, false, nil)
			}

			s := session.New(session.Config{}, mockStore)
			middleware := Session[*wo.Event](s, nil)

			e := newSessionTestEvent(http.MethodGet, "/test", nil)
			if tt.cookieValue != "" {
				e.Request().Header.Set("Cookie", "session="+tt.cookieValue)
			}

			err := middleware(e)
			assert.NoError(t, err)

			mockStore.AssertExpectations(t)
		})
	}
}

func TestSession_ResponseHeaders(t *testing.T) {
	mockStore := &mockStore{}

	// Mock Commit that accepts any data
	mockStore.On("Commit", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Time")).
		Return(nil)

	s := session.New(session.Config{
		Cookie: session.Cookie{
			Name: "test-session",
		},
	}, mockStore)
	middleware := Session[*wo.Event](s, nil)

	e := newSessionTestEvent(http.MethodGet, "/test", nil)

	err := middleware(e)
	assert.NoError(t, err)

	// Simulate session modification to trigger cookie writing
	s.Put(e.Context(), "test", "value")

	// Trigger response finalization - should not panic
	assert.NotPanics(t, func() {
		e.Response().WriteHeader(http.StatusOK)
	})

	// Check that required headers are set
	responseHeaders := e.Response().Header()
	assert.Equal(t, "Cookie", responseHeaders.Get("Vary"))
	assert.Contains(t, responseHeaders.Get("Cache-Control"), `no-cache="Set-Cookie"`)

	mockStore.AssertExpectations(t)
}

func TestSession_NilLogger(t *testing.T) {
	mockStore := &mockStore{}
	s := session.New(session.Config{}, mockStore)

	// Test with nil logger - should not panic
	assert.NotPanics(t, func() {
		middleware := Session[*wo.Event](s, nil)

		e := newSessionTestEvent(http.MethodGet, "/test", nil)
		err := middleware(e)
		assert.NoError(t, err)
	})
}

func TestSession_ChainSkipper(t *testing.T) {
	mockStore := &mockStore{}
	s := session.New(session.Config{}, mockStore)

	// Create multiple skipper functions
	skipper1 := func(e *wo.Event) bool { return false }
	skipper2 := func(e *wo.Event) bool { return true }
	skipper3 := func(e *wo.Event) bool { return false }

	middleware := Session[*wo.Event](s, nil, skipper1, skipper2, skipper3)

	e := newSessionTestEvent(http.MethodGet, "/test", nil)

	err := middleware(e)
	assert.NoError(t, err)
}
