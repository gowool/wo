package wo

import (
	"testing"

	"github.com/gowool/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteCreation tests the creation and initialization of Route
func TestRouteCreation(t *testing.T) {
	route := &Route[TestEvent]{}

	assert.NotNil(t, route)
	assert.Nil(t, route.excludedMiddlewares)
	assert.Empty(t, route.Method)
	assert.Empty(t, route.Path)
	assert.Nil(t, route.Action)
	assert.Empty(t, route.Middlewares)
}

// TestRouteCreationWithValues tests creating a route with initial values
func TestRouteCreationWithValues(t *testing.T) {
	action := func(e TestEvent) error { return nil }
	route := &Route[TestEvent]{
		Method: "GET",
		Path:   "/test",
		Action: action,
	}

	assert.Equal(t, "GET", route.Method)
	assert.Equal(t, "/test", route.Path)
	assert.NotNil(t, route.Action)
	assert.Empty(t, route.Middlewares)
	assert.Nil(t, route.excludedMiddlewares)
}

// TestRouteBindFunc tests binding anonymous middleware functions
func TestRouteBindFunc(t *testing.T) {
	route := &Route[TestEvent]{}

	called := false
	middlewareFunc := func(e TestEvent) error {
		called = true
		return nil
	}

	result := route.BindFunc(middlewareFunc)

	assert.Equal(t, route, result) // Should return the same route for chaining
	require.Len(t, route.Middlewares, 1)
	assert.NotNil(t, route.Middlewares[0].Func)
	assert.Empty(t, route.Middlewares[0].ID)                        // Anonymous middleware has no ID
	assert.Equal(t, int64(0), int64(route.Middlewares[0].Priority)) // Default priority

	// Test the middleware function works
	var event TestEvent
	err := route.Middlewares[0].Func(event)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestRouteBindFuncMultiple tests binding multiple middleware functions
func TestRouteBindFuncMultiple(t *testing.T) {
	route := &Route[TestEvent]{}

	callOrder := []int{}

	middleware1 := func(e TestEvent) error {
		callOrder = append(callOrder, 1)
		return nil
	}
	middleware2 := func(e TestEvent) error {
		callOrder = append(callOrder, 2)
		return nil
	}
	middleware3 := func(e TestEvent) error {
		callOrder = append(callOrder, 3)
		return nil
	}

	result := route.BindFunc(middleware1, middleware2, middleware3)

	assert.Equal(t, route, result) // Should return the same route for chaining
	require.Len(t, route.Middlewares, 3)

	// Test execution order
	var event TestEvent
	for _, middleware := range route.Middlewares {
		err := middleware.Func(event)
		assert.NoError(t, err)
	}

	assert.Equal(t, []int{1, 2, 3}, callOrder)
}

// TestRouteBindFuncEmpty tests binding with no middleware functions
func TestRouteBindFuncEmpty(t *testing.T) {
	route := &Route[TestEvent]{}

	result := route.BindFunc() // No arguments

	assert.Equal(t, route, result)
	assert.Empty(t, route.Middlewares)
}

// TestRouteBind tests binding named middleware handlers
func TestRouteBind(t *testing.T) {
	route := &Route[TestEvent]{}

	handler := &hook.Handler[TestEvent]{
		ID:       "test-middleware",
		Func:     func(e TestEvent) error { return nil },
		Priority: 10,
	}

	result := route.Bind(handler)

	assert.Equal(t, route, result) // Should return the same route for chaining
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, handler, route.Middlewares[0])
}

// TestRouteBindMultiple tests binding multiple middleware handlers
func TestRouteBindMultiple(t *testing.T) {
	route := &Route[TestEvent]{}

	handler1 := &hook.Handler[TestEvent]{
		ID:   "middleware-1",
		Func: func(e TestEvent) error { return nil },
	}
	handler2 := &hook.Handler[TestEvent]{
		ID:   "middleware-2",
		Func: func(e TestEvent) error { return nil },
	}

	result := route.Bind(handler1, handler2)

	assert.Equal(t, route, result) // Should return the same route for chaining
	require.Len(t, route.Middlewares, 2)
	assert.Equal(t, handler1, route.Middlewares[0])
	assert.Equal(t, handler2, route.Middlewares[1])
}

// TestRouteBindWithExcludedMiddleware tests binding middleware that was previously excluded
func TestRouteBindWithExcludedMiddleware(t *testing.T) {
	route := &Route[TestEvent]{}
	route.excludedMiddlewares = map[string]struct{}{
		"test-middleware": {},
	}

	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	result := route.Bind(handler)

	assert.Equal(t, route, result)

	// The middleware should be removed from excluded list
	assert.Empty(t, route.excludedMiddlewares)
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, handler, route.Middlewares[0])
}

