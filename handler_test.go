package wo

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gowool/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEvent(req *http.Request, res http.ResponseWriter) *Event {
	event := &Event{}
	event.Reset(res, req)
	return event
}

// mockFileResolver implements interfaces needed for FileFS and StaticFS testing
type mockFileResolver struct {
	hook.Resolver
	request        *http.Request
	response       http.ResponseWriter
	fileFSCalled   bool
	fileFSFsys     fs.FS
	fileFSFilename string
	staticFSCalled bool
	staticFSFsys   fs.FS
	staticFSIndex  bool
	fileFSError    error
	staticFSError  error
}

func (m *mockFileResolver) SetRequest(r *http.Request) {
	m.request = r
}

func (m *mockFileResolver) Request() *http.Request {
	return m.request
}

func (m *mockFileResolver) SetResponse(w http.ResponseWriter) {
	m.response = w
}

func (m *mockFileResolver) Response() http.ResponseWriter {
	return m.response
}

func (m *mockFileResolver) FileFS(fsys fs.FS, filename string) error {
	m.fileFSCalled = true
	m.fileFSFsys = fsys
	m.fileFSFilename = filename
	return m.fileFSError
}

func (m *mockFileResolver) StaticFS(fsys fs.FS, indexFallback bool) error {
	m.staticFSCalled = true
	m.staticFSFsys = fsys
	m.staticFSIndex = indexFallback
	return m.staticFSError
}

// TestWrapMiddleware tests the WrapMiddleware function
func TestWrapMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		middleware      func(http.Handler) http.Handler
		expectedStatus  int
		expectedHeaders map[string]string
	}{
		{
			name: "middleware executes successfully",
			middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Middleware", "executed")
					next.ServeHTTP(w, r)
				})
			},
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"X-Middleware": "executed",
			},
		},
		{
			name: "middleware writes response and stops chain",
			middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte("middleware response"))
				})
			},
			expectedStatus:  http.StatusAccepted,
			expectedHeaders: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP request and response
			req := httptest.NewRequest("GET", "/", nil)
			resp := httptest.NewRecorder()

			// Create the Event with the request and response
			event := newTestEvent(req, resp)

			// Create the middleware function
			middlewareFunc := WrapMiddleware[*Event](tt.middleware)

			// Execute the middleware
			err := middlewareFunc(event)

			// Verify results
			assert.NoError(t, err)

			// Verify that request and response were set
			assert.Equal(t, req, event.Request())
			assert.Equal(t, resp, MustUnwrapResponse(event.Response()).Unwrap())
			assert.Equal(t, tt.expectedStatus, resp.Code)

			// Verify expected headers
			if tt.expectedHeaders != nil {
				for key, value := range tt.expectedHeaders {
					assert.Equal(t, value, resp.Header().Get(key))
				}
			}
		})
	}
}

// TestWrapMiddlewareWithResponse tests WrapMiddleware with Response wrapper
func TestWrapMiddlewareWithResponse(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	response := NewResponse(resp)

	event := newTestEvent(req, response)

	// Create middleware that checks the response type
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify that w is our Response type
			_, ok := w.(*Response)
			assert.True(t, ok, "Expected Response type in middleware")
			w.WriteHeader(http.StatusOK)
		})
	}

	middlewareFunc := WrapMiddleware[*Event](middleware)
	err := middlewareFunc(event)

	assert.NoError(t, err)
	assert.Equal(t, req, event.Request())
	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestWrapMiddlewareWithNonResponseWriter tests WrapMiddleware with non-Response ResponseWriter
func TestWrapMiddlewareWithNonResponseWriter(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()

	// Don't wrap with our Response type
	event := newTestEvent(req, NewResponse(resp))

	// Create middleware that uses raw ResponseWriter
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			next.ServeHTTP(w, r)
		})
	}

	middlewareFunc := WrapMiddleware[*Event](middleware)
	err := middlewareFunc(event)

	assert.NoError(t, err)
	assert.Equal(t, req, event.Request())
	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestWrapHandler tests the WrapHandler function
func TestWrapHandler(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.Handler
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "handler executes successfully",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("handler response"))
			}),
			expectedStatus: http.StatusOK,
			expectedBody:   "handler response",
		},
		{
			name: "handler with different status",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("created"))
			}),
			expectedStatus: http.StatusCreated,
			expectedBody:   "created",
		},
		{
			name: "handler without explicit status",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("no status"))
			}),
			expectedStatus: http.StatusOK,
			expectedBody:   "no status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			resp := httptest.NewRecorder()
			response := NewResponse(resp)

			event := newTestEvent(req, response)

			// Create the handler function
			handlerFunc := WrapHandler[*Event](tt.handler)

			// Execute the handler
			err := handlerFunc(event)

			// Verify results
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.Equal(t, tt.expectedBody, resp.Body.String())
		})
	}
}

// TestWrapHandlerWithNilHandler tests WrapHandler with nil handler
func TestWrapHandlerWithNilHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	response := NewResponse(resp)

	event := newTestEvent(req, response)

	// Create handler function with nil handler
	handlerFunc := WrapHandler[*Event](nil)

	// Execute the handler - this should panic due to nil handler
	assert.Panics(t, func() {
		_ = handlerFunc(event)
	}, "WrapHandler with nil handler should panic")
}

