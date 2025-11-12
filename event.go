package wo

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/gowool/hook"

	"github.com/gowool/wo/internal/convert"
	"github.com/gowool/wo/internal/encode"
)

const (
	IndexPage        = "index.html"
	DefaultMaxMemory = 32 << 20 // 32mb
	defaultIndent    = "  "
	xmlHTTPRequest   = "XMLHttpRequest"
)

type Event struct {
	hook.Event

	response *Response
	request  *http.Request

	query url.Values
	start time.Time
}

func (e *Event) Reset(w *Response, r *http.Request) {
	e.response = w
	e.request = r
	e.query = nil
	e.start = time.Now()
}

func (e *Event) SetRequest(r *http.Request) {
	e.request = r
}

func (e *Event) Request() *http.Request {
	return e.request
}

func (e *Event) SetResponse(w *Response) {
	e.response = w
}

func (e *Event) Response() *Response {
	return e.response
}

func (e *Event) Context() context.Context {
	return e.request.Context()
}

func (e *Event) StartTime() time.Time {
	return e.start
}

// Flush flushes buffered data to the current response.
//
// Returns [http.ErrNotSupported] if e.response doesn't implement the [http.Flusher] interface
// (all router package handlers receives a ResponseWriter that implements it unless explicitly replaced with a custom one).
func (e *Event) Flush() error {
	return e.response.FlushError()
}

// Written reports whether the current response has already been written.
func (e *Event) Written() bool {
	return e.response.Written
}

// Status reports the status code of the current response.
func (e *Event) Status() int {
	return e.response.Status
}

func (e *Event) UserAgent() string {
	return e.request.UserAgent()
}