// TestRouteBindWithAnonymousAndNamedHandlers tests binding mix of anonymous and named handlers
func TestRouteBindWithAnonymousAndNamedHandlers(t *testing.T) {
	route := &Route[TestEvent]{}
	route.excludedMiddlewares = map[string]struct{}{
		"named-middleware": {},
	}

	anonymousHandler := &hook.Handler[TestEvent]{
		Func: func(e TestEvent) error { return nil },
	}
	namedHandler := &hook.Handler[TestEvent]{
		ID:   "named-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	result := route.Bind(anonymousHandler, namedHandler)

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 2)
	assert.Equal(t, anonymousHandler, route.Middlewares[0])
	assert.Equal(t, namedHandler, route.Middlewares[1])

	// Only named middleware should be removed from excluded list
	assert.Empty(t, route.excludedMiddlewares)
}

// TestRouteBindEmpty tests binding with no middleware handlers
func TestRouteBindEmpty(t *testing.T) {
	route := &Route[TestEvent]{}

	result := route.Bind() // No arguments

	assert.Equal(t, route, result)
	assert.Empty(t, route.Middlewares)
}

// TestRouteUnbind tests unbinding middleware by ID
func TestRouteUnbind(t *testing.T) {
	route := &Route[TestEvent]{}

	// Add some middlewares
	handler1 := &hook.Handler[TestEvent]{
		ID:   "middleware-1",
		Func: func(e TestEvent) error { return nil },
	}
	handler2 := &hook.Handler[TestEvent]{
		ID:   "middleware-2",
		Func: func(e TestEvent) error { return nil },
	}
	anonymousHandler := &hook.Handler[TestEvent]{
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler1, handler2, anonymousHandler)
	require.Len(t, route.Middlewares, 3)

	// Unbind one middleware
	result := route.Unbind("middleware-1")

	assert.Equal(t, route, result) // Should return the same route for chaining
	require.Len(t, route.Middlewares, 2)
	assert.Equal(t, handler2, route.Middlewares[0])
	assert.Equal(t, anonymousHandler, route.Middlewares[1])

	// Verify middleware is added to excluded list
	require.NotNil(t, route.excludedMiddlewares)
	_, exists := route.excludedMiddlewares["middleware-1"]
	assert.True(t, exists)
}

// TestRouteUnbindMultiple tests unbinding multiple middlewares
func TestRouteUnbindMultiple(t *testing.T) {
	route := &Route[TestEvent]{}

	handler1 := &hook.Handler[TestEvent]{
		ID:   "middleware-1",
		Func: func(e TestEvent) error { return nil },
	}
	handler2 := &hook.Handler[TestEvent]{
		ID:   "middleware-2",
		Func: func(e TestEvent) error { return nil },
	}
	handler3 := &hook.Handler[TestEvent]{
		ID:   "middleware-3",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler1, handler2, handler3)
	require.Len(t, route.Middlewares, 3)

	// Unbind multiple middlewares
	result := route.Unbind("middleware-1", "middleware-3")

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, handler2, route.Middlewares[0])

	// Verify all are added to excluded list
	_, exists1 := route.excludedMiddlewares["middleware-1"]
	_, exists3 := route.excludedMiddlewares["middleware-3"]
	assert.True(t, exists1)
	assert.True(t, exists3)
}

