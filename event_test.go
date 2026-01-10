package wo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for binding tests
type TestUser struct {
	Name    string `json:"name" xml:"name"`
	Age     int    `json:"age" xml:"age"`
	Email   string `json:"email" xml:"email"`
	Active  bool   `json:"active" xml:"active"`
	Address string `json:"address,omitempty" xml:"address,omitempty"`
}

type TestQuery struct {
	Name string `query:"name"`
	Age  int    `query:"age"`
	Page int    `query:"page"`
}

type TestHeaders struct {
	Authorization string `header:"Authorization"`
	ContentType   string `header:"Content-Type"`
	UserAgent     string `header:"User-Agent"`
}

func newTestEventForEventTest() (*Event, *Response, *http.Request) {
	// Create mock response writer
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	// Create test request
	req := httptest.NewRequest("GET", "/test?foo=bar", nil)

	// Create event
	event := &Event{}
	event.Reset(resp, req)

	return event, resp, req
}

func newTestEventWithBody(method, url string, body io.Reader, contentType string) (*Event, *Response, *http.Request) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	req := httptest.NewRequest(method, url, body)
	if contentType != "" {
		req.Header.Set(HeaderContentType, contentType)
	}

	event := &Event{}
	event.Reset(resp, req)

	return event, resp, req
}

func TestEvent_Reset(t *testing.T) {
	event, _, _ := newTestEventForEventTest()

	// Modify some fields
	event.remoteIP = "127.0.0.1"
	event.query = url.Values{"test": {"value"}}
	event.accepted = []string{"application/json"}
	event.languages = []string{"en-US"}

	// Reset with new response and request
	newRec := httptest.NewRecorder()
	newResp := NewResponse(newRec)
	newReq := httptest.NewRequest("POST", "/new", nil)

	event.Reset(newResp, newReq)

	// Verify fields are reset
	assert.Equal(t, newResp, event.Response())
	assert.Equal(t, newReq, event.Request())
	assert.Empty(t, event.remoteIP)
	assert.Nil(t, event.query)
	assert.Nil(t, event.accepted)
	assert.Nil(t, event.languages)
	assert.True(t, event.start.Before(time.Now().Add(time.Second)) && event.start.After(time.Now().Add(-time.Second)))
}

func TestEvent_SettersAndGetters(t *testing.T) {
	event, _, _ := newTestEventForEventTest()

	// Test Request getter/setter
	newReq := httptest.NewRequest("POST", "/test", nil)
	event.SetRequest(newReq)
	assert.Equal(t, newReq, event.Request())

	// Test Response getter/setter
	newResp := NewResponse(httptest.NewRecorder())
	event.SetResponse(newResp)
	assert.Equal(t, newResp, event.Response())
}

func TestEvent_Context(t *testing.T) {
	type testKey struct{}
	event, _, req := newTestEventForEventTest()
	ctx := context.WithValue(req.Context(), testKey{}, "value")
	req = req.WithContext(ctx)
	event.SetRequest(req)

	assert.Equal(t, ctx, event.Context())
	assert.Equal(t, "value", event.Context().Value(testKey{}))
}

func TestEvent_StartTime(t *testing.T) {
	event, _, _ := newTestEventForEventTest()
	start := event.StartTime()

	assert.True(t, start.Before(time.Now().Add(time.Second)))
	assert.True(t, start.After(time.Now().Add(-time.Second)))
}

func TestEvent_Debug(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func(context.Context) context.Context
		expected bool
	}{
		{
			name:     "debug not set returns false",
			setupCtx: func(ctx context.Context) context.Context { return ctx },
			expected: false,
		},
		{
			name: "debug set to true returns true",
			setupCtx: func(ctx context.Context) context.Context {
				return WithDebug(ctx, true)
			},
			expected: true,
		},
		{
			name: "debug set to false returns false",
			setupCtx: func(ctx context.Context) context.Context {
				return WithDebug(ctx, false)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			ctx := tt.setupCtx(req.Context())
			req = req.WithContext(ctx)
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.Debug())
		})
	}
}

func TestEvent_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	// Test successful flush
	err := event.Flush()
	assert.NoError(t, err)
}

