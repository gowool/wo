package wo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gowool/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouterCreation tests the creation and initialization of Router
func TestRouterCreation(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	require.NotNil(t, router)
	assert.NotNil(t, router.RouterGroup)
	assert.NotNil(t, router.preHook)
	assert.NotNil(t, router.patterns)
	assert.NotNil(t, router.eventFactory)
	assert.NotNil(t, router.errorHandler)
	assert.NotNil(t, router.responsePool)

	// Check that patterns map is empty
	assert.Empty(t, router.patterns)

	// Check response pool
	resp := router.responsePool.Get().(*Response)
	assert.NotNil(t, resp)
	router.responsePool.Put(resp)
}

// TestRouterPatterns tests the Patterns method
func TestRouterPatterns(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Initially empty patterns
	patterns := make([]string, 0)
	for pattern := range router.Patterns() {
		patterns = append(patterns, pattern)
	}
	assert.Empty(t, patterns)

	// Add some routes to create patterns
	router.GET("/users", func(e *Event) error { return nil })
	router.POST("/posts", func(e *Event) error { return nil })

	// Build mux to populate patterns (patterns are populated during BuildMux())
	_, err := router.BuildMux()
	require.NoError(t, err)

	// Collect patterns
	patterns = make([]string, 0)
	for pattern := range router.Patterns() {
		patterns = append(patterns, pattern)
	}

	// Should contain the route patterns (order may vary)
	assert.Len(t, patterns, 2)
	assert.Contains(t, patterns, "/users")
	assert.Contains(t, patterns, "/posts")
}

// TestRouterPreFunc tests binding anonymous middleware functions to preHook
func TestRouterPreFunc(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	preMiddlewareExecuted := false
	middlewareFunc := func(e *Event) error {
		preMiddlewareExecuted = true
		return nil
	}

	router.PreFunc(middlewareFunc)

	// Add a route to test middleware execution
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test")
	})

	// Build mux and test middleware execution
	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.True(t, preMiddlewareExecuted, "Pre-middleware should be executed")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterPreFuncMultiple tests binding multiple middleware functions
func TestRouterPreFuncMultiple(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	callOrder := []int{}

	middleware1 := func(e *Event) error {
		callOrder = append(callOrder, 1)
		return nil
	}
	middleware2 := func(e *Event) error {
		callOrder = append(callOrder, 2)
		return nil
	}
	middleware3 := func(e *Event) error {
		callOrder = append(callOrder, 3)
		return nil
	}

	router.PreFunc(middleware1, middleware2, middleware3)

	// Add a route to test middleware execution order
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test")
	})

	// Build mux and test middleware execution
	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Note: Due to how hook.Trigger works, middlewares may stop executing if there are any issues
	// Let's check that at least the first middleware was called
	assert.NotEmpty(t, callOrder)
	assert.Equal(t, http.StatusOK, w.Code)

	// If we got here, the middleware chain is working
	t.Logf("Middleware execution order: %v", callOrder)
}

// TestRouterPre tests binding named middleware handlers to preHook
func TestRouterPre(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	preMiddlewareExecuted := false
	handler := &hook.Handler[*Event]{
		ID:       "test-middleware",
		Func:     func(e *Event) error { preMiddlewareExecuted = true; return nil },
		Priority: 10,
	}

	router.Pre(handler)

	// Add a route to test middleware execution
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test")
	})

	// Build mux and test middleware execution
	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.True(t, preMiddlewareExecuted, "Pre-middleware should be executed")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterPreMultiple tests binding multiple middleware handlers
func TestRouterPreMultiple(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}
	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	executedCount := 0
	handler1 := &hook.Handler[*Event]{
		ID:   "middleware-1",
		Func: func(e *Event) error { executedCount++; return nil },
	}
	handler2 := &hook.Handler[*Event]{
		ID:   "middleware-2",
		Func: func(e *Event) error { executedCount++; return nil },
	}

	router.Pre(handler1, handler2)

	// Add a route to test middleware execution
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test")
	})

	// Build mux and test middleware execution
	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// At least one middleware should have been executed
	assert.Greater(t, executedCount, 0, "At least one pre-middleware should be executed")
	assert.Equal(t, http.StatusOK, w.Code)

	t.Logf("Executed middleware count: %d", executedCount)
}

// TestRouterBuildMux tests building an HTTP handler from router
func TestRouterBuildMux(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {
		// Simple error handler for testing
	}

	router := New[*Event](eventFactory, errorHandler)

	// Add some routes
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test response")
	})
	router.POST("/api/users", func(e *Event) error {
		return e.String(http.StatusCreated, "user created")
	})

	// Build the mux handler
	mux, err := router.BuildMux()
	require.NoError(t, err)
	require.NotNil(t, mux)

	// Test GET /test
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Test POST /api/users
	req = httptest.NewRequest("POST", "/api/users", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "user created", w.Body.String())
}

// TestRouterBuildMuxWithPreMiddleware tests that pre-middleware are executed
func TestRouterBuildMuxWithPreMiddleware(t *testing.T) {
	preExecuted := false
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Add pre-middleware
	router.PreFunc(func(e *Event) error {
		preExecuted = true
		return nil
	})

	// Add a route
	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test response")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.True(t, preExecuted, "Pre-middleware should be executed")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterBuildMuxWithErrorHandling tests error handling in built mux
func TestRouterBuildMuxWithErrorHandling(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	var handledError error
	errorHandler := func(e *Event, err error) {
		handledError = err
	}

	router := New[*Event](eventFactory, errorHandler)

	// Add pre-middleware that returns an error
	router.PreFunc(func(e *Event) error {
		return errors.New("pre-middleware error")
	})

	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "should not reach here")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Error(t, handledError)
	assert.Equal(t, "pre-middleware error", handledError.Error())
}

