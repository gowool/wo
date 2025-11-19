package wo

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	_ error = (*HTTPError)(nil)
	_ error = (*RedirectError)(nil)
)

var (
	ErrBadRequest                    = NewHTTPError(http.StatusBadRequest)                    // HTTP 400 Bad Request
	ErrUnauthorized                  = NewHTTPError(http.StatusUnauthorized)                  // HTTP 401 Unauthorized
	ErrPaymentRequired               = NewHTTPError(http.StatusPaymentRequired)               // HTTP 402 Payment Required
	ErrForbidden                     = NewHTTPError(http.StatusForbidden)                     // HTTP 403 Forbidden
	ErrNotFound                      = NewHTTPError(http.StatusNotFound)                      // HTTP 404 Not Found
	ErrMethodNotAllowed              = NewHTTPError(http.StatusMethodNotAllowed)              // HTTP 405 Method Not Allowed
	ErrNotAcceptable                 = NewHTTPError(http.StatusNotAcceptable)                 // HTTP 406 Not Acceptable
	ErrProxyAuthRequired             = NewHTTPError(http.StatusProxyAuthRequired)             // HTTP 407 Proxy AuthRequired
	ErrRequestTimeout                = NewHTTPError(http.StatusRequestTimeout)                // HTTP 408 Request Timeout
	ErrConflict                      = NewHTTPError(http.StatusConflict)                      // HTTP 409 Conflict
	ErrGone                          = NewHTTPError(http.StatusGone)                          // HTTP 410 Gone
	ErrLengthRequired                = NewHTTPError(http.StatusLengthRequired)                // HTTP 411 Length Required
	ErrPreconditionFailed            = NewHTTPError(http.StatusPreconditionFailed)            // HTTP 412 Precondition Failed
	ErrStatusRequestEntityTooLarge   = NewHTTPError(http.StatusRequestEntityTooLarge)         // HTTP 413 Payload Too Large
	ErrRequestURITooLong             = NewHTTPError(http.StatusRequestURITooLong)             // HTTP 414 URI Too Long
	ErrUnsupportedMediaType          = NewHTTPError(http.StatusUnsupportedMediaType)          // HTTP 415 Unsupported Media Type
	ErrRequestedRangeNotSatisfiable  = NewHTTPError(http.StatusRequestedRangeNotSatisfiable)  // HTTP 416 Range Not Satisfiable
	ErrExpectationFailed             = NewHTTPError(http.StatusExpectationFailed)             // HTTP 417 Expectation Failed
	ErrTeapot                        = NewHTTPError(http.StatusTeapot)                        // HTTP 418 I'm a teapot
	ErrMisdirectedRequest            = NewHTTPError(http.StatusMisdirectedRequest)            // HTTP 421 Misdirected Request
	ErrUnprocessableEntity           = NewHTTPError(http.StatusUnprocessableEntity)           // HTTP 422 Unprocessable Entity
	ErrLocked                        = NewHTTPError(http.StatusLocked)                        // HTTP 423 Locked
	ErrFailedDependency              = NewHTTPError(http.StatusFailedDependency)              // HTTP 424 Failed Dependency
	ErrTooEarly                      = NewHTTPError(http.StatusTooEarly)                      // HTTP 425 Too Early
	ErrUpgradeRequired               = NewHTTPError(http.StatusUpgradeRequired)               // HTTP 426 Upgrade Required
	ErrPreconditionRequired          = NewHTTPError(http.StatusPreconditionRequired)          // HTTP 428 Precondition Required
	ErrTooManyRequests               = NewHTTPError(http.StatusTooManyRequests)               // HTTP 429 Too Many Requests
	ErrRequestHeaderFieldsTooLarge   = NewHTTPError(http.StatusRequestHeaderFieldsTooLarge)   // HTTP 431 Request Header Fields Too Large
	ErrUnavailableForLegalReasons    = NewHTTPError(http.StatusUnavailableForLegalReasons)    // HTTP 451 Unavailable For Legal Reasons
	ErrInternalServerError           = NewHTTPError(http.StatusInternalServerError)           // HTTP 500 Internal Server Error
	ErrNotImplemented                = NewHTTPError(http.StatusNotImplemented)                // HTTP 501 Not Implemented
	ErrBadGateway                    = NewHTTPError(http.StatusBadGateway)                    // HTTP 502 Bad Gateway
	ErrServiceUnavailable            = NewHTTPError(http.StatusServiceUnavailable)            // HTTP 503 Service Unavailable
	ErrGatewayTimeout                = NewHTTPError(http.StatusGatewayTimeout)                // HTTP 504 Gateway Timeout
	ErrHTTPVersionNotSupported       = NewHTTPError(http.StatusHTTPVersionNotSupported)       // HTTP 505 HTTP Version Not Supported
	ErrVariantAlsoNegotiates         = NewHTTPError(http.StatusVariantAlsoNegotiates)         // HTTP 506 Variant Also Negotiates
	ErrInsufficientStorage           = NewHTTPError(http.StatusInsufficientStorage)           // HTTP 507 Insufficient Storage
	ErrLoopDetected                  = NewHTTPError(http.StatusLoopDetected)                  // HTTP 508 Loop Detected
	ErrNotExtended                   = NewHTTPError(http.StatusNotExtended)                   // HTTP 510 Not Extended
	ErrNetworkAuthenticationRequired = NewHTTPError(http.StatusNetworkAuthenticationRequired) // HTTP 511 Network Authentication Required

	ErrRendererNotRegistered = errors.New("renderer not registered")
	ErrInvalidRedirectCode   = errors.New("invalid redirect Status code")
)