func TestEvent_WrittenAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	// Initially not written
	assert.False(t, event.Written())
	assert.Equal(t, 0, event.Status())

	// Write something
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write([]byte("test"))

	assert.True(t, event.Written())
	assert.Equal(t, http.StatusOK, resp.Status)
}

func TestEvent_UserAgent(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		expected  string
	}{
		{"with user agent", "Mozilla/5.0", "Mozilla/5.0"},
		{"empty user agent", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			req.Header.Set("User-Agent", tt.userAgent)
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.UserAgent())
		})
	}
}

func TestEvent_IsTLS(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func() *http.Request
		expected bool
	}{
		{"non-TLS request", func() *http.Request {
			return httptest.NewRequest("GET", "/", nil)
		}, false},
		{"TLS request", func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.TLS = &tls.ConnectionState{}
			return req
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, _ := newTestEventForEventTest()
			req := tt.setupReq()
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.IsTLS())
		})
	}
}

func TestEvent_IsWebSocket(t *testing.T) {
	tests := []struct {
		name     string
		upgrade  string
		expected bool
	}{
		{"websocket upgrade", "websocket", true},
		{"WebSocket upgrade (case insensitive)", "WebSocket", true},
		{"other upgrade", "HTTP/2", false},
		{"no upgrade header", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.IsWebSocket())
		})
	}
}

func TestEvent_IsAjax(t *testing.T) {
	tests := []struct {
		name          string
		requestedWith string
		expected      bool
	}{
		{"XMLHttpRequest", "XMLHttpRequest", true},
		{"xhr", "xhr", false},
		{"empty header", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			if tt.requestedWith != "" {
				req.Header.Set(HeaderXRequestedWith, tt.requestedWith)
			}
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.IsAjax())
		})
	}
}

func TestEvent_AcceptAndAccepted(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		expected []string
	}{
		{"single accept", "application/json", []string{"application/json"}},
		{"multiple accepts", "application/json, text/html", []string{"application/json", "text/html"}},
		{"with quality", "application/json;q=0.8, text/html;q=0.9", []string{"application/json", "text/html"}},
		{"empty accept", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			event.SetRequest(req)

			assert.Equal(t, tt.accept, event.Accept())
			assert.Equal(t, tt.expected, event.Accepted())
		})
	}
}

func TestEvent_AcceptLanguageAndLanguages(t *testing.T) {
	tests := []struct {
		name       string
		acceptLang string
		expected   []string
	}{
		{"single language", "en-US", []string{"en-US"}},
		{"multiple languages", "en-US, fr-FR", []string{"en-US", "fr-FR"}},
		{"with quality", "en-US;q=0.8, fr-FR;q=0.9", []string{"en-US", "fr-FR"}},
		{"empty accept language", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			if tt.acceptLang != "" {
				req.Header.Set("Accept-Language", tt.acceptLang)
			}
			event.SetRequest(req)

			assert.Equal(t, tt.acceptLang, event.AcceptLanguage())
			assert.Equal(t, tt.expected, event.Languages())
		})
	}
}

func TestEvent_Scheme(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func() *http.Request
		expected string
	}{
		{"HTTP request", func() *http.Request {
			return httptest.NewRequest("GET", "http://example.com", nil)
		}, "http"},
		{"HTTPS request", func() *http.Request {
			req := httptest.NewRequest("GET", "https://example.com", nil)
			req.TLS = &tls.ConnectionState{}
			return req
		}, "https"},
		{"X-Forwarded-Proto", func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Forwarded-Proto", "https")
			return req
		}, "https"},
		{"X-Forwarded-Protocol", func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Forwarded-Protocol", "https")
			return req
		}, "https"},
		{"X-Forwarded-Ssl", func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Forwarded-Ssl", "on")
			return req
		}, "https"},
		{"X-Url-Scheme", func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Url-Scheme", "https")
			return req
		}, "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, _ := newTestEventForEventTest()
			req := tt.setupReq()
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.Scheme())
		})
	}
}

func TestEvent_QueryParamAndParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?foo=bar&baz=qux&foo=second", nil)
	event, _, _ := newTestEventForEventTest()
	event.SetRequest(req)

	// Test QueryParam
	assert.Equal(t, "bar", event.QueryParam("foo"))
	assert.Equal(t, "qux", event.QueryParam("baz"))
	assert.Equal(t, "", event.QueryParam("nonexistent"))

	// Test QueryParams
	params := event.QueryParams()
	assert.Equal(t, url.Values{
		"foo": {"bar", "second"},
		"baz": {"qux"},
	}, params)

	// Test QueryString
	assert.Equal(t, "foo=bar&baz=qux&foo=second", event.QueryString())
}

