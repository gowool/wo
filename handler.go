package wo

import (
	"io/fs"
	"net/http"
)

func WrapMiddleware[T Resolver](m func(http.Handler) http.Handler) func(T) error {
	return func(e T) (err error) {
		m(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.SetRequest(r)
			switch resp := w.(type) {
			case *Response:
				e.SetResponse(resp)
			default:
				e.SetResponse(NewResponse(w))
			}
			err = e.Next()
		})).ServeHTTP(e.Response(), e.Request())
		return
	}
}

func WrapHandler[T Resolver](h http.Handler) func(T) error {
	return func(e T) error {
		h.ServeHTTP(e.Response(), e.Request())
		return nil
	}
}

func FileFS[T interface{ FileFS(fs.FS, string) error }](fsys fs.FS, filename string) func(T) error {
	if fsys == nil {
		panic("FileFS: the provided fs.FS argument is nil")
	}

	return func(e T) error {
		return e.FileFS(fsys, filename)
	}
}

func StaticFS[T interface{ StaticFS(fs.FS, bool) error }](fsys fs.FS, indexFallback bool) func(T) error {
	if fsys == nil {
		panic("StaticFS: the provided fs.FS argument is nil")
	}

	return func(e T) error {
		return e.StaticFS(fsys, indexFallback)
	}
}
