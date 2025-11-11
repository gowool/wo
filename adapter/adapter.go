package adapter

import (
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/wo"
)

var _ huma.Adapter = (*Adapter[*wo.Event])(nil)

type router[T wo.Resolver] interface {
	Route(method string, path string, action func(e T) error) *wo.Route[T]
}

type Adapter[R wo.Resolver] struct {
	http.Handler
	router router[R]
	pool   *sync.Pool
}

func NewAdapter[R wo.Resolver](handler http.Handler, router router[R]) *Adapter[R] {
	return &Adapter[R]{
		Handler: handler,
		router:  router,
		pool:    &sync.Pool{New: func() any { return new(woContext[R]) }},
	}
}

func (a *Adapter[R]) Handle(op *huma.Operation, handler func(huma.Context)) {
	a.router.Route(op.Method, op.Path, func(e R) error {
		ctx := a.pool.Get().(*woContext[R])
		ctx.reset(op, e)

		defer func() {
			ctx.reset(nil, nil)
			a.pool.Put(ctx)
		}()

		handler(ctx)
		return nil
	})
}
