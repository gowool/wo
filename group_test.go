package wo

import (
	"net/http"
	"testing"

	"github.com/gowool/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvent is a simple implementation of hook.Resolver for testing
type TestEvent struct {
	hook.Resolver
}

// TestRouterGroupCreation tests the creation and initialization of RouterGroup
func TestRouterGroupCreation(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	assert.NotNil(t, group)
	assert.Nil(t, group.excludedMiddlewares)
	assert.Empty(t, group.children)
	assert.Empty(t, group.Prefix)
	assert.Empty(t, group.Middlewares)
}

// TestRouterGroupGroup tests creating nested router groups
func TestRouterGroupGroup(t *testing.T) {
	parent := &RouterGroup[TestEvent]{Prefix: "/api"}

	// Test creating a child group
	child := parent.Group("/v1")

	require.NotNil(t, child)
	assert.Equal(t, "/v1", child.Prefix)
	assert.Empty(t, child.Middlewares)
	assert.Empty(t, child.children)

	// Verify the child is added to parent's children
	require.Len(t, parent.children, 1)
	require.IsType(t, &RouterGroup[TestEvent]{}, parent.children[0])
	assert.Equal(t, "/v1", parent.children[0].(*RouterGroup[TestEvent]).Prefix)
}

// TestRouterGroupGroupMultiple tests creating multiple child groups
func TestRouterGroupGroupMultiple(t *testing.T) {
	parent := &RouterGroup[TestEvent]{Prefix: "/api"}

	_ = parent.Group("/v1")
	_ = parent.Group("/v2")
	_ = parent.Group("/admin")

	require.Len(t, parent.children, 3)
	assert.Equal(t, "/v1", parent.children[0].(*RouterGroup[TestEvent]).Prefix)
	assert.Equal(t, "/v2", parent.children[1].(*RouterGroup[TestEvent]).Prefix)
	assert.Equal(t, "/admin", parent.children[2].(*RouterGroup[TestEvent]).Prefix)
}

// TestRouterGroupBindFunc tests binding anonymous middleware functions
func TestRouterGroupBindFunc(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	called := false
	middlewareFunc := func(e TestEvent) error {
		called = true
		return nil
	}

	result := group.BindFunc(middlewareFunc)

	assert.Equal(t, group, result) // Should return the same group for chaining
	require.Len(t, group.Middlewares, 1)
	assert.NotNil(t, group.Middlewares[0].Func)
	assert.Empty(t, group.Middlewares[0].ID) // Anonymous middleware has no ID

	// Test the middleware function works
	var event TestEvent
	err := group.Middlewares[0].Func(event)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestRouterGroupBindFuncMultiple tests binding multiple middleware functions
func TestRouterGroupBindFuncMultiple(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

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

	group.BindFunc(middleware1, middleware2, middleware3)

	require.Len(t, group.Middlewares, 3)

	// Test execution order
	var event TestEvent
	for _, middleware := range group.Middlewares {
		err := middleware.Func(event)
		assert.NoError(t, err)
	}

	assert.Equal(t, []int{1, 2, 3}, callOrder)
}

// TestRouterGroupBind tests binding named middleware handlers
func TestRouterGroupBind(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	handler := &hook.Handler[TestEvent]{
		ID:       "test-middleware",
		Func:     func(e TestEvent) error { return nil },
		Priority: 10,
	}

	result := group.Bind(handler)

	assert.Equal(t, group, result) // Should return the same group for chaining
	require.Len(t, group.Middlewares, 1)
	assert.Equal(t, handler, group.Middlewares[0])
}

// TestRouterGroupBindMultiple tests binding multiple middleware handlers
func TestRouterGroupBindMultiple(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	handler1 := &hook.Handler[TestEvent]{
		ID:   "middleware-1",
		Func: func(e TestEvent) error { return nil },
	}
	handler2 := &hook.Handler[TestEvent]{
		ID:   "middleware-2",
		Func: func(e TestEvent) error { return nil },
	}

	group.Bind(handler1, handler2)

	require.Len(t, group.Middlewares, 2)
	assert.Equal(t, handler1, group.Middlewares[0])
	assert.Equal(t, handler2, group.Middlewares[1])
}

// TestRouterGroupBindWithExcludedMiddleware tests binding middleware that was previously excluded
func TestRouterGroupBindWithExcludedMiddleware(t *testing.T) {
	group := &RouterGroup[TestEvent]{}
	group.excludedMiddlewares = map[string]struct{}{
		"test-middleware": {},
	}

	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	group.Bind(handler)

	// The middleware should be removed from excluded list
	assert.Empty(t, group.excludedMiddlewares)
	require.Len(t, group.Middlewares, 1)
}

// TestRouterGroupUnbind tests unbinding middleware by ID
func TestRouterGroupUnbind(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

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

	group.Bind(handler1, handler2, anonymousHandler)
	require.Len(t, group.Middlewares, 3)

	// Unbind one middleware
	result := group.Unbind("middleware-1")

	assert.Equal(t, group, result) // Should return the same group for chaining
	require.Len(t, group.Middlewares, 2)
	assert.Equal(t, handler2, group.Middlewares[0])
	assert.Equal(t, anonymousHandler, group.Middlewares[1])

	// Verify middleware is added to excluded list
	require.NotNil(t, group.excludedMiddlewares)
	_, exists := group.excludedMiddlewares["middleware-1"]
	assert.True(t, exists)
}

// TestRouterGroupUnbindMultiple tests unbinding multiple middlewares
func TestRouterGroupUnbindMultiple(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

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

	group.Bind(handler1, handler2, handler3)
	require.Len(t, group.Middlewares, 3)

	// Unbind multiple middlewares
	group.Unbind("middleware-1", "middleware-3")

	require.Len(t, group.Middlewares, 1)
	assert.Equal(t, handler2, group.Middlewares[0])

	// Verify all are added to excluded list
	_, exists1 := group.excludedMiddlewares["middleware-1"]
	_, exists3 := group.excludedMiddlewares["middleware-3"]
	assert.True(t, exists1)
	assert.True(t, exists3)
}

// TestRouterGroupUnbindAnonymous tests that anonymous middlewares are not removed
func TestRouterGroupUnbindAnonymous(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	anonymousHandler := &hook.Handler[TestEvent]{
		Func: func(e TestEvent) error { return nil },
	}

	group.Bind(anonymousHandler)
	require.Len(t, group.Middlewares, 1)

	// Try to unbind with empty string (should do nothing)
	group.Unbind("")

	require.Len(t, group.Middlewares, 1)
	assert.Equal(t, anonymousHandler, group.Middlewares[0])
}

// TestRouterGroupUnbindNonExistent tests unbinding a middleware that doesn't exist
func TestRouterGroupUnbindNonExistent(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	handler := &hook.Handler[TestEvent]{
		ID:   "existing-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	group.Bind(handler)
	require.Len(t, group.Middlewares, 1)

	// Try to unbind non-existent middleware
	group.Unbind("non-existent-middleware")

	// Should still have the original middleware
	require.Len(t, group.Middlewares, 1)
	assert.Equal(t, handler, group.Middlewares[0])

	// Non-existent middleware should be added to excluded list
	_, exists := group.excludedMiddlewares["non-existent-middleware"]
	assert.True(t, exists)
}

// TestRouterGroupUnbindWithChildren tests unbinding middleware propagates to children
func TestRouterGroupUnbindWithChildren(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	// Create a child group
	childGroup := group.Group("/child")

	// Create a child route
	childRoute := group.Route("GET", "/test", func(e TestEvent) error { return nil })

	// Add middleware with ID to all levels
	middleware := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}

	group.Bind(middleware)
	childGroup.Bind(middleware)
	childRoute.Bind(middleware)

	// Verify middleware exists at all levels
	require.Len(t, group.Middlewares, 1)
	require.Len(t, childGroup.Middlewares, 1)
	require.Len(t, childRoute.Middlewares, 1)

	// Unbind from parent
	group.Unbind("test-middleware")

	// Should be removed from all levels
	assert.Empty(t, group.Middlewares)
	assert.Empty(t, childGroup.Middlewares)
	assert.Empty(t, childRoute.Middlewares)
}

// TestRouterGroupRoute tests creating a new route
func TestRouterGroupRoute(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	called := false
	action := func(e TestEvent) error {
		called = true
		return nil
	}

	route := group.Route("GET", "/test", action)

	require.NotNil(t, route)
	assert.Equal(t, "GET", route.Method)
	assert.Equal(t, "/test", route.Path)
	assert.NotNil(t, route.Action) // Can't compare functions directly
	assert.Empty(t, route.Middlewares)
	assert.Nil(t, route.excludedMiddlewares)

	// Verify the route is added to group's children
	require.Len(t, group.children, 1)
	require.IsType(t, &Route[TestEvent]{}, group.children[0])
	assert.Equal(t, route, group.children[0].(*Route[TestEvent]))

	// Test the action function works
	var event TestEvent
	err := route.Action(event)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestRouterGroupRouteMultiple tests creating multiple routes
func TestRouterGroupRouteMultiple(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	action1 := func(e TestEvent) error { return nil }
	action2 := func(e TestEvent) error { return nil }

	route1 := group.Route("GET", "/test1", action1)
	route2 := group.Route("POST", "/test2", action2)

	require.Len(t, group.children, 2)
	assert.Equal(t, route1, group.children[0].(*Route[TestEvent]))
	assert.Equal(t, route2, group.children[1].(*Route[TestEvent]))
}

// TestRouterGroupHTTPMethodShortcuts tests HTTP method shorthand functions
func TestRouterGroupHTTPMethodShortcuts(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	action := func(e TestEvent) error { return nil }

	tests := []struct {
		name     string
		method   string
		path     string
		createFn func(string, func(TestEvent) error) *Route[TestEvent]
	}{
		{
			name:     "Any",
			method:   "",
			path:     "/any",
			createFn: group.Any,
		},
		{
			name:     "GET",
			method:   http.MethodGet,
			path:     "/get",
			createFn: group.GET,
		},
		{
			name:     "SEARCH",
			method:   "SEARCH",
			path:     "/search",
			createFn: group.SEARCH,
		},
		{
			name:     "POST",
			method:   http.MethodPost,
			path:     "/post",
			createFn: group.POST,
		},
		{
			name:     "DELETE",
			method:   http.MethodDelete,
			path:     "/delete",
			createFn: group.DELETE,
		},
		{
			name:     "PATCH",
			method:   http.MethodPatch,
			path:     "/patch",
			createFn: group.PATCH,
		},
		{
			name:     "PUT",
			method:   http.MethodPut,
			path:     "/put",
			createFn: group.PUT,
		},
		{
			name:     "HEAD",
			method:   http.MethodHead,
			path:     "/head",
			createFn: group.HEAD,
		},
		{
			name:     "OPTIONS",
			method:   http.MethodOptions,
			path:     "/options",
			createFn: group.OPTIONS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear children for each test
			group.children = nil

			route := tt.createFn(tt.path, action)

			require.NotNil(t, route)
			assert.Equal(t, tt.method, route.Method)
			assert.Equal(t, tt.path, route.Path)
			assert.NotNil(t, route.Action) // Can't compare functions directly

			// Verify the route is added to group's children
			require.Len(t, group.children, 1)
			require.IsType(t, &Route[TestEvent]{}, group.children[0])
			assert.Equal(t, route, group.children[0].(*Route[TestEvent]))
		})
	}
}

// TestRouterGroupChaining tests method chaining functionality
func TestRouterGroupChaining(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	action := func(e TestEvent) error { return nil }
	middlewareFunc := func(e TestEvent) error { return nil }
	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: middlewareFunc,
	}

	// Test chaining multiple methods together
	apiGroup := group.
		BindFunc(middlewareFunc).
		Bind(handler).
		Group("/api")

	getRoute := apiGroup.GET("/users", action)
	postRoute := apiGroup.POST("/users", action)

	// Verify chaining works
	require.NotNil(t, getRoute)
	require.NotNil(t, postRoute)
	assert.IsType(t, &Route[TestEvent]{}, getRoute)
	assert.IsType(t, &Route[TestEvent]{}, postRoute)

	// Verify the group structure
	require.Len(t, group.children, 1) // Should have one child group
	childGroup := group.children[0].(*RouterGroup[TestEvent])
	assert.Equal(t, "/api", childGroup.Prefix)
	require.Len(t, childGroup.children, 2) // Should have two routes
}

// TestRouterGroupComplexHierarchy tests complex nested group structures
func TestRouterGroupComplexHierarchy(t *testing.T) {
	root := &RouterGroup[TestEvent]{}

	// Create API group
	api := root.Group("/api")
	api.BindFunc(func(e TestEvent) error { return nil }) // API-level middleware

	// Create v1 group
	v1 := api.Group("/v1")
	v1.BindFunc(func(e TestEvent) error { return nil }) // v1-level middleware

	// Create users group
	users := v1.Group("/users")
	users.BindFunc(func(e TestEvent) error { return nil }) // users-level middleware

	// Add routes to users group
	users.GET("", func(e TestEvent) error { return nil })
	users.POST("", func(e TestEvent) error { return nil })
	users.GET("/:id", func(e TestEvent) error { return nil })

	// Create posts group
	posts := v1.Group("/posts")
	posts.GET("", func(e TestEvent) error { return nil })
	posts.POST("", func(e TestEvent) error { return nil })

	// Verify hierarchy
	require.Len(t, root.children, 1)
	require.IsType(t, &RouterGroup[TestEvent]{}, root.children[0])

	apiGroup := root.children[0].(*RouterGroup[TestEvent])
	assert.Equal(t, "/api", apiGroup.Prefix)
	require.Len(t, apiGroup.children, 1)
	require.Len(t, apiGroup.Middlewares, 1)

	v1Group := apiGroup.children[0].(*RouterGroup[TestEvent])
	assert.Equal(t, "/v1", v1Group.Prefix)
	require.Len(t, v1Group.children, 2)
	require.Len(t, v1Group.Middlewares, 1)

	usersGroup := v1Group.children[0].(*RouterGroup[TestEvent])
	assert.Equal(t, "/users", usersGroup.Prefix)
	require.Len(t, usersGroup.children, 3) // 3 routes
	require.Len(t, usersGroup.Middlewares, 1)

	postsGroup := v1Group.children[1].(*RouterGroup[TestEvent])
	assert.Equal(t, "/posts", postsGroup.Prefix)
	require.Len(t, postsGroup.children, 2) // 2 routes
	require.Empty(t, postsGroup.Middlewares)
}

// TestRouterGroupExcludedMiddlewareInitialization tests lazy initialization of excludedMiddlewares
func TestRouterGroupExcludedMiddlewareInitialization(t *testing.T) {
	group := &RouterGroup[TestEvent]{}

	// excludedMiddlewares should be nil initially
	assert.Nil(t, group.excludedMiddlewares)

	// Add a middleware
	handler := &hook.Handler[TestEvent]{
		ID:   "test-middleware",
		Func: func(e TestEvent) error { return nil },
	}
	group.Bind(handler)

	// excludedMiddlewares should still be nil
	assert.Nil(t, group.excludedMiddlewares)

	// Unbind the middleware
	group.Unbind("test-middleware")

	// excludedMiddlewares should now be initialized
	require.NotNil(t, group.excludedMiddlewares)
	_, exists := group.excludedMiddlewares["test-middleware"]
	assert.True(t, exists)
}