// IsTLS reports whether the connection on which the request was received is TLS.
func (e *Event) IsTLS() bool {
	return e.request.TLS != nil
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false.
func (e *Event) IsWebSocket() bool {
	upgrade := e.request.Header.Get(HeaderUpgrade)
	return strings.EqualFold(upgrade, "websocket")
}

// IsAjax returns true if the HTTP request was made with XMLHttpRequest.
func (e *Event) IsAjax() bool {
	return e.Request().Header.Get(HeaderXRequestedWith) == xmlHTTPRequest
}

// AcceptLanguage returns the value of the Accept header.
func (e *Event) AcceptLanguage() string {
	return e.request.Header.Get(HeaderAcceptLanguage)
}

// AcceptedLanguages returns a slice of accepted languages from the Accept-Language header.
func (e *Event) AcceptedLanguages() []string {
	accepted := e.AcceptLanguage()
	if accepted == "" {
		return nil
	}

	options := strings.Split(accepted, ",")
	l := len(options)
	languages := make([]string, l)

	for i := 0; i < l; i++ {
		locale := strings.SplitN(options[i], ";", 2)
		languages[i] = strings.Trim(locale[0], " ")
	}

	return languages
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func (e *Event) Scheme() string {
	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0
	if e.IsTLS() {
		return "https"
	}
	if scheme := e.request.Header.Get(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := e.request.Header.Get(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := e.request.Header.Get(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := e.request.Header.Get(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// QueryParam returns the query param for the provided name.
func (e *Event) QueryParam(name string) string {
	if e.query == nil {
		e.query = e.request.URL.Query()
	}
	return e.query.Get(name)
}

// QueryParams returns the query parameters as `url.Values`.
func (e *Event) QueryParams() url.Values {
	if e.query == nil {
		e.query = e.request.URL.Query()
	}
	return e.query
}

// QueryString returns the URL query string.
func (e *Event) QueryString() string {
	return e.request.URL.RawQuery
}

// FormValue returns the form field value for the provided name.
func (e *Event) FormValue(name string) string {
	return e.request.FormValue(name)
}

// FormParams returns the form parameters as `url.Values`.
func (e *Event) FormParams() (url.Values, error) {
	if strings.HasPrefix(e.request.Header.Get(HeaderContentType), MIMEMultipartForm) {
		if err := e.request.ParseMultipartForm(DefaultMaxMemory); err != nil {
			return nil, err
		}
	} else {
		if err := e.request.ParseForm(); err != nil {
			return nil, err
		}
	}
	return e.request.Form, nil
}

// FormFile returns the multipart form file for the provided name.
func (e *Event) FormFile(name string) (*multipart.FileHeader, error) {
	f, fh, err := e.request.FormFile(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return fh, nil
}

// Param returns the value for the named path wildcard in the pattern
// that matched the request.
// It returns the empty string if the request was not matched against a pattern
// or there is no such wildcard in the pattern.
func (e *Event) Param(name string) string {
	return e.request.PathValue(name)
}

// SetParam sets name to value, so that subsequent calls to e.Param(name)
// return value.
func (e *Event) SetParam(name, value string) {
	e.request.SetPathValue(name, value)
}

// MultipartForm returns the multipart form.
func (e *Event) MultipartForm() (*multipart.Form, error) {
	err := e.request.ParseMultipartForm(DefaultMaxMemory)
	return e.request.MultipartForm, err
}

// Cookie returns the named cookie provided in the request.
func (e *Event) Cookie(name string) (*http.Cookie, error) {
	return e.request.Cookie(name)
}

// Cookies returns the HTTP cookies sent with the request.
func (e *Event) Cookies() []*http.Cookie {
	return e.request.Cookies()
}

// SetCookie is an alias for [http.SetCookie].
//
// SetCookie adds a Set-Cookie header to the current response's headers.
// The provided cookie must have a valid Name.
// Invalid cookies may be silently dropped.
func (e *Event) SetCookie(cookie *http.Cookie) {
	http.SetCookie(e.response, cookie)
}

// RemoteIP returns the IP address of the client that sent the request.
//
// IPv6 addresses are returned expanded.
// For example, "2001:db8::1" becomes "2001:0db8:0000:0000:0000:0000:0000:0001".
//
// Note that if you are behind reverse proxy(ies), this method returns
// the IP of the last connecting proxy.
func (e *Event) RemoteIP() string {
	ip, _, _ := net.SplitHostPort(e.request.RemoteAddr)
	parsed, _ := netip.ParseAddr(ip)
	return parsed.StringExpanded()
}

// Response writers
// -------------------------------------------------------------------

func (e *Event) setResponseHeaderIfEmpty(key, value string) {
	header := e.response.Header()
	if header.Get(key) == "" {
		header.Set(key, value)
	}
}

// HTML writes an HTML response.
func (e *Event) HTML(status int, data string) error {
	return e.HTMLBlob(status, convert.StringToBytes(data))
}

// HTMLBlob sends an HTTP blob response with status code.
func (e *Event) HTMLBlob(status int, b []byte) error {
	return e.Blob(status, MIMETextHTMLCharsetUTF8, b)
}

// String writes a plain string response.
func (e *Event) String(status int, data string) error {
	return e.Blob(status, MIMETextPlainCharsetUTF8, convert.StringToBytes(data))
}

func (e *Event) jsonPBlob(status int, callback string, i any) error {
	indent := ""
	if _, pretty := e.QueryParams()["pretty"]; pretty {
		indent = defaultIndent
	}

	e.setResponseHeaderIfEmpty(HeaderContentType, MIMEApplicationJavaScriptCharsetUTF8)
	e.response.WriteHeader(status)

	if _, err := e.response.Write(convert.StringToBytes(callback + "(")); err != nil {
		return err
	}

	if err := encode.MarshalJSON(e.response, i, indent); err != nil {
		return err
	}

	if _, err := e.response.Write(convert.StringToBytes(");")); err != nil {
		return err
	}
	return nil
}

func (e *Event) json(status int, i any, indent string) error {
	e.setResponseHeaderIfEmpty(HeaderContentType, MIMEApplicationJSON)
	e.response.WriteHeader(status)

	return encode.MarshalJSON(e.response, i, indent)
}

// JSON sends a JSON response with status code.
func (e *Event) JSON(status int, i any) error {
	indent := ""
	if _, pretty := e.QueryParams()["pretty"]; pretty {
		indent = defaultIndent
	}
	return e.json(status, i, indent)
}

// JSONPretty sends a pretty-print JSON with status code.
func (e *Event) JSONPretty(status int, i any, indent string) error {
	return e.json(status, i, indent)
}

func (e *Event) JSONBlob(status int, b []byte) error {
	return e.Blob(status, MIMEApplicationJSON, b)
}

// JSONP sends a JSONP response with status code. It uses `callback` to construct
// the JSONP payload.
func (e *Event) JSONP(status int, callback string, i any) error {
	return e.jsonPBlob(status, callback, i)
}

// JSONPBlob sends a JSONP blob response with status code. It uses `callback`
// to construct the JSONP payload.
func (e *Event) JSONPBlob(status int, callback string, b []byte) error {
	e.setResponseHeaderIfEmpty(HeaderContentType, MIMEApplicationJavaScriptCharsetUTF8)
	e.response.WriteHeader(status)

	if _, err := e.response.Write(convert.StringToBytes(callback + "(")); err != nil {
		return err
	}
	if _, err := e.response.Write(b); err != nil {
		return err
	}
	_, err := e.response.Write(convert.StringToBytes(");"))
	return err
}

func (e *Event) xml(status int, i any, indent string) error {
	e.setResponseHeaderIfEmpty(HeaderContentType, MIMEApplicationXMLCharsetUTF8)
	e.response.WriteHeader(status)

	enc := xml.NewEncoder(e.response)
	enc.Indent("", indent)
	if _, err := e.response.Write(convert.StringToBytes(xml.Header)); err != nil {
		return err
	}
	return enc.Encode(i)
}

// XML writes an XML response.
// It automatically prepends the generic [xml.Header] string to the response.
func (e *Event) XML(status int, i any) error {
	indent := ""
	if _, pretty := e.QueryParams()["pretty"]; pretty {
		indent = defaultIndent
	}
	return e.xml(status, i, indent)
}

// XMLPretty sends a pretty-print XML with status code.
func (e *Event) XMLPretty(status int, i any, indent string) error {
	return e.xml(status, i, indent)
}

// XMLBlob sends an XML blob response with status code.
func (e *Event) XMLBlob(status int, b []byte) error {
	e.setResponseHeaderIfEmpty(HeaderContentType, MIMEApplicationXMLCharsetUTF8)
	e.response.WriteHeader(status)

	if _, err := e.response.Write(convert.StringToBytes(xml.Header)); err != nil {
		return err
	}
	_, err := e.response.Write(b)
	return err
}

// Blob writes a blob (bytes slice) response.
func (e *Event) Blob(status int, contentType string, b []byte) error {
	e.setResponseHeaderIfEmpty(HeaderContentType, contentType)
	e.response.WriteHeader(status)
	_, err := e.response.Write(b)
	return err
}

// Stream streams the specified reader into the response.
func (e *Event) Stream(status int, contentType string, reader io.Reader) error {
	e.response.Header().Set(HeaderContentType, contentType)
	e.response.WriteHeader(status)
	_, err := io.Copy(e.response, reader)
	return err
}

// NoContent writes a response with no body (ex. 204).
func (e *Event) NoContent(status int) error {
	e.response.WriteHeader(status)
	return nil
}

// Redirect writes a redirect response to the specified url.
// The status code must be in between 300 – 308 range.
func (e *Event) Redirect(status int, url string) error {
	if status < 300 || status > 308 {
		return ErrInvalidRedirectCode
	}
	e.response.Header().Set(HeaderLocation, url)
	e.response.WriteHeader(status)
	return nil
}

// Binders
// -------------------------------------------------------------------

// BindQueryParams binds query params to bindable object
func (e *Event) BindQueryParams(dst any) error {
	if err := BindData(dst, e.QueryParams(), "query", nil); err != nil {
		return ErrBadRequest.WithInternal(err)
	}
	return nil
}

// BindHeaders binds HTTP headers to a bindable object
func (e *Event) BindHeaders(dst any) error {
	if err := BindData(dst, e.request.Header, "header", nil); err != nil {
		return ErrBadRequest.WithInternal(err)
	}
	return nil
}

// BindBody binds request body contents to bindable object
// NB: then binding forms take note that this implementation uses standard library form parsing
// which parses form data from BOTH URL and BODY if content type is not MIMEMultipartForm
// See non-MIMEMultipartForm: https://golang.org/pkg/net/http/#Request.ParseForm
// See MIMEMultipartForm: https://golang.org/pkg/net/http/#Request.ParseMultipartForm
func (e *Event) BindBody(dst any) error {
	if e.request.ContentLength == 0 {
		return nil
	}

	// mediatype is found like `mime.ParseMediaType()` does it
	base, _, _ := strings.Cut(e.request.Header.Get(HeaderContentType), ";")
	mediatype := strings.TrimSpace(base)

	switch mediatype {
	case MIMEApplicationJSON:
		if err := encode.UnmarshalJSON(e.request.Body, dst); err != nil {
			return ErrBadRequest.WithInternal(err)
		}
		// manually call Reread because single call of encode.UnmarshalJSON
		// doesn't ensure that the entire body is a valid json string
		// and it is not guaranteed that it will reach EOF to trigger the reread reset
		// (ex. in case of trailing spaces or invalid trailing parts like: `{"test":1},something`)
		if body, ok := e.request.Body.(interface{ Reread() }); ok {
			body.Reread()
		}
	case MIMEApplicationXML, MIMETextXML:
		if err := xml.NewDecoder(e.request.Body).Decode(dst); err != nil {
			var ute *xml.UnsupportedTypeError
			if errors.As(err, &ute) {
				return ErrBadRequest.WithInternal(err).SetMessage(fmt.Sprintf("Unsupported type error: type=%v, error=%v", ute.Type, ute.Error()))
			}
			var se *xml.SyntaxError
			if errors.As(err, &se) {
				return ErrBadRequest.WithInternal(err).SetMessage(fmt.Sprintf("Syntax error: line=%v, error=%v", se.Line, se.Error()))
			}
			return ErrBadRequest.WithInternal(err)
		}
	case MIMEApplicationForm:
		params, err := e.FormParams()
		if err != nil {
			return ErrBadRequest.WithInternal(err)
		}
		if err = BindData(dst, params, "form", nil); err != nil {
			return ErrBadRequest.WithInternal(err)
		}
	case MIMEMultipartForm:
		params, err := e.MultipartForm()
		if err != nil {
			return ErrBadRequest.WithInternal(err)
		}
		if err = BindData(dst, params.Value, "form", params.File); err != nil {
			return ErrBadRequest.WithInternal(err)
		}
	default:
		return ErrUnsupportedMediaType
	}
	return nil
}