// TestRouteUnbindAnonymous tests that anonymous middlewares are not removed
func TestRouteUnbindAnonymous(t *testing.T) {
	route := &Route[TestEvent]{}

	anonymousHandler := &hook.Handler[TestEvent]{
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(anonymousHandler)
	require.Len(t, route.Middlewares, 1)

	// Try to unbind with empty string (should do nothing)
	result := route.Unbind("")

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, anonymousHandler, route.Middlewares[0])
}

// TestRouteUnbindNonExistent tests unbinding a middleware that doesn't exist
func TestRouteUnbindNonExistent(t *testing.T) {
	route := &Route[TestEvent]{}

	handler := &hook.Handler[TestEvent]{
		ID:   "existing-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler)
	require.Len(t, route.Middlewares, 1)

	// Try to unbind non-existent middleware
	result := route.Unbind("non-existent-middleware")

	assert.Equal(t, route, result)

	// Should still have the original middleware
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, handler, route.Middlewares[0])

	// Non-existent middleware should be added to excluded list
	require.NotNil(t, route.excludedMiddlewares)
	_, exists := route.excludedMiddlewares["non-existent-middleware"]
	assert.True(t, exists)
}

// TestRouteUnbindEmpty tests unbinding with no middleware IDs
func TestRouteUnbindEmpty(t *testing.T) {
	route := &Route[TestEvent]{}

	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler)
	require.Len(t, route.Middlewares, 1)

	// Unbind with no arguments
	result := route.Unbind()

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 1) // Should still have the middleware
	assert.Equal(t, handler, route.Middlewares[0])
	assert.Nil(t, route.excludedMiddlewares) // Should not initialize excluded list
}

// TestRouteUnbindWithDuplicateMiddlewares tests unbinding when there are duplicate middleware IDs
func TestRouteUnbindWithDuplicateMiddlewares(t *testing.T) {
	route := &Route[TestEvent]{}

	handler1 := &hook.Handler[TestEvent]{
		ID:   "duplicate-id",
		Func: func(e TestEvent) error { return nil },
	}
	handler2 := &hook.Handler[TestEvent]{
		ID:   "duplicate-id", // Same ID
		Func: func(e TestEvent) error { return nil },
	}
	otherHandler := &hook.Handler[TestEvent]{
		ID:   "other-id",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler1, handler2, otherHandler)
	require.Len(t, route.Middlewares, 3)

	// Unbind the duplicate ID - should remove both
	result := route.Unbind("duplicate-id")

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 1)
	assert.Equal(t, otherHandler, route.Middlewares[0])

	// Should be added to excluded list
	_, exists := route.excludedMiddlewares["duplicate-id"]
	assert.True(t, exists)
}

// TestRouteUnbindWithExistingExcludedList tests unbinding when excludedMiddlewares already exists
func TestRouteUnbindWithExistingExcludedList(t *testing.T) {
	route := &Route[TestEvent]{}
	route.excludedMiddlewares = map[string]struct{}{
		"existing-excluded": {},
	}

	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(handler)
	result := route.Unbind("test-middleware")

	assert.Equal(t, route, result)

	// Should have both existing and new excluded middlewares
	require.Len(t, route.excludedMiddlewares, 2)
	_, exists1 := route.excludedMiddlewares["existing-excluded"]
	_, exists2 := route.excludedMiddlewares["test-middleware"]
	assert.True(t, exists1)
	assert.True(t, exists2)
}

// TestRouteMiddlewareOrder tests that middleware execution order is preserved
func TestRouteMiddlewareOrder(t *testing.T) {
	route := &Route[TestEvent]{}

	executionOrder := []string{}

	middleware1 := &hook.Handler[TestEvent]{
		ID: "first",
		Func: func(e TestEvent) error {
			executionOrder = append(executionOrder, "first")
			return nil
		},
	}
	middleware2 := &hook.Handler[TestEvent]{
		ID: "second",
		Func: func(e TestEvent) error {
			executionOrder = append(executionOrder, "second")
			return nil
		},
	}
	middleware3 := &hook.Handler[TestEvent]{
		ID: "third",
		Func: func(e TestEvent) error {
			executionOrder = append(executionOrder, "third")
			return nil
		},
	}

	// Add middlewares in a specific order
	route.Bind(middleware1, middleware2, middleware3)

	// Verify order is preserved
	require.Len(t, route.Middlewares, 3)
	assert.Equal(t, middleware1, route.Middlewares[0])
	assert.Equal(t, middleware2, route.Middlewares[1])
	assert.Equal(t, middleware3, route.Middlewares[2])

	// Test execution order
	var event TestEvent
	for _, middleware := range route.Middlewares {
		err := middleware.Func(event)
		assert.NoError(t, err)
	}

	assert.Equal(t, []string{"first", "second", "third"}, executionOrder)
}