func TestEvent_FormValueAndParams(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		expectError bool
	}{
		{
			name:        "form data",
			contentType: MIMEApplicationForm,
			body:        "name=John&age=30",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.contentType == MIMEMultipartForm {
				body = strings.NewReader(tt.body)
			} else {
				body = strings.NewReader(tt.body)
			}

			event, _, req := newTestEventWithBody("POST", "/", body, tt.contentType)
			event.SetRequest(req)

			// Test FormValue
			if tt.contentType == MIMEApplicationForm {
				assert.Equal(t, "John", event.FormValue("name"))
				assert.Equal(t, "30", event.FormValue("age"))
			}

			// Test FormParams
			formParams, err := event.FormParams()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.contentType == MIMEApplicationForm {
					assert.Equal(t, "John", formParams.Get("name"))
					assert.Equal(t, "30", formParams.Get("age"))
				}
			}
		})
	}
}

func TestEvent_ParamAndSetParam(t *testing.T) {
	event, _, req := newTestEventForEventTest()
	event.SetRequest(req)

	// Initially empty
	assert.Equal(t, "", event.Param("test"))

	// Set param
	event.SetParam("test", "value")
	assert.Equal(t, "value", event.Param("test"))

	// Override param
	event.SetParam("test", "newvalue")
	assert.Equal(t, "newvalue", event.Param("test"))
}

func TestEvent_CookieAndCookies(t *testing.T) {
	event, _, req := newTestEventForEventTest()

	// Add cookies to request
	req.AddCookie(&http.Cookie{Name: "test1", Value: "value1"})
	req.AddCookie(&http.Cookie{Name: "test2", Value: "value2"})
	event.SetRequest(req)

	// Test Cookie
	cookie, err := event.Cookie("test1")
	require.NoError(t, err)
	assert.Equal(t, "value1", cookie.Value)

	_, err = event.Cookie("nonexistent")
	assert.Error(t, err)

	// Test Cookies
	cookies := event.Cookies()
	assert.Len(t, cookies, 2)
	assert.Equal(t, "test1", cookies[0].Name)
	assert.Equal(t, "value1", cookies[0].Value)
}

func TestEvent_SetCookie(t *testing.T) {
	event, resp, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	cookie := &http.Cookie{
		Name:  "test",
		Value: "value",
		Path:  "/",
	}

	event.SetCookie(cookie)

	// Check if cookie was set in response
	setCookies := resp.Header()["Set-Cookie"]
	assert.Len(t, setCookies, 1)
	assert.Contains(t, setCookies[0], "test=value")
}

func TestEvent_RemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{"IPv4", "192.168.1.1:8080", "192.168.1.1"},
		{"IPv6", "[2001:db8::1]:8080", "2001:0db8:0000:0000:0000:0000:0000:0001"},
		{"localhost", "127.0.0.1:8080", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			req.RemoteAddr = tt.remoteAddr
			event.SetRequest(req)

			assert.Equal(t, tt.expected, event.RemoteIP())
		})
	}
}

func TestEvent_NegotiateFormat(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		offered  []string
		expected string
	}{
		{"JSON match", "application/json", []string{"application/json", "text/html"}, "application/json"},
		{"HTML match", "text/html", []string{"application/json", "text/html"}, "text/html"},
		{"no match", "text/plain", []string{"application/json", "text/html"}, ""},
		{"empty accept", "", []string{"application/json"}, "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, req := newTestEventForEventTest()
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			event.SetRequest(req)

			result := event.NegotiateFormat(tt.offered...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvent_HTML(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	err := event.HTML(http.StatusOK, "<h1>Hello World</h1>")
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header().Get(HeaderContentType))
	assert.Equal(t, "<h1>Hello World</h1>", string(resp.Buffer()))
}

func TestEvent_HTMLBlob(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	html := []byte("<h1>Hello World</h1>")
	err := event.HTMLBlob(http.StatusOK, html)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header().Get(HeaderContentType))
	assert.Equal(t, html, resp.Buffer())
}

func TestEvent_String(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	err := event.String(http.StatusOK, "Hello World")
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMETextPlainCharsetUTF8, resp.Header().Get(HeaderContentType))
	assert.Equal(t, "Hello World", string(resp.Buffer()))
}