func AsHTTPError(err error) *HTTPError {
	var he *HTTPError
	if errors.As(err, &he) {
		if he.Internal != nil { // max 2 levels of checks even if internal could have also internal
			errors.As(he.Internal, &he)
		}
	}
	return he
}

// HTTPError represents an error that occurred while handling a request.
type HTTPError struct {
	Internal error
	Message  any
	Status   int
	Debug    bool
}

// NewHTTPError creates a new HTTPError instance.
func NewHTTPError(status int, message ...any) *HTTPError {
	he := &HTTPError{Status: status, Message: http.StatusText(status)}
	if len(message) > 0 {
		he.Message = message[0]
	}
	return he
}

// SetInternal sets error to HTTPError.Internal
func (he *HTTPError) SetInternal(err error) *HTTPError {
	he.Internal = err
	return he
}

// WithInternal returns clone of HTTPError with err set to HTTPError.Internal field
func (he *HTTPError) WithInternal(err error) *HTTPError {
	return &HTTPError{
		Status:   he.Status,
		Message:  he.Message,
		Internal: err,
	}
}

// SetMessage sets message to HTTPError.Message
func (he *HTTPError) SetMessage(message any) *HTTPError {
	he.Message = message
	return he
}

// WithMessage returns clone of HTTPError with message set to HTTPError.Message field
func (he *HTTPError) WithMessage(message any) *HTTPError {
	return &HTTPError{
		Status:   he.Status,
		Internal: he.Internal,
		Message:  message,
	}
}

// Error makes it compatible with `error` interface.
func (he *HTTPError) Error() string {
	if he.Internal == nil {
		return fmt.Sprintf("Status=%d, message=%v", he.Status, he.Message)
	}
	return fmt.Sprintf("Status=%d, message=%v, internal=%v", he.Status, he.Message, he.Internal)
}

// Unwrap satisfies the Go 1.13 error wrapper interface.
func (he *HTTPError) Unwrap() error {
	return he.Internal
}

func (he *HTTPError) ToMap() map[string]any {
	data := map[string]any{
		"status": he.Status,
		"title":  he.title(),
	}

	if detail := he.detail(); detail != nil {
		data["detail"] = detail
	}

	if internal := he.internal(); internal != "" {
		data["internal"] = internal
	}

	return data
}

type errData struct {
	Status   int    `json:"status"`
	Title    string `json:"title"`
	Detail   any    `json:"detail,omitempty"`
	Internal string `json:"internal,omitempty"`
}

func (he *HTTPError) title() string {
	return http.StatusText(he.Status)
}

func (he *HTTPError) detail() any {
	switch m := he.Message.(type) {
	case error:
		return m.Error()
	case string:
		if he.title() == m {
			return nil
		}
	}
	return he.Message
}

func (he *HTTPError) internal() string {
	if he.Debug && he.Internal != nil {
		return he.Internal.Error()
	}
	return ""
}

type RedirectError struct {
	Status int
	URL    string
}

func (r *RedirectError) Error() string {
	return fmt.Sprintf("[%d] %s", r.Status, r.URL)
}

func NewFoundRedirectError(url string) *RedirectError {
	return &RedirectError{
		Status: http.StatusFound,
		URL:    url,
	}
}

func NewPermanentlyRedirectError(url string) *RedirectError {
	return &RedirectError{
		Status: http.StatusMovedPermanently,
		URL:    url,
	}
}
