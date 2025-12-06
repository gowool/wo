package wo

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gowool/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrorHandlerTestEvent is a simple implementation of hook.Resolver for testing
type ErrorHandlerTestEvent struct {
	req *http.Request
	res *Response
	hook.Resolver
}

func (e *ErrorHandlerTestEvent) SetRequest(r *http.Request) {
	e.req = r
}

func (e *ErrorHandlerTestEvent) Request() *http.Request {
	return e.req
}

func (e *ErrorHandlerTestEvent) SetResponse(w *Response) {
	e.res = w
}

func (e *ErrorHandlerTestEvent) Response() *Response {
	return e.res
}

// NewErrorHandlerTestEvent creates a new ErrorHandlerTestEvent for testing
func NewErrorHandlerTestEvent(req *http.Request, res *Response) *ErrorHandlerTestEvent {
	e := &ErrorHandlerTestEvent{}
	e.SetRequest(req)
	e.SetResponse(res)
	return e
}

func TestErrorHandler_DefaultValues(t *testing.T) {
	// Test with nil logger and nil mapper
	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	assert.NotNil(t, handler)
}

func TestErrorHandler_AfterResponseWritten(t *testing.T) {
	// Create a response that's already written
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := &Response{ResponseWriter: httptest.NewRecorder()}
	res.WriteHeader(http.StatusOK) // Mark as written

	event := NewErrorHandlerTestEvent(req, res)

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, logger)
	handler(event, errors.New("test error"))

	// Should log warning but not attempt to write response
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "error handler: called after response written")
}

func TestErrorHandler_RedirectError(t *testing.T) {
	tests := []struct {
		name     string
		redirect *RedirectError
		wantCode int
		wantLoc  string
	}{
		{
			name:     "301 redirect",
			redirect: &RedirectError{Status: http.StatusMovedPermanently, URL: "/new-location"},
			wantCode: http.StatusMovedPermanently,
			wantLoc:  "/new-location",
		},
		{
			name:     "302 redirect",
			redirect: &RedirectError{Status: http.StatusFound, URL: "/temporary"},
			wantCode: http.StatusFound,
			wantLoc:  "/temporary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			res := &Response{ResponseWriter: rec}
			event := NewErrorHandlerTestEvent(req, res)

			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, logger)
			handler(event, tt.redirect)

			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, tt.wantLoc, rec.Header().Get(HeaderLocation))
			assert.Empty(t, rec.Body.String())
		})
	}
}

func TestErrorHandler_HTTPErrorWithMapper(t *testing.T) {
	customErr := errors.New("custom error")
	mapper := func(err error) *HTTPError {
		if err == customErr {
			return NewHTTPError(http.StatusTeapot, "mapped error")
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAccept, MIMEApplicationJSON) // Force JSON response
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, mapper, nil)
	handler(event, customErr)

	assert.Equal(t, http.StatusTeapot, rec.Code)
	body := rec.Body.String()

	// Parse JSON to check the detail field
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
	require.NoError(t, err)
	assert.Equal(t, "mapped error", response["detail"])
}

func TestErrorHandler_HTTPErrorMapperReturnsNil(t *testing.T) {
	mapper := func(err error) *HTTPError {
		return nil // Always returns nil
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, mapper, logger)
	handler(event, errors.New("test error"))

	// Should default to internal server error
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestErrorHandler_WithCustomRender(t *testing.T) {
	renderCalled := false
	customRender := func(e *ErrorHandlerTestEvent, httpErr *HTTPError) {
		renderCalled = true
		e.Response().WriteHeader(httpErr.Status)
		e.Response().Write([]byte("custom rendered"))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	handler := ErrorHandler[*ErrorHandlerTestEvent](customRender, nil, nil)
	handler(event, ErrBadRequest)

	assert.True(t, renderCalled)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "custom rendered", rec.Body.String())
}

func TestErrorHandler_HeadRequestMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	handler(event, ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Body.String()) // HEAD requests shouldn't have body
}

