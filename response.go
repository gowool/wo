package wo

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
)

var (
	_ FlushErrorer  = (*Response)(nil)
	_ http.Flusher  = (*Response)(nil)
	_ http.Hijacker = (*Response)(nil)
	_ http.Pusher   = (*Response)(nil)
)

// RWUnwrapper specifies that a http.ResponseWriter could be "unwrapped"
// (usually used with [http.ResponseController]).
type RWUnwrapper interface {
	Unwrap() http.ResponseWriter
}

type FlushErrorer interface {
	FlushError() error
}

// Response wraps an http.ResponseWriter and implements its interface to be used
// by an HTTP handler to construct an HTTP response.
// See: https://golang.org/pkg/net/http/#ResponseWriter
type Response struct {
	http.ResponseWriter
	buffer      *bytes.Buffer
	beforeFuncs []func()
	afterFuncs  []func()
	Written     bool
	Buffering   bool
	Status      int
	Size        int64
}

// NewResponse creates a new instance of Response.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{ResponseWriter: w, buffer: bytes.NewBuffer(nil)}
}

func (r *Response) Buffer() []byte {
	return r.buffer.Bytes()
}

// Before registers a function which is called just before the response is Written.
func (r *Response) Before(fn func()) {
	r.beforeFuncs = append(r.beforeFuncs, fn)
}

// After registers a function which is called just after the response is Written.
// If the `Content-Length` is unknown, none of the after function is executed.
func (r *Response) After(fn func()) {
	r.afterFuncs = append(r.afterFuncs, fn)
}

// WriteHeader sends an HTTP response header with Status code. If WriteHeader is
// not called explicitly, the first call to Write will trigger an implicit
// WriteHeader(http.StatusOK). Thus explicit calls to WriteHeader are mainly
// used to send error codes.
func (r *Response) WriteHeader(status int) {
	if r.Written {
		return
	}

	r.Header().Del(HeaderContentLength)

	r.Status = status

	if r.Buffering {
		r.Written = true
		return
	}

	for _, fn := range r.beforeFuncs {
		fn()
	}
	r.ResponseWriter.WriteHeader(status)
	r.Written = true
}

// Write writes the data to the connection as part of an HTTP reply.
func (r *Response) Write(b []byte) (n int, err error) {
	if !r.Written {
		r.WriteHeader(http.StatusOK)
	}

	if r.Buffering {
		_, err = r.buffer.Write(b)
		return
	}

	n, err = r.ResponseWriter.Write(b)
	r.Size += int64(n)
	for _, fn := range r.afterFuncs {
		fn()
	}
	return
}

// Flush implements the http.Flusher interface to allow an HTTP handler to flush
// buffered data to the client.
// See [http.Flusher](https://golang.org/pkg/net/http/#Flusher)
func (r *Response) Flush() {
	if err := r.FlushError(); err != nil && errors.Is(err, http.ErrNotSupported) {
		panic(errors.New("response writer flushing is not supported"))
	}
}

// FlushError is similar to [Flush] but returns [http.ErrNotSupported]
// if the wrapped writer doesn't support it.
func (r *Response) FlushError() error {
	err := http.NewResponseController(r.ResponseWriter).Flush()
	if err == nil || !errors.Is(err, http.ErrNotSupported) {
		r.Written = true
	}
	return err
}

// Hijack implements the http.Hijacker interface to allow an HTTP handler to
// take over the connection.
// See [http.Hijacker](https://golang.org/pkg/net/http/#Hijacker)
func (r *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(r.ResponseWriter).Hijack()
}

// Push implements [http.Pusher] to indicate HTTP/2 server push support.
func (r *Response) Push(target string, opts *http.PushOptions) error {
	w := r.ResponseWriter
	for {
		switch p := w.(type) {
		case http.Pusher:
			return p.Push(target, opts)
		case RWUnwrapper:
			w = p.Unwrap()
		default:
			return http.ErrNotSupported
		}
	}
}

// ReadFrom implements [io.ReaderFrom] by checking if the underlying writer supports it.
// Otherwise calls [io.Copy].
func (r *Response) ReadFrom(reader io.Reader) (n int64, err error) {
	if !r.Written {
		r.WriteHeader(http.StatusOK)
	}

	w := r.ResponseWriter
	for {
		switch rf := w.(type) {
		case io.ReaderFrom:
			return rf.ReadFrom(reader)
		case RWUnwrapper:
			w = rf.Unwrap()
		default:
			return io.Copy(r.ResponseWriter, reader)
		}
	}
}

// Unwrap returns the original http.ResponseWriter.
// ResponseController can be used to access the original http.ResponseWriter.
// See [https://go.dev/blog/go1.20]
func (r *Response) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *Response) Reset(w http.ResponseWriter) {
	r.ResponseWriter = w
	r.buffer.Reset()
	r.beforeFuncs = nil
	r.afterFuncs = nil
	r.Written = false
	r.Buffering = false
	r.Status = 0
	r.Size = 0
}

// UnwrapResponse unwraps given ResponseWriter to return contexts original Response. rw has to implement
// following method `Unwrap() http.ResponseWriter`
func UnwrapResponse(rw http.ResponseWriter) (*Response, error) {
	for {
		switch t := rw.(type) {
		case *Response:
			return t, nil
		case RWUnwrapper:
			rw = t.Unwrap()
			continue
		default:
			return nil, errors.New("ResponseWriter does not implement 'Unwrap() http.ResponseWriter' interface")
		}
	}
}

func MustUnwrapResponse(rw http.ResponseWriter) *Response {
	r, err := UnwrapResponse(rw)
	if err != nil {
		panic(err)
	}
	return r
}