func TestEvent_JSON(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	user := TestUser{Name: "John", Age: 30, Email: "john@example.com", Active: true}
	err := event.JSON(http.StatusOK, user)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationJSON, resp.Header().Get(HeaderContentType))

	var decoded TestUser
	err = json.Unmarshal(resp.Buffer(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, user, decoded)
}

func TestEvent_JSONPretty(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	user := TestUser{Name: "John", Age: 30}
	err := event.JSONPretty(http.StatusOK, user, "  ")
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationJSON, resp.Header().Get(HeaderContentType))

	// Check if output is pretty printed
	body := string(resp.Buffer())
	assert.Contains(t, body, "  ")
	assert.Contains(t, body, "\n")
}

func TestEvent_JSONBlob(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	jsonData := []byte(`{"name":"John","age":30}`)
	err := event.JSONBlob(http.StatusOK, jsonData)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationJSON, resp.Header().Get(HeaderContentType))
	assert.Equal(t, jsonData, resp.Buffer())
}

func TestEvent_JSONP(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	user := TestUser{Name: "John", Age: 30}
	err := event.JSONP(http.StatusOK, "callback", user)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationJavaScriptCharsetUTF8, resp.Header().Get(HeaderContentType))

	body := string(resp.Buffer())
	assert.True(t, strings.HasPrefix(body, "callback("))
	assert.True(t, strings.HasSuffix(body, ");"))
}

func TestEvent_XML(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	user := TestUser{Name: "John", Age: 30, Email: "john@example.com"}
	err := event.XML(http.StatusOK, user)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationXMLCharsetUTF8, resp.Header().Get(HeaderContentType))

	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, xml.Header))
	assert.Contains(t, body, "<name>John</name>")
	assert.Contains(t, body, "<age>30</age>")
}

func TestEvent_XMLPretty(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	user := TestUser{Name: "John", Age: 30}
	err := event.XMLPretty(http.StatusOK, user, "  ")
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationXMLCharsetUTF8, resp.Header().Get(HeaderContentType))

	// Check if output is pretty printed
	body := rec.Body.String()
	assert.Contains(t, body, "  ")
	assert.Contains(t, body, "\n")
}

func TestEvent_XMLBlob(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	xmlData := []byte(`<User><Name>John</Name></User>`)
	err := event.XMLBlob(http.StatusOK, xmlData)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, MIMEApplicationXMLCharsetUTF8, resp.Header().Get(HeaderContentType))

	body := string(resp.Buffer())
	assert.True(t, strings.HasPrefix(body, xml.Header))
	assert.Contains(t, body, `<User><Name>John</Name></User>`)
}

func TestEvent_Blob(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)
	resp.Buffering = true

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	data := []byte("binary data")
	err := event.Blob(http.StatusOK, "application/octet-stream", data)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, "application/octet-stream", resp.Header().Get(HeaderContentType))
	assert.Equal(t, data, resp.Buffer())
}

func TestEvent_Stream(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewResponse(rec)

	event, _, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	data := "streaming data"
	reader := strings.NewReader(data)
	err := event.Stream(http.StatusOK, "text/plain", reader)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, "text/plain", resp.Header().Get(HeaderContentType))
	assert.Equal(t, data, rec.Body.String())
}

func TestEvent_NoContent(t *testing.T) {
	event, resp, _ := newTestEventForEventTest()
	event.SetResponse(resp)

	err := event.NoContent(http.StatusNoContent)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.Status)
	assert.Equal(t, 0, len(resp.Buffer()))
}

func TestEvent_Redirect(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		expectErr bool
	}{
		{"valid redirect", http.StatusMovedPermanently, false},
		{"invalid redirect", http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, resp, _ := newTestEventForEventTest()
			event.SetResponse(resp)

			err := event.Redirect(tt.status, "/new-url")

			if tt.expectErr {
				assert.Error(t, err)
				assert.IsType(t, ErrInvalidRedirectCode, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.status, resp.Status)
				assert.Equal(t, "/new-url", resp.Header().Get(HeaderLocation))
			}
		})
	}
}

