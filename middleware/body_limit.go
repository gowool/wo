package middleware

import (
	"io"

	"github.com/gowool/wo"
)

const maxBodySize int64 = 32 << 20

type BodyLimitConfig struct {
	// Maximum allowed size for a request body, default is 32MB.
	// If Limit is less to 0, no limit is applied.
	Limit int64 `json:"limit,omitempty" yaml:"limit,omitempty"`
}

func (c *BodyLimitConfig) SetDefaults() {
	if c.Limit == 0 {
		c.Limit = maxBodySize
	}
}

func BodyLimit[T wo.Resolver](cfg BodyLimitConfig, skippers ...Skipper[T]) func(T) error {
	skip := ChainSkipper[T](skippers...)

	return func(e T) error {
		if skip(e) || cfg.Limit <= 0 {
			return e.Next()
		}

		// optimistically check the submitted request content length
		if e.Request().ContentLength > cfg.Limit {
			return wo.ErrStatusRequestEntityTooLarge
		}

		// replace the request body
		//
		// note: we don't use sync.Pool since the size of the elements could vary too much
		// and it might not be efficient (see https://github.com/golang/go/issues/23199)
		e.Request().Body = &limitedReader{ReadCloser: e.Request().Body, limit: cfg.Limit}

		return e.Next()
	}
}

type limitedReader struct {
	io.ReadCloser
	limit     int64
	totalRead int64
}

func (r *limitedReader) Read(b []byte) (int, error) {
	n, err := r.ReadCloser.Read(b)
	if err != nil {
		return n, err
	}

	r.totalRead += int64(n)
	if r.totalRead > r.limit {
		return n, wo.ErrStatusRequestEntityTooLarge
	}
	return n, nil
}

func (r *limitedReader) Reread() {
	if rr, ok := r.ReadCloser.(wo.Rereader); ok {
		rr.Reread()
	}
}
