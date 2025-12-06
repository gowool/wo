package wo

// Copied from github.com/pocketbase/pocketbase to avoid nuances around the specific
//
// -------------------------------------------------------------------
// The MIT License (MIT) Copyright (c) 2022 - present, Gani Georgiev
// Permission is hereby granted, free of charge, to any person obtaining a copy of this
// software and associated documentation files (the "Software"), to deal in the Software
// without restriction, including without limitation the rights to use, copy, modify,
// merge, publish, distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to the following
// conditions:
// The above copyright notice and this permission notice shall be included in all copies
// or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
// INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
// PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT
// OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
// -------------------------------------------------------------------

import (
	"net/http"

	"github.com/gowool/hook"
)

type RouterGroup[T hook.Resolver] struct {
	excludedMiddlewares map[string]struct{}
	children            []any // Route or Group

	Prefix      string
	Middlewares []*hook.Handler[T]
}

// Group creates and register a new child RouterGroup into the current one
// with the specified prefix.
//
// The prefix follows the standard Go net/http ServeMux pattern format ("[HOST]/[PATH]")
// and will be concatenated recursively into the final route path, meaning that
// only the root level group could have HOST as part of the prefix.
//
// Returns the newly created group to allow chaining and registering
// sub-routes and group specific middlewares.
func (group *RouterGroup[T]) Group(prefix string) *RouterGroup[T] {
	newGroup := &RouterGroup[T]{}
	newGroup.Prefix = prefix

	group.children = append(group.children, newGroup)

	return newGroup
}

// BindFunc registers one or multiple middleware functions to the current group.
//
// The registered middleware functions are "anonymous" and with default priority,
// aka. executes in the order they were registered.
//
// If you need to specify a named middleware (ex. so that it can be removed)
// or middleware with custom exec priority, use [RouterGroup.Bind] method.
func (group *RouterGroup[T]) BindFunc(middlewareFuncs ...func(e T) error) *RouterGroup[T] {
	for _, m := range middlewareFuncs {
		group.Middlewares = append(group.Middlewares, &hook.Handler[T]{Func: m})
	}

	return group
}

// Bind registers one or multiple middleware handlers to the current group.
func (group *RouterGroup[T]) Bind(middlewares ...*hook.Handler[T]) *RouterGroup[T] {
	group.Middlewares = append(group.Middlewares, middlewares...)

	// unmark the newly added middlewares in case they were previously "excluded"
	if group.excludedMiddlewares != nil {
		for _, m := range middlewares {
			if m.ID != "" {
				delete(group.excludedMiddlewares, m.ID)
			}
		}
	}

	return group
}

// Unbind removes one or more middlewares with the specified id(s)
// from the current group and its children (if any).
//
// Anonymous middlewares are not removable, aka. this method does nothing
// if the middleware id is an empty string.
func (group *RouterGroup[T]) Unbind(middlewareIDs ...string) *RouterGroup[T] {
	for _, middlewareID := range middlewareIDs {
		if middlewareID == "" {
			continue
		}

		// remove from the group middlewares
		for i := len(group.Middlewares) - 1; i >= 0; i-- {
			if group.Middlewares[i].ID == middlewareID {
				group.Middlewares = append(group.Middlewares[:i], group.Middlewares[i+1:]...)
			}
		}

		// remove from the group children
		for i := len(group.children) - 1; i >= 0; i-- {
			switch v := group.children[i].(type) {
			case *RouterGroup[T]:
				v.Unbind(middlewareID)
			case *Route[T]:
				v.Unbind(middlewareID)
			}
		}

		// add to the exclude list
		if group.excludedMiddlewares == nil {
			group.excludedMiddlewares = map[string]struct{}{}
		}
		group.excludedMiddlewares[middlewareID] = struct{}{}
	}

	return group
}

// Route registers a single route into the current group.
//
// Note that the final route path will be the concatenation of all parent groups prefixes + the route path.
// The path follows the standard Go net/http ServeMux format ("[HOST]/[PATH]"),
// meaning that only a top level group route could have HOST as part of the prefix.
//
// Returns the newly created route to allow attaching route-only middlewares.
func (group *RouterGroup[T]) Route(method string, path string, action func(e T) error) *Route[T] {
	route := &Route[T]{
		Method: method,
		Path:   path,
		Action: action,
	}

	group.children = append(group.children, route)

	return route
}

// Any is a shorthand for [RouterGroup.Route] with "" as route method (aka. matches any method).
func (group *RouterGroup[T]) Any(path string, action func(e T) error) *Route[T] {
	return group.Route("", path, action)
}

// GET is a shorthand for [RouterGroup.Route] with GET as route method.
func (group *RouterGroup[T]) GET(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodGet, path, action)
}

// SEARCH is a shorthand for [RouterGroup.Route] with SEARCH as route method.
func (group *RouterGroup[T]) SEARCH(path string, action func(e T) error) *Route[T] {
	return group.Route("SEARCH", path, action)
}

// POST is a shorthand for [RouterGroup.Route] with POST as route method.
func (group *RouterGroup[T]) POST(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPost, path, action)
}

// DELETE is a shorthand for [RouterGroup.Route] with DELETE as route method.
func (group *RouterGroup[T]) DELETE(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodDelete, path, action)
}

// PATCH is a shorthand for [RouterGroup.Route] with PATCH as route method.
func (group *RouterGroup[T]) PATCH(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPatch, path, action)
}

// PUT is a shorthand for [RouterGroup.Route] with PUT as route method.
func (group *RouterGroup[T]) PUT(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodPut, path, action)
}

// HEAD is a shorthand for [RouterGroup.Route] with HEAD as route method.
func (group *RouterGroup[T]) HEAD(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodHead, path, action)
}

// OPTIONS is a shorthand for [RouterGroup.Route] with OPTIONS as route method.
func (group *RouterGroup[T]) OPTIONS(path string, action func(e T) error) *Route[T] {
	return group.Route(http.MethodOptions, path, action)
}
