package adapter

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/wo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRouter is a mock implementation of the router interface
type mockRouter[R wo.Resolver] struct {
	routes      []mockRoute[R]
	mu          sync.Mutex
	routeCalled bool
	lastMethod  string
	lastPath    string
}

type mockRoute[R wo.Resolver] struct {
	method string
	path   string
	action func(e R) error
}

func (m *mockRouter[R]) Route(method string, path string, action func(e R) error) *wo.Route[R] {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.routeCalled = true
	m.lastMethod = method
	m.lastPath = path
	m.routes = append(m.routes, mockRoute[R]{
		method: method,
		path:   path,
		action: action,
	})

	// Return a mock route
	return &wo.Route[R]{}
}

func (m *mockRouter[R]) getLastRoute() (method, path string, found bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.routes) == 0 {
		return "", "", false
	}

	lastRoute := m.routes[len(m.routes)-1]
	return lastRoute.method, lastRoute.path, true
}

func (m *mockRouter[R]) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.routes = []mockRoute[R]{}
	m.routeCalled = false
	m.lastMethod = ""
	m.lastPath = ""
}

// mockHandler is a mock HTTP handler
type mockHandler struct {
	called bool
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	w.WriteHeader(http.StatusOK)
}

func TestNewAdapter(t *testing.T) {
	t.Run("CreatesAdapterSuccessfully", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		assert.NotNil(t, adapter)
		assert.Equal(t, handler, adapter.Handler)
		assert.Equal(t, router, adapter.router)
		assert.NotNil(t, adapter.pool)
	})

	t.Run("PoolCreatesCorrectType", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		// Get an object from the pool
		obj := adapter.pool.Get()
		defer adapter.pool.Put(obj)

		// Check that it's the correct type
		_, ok := obj.(*woContext[*wo.Event])
		assert.True(t, ok, "Pool should create woContext objects")
	})
}

func TestAdapter_Handle(t *testing.T) {
	t.Run("RoutesOperationCorrectly", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		op := &huma.Operation{
			Method: "POST",
			Path:   "/api/users",
		}

		humaHandler := func(ctx huma.Context) {
			// Handler implementation
		}

		adapter.Handle(op, humaHandler)

		// Verify the router was called with correct parameters
		method, path, found := router.getLastRoute()
		assert.True(t, found, "Router should have been called")
		assert.Equal(t, "POST", method)
		assert.Equal(t, "/api/users", path)
	})

	t.Run("HandlerExecutionContext", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var capturedContext huma.Context
		humaHandler := func(ctx huma.Context) {
			capturedContext = ctx
		}

		adapter.Handle(op, humaHandler)

		// Get the registered route and execute it
		method, path, found := router.getLastRoute()
		require.True(t, found, "Router should have been called")
		assert.Equal(t, "GET", method)
		assert.Equal(t, "/test", path)

		// Execute the action to test context creation
		if len(router.routes) > 0 {
			// Create a test event
			req := httptest.NewRequest("GET", "/test", nil)
			resp := httptest.NewRecorder()
			woResp := &wo.Response{ResponseWriter: resp}
			event := &wo.Event{}
			event.Reset(woResp, req)

			// Execute the registered action
			err := router.routes[0].action(event)
			assert.NoError(t, err)
			assert.NotNil(t, capturedContext)
		}
	})

	t.Run("PoolObjectReuse", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		op := &huma.Operation{
			Method: "PUT",
			Path:   "/api/items",
		}

		// Get an object from pool before handling
		obj1 := adapter.pool.Get()
		adapter.pool.Put(obj1)

		humaHandler := func(ctx huma.Context) {}

		adapter.Handle(op, humaHandler)

		// Get another object from pool - should potentially reuse the same one
		obj2 := adapter.pool.Get()
		adapter.pool.Put(obj2)

		// Both should be of the same type
		_, ok1 := obj1.(*woContext[*wo.Event])
		_, ok2 := obj2.(*woContext[*wo.Event])
		assert.True(t, ok1)
		assert.True(t, ok2)
	})

	t.Run("MultipleOperations", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		operations := []*huma.Operation{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
			{Method: "PUT", Path: "/users/{id}"},
			{Method: "DELETE", Path: "/users/{id}"},
		}

		for _, op := range operations {
			adapter.Handle(op, func(ctx huma.Context) {})
		}

		// Verify all routes were created
		assert.Equal(t, len(operations), len(router.routes))

		// Verify each operation was routed correctly
		expectedRoutes := [][2]string{
			{"GET", "/users"},
			{"POST", "/users"},
			{"PUT", "/users/{id}"},
			{"DELETE", "/users/{id}"},
		}

		for i, expected := range expectedRoutes {
			assert.Equal(t, expected[0], router.routes[i].method)
			assert.Equal(t, expected[1], router.routes[i].path)
		}
	})
}