// TestFileFSTests the FileFS function
func TestFileFS(t *testing.T) {
	tests := []struct {
		name        string
		fsys        fs.FS
		filename    string
		expectPanic bool
		expectError bool
	}{
		{
			name:        "valid filesystem and filename",
			fsys:        &mockFS{},
			filename:    "test.txt",
			expectPanic: false,
			expectError: false,
		},
		{
			name:        "nil filesystem should panic",
			fsys:        nil,
			filename:    "test.txt",
			expectPanic: true,
			expectError: false,
		},
		{
			name:        "empty filename",
			fsys:        &mockFS{},
			filename:    "",
			expectPanic: false,
			expectError: false,
		},
		{
			name:        "resolver returns error",
			fsys:        &mockFS{},
			filename:    "error.txt",
			expectPanic: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					FileFS[*mockFileResolver](tt.fsys, tt.filename)
				})
				return
			}

			resolver := &mockFileResolver{}
			if tt.expectError {
				resolver.fileFSError = assert.AnError
			}

			// Create the FileFS function
			fileFunc := FileFS[*mockFileResolver](tt.fsys, tt.filename)

			// Execute the function
			err := fileFunc(resolver)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, assert.AnError, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that FileFS was called with correct parameters
			assert.True(t, resolver.fileFSCalled)
			assert.Equal(t, tt.fsys, resolver.fileFSFsys)
			assert.Equal(t, tt.filename, resolver.fileFSFilename)
		})
	}
}

// TestStaticFSTests the StaticFS function
func TestStaticFS(t *testing.T) {
	tests := []struct {
		name          string
		fsys          fs.FS
		indexFallback bool
		expectPanic   bool
		expectError   bool
	}{
		{
			name:          "valid filesystem with index fallback",
			fsys:          &mockFS{},
			indexFallback: true,
			expectPanic:   false,
			expectError:   false,
		},
		{
			name:          "valid filesystem without index fallback",
			fsys:          &mockFS{},
			indexFallback: false,
			expectPanic:   false,
			expectError:   false,
		},
		{
			name:          "nil filesystem should panic",
			fsys:          nil,
			indexFallback: true,
			expectPanic:   true,
			expectError:   false,
		},
		{
			name:          "resolver returns error",
			fsys:          &mockFS{},
			indexFallback: false,
			expectPanic:   false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					StaticFS[*mockFileResolver](tt.fsys, tt.indexFallback)
				})
				return
			}

			resolver := &mockFileResolver{}
			if tt.expectError {
				resolver.staticFSError = assert.AnError
			}

			// Create the StaticFS function
			staticFunc := StaticFS[*mockFileResolver](tt.fsys, tt.indexFallback)

			// Execute the function
			err := staticFunc(resolver)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, assert.AnError, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that StaticFS was called with correct parameters
			assert.True(t, resolver.staticFSCalled)
			assert.Equal(t, tt.fsys, resolver.staticFSFsys)
			assert.Equal(t, tt.indexFallback, resolver.staticFSIndex)
		})
	}
}

// mockFS is a mock implementation of fs.FS for testing
type mockFS struct{}

func (m *mockFS) Open(string) (fs.File, error) {
	return &mockFile{}, nil
}

// mockFile is a mock implementation of fs.File for testing
type mockFile struct{}

func (m *mockFile) Stat() (fs.FileInfo, error) {
	return &mockFileInfo{}, nil
}

func (m *mockFile) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (m *mockFile) Close() error {
	return nil
}

// mockFileInfo is a mock implementation of fs.FileInfo for testing
type mockFileInfo struct{}

func (m *mockFileInfo) Name() string       { return "mock" }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// TestWrapMiddlewareIntegration tests integration with real HTTP handlers
func TestWrapMiddlewareIntegration(t *testing.T) {
	// Create a real middleware chain
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-1", "true")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware-2", "true")
			next.ServeHTTP(w, r)
		})
	}

	// Create a final handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Final", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	response := NewResponse(resp)

	event := newTestEvent(req, response)

	// Apply first middleware
	wrapped := WrapMiddleware[*Event](middleware1)
	err1 := wrapped(event)
	require.NoError(t, err1)

	// Apply second middleware
	wrapped2 := WrapMiddleware[*Event](middleware2)
	err2 := wrapped2(event)
	require.NoError(t, err2)

	// Apply final handler
	handlerFunc := WrapHandler[*Event](finalHandler)
	err3 := handlerFunc(event)
	require.NoError(t, err3)

	// Verify the middleware chain executed
	assert.Equal(t, "true", resp.Header().Get("X-Middleware-1"))
	assert.Equal(t, "true", resp.Header().Get("X-Middleware-2"))
	assert.Equal(t, "true", resp.Header().Get("X-Final"))
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "success", resp.Body.String())
}

// BenchmarkWrapMiddleware benchmarks the WrapMiddleware function
func BenchmarkWrapMiddleware(b *testing.B) {
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	response := NewResponse(resp)

	event := newTestEvent(req, response)

	middlewareFunc := WrapMiddleware[*Event](middleware)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = middlewareFunc(event)
	}
}

// BenchmarkWrapHandler benchmarks the WrapHandler function
func BenchmarkWrapHandler(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	response := NewResponse(resp)

	event := newTestEvent(req, response)

	handlerFunc := WrapHandler[*Event](handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handlerFunc(event)
	}
}