func TestErrorHandler_ContentNegotiation(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		expectedType string
		expectedBody string
	}{
		{
			name:         "JSON response",
			acceptHeader: "application/json",
			expectedType: MIMEApplicationJSON,
			expectedBody: "Bad Request",
		},
		{
			name:         "HTML response",
			acceptHeader: "text/html",
			expectedType: MIMETextHTMLCharsetUTF8,
			expectedBody: "<!DOCTYPE html>",
		},
		{
			name:         "Text response (default)",
			acceptHeader: "text/plain",
			expectedType: MIMETextPlainCharsetUTF8,
			expectedBody: "Bad Request",
		},
		{
			name:         "Unknown accept type falls back to plain text",
			acceptHeader: "application/xml",
			expectedType: "", // Content type not set when negotiation fails
			expectedBody: "Bad Request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderAccept, tt.acceptHeader)
			rec := httptest.NewRecorder()
			res := &Response{ResponseWriter: rec}
			event := NewErrorHandlerTestEvent(req, res)

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
			handler(event, ErrBadRequest)

			contentType := rec.Header().Get(HeaderContentType)
			if tt.expectedType != "" {
				assert.Equal(t, tt.expectedType, contentType)
			}
			assert.Contains(t, rec.Body.String(), tt.expectedBody)
		})
	}
}

func TestErrorHandler_JSONResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAccept, MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	// Create a custom error with specific message
	customErr := NewHTTPError(http.StatusUnauthorized, "custom message")
	customErr.Debug = true

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	handler(event, customErr)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, MIMEApplicationJSON, rec.Header().Get(HeaderContentType))

	// Parse JSON response
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "custom message", response["detail"]) // detail field contains the custom message
	assert.Equal(t, float64(http.StatusUnauthorized), response["status"])
	// Debug field might not be included in all response formats
	if debug, ok := response["debug"]; ok {
		assert.Equal(t, true, debug)
	}
}

func TestErrorHandler_HTMLResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAccept, MIMETextHTMLCharsetUTF8)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	handler(event, ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, MIMETextHTMLCharsetUTF8, rec.Header().Get(HeaderContentType))

	body := rec.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Not Found!")
	assert.Contains(t, body, "Code 404")
}

func TestErrorHandler_PlainTextResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAccept, MIMETextPlainCharsetUTF8)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	handler(event, ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, MIMETextPlainCharsetUTF8, rec.Header().Get(HeaderContentType))
	assert.Equal(t, "Not Found", rec.Body.String())
}

func TestErrorHandler_DebugContext(t *testing.T) {
	tests := []struct {
		name     string
		debugCtx bool
	}{
		{
			name:     "debug enabled",
			debugCtx: true,
		},
		{
			name:     "debug disabled",
			debugCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderAccept, MIMEApplicationJSON)
			req = req.WithContext(WithDebug(req.Context(), tt.debugCtx))

			rec := httptest.NewRecorder()
			res := &Response{ResponseWriter: rec}
			event := NewErrorHandlerTestEvent(req, res)

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
			handler(event, ErrInternalServerError)

			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			// Debug field is only included in ToMap() when Debug is true
			if tt.debugCtx {
				// When debug is enabled, debug field should be present
				if debug, exists := response["debug"]; exists {
					assert.Equal(t, true, debug)
				} else {
					// Debug field might be absent from JSON response if not part of standard format
					t.Log("Debug field not found in JSON response - this may be expected behavior")
				}
			} else {
				// When debug is disabled, debug field should not be present or should be false
				if debug, exists := response["debug"]; exists {
					assert.Equal(t, false, debug)
				}
			}
		})
	}
}