// TestRouteChaining tests method chaining functionality
func TestRouteChaining(t *testing.T) {
	route := &Route[TestEvent]{}

	middlewareFunc := func(e TestEvent) error { return nil }
	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: middlewareFunc,
	}

	// Test chaining multiple methods together
	result := route.
		BindFunc(middlewareFunc).
		Bind(handler).
		Unbind("non-existent") // Should still work

	assert.Equal(t, route, result)
	require.Len(t, route.Middlewares, 2) // Bind and BindFunc added middlewares
}

// TestRouteActionFunction tests the action function behavior
func TestRouteActionFunction(t *testing.T) {
	called := false
	action := func(e TestEvent) error {
		called = true
		return nil
	}

	route := &Route[TestEvent]{
		Method: "GET",
		Path:   "/test",
		Action: action,
	}

	assert.NotNil(t, route.Action)

	// Test the action function works
	var event TestEvent
	err := route.Action(event)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestRouteComplexScenario tests a complex real-world scenario
func TestRouteComplexScenario(t *testing.T) {
	route := &Route[TestEvent]{
		Method: "POST",
		Path:   "/api/users",
		Action: func(e TestEvent) error { return nil },
	}

	// Add initial middlewares
	route.BindFunc(
		func(e TestEvent) error { return nil }, // logging
		func(e TestEvent) error { return nil }, // cors
	)

	// Add named middlewares
	authMiddleware := &hook.Handler[TestEvent]{
		ID:   "auth",
		Func: func(e TestEvent) error { return nil },
	}
	rateLimitMiddleware := &hook.Handler[TestEvent]{
		ID:   "rate-limit",
		Func: func(e TestEvent) error { return nil },
	}

	route.Bind(authMiddleware, rateLimitMiddleware)

	// Verify initial state
	require.Len(t, route.Middlewares, 4)
	assert.Equal(t, "POST", route.Method)
	assert.Equal(t, "/api/users", route.Path)

	// Exclude some middleware
	route.Unbind("rate-limit")

	require.Len(t, route.Middlewares, 3)
	require.NotNil(t, route.excludedMiddlewares)
	_, exists := route.excludedMiddlewares["rate-limit"]
	assert.True(t, exists)

	// Add more middlewares after exclusion
	cacheMiddleware := &hook.Handler[TestEvent]{
		ID:   "cache",
		Func: func(e TestEvent) error { return nil },
	}
	route.Bind(cacheMiddleware)

	require.Len(t, route.Middlewares, 4)
	// cache middleware should be added to the end
	assert.Equal(t, cacheMiddleware, route.Middlewares[3])

	// rate-limit should still be in excluded list (not removed by Bind)
	_, rateLimitExists := route.excludedMiddlewares["rate-limit"]
	assert.True(t, rateLimitExists)
}

// TestRouteExcludedMiddlewareInitialization tests lazy initialization of excludedMiddlewares
func TestRouteExcludedMiddlewareInitialization(t *testing.T) {
	route := &Route[TestEvent]{}

	// excludedMiddlewares should be nil initially
	assert.Nil(t, route.excludedMiddlewares)

	// Add a middleware
	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}
	route.Bind(handler)

	// excludedMiddlewares should still be nil
	assert.Nil(t, route.excludedMiddlewares)

	// Unbind the middleware
	route.Unbind("test-middleware")

	// excludedMiddlewares should now be initialized
	require.NotNil(t, route.excludedMiddlewares)
	_, exists := route.excludedMiddlewares["test-middleware"]
	assert.True(t, exists)
}