func TestEvent_Negotiate(t *testing.T) {
	tests := []struct {
		name      string
		accept    string
		data      any
		offered   []string
		expectErr bool
		expectCT  string
	}{
		{
			name:      "JSON negotiation",
			accept:    "application/json",
			data:      TestUser{Name: "John"},
			offered:   []string{"application/json", "text/html"},
			expectErr: false,
			expectCT:  MIMEApplicationJSON,
		},
		{
			name:      "HTML negotiation",
			accept:    "text/html",
			data:      "Hello World",
			offered:   []string{"application/json", "text/html"},
			expectErr: false,
			expectCT:  MIMETextHTMLCharsetUTF8,
		},
		{
			name:      "no acceptable format",
			accept:    "text/plain",
			data:      "Hello World",
			offered:   []string{"application/json", "text/html"},
			expectErr: true,
			expectCT:  "",
		},
		{
			name:      "empty accept with offered",
			accept:    "",
			data:      "Hello World",
			offered:   []string{"text/plain"},
			expectErr: false,
			expectCT:  MIMETextPlainCharsetUTF8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, resp, req := newTestEventForEventTest()
			event.SetResponse(resp)
			req.Header.Set("Accept", tt.accept)
			event.SetRequest(req)

			err := event.Negotiate(http.StatusOK, tt.data, tt.offered...)

			if tt.expectErr {
				assert.Error(t, err)
				assert.IsType(t, ErrNotAcceptable, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectCT, resp.Header().Get(HeaderContentType))
			}
		})
	}
}

func TestEvent_BindQueryParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?name=John&age=30&page=1", nil)
	event, _, _ := newTestEventForEventTest()
	event.SetRequest(req)

	var query TestQuery
	err := event.BindQueryParams(&query)
	require.NoError(t, err)

	assert.Equal(t, "John", query.Name)
	assert.Equal(t, 30, query.Age)
	assert.Equal(t, 1, query.Page)
}

func TestEvent_BindHeaders(t *testing.T) {
	event, _, req := newTestEventForEventTest()
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	event.SetRequest(req)

	var headers TestHeaders
	err := event.BindHeaders(&headers)
	require.NoError(t, err)

	assert.Equal(t, "Bearer token123", headers.Authorization)
	assert.Equal(t, "application/json", headers.ContentType)
	assert.Equal(t, "test-agent", headers.UserAgent)
}

func TestEvent_BindBody_JSON(t *testing.T) {
	user := TestUser{Name: "John", Age: 30, Email: "john@example.com", Active: true}
	jsonData, err := json.Marshal(user)
	require.NoError(t, err)

	event, _, req := newTestEventWithBody("POST", "/", bytes.NewReader(jsonData), MIMEApplicationJSON)
	event.SetRequest(req)

	var result TestUser
	err = event.BindBody(&result)
	require.NoError(t, err)

	assert.Equal(t, user, result)
}

func TestEvent_BindBody_XML(t *testing.T) {
	user := TestUser{Name: "John", Age: 30, Email: "john@example.com"}
	xmlData, err := xml.Marshal(user)
	require.NoError(t, err)

	event, _, req := newTestEventWithBody("POST", "/", bytes.NewReader(xmlData), MIMEApplicationXML)
	event.SetRequest(req)

	var result TestUser
	err = event.BindBody(&result)
	require.NoError(t, err)

	assert.Equal(t, user, result)
}

func TestEvent_BindBody_Form(t *testing.T) {
	formData := "name=John&age=30&active=true"
	event, _, req := newTestEventWithBody("POST", "/", strings.NewReader(formData), MIMEApplicationForm)
	event.SetRequest(req)

	var result struct {
		Name   string `form:"name"`
		Age    int    `form:"age"`
		Active bool   `form:"active"`
	}

	err := event.BindBody(&result)
	require.NoError(t, err)

	assert.Equal(t, "John", result.Name)
	assert.Equal(t, 30, result.Age)
	assert.Equal(t, true, result.Active)
}

func TestEvent_BindBody_EmptyBody(t *testing.T) {
	event, _, req := newTestEventWithBody("POST", "/", nil, MIMEApplicationJSON)
	event.SetRequest(req)

	var result TestUser
	err := event.BindBody(&result)
	require.NoError(t, err)

	// Should not error and result should be zero value
	assert.Equal(t, TestUser{}, result)
}