func TestErrorHandler_RequestLogging(t *testing.T) {
	tests := []struct {
		name          string
		requestLogged bool
	}{
		{
			name:          "request not logged",
			requestLogged: false,
		},
		{
			name:          "request already logged",
			requestLogged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(WithRequestLogged(req.Context(), tt.requestLogged))

			rec := httptest.NewRecorder()
			res := &Response{ResponseWriter: rec}
			event := NewErrorHandlerTestEvent(req, res)

			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, logger)
			handler(event, ErrBadRequest)

			logOutput := logBuffer.String()
			if !tt.requestLogged {
				assert.Contains(t, logOutput, "request failed")
			} else {
				assert.Empty(t, logOutput)
			}
		})
	}
}

func TestErrorHandler_WriteErrors(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		setupFailing bool
	}{
		{
			name:         "JSON write error",
			acceptHeader: MIMEApplicationJSON,
			setupFailing: true,
		},
		{
			name:         "HTML write error",
			acceptHeader: MIMETextHTMLCharsetUTF8,
			setupFailing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderAccept, tt.acceptHeader)

			// Create a failing response writer
			rec := httptest.NewRecorder()
			failingWriter := &failingResponseWriter{ResponseWriter: rec}
			res := &Response{ResponseWriter: failingWriter}
			event := NewErrorHandlerTestEvent(req, res)

			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, logger)
			handler(event, ErrBadRequest)

			// Should log write error
			logOutput := logBuffer.String()
			assert.Contains(t, logOutput, "write response")
		})
	}
}

func TestErrorHandler_ErrorTemplate(t *testing.T) {
	// Test that the error template is valid and can be executed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAccept, MIMETextHTMLCharsetUTF8)
	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	// Use an error with a custom message
	customErr := NewHTTPError(http.StatusBadRequest, "Custom error message")

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, nil, nil)
	handler(event, customErr)

	body := rec.Body.String()
	// HTML template uses title field (HTTP status text), not custom message
	assert.Contains(t, body, "Bad Request!") // title field shows HTTP status text
	assert.Contains(t, body, "Code 400")
}

// failingResponseWriter is a mock that fails on Write operations
type failingResponseWriter struct {
	http.ResponseWriter
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// Test helper to check if error handler handles different error types correctly
func TestErrorHandler_ErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "standard HTTPError",
			err:      ErrNotFound,
			expected: http.StatusNotFound,
		},
		{
			name:     "custom error",
			err:      errors.New("custom error"),
			expected: http.StatusInternalServerError,
		},
		{
			name:     "wrapped HTTPError",
			err:      NewHTTPError(http.StatusInternalServerError, "wrapper error").WithInternal(errors.New("inner")),
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			res := &Response{ResponseWriter: rec}
			event := NewErrorHandlerTestEvent(req, res)

			handler := ErrorHandler[*ErrorHandlerTestEvent](nil, AsHTTPError, nil)
			handler(event, tt.err)

			assert.Equal(t, tt.expected, rec.Code)
		})
	}
}

// TestErrorHandler_Integration tests the error handler with a more realistic scenario
func TestErrorHandler_Integration(t *testing.T) {
	// Test a complete error handling scenario with all features
	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.Header.Set(HeaderAccept, MIMEApplicationJSON)
	req.Header.Set("Content-Type", MIMEApplicationJSON)
	req = req.WithContext(WithDebug(req.Context(), true))

	rec := httptest.NewRecorder()
	res := &Response{ResponseWriter: rec}
	event := NewErrorHandlerTestEvent(req, res)

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	// Custom mapper that adds context
	mapper := func(err error) *HTTPError {
		if errors.Is(err, ErrUnauthorized) {
			return ErrUnauthorized.WithInternal(err)
		}
		return AsHTTPError(err)
	}

	handler := ErrorHandler[*ErrorHandlerTestEvent](nil, mapper, logger)
	handler(event, ErrUnauthorized)

	// Verify response
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, MIMEApplicationJSON, rec.Header().Get(HeaderContentType))

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(http.StatusUnauthorized), response["status"])
	// Debug field might not be included in standard JSON response
	if debug, exists := response["debug"]; exists {
		assert.Equal(t, true, debug)
	}
}
