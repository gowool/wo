package adapter

import (
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/wo"
)

var _ huma.Context = (*woContext)(nil)

type woContext[R wo.Resolver] struct {
	op    *huma.Operation
	query url.Values
	e     R
}

func (c *woContext[R]) reset(op *huma.Operation, e R) {
	c.op = op
	c.e = e
	c.query = nil
}

func (c *woContext[R]) Event() R {
	return c.e
}

// Operation returns the OpenAPI operation that matched the request.
func (c *woContext[R]) Operation() *huma.Operation {
	return c.op
}

// Context returns the underlying request context.
func (c *woContext[R]) Context() context.Context {
	return c.e.Request().Context()
}

// TLS / SSL connection information.
func (c *woContext[R]) TLS() *tls.ConnectionState {
	return c.e.Request().TLS
}

// Version of the HTTP protocol as text and integers.
func (c *woContext[R]) Version() huma.ProtoVersion {
	r := c.e.Request()
	return huma.ProtoVersion{
		Proto:      r.Proto,
		ProtoMajor: r.ProtoMajor,
		ProtoMinor: r.ProtoMinor,
	}
}

// Method returns the HTTP method for the request.
func (c *woContext[R]) Method() string {
	return c.e.Request().Method
}

// Host returns the HTTP host for the request.
func (c *woContext[R]) Host() string {
	return c.e.Request().Host
}

// RemoteAddr returns the remote address of the client.
func (c *woContext[R]) RemoteAddr() string {
	return c.e.Request().RemoteAddr
}

// URL returns the full URL for the request.
func (c *woContext[R]) URL() url.URL {
	return *c.e.Request().URL
}

// Param returns the value for the given path parameter.
func (c *woContext[R]) Param(name string) string {
	return c.e.Request().PathValue(name)
}

// Query returns the value for the given query parameter.
func (c *woContext[R]) Query(name string) string {
	if c.query == nil {
		c.query = c.e.Request().URL.Query()
	}
	return c.query.Get(name)
}

// Header returns the value for the given header.
func (c *woContext[R]) Header(name string) string {
	return c.e.Request().Header.Get(name)
}

// EachHeader iterates over all headers and calls the given callback with
// the header name and value.
func (c *woContext[R]) EachHeader(cb func(name, value string)) {
	for name, values := range c.e.Request().Header {
		for _, value := range values {
			cb(name, value)
		}
	}
}

// BodyReader returns the request body reader.
func (c *woContext[R]) BodyReader() io.Reader {
	return c.e.Request().Body
}

// GetMultipartForm returns the parsed multipart form, if any.
func (c *woContext[R]) GetMultipartForm() (*multipart.Form, error) {
	err := c.e.Request().ParseMultipartForm(wo.DefaultMaxMemory)
	return c.e.Request().MultipartForm, err
}

// SetReadDeadline sets the read deadline for the request body.
func (c *woContext[R]) SetReadDeadline(deadline time.Time) error {
	return huma.SetReadDeadline(c.e.Response(), deadline)
}

// SetStatus sets the HTTP status code for the response.
func (c *woContext[R]) SetStatus(code int) {
	c.e.Response().WriteHeader(code)
}

// Status returns the HTTP status code for the response.
func (c *woContext[R]) Status() int {
	return c.e.Response().Status
}

// SetHeader sets the given header to the given value, overwriting any
// existing value. Use `AppendHeader` to append a value instead.
func (c *woContext[R]) SetHeader(name, value string) {
	c.e.Response().Header().Set(name, value)
}

// AppendHeader appends the given value to the given header.
func (c *woContext[R]) AppendHeader(name, value string) {
	c.e.Response().Header().Add(name, value)
}

// BodyWriter returns the response body writer.
func (c *woContext[R]) BodyWriter() io.Writer {
	return c.e.Response()
}
