package middleware

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gowool/wo"
)

const gzipScheme = "gzip"

type CompressConfig struct {
	// Gzip compression level.
	// Optional. Default value -1.
	Level int `env:"LEVEL" json:"level,omitempty" yaml:"level,omitempty"`

	// Length threshold before gzip compression is applied.
	// Optional. Default value 1024.
	//
	// Most of the time you will not need to change the default. Compressing
	// a short response might increase the transmitted data because of the
	// gzip format overhead. Compressing the response will also consume CPU
	// and time on the server and the client (for decompressing). Depending on
	// your use case such a threshold might be useful.
	//
	// See also:
	// https://webmasters.stackexchange.com/questions/31750/what-is-recommended-minimum-object-size-for-gzip-performance-benefits
	MinLength int `env:"MIN_LENGTH" json:"minLength,omitempty" yaml:"minLength,omitempty"`
}

func (c *CompressConfig) SetDefaults() {
	if c.Level == 0 {
		c.Level = -1
	}
	if c.MinLength <= 0 {
		c.MinLength = 1024 // 1KB
	}
}

func (c *CompressConfig) Validate() error {
	if c.Level < -2 || c.Level > 9 { // these are consts: gzip.HuffmanOnly and gzip.BestCompression
		return errors.New("invalid gzip level")
	}
	return nil
}

func Compress[T wo.Resolver](cfg CompressConfig, skippers ...Skipper[T]) func(T) error {
	cfg.SetDefaults()

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	skip := ChainSkipper[T](skippers...)

	pool := sync.Pool{
		New: func() any {
			w, err := gzip.NewWriterLevel(io.Discard, cfg.Level)
			if err != nil {
				return err
			}
			return w
		},
	}

	bpool := sync.Pool{
		New: func() any {
			b := &bytes.Buffer{}
			return b
		},
	}

	return func(e T) error {
		if skip(e) {
			return e.Next()
		}

		res := e.Response()
		res.Header().Add(wo.HeaderVary, wo.HeaderAcceptEncoding)

		if !strings.Contains(e.Request().Header.Get(wo.HeaderAcceptEncoding), gzipScheme) {
			return e.Next()
		}

		i := pool.Get()
		w, ok := i.(*gzip.Writer)
		if !ok {
			return wo.ErrInternalServerError.WithInternal(i.(error))
		}
		rw := res
		w.Reset(rw)

		buf := bpool.Get().(*bytes.Buffer)
		buf.Reset()

		grw := &gzipResponseWriter{Writer: w, ResponseWriter: rw, minLength: cfg.MinLength, buffer: buf}
		e.SetResponse(grw)

		defer func() {
			// There are different reasons for cases when we have not yet written response to the client and now need to do so.
			// a) handler response had only response code and no response body (ala 404 or redirects etc). Response code need to be written now.
			// b) body is shorter than our minimum length threshold and being buffered currently and needs to be written
			if !grw.wroteBody {
				if res.Header().Get(wo.HeaderContentEncoding) == gzipScheme {
					res.Header().Del(wo.HeaderContentEncoding)
				}
				if grw.wroteHeader {
					rw.WriteHeader(grw.code)
				}
				// We have to reset response to it's pristine state when
				// nothing is written to body or error is returned.
				// See issue echo#424, echo#407.
				e.SetResponse(rw)
				w.Reset(io.Discard)
			} else if !grw.minLengthExceeded {
				// Write uncompressed response
				e.SetResponse(rw)
				if grw.wroteHeader {
					grw.ResponseWriter.WriteHeader(grw.code)
				}
				_, _ = grw.buffer.WriteTo(rw)
				w.Reset(io.Discard)
			}
			_ = w.Close()
			bpool.Put(buf)
			pool.Put(w)
		}()

		return e.Next()
	}
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	buffer            *bytes.Buffer
	minLength         int
	code              int
	wroteHeader       bool
	wroteBody         bool
	minLengthExceeded bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del(wo.HeaderContentLength) // Issue echo#444

	w.wroteHeader = true

	// Delay writing of the header until we know if we'll actually compress the response
	w.code = code
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.Header().Get(wo.HeaderContentType) == "" {
		w.Header().Set(wo.HeaderContentType, http.DetectContentType(b))
	}
	w.wroteBody = true

	if !w.minLengthExceeded {
		n, err := w.buffer.Write(b)

		if w.buffer.Len() >= w.minLength {
			w.minLengthExceeded = true

			// The minimum length is exceeded, add Content-Encoding header and write the header
			w.Header().Set(wo.HeaderContentEncoding, gzipScheme) // Issue #806
			if w.wroteHeader {
				w.ResponseWriter.WriteHeader(w.code)
			}

			return w.Writer.Write(w.buffer.Bytes())
		}

		return n, err
	}

	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if !w.minLengthExceeded {
		// Enforce compression because we will not know how much more data will come
		w.minLengthExceeded = true
		w.Header().Set(wo.HeaderContentEncoding, gzipScheme) // Issue #806
		if w.wroteHeader {
			w.ResponseWriter.WriteHeader(w.code)
		}

		_, _ = w.Writer.Write(w.buffer.Bytes())
	}

	_ = w.Writer.(*gzip.Writer).Flush()
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
