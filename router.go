package wo

import (
	"context"
	"errors"
	"iter"
	"maps"
	"net/http"
	"sync"

	"github.com/gowool/hook"
)

type (
	ctxEventKey struct{}
	ctxErrorKey struct{}
)

func WrapStdHandler[T Resolver](h http.Handler) func(T) error {
	return func(e T) error {
		h.ServeHTTP(e.Response(), e.Request())
		return nil
	}
}

type Resolver interface {
	hook.Resolver

	SetRequest(r *http.Request)
	Request() *http.Request

	SetResponse(w *Response)
	Response() *Response
}

type ErrorHandler[T Resolver] interface {
	Handle(T, error)
}

type EventCleanupFunc func()

// EventFactoryFunc defines the function responsible for creating a Route specific event
// based on the provided request handler ServeHTTP data.
//
// Optionally return a cleanup function that will be invoked right after the route execution.
type EventFactoryFunc[T Resolver] func(w *Response, r *http.Request) (T, EventCleanupFunc)

type Router[T Resolver] struct {
	*RouterGroup[T]

	patterns     map[string]struct{}
	eventFactory EventFactoryFunc[T]
	errorHandler ErrorHandler[T]
	preHook      *hook.Hook[T]
	responsePool sync.Pool
	rereaderPool sync.Pool
}

func New[T Resolver](eventFactory EventFactoryFunc[T], errorHandler ErrorHandler[T]) *Router[T] {
	return &Router[T]{
		RouterGroup:  new(RouterGroup[T]),
		preHook:      new(hook.Hook[T]),
		patterns:     make(map[string]struct{}),
		eventFactory: eventFactory,
		errorHandler: errorHandler,
		responsePool: sync.Pool{
			New: func() any { return NewResponse(nil) },
		},
		rereaderPool: sync.Pool{
			New: func() any { return new(RereadableReadCloser) },
		},
	}
}

func (r *Router[T]) Patterns() iter.Seq[string] {
	return maps.Keys(r.patterns)
}

func (r *Router[T]) PreFunc(middlewareFuncs ...func(e T) error) {
	for _, middlewareFunc := range middlewareFuncs {
		r.preHook.BindFunc(middlewareFunc)
	}
}

func (r *Router[T]) Pre(middlewares ...*hook.Handler[T]) {
	for _, middleware := range middlewares {
		r.preHook.Bind(middleware)
	}
}

// BuildMux constructs a new mux [http.Handler] instance from the current router configurations.
func (r *Router[T]) BuildMux() (http.Handler, error) {
	mux := http.NewServeMux()

	if err := r.build(mux, r.RouterGroup, nil); err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// wrap the response to add write and status tracking
		resp := r.responsePool.Get().(*Response)
		resp.Reset(w)

		// wrap the request body to allow multiple reads
		read := r.rereaderPool.Get().(*RereadableReadCloser)
		read.Reset(req.Body)
		req.Body = read

		defer func() {
			resp.Reset(nil)
			r.responsePool.Put(resp)

			read.Reset(nil)
			r.rereaderPool.Put(read)
		}()

		event, cleanupFunc := r.eventFactory(resp, req)
		defer func() {
			if cleanupFunc != nil {
				cleanupFunc()
			}
		}()

		if err := r.preHook.Trigger(event, func(e T) error {
			ctx := context.WithValue(e.Request().Context(), ctxEventKey{}, e)
			e.SetRequest(e.Request().WithContext(ctx))

			mux.ServeHTTP(e.Response(), e.Request())

			err, _ := e.Request().Context().Value(ctxErrorKey{}).(error)
			return err
		}); err != nil && r.errorHandler != nil {
			r.errorHandler.Handle(event, err)
		}
	}), nil
}

func (r *Router[T]) build(mux *http.ServeMux, group *RouterGroup[T], parents []*RouterGroup[T]) error {
	for _, child := range group.children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if err := r.build(mux, v, append(parents, group)); err != nil {
				return err
			}
		case *Route[T]:
			routeHook := new(hook.Hook[T])

			var pattern string

			// add parent groups middlewares
			for _, p := range parents {
				pattern += p.Prefix
				for _, h := range p.Middlewares {
					if _, ok := p.excludedMiddlewares[h.ID]; !ok {
						if _, ok = group.excludedMiddlewares[h.ID]; !ok {
							if _, ok = v.excludedMiddlewares[h.ID]; !ok {
								routeHook.Bind(h)
							}
						}
					}
				}
			}

			// add current groups middlewares
			pattern += group.Prefix
			for _, h := range group.Middlewares {
				if _, ok := group.excludedMiddlewares[h.ID]; !ok {
					if _, ok = v.excludedMiddlewares[h.ID]; !ok {
						routeHook.Bind(h)
					}
				}
			}

			// add current route middlewares
			pattern += v.Path
			for _, h := range v.Middlewares {
				if _, ok := v.excludedMiddlewares[h.ID]; !ok {
					routeHook.Bind(h)
				}
			}

			r.patterns[pattern] = struct{}{}

			if v.Method != "" {
				pattern = v.Method + " " + pattern
			}

			mux.HandleFunc(pattern, func(_ http.ResponseWriter, req *http.Request) {
				event := req.Context().Value(ctxEventKey{}).(T)
				event.SetRequest(req)

				if err := routeHook.Trigger(event, v.Action); err != nil {
					ctx := context.WithValue(req.Context(), ctxErrorKey{}, err)
					event.SetRequest(event.Request().WithContext(ctx))
				}
			})
		default:
			return errors.New("invalid RouterGroup item type")
		}
	}
	return nil
}