func TestAdapter_ContextManagement(t *testing.T) {
	t.Run("ContextResetAfterHandling", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var handlerWasCalled bool
		var capturedContext huma.Context

		humaHandler := func(ctx huma.Context) {
			handlerWasCalled = true
			capturedContext = ctx
			// Verify the context has the expected operation
			assert.Equal(t, op, ctx.Operation())
		}

		adapter.Handle(op, humaHandler)

		// Execute the action to test context lifecycle
		if len(router.routes) > 0 {
			// Create a test event
			req := httptest.NewRequest("GET", "/test", nil)
			resp := httptest.NewRecorder()
			woResp := &wo.Response{ResponseWriter: resp}
			event := &wo.Event{}
			event.Reset(woResp, req)

			// Execute the registered action
			err := router.routes[0].action(event)
			assert.NoError(t, err)

			// The handler should have been called
			assert.True(t, handlerWasCalled)

			// The context should be available during handler execution
			assert.NotNil(t, capturedContext)

			// Verify the context implements the expected interface
			assert.Implements(t, (*huma.Context)(nil), capturedContext)

			// Verify pool operation works correctly
			poolObj := adapter.pool.Get()
			assert.NotNil(t, poolObj)
			adapter.pool.Put(poolObj)
		}
	})
}

func TestAdapter_ConcurrentHandling(t *testing.T) {
	t.Run("ConcurrentRouteRegistration", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		const numGoroutines = 10
		const numOperations = 5

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					op := &huma.Operation{
						Method: "GET",
						Path:   "/test/" + string(rune(goroutineID)) + "/" + string(rune(j)),
					}

					adapter.Handle(op, func(ctx huma.Context) {})
				}
			}(i)
		}

		wg.Wait()

		// Verify all routes were registered
		expectedTotalRoutes := numGoroutines * numOperations
		assert.Equal(t, expectedTotalRoutes, len(router.routes))
	})

	t.Run("ConcurrentPoolAccess", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		const numGoroutines = 50
		const numOperations = 10

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					op := &huma.Operation{
						Method: "POST",
						Path:   "/concurrent/test",
					}

					adapter.Handle(op, func(ctx huma.Context) {})

					// Test pool access
					obj := adapter.pool.Get()
					_, ok := obj.(*woContext[*wo.Event])
					assert.True(t, ok)
					adapter.pool.Put(obj)
				}
			}()
		}

		wg.Wait()

		// Should not panic and all operations should complete
		assert.Equal(t, numGoroutines*numOperations, len(router.routes))
	})
}

func TestAdapter_ErrorHandling(t *testing.T) {
	t.Run("HandlerPanicsArePropagated", func(t *testing.T) {
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/panic",
		}

		humaHandler := func(ctx huma.Context) {
			panic("test panic")
		}

		adapter.Handle(op, humaHandler)

		// Execute the action and verify panic is propagated
		if len(router.routes) > 0 {
			// Create a test event
			req := httptest.NewRequest("GET", "/panic", nil)
			resp := httptest.NewRecorder()
			woResp := &wo.Response{ResponseWriter: resp}
			event := &wo.Event{}
			event.Reset(woResp, req)

			// Execute the registered action
			assert.Panics(t, func() {
				_ = router.routes[0].action(event)
			})
		}
	})
}

func TestAdapter_GenericTypes(t *testing.T) {
	t.Run("WithDifferentResolverTypes", func(t *testing.T) {
		// Test that adapter works with different resolver types
		handler := &mockHandler{}

		// Test with *wo.Event resolver
		router1 := &mockRouter[*wo.Event]{}
		adapter1 := NewAdapter(handler, router1)

		op1 := &huma.Operation{Method: "GET", Path: "/event"}
		adapter1.Handle(op1, func(ctx huma.Context) {})

		assert.Equal(t, 1, len(router1.routes))

		// The adapter should work with any type that satisfies wo.Resolver
		// This is verified at compile time by the generic constraint
		assert.NotNil(t, adapter1)
		assert.NotNil(t, adapter1.pool)
	})
}

func TestAdapter_InterfaceCompliance(t *testing.T) {
	t.Run("ImplementsHumaAdapter", func(t *testing.T) {
		// Verify that Adapter implements huma.Adapter
		handler := &mockHandler{}
		router := &mockRouter[*wo.Event]{}

		adapter := NewAdapter(handler, router)

		var _ huma.Adapter = adapter
		assert.Implements(t, (*huma.Adapter)(nil), adapter)
	})
}

// Benchmark tests
func BenchmarkNewAdapter(b *testing.B) {
	handler := &mockHandler{}
	router := &mockRouter[*wo.Event]{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAdapter(handler, router)
	}
}

func BenchmarkAdapter_Handle(b *testing.B) {
	handler := &mockHandler{}
	router := &mockRouter[*wo.Event]{}
	adapter := NewAdapter(handler, router)

	op := &huma.Operation{
		Method: "GET",
		Path:   "/benchmark",
	}

	humaHandler := func(ctx huma.Context) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.reset() // Reset router for each iteration
		adapter.Handle(op, humaHandler)
	}
}

func BenchmarkAdapter_PoolOperations(b *testing.B) {
	handler := &mockHandler{}
	router := &mockRouter[*wo.Event]{}
	adapter := NewAdapter(handler, router)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := adapter.pool.Get()
		adapter.pool.Put(obj)
	}
}