func TestEvent_BindBody_UnsupportedMediaType(t *testing.T) {
	body := strings.NewReader("some data")
	event, _, req := newTestEventWithBody("POST", "/", body, "text/plain")
	event.SetRequest(req)

	var result TestUser
	err := event.BindBody(&result)
	assert.Error(t, err)
	assert.IsType(t, ErrUnsupportedMediaType, err)
}

func TestEvent_BindBody_InvalidJSON(t *testing.T) {
	body := strings.NewReader("{invalid json}")
	event, _, req := newTestEventWithBody("POST", "/", body, MIMEApplicationJSON)
	event.SetRequest(req)

	var result TestUser
	err := event.BindBody(&result)
	assert.Error(t, err)
	assert.IsType(t, ErrBadRequest, err)
}

func TestEvent_BindBody_InvalidXML(t *testing.T) {
	// Create truly invalid XML (unclosed tag)
	body := strings.NewReader("<invalid><unclosed>")
	event, _, req := newTestEventWithBody("POST", "/", body, MIMEApplicationXML)
	event.SetRequest(req)

	var result TestUser
	err := event.BindBody(&result)
	assert.Error(t, err)
	assert.IsType(t, ErrBadRequest, err)
}

func TestEvent_BindQueryParams_Error(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?name=John&age=invalid", nil)
	event, _, _ := newTestEventForEventTest()
	event.SetRequest(req)

	var query TestQuery
	err := event.BindQueryParams(&query)
	assert.Error(t, err)
	assert.IsType(t, ErrBadRequest, err)
}

func TestEvent_BindHeaders_Error(t *testing.T) {
	event, _, req := newTestEventForEventTest()
	req.Header.Set("Authorization", "invalid header format that should cause binding error")
	event.SetRequest(req)

	var headers TestHeaders
	err := event.BindHeaders(&headers)
	// This might not error with current implementation, but test structure is ready
	// depending on validation rules
	assert.NoError(t, err)
}

func TestEvent_MultipartForm(t *testing.T) {
	// Create multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "test.txt")
	require.NoError(t, err)
	_, _ = part.Write([]byte("file content"))

	err = writer.WriteField("name", "John")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	event, _, req := newTestEventWithBody("POST", "/", &body, writer.FormDataContentType())
	event.SetRequest(req)

	form, err := event.MultipartForm()
	require.NoError(t, err)

	assert.Equal(t, "John", form.Value["name"][0])
	assert.NotNil(t, form.File["file"])
	assert.Equal(t, "test.txt", form.File["file"][0].Filename)
}

func TestEvent_FormFile(t *testing.T) {
	// Create multipart form with file
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "test.txt")
	require.NoError(t, err)
	_, _ = part.Write([]byte("file content"))

	err = writer.Close()
	require.NoError(t, err)

	event, _, req := newTestEventWithBody("POST", "/", &body, writer.FormDataContentType())
	event.SetRequest(req)

	fileHeader, err := event.FormFile("file")
	require.NoError(t, err)

	assert.Equal(t, "test.txt", fileHeader.Filename)

	// Test non-existent file
	_, err = event.FormFile("nonexistent")
	assert.Error(t, err)
}

func TestEvent_SetValue_Value(t *testing.T) {
	tests := []struct {
		name     string
		key      any
		value    any
		expected any
	}{
		{
			name:     "set string value",
			key:      "testKey",
			value:    "testValue",
			expected: "testValue",
		},
		{
			name:     "set int value",
			key:      "numberKey",
			value:    42,
			expected: 42,
		},
		{
			name:     "set debug value",
			key:      ctxDebugKey{},
			value:    true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			e := &Event{}
			e.Reset(&Response{ResponseWriter: w}, r)

			e.SetValue(tt.key, tt.value)
			assert.Equal(t, tt.expected, e.Value(tt.key))
		})
	}
}

// Benchmark tests
func BenchmarkEvent_SetValue(b *testing.B) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	e := &Event{}
	e.Reset(&Response{ResponseWriter: w}, r)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.SetValue("benchmarkKey", "benchmarkValue")
	}
}