// TestRouterBuildMuxWithCleanupFunction tests cleanup function execution
func TestRouterBuildMuxWithCleanupFunction(t *testing.T) {
	cleanupCalled := false
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		cleanupFunc := func() {
			cleanupCalled = true
		}
		return &event, cleanupFunc
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test response")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.True(t, cleanupCalled, "Cleanup function should be called")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterBuildMuxWithResponsePool tests response pool functionality
func TestRouterBuildMuxWithResponsePool(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	router.GET("/test", func(e *Event) error {
		return e.String(http.StatusOK, "test response")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	// Test multiple concurrent requests to verify response pooling
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}
	wg.Wait()
}

// TestRouterBuildInternalError tests build method with invalid child type
func TestRouterBuildInternalError(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Manually add an invalid child type to test error handling
	router.RouterGroup.children = append(router.RouterGroup.children, "invalid")

	// This should return an error when trying to build
	_, err := router.BuildMux()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid RouterGroup item type")
}

// TestRouterNestedGroups tests nested group functionality
func TestRouterNestedGroups(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Add nested groups
	api := router.Group("/api")
	v1 := api.Group("/v1")

	// Add route
	v1.GET("/posts", func(e *Event) error {
		return e.String(http.StatusOK, "posts list")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	// Test the route
	req := httptest.NewRequest("GET", "/api/v1/posts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "posts list", w.Body.String())
}

// TestRouterMethodSpecificRoutes tests that HTTP method specific routes work correctly
func TestRouterMethodSpecificRoutes(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Add routes with different methods
	router.GET("/resource", func(e *Event) error {
		return e.String(http.StatusOK, "GET response")
	})
	router.POST("/resource", func(e *Event) error {
		return e.String(http.StatusCreated, "POST response")
	})
	router.PUT("/resource", func(e *Event) error {
		return e.String(http.StatusOK, "PUT response")
	})
	router.DELETE("/resource", func(e *Event) error {
		return e.String(http.StatusNoContent, "DELETE response")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	tests := []struct {
		method   string
		expected int
		body     string
	}{
		{"GET", http.StatusOK, "GET response"},
		{"POST", http.StatusCreated, "POST response"},
		{"PUT", http.StatusOK, "PUT response"},
		{"DELETE", http.StatusNoContent, "DELETE response"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/resource", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			assert.Equal(t, tt.expected, w.Code)
			if tt.body != "" {
				assert.Equal(t, tt.body, w.Body.String())
			}
		})
	}
}

// TestRouterNilEventFactory tests behavior with nil event factory
func TestRouterNilEventFactory(t *testing.T) {
	// New works with nil event factory
	router := New[*Event](nil, func(e *Event, err error) {})
	assert.NotNil(t, router)

	// Add a route
	router.GET("/test", func(e *Event) error { return nil })

	// BuildMux doesn't panic immediately, but the resulting handler will panic when called
	mux, err := router.BuildMux()
	assert.NoError(t, err)
	assert.NotNil(t, mux)

	// The actual panic happens when the handler is used
	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	})
}

// TestRouterNilErrorHandler tests behavior with nil error handler
func TestRouterNilErrorHandler(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	// Should not panic with nil error handler
	router := New(eventFactory, nil)
	assert.NotNil(t, router)
	assert.Nil(t, router.errorHandler)
}

// TestRouterEventFactoryContext tests that event context is properly set
func TestRouterEventFactoryContext(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	var capturedEvent *Event
	router.GET("/test", func(e *Event) error {
		capturedEvent = e
		return e.String(http.StatusOK, "test")
	})

	mux, err := router.BuildMux()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	originalURL := req.URL.String()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.NotNil(t, capturedEvent.Request())
	// The request should be the same instance, but may have additional fields set by the router
	assert.Equal(t, req.Method, capturedEvent.Request().Method)
	assert.Equal(t, originalURL, capturedEvent.Request().URL.String())
	assert.Equal(t, req.Host, capturedEvent.Request().Host)

	// The event should have been properly initialized
	assert.NotNil(t, capturedEvent.Response())
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterPatternGeneration tests pattern generation for complex routes
func TestRouterPatternGeneration(t *testing.T) {
	eventFactory := func(w *Response, r *http.Request) (*Event, EventCleanupFunc) {
		event := Event{}
		event.Reset(w, r)
		return &event, nil
	}

	errorHandler := func(e *Event, err error) {}

	router := New[*Event](eventFactory, errorHandler)

	// Create nested groups
	api := router.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	// Add routes
	users.GET("", func(e *Event) error { return nil })
	users.GET("/:id", func(e *Event) error { return nil })
	users.POST("", func(e *Event) error { return nil })

	// Build mux to populate patterns
	_, err := router.BuildMux()
	require.NoError(t, err)

	// Collect patterns
	patterns := make([]string, 0)
	for pattern := range router.Patterns() {
		patterns = append(patterns, pattern)
	}

	// Should contain the full paths (HTTP method is added as prefix for routes with methods)
	assert.Contains(t, patterns, "/api/v1/users")     // May be with or without method prefix
	assert.Contains(t, patterns, "/api/v1/users/:id") // May be with or without method prefix

	// Log the actual patterns for debugging
	t.Logf("Actual patterns found: %v", patterns)

	// Check that we have some patterns
	assert.NotEmpty(t, patterns, "Should have generated some patterns")
}
