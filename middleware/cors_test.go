package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gowool/wo"
)

// newCORSTestEvent creates a test event for CORS middleware testing purposes
func newCORSTestEvent(method, url string, headers map[string]string) *wo.Event {
	req := httptest.NewRequest(method, url, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(wo.NewResponse(rec), req)

	return e
}

// testCORSEvent wraps an event to track Next() calls for testing
type testCORSEvent struct {
	*wo.Event
	nextCalled bool
}

func (e *testCORSEvent) Next() error {
	e.nextCalled = true
	return e.Event.Next()
}

// newTestCORSEvent creates a test event that tracks Next() calls
func newTestCORSEvent(method, url string, headers map[string]string) *testCORSEvent {
	baseEvent := newCORSTestEvent(method, url, headers)
	return &testCORSEvent{Event: baseEvent}
}

func TestCORSConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   CORSConfig
		expected CORSConfig
	}{
		{
			name:   "empty config should get all defaults",
			config: CORSConfig{},
			expected: CORSConfig{
				AllowOrigins: []string{"*"},
				AllowMethods: []string{"GET", "HEAD", "PUT", "PATCH", "POST", "DELETE"},
			},
		},
		{
			name: "config with origins should keep origins and set methods",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			expected: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
				AllowMethods: []string{"GET", "HEAD", "PUT", "PATCH", "POST", "DELETE"},
			},
		},
		{
			name: "config with methods should keep methods and set origins",
			config: CORSConfig{
				AllowMethods: []string{"GET", "POST"},
			},
			expected: CORSConfig{
				AllowOrigins: []string{"*"},
				AllowMethods: []string{"GET", "POST"},
			},
		},
		{
			name: "fully populated config should remain unchanged",
			config: CORSConfig{
				AllowOrigins:     []string{"https://example.com"},
				AllowMethods:     []string{"GET", "POST"},
				AllowHeaders:     []string{"Content-Type"},
				ExposeHeaders:    []string{"X-Custom"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			expected: CORSConfig{
				AllowOrigins:     []string{"https://example.com"},
				AllowMethods:     []string{"GET", "POST"},
				AllowHeaders:     []string{"Content-Type"},
				ExposeHeaders:    []string{"X-Custom"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.SetDefaults()
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestCORS_AllowOrigins_ExactMatch(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectAllowed bool
	}{
		{
			name: "allowed exact origin should be permitted",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:        "https://example.com",
			expectAllowed: true,
		},
		{
			name: "disallowed origin should be denied",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:        "https://malicious.com",
			expectAllowed: false,
		},
		{
			name: "multiple origins with matching one",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com", "https://trusted.com"},
			},
			origin:        "https://trusted.com",
			expectAllowed: true,
		},
		{
			name: "case sensitive origin matching",
			config: CORSConfig{
				AllowOrigins: []string{"https://Example.com"},
			},
			origin:        "https://example.com",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(map[string]string)
			if tt.origin != "" {
				headers[wo.HeaderOrigin] = tt.origin
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectAllowed {
				assert.Equal(t, tt.origin, allowOrigin, "Origin should be allowed")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}
		})
	}
}

func TestCORS_AllowOrigins_Wildcard(t *testing.T) {
	tests := []struct {
		name             string
		config           CORSConfig
		origin           string
		expectAllowed    bool
		allowCredentials bool
		unsafeWildcard   bool
		expectedOrigin   string
	}{
		{
			name: "wildcard should allow any origin without credentials",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			origin:         "https://any-origin.com",
			expectAllowed:  true,
			expectedOrigin: "*",
		},
		{
			name: "wildcard with credentials should be allowed by default (insecure behavior)",
			config: CORSConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
			},
			origin:         "https://any-origin.com",
			expectAllowed:  true,
			expectedOrigin: "*",
		},
		{
			name: "wildcard with credentials and unsafe flag should reflect origin",
			config: CORSConfig{
				AllowOrigins:                             []string{"*"},
				AllowCredentials:                         true,
				UnsafeWildcardOriginWithAllowCredentials: true,
			},
			origin:         "https://any-origin.com",
			expectAllowed:  true,
			expectedOrigin: "https://any-origin.com",
		},
		{
			name: "wildcard without credentials should still set credentials header when false",
			config: CORSConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: false,
			},
			origin:         "https://any-origin.com",
			expectAllowed:  true,
			expectedOrigin: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			allowCreds := event.Response().Header().Get(wo.HeaderAccessControlAllowCredentials)

			if tt.expectAllowed {
				assert.Equal(t, tt.expectedOrigin, allowOrigin, "Origin should be allowed with correct value")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}

			if tt.config.AllowCredentials && tt.expectAllowed {
				assert.Equal(t, "true", allowCreds, "Credentials header should be set when allowed")
			} else {
				assert.Empty(t, allowCreds, "Credentials header should not be set")
			}
		})
	}
}

func TestCORS_AllowOrigins_Patterns(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectAllowed bool
	}{
		{
			name: "wildcard pattern should match subdomains",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "https://api.example.com",
			expectAllowed: true,
		},
		{
			name: "wildcard pattern should match multiple subdomains",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "https://v2.api.example.com",
			expectAllowed: true,
		},
		{
			name: "wildcard pattern should not match different domain",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "https://api.malicious.com",
			expectAllowed: false,
		},
		{
			name: "question mark pattern should match single character",
			config: CORSConfig{
				AllowOrigins: []string{"https://api?.example.com"},
			},
			origin:        "https://api1.example.com",
			expectAllowed: true,
		},
		{
			name: "question mark pattern should not match multiple characters",
			config: CORSConfig{
				AllowOrigins: []string{"https://api?.example.com"},
			},
			origin:        "https://api12.example.com",
			expectAllowed: false,
		},
		{
			name: "complex pattern with multiple wildcards",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.*.example.com"},
			},
			origin:        "https://api.v1.example.com",
			expectAllowed: true,
		},
		{
			name: "invalid regex pattern should be ignored",
			config: CORSConfig{
				AllowOrigins: []string{"https://[invalid.example.com"},
			},
			origin:        "https://test.example.com",
			expectAllowed: false, // Should fall back to default "*"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectAllowed {
				assert.Equal(t, tt.origin, allowOrigin, "Origin should be allowed")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}
		})
	}
}

func TestCORS_AllowOriginFunc(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectAllowed bool
		expectError   bool
		customFunc    func(origin string) (bool, error)
	}{
		{
			name: "custom function should allow valid origin",
			config: CORSConfig{
				AllowOriginFunc: func(origin string) (bool, error) {
					return origin == "https://trusted.com", nil
				},
			},
			origin:        "https://trusted.com",
			expectAllowed: true,
		},
		{
			name: "custom function should deny invalid origin",
			config: CORSConfig{
				AllowOriginFunc: func(origin string) (bool, error) {
					return origin == "https://trusted.com", nil
				},
			},
			origin:        "https://malicious.com",
			expectAllowed: false,
		},
		{
			name: "custom function error should be returned",
			config: CORSConfig{
				AllowOriginFunc: func(origin string) (bool, error) {
					return false, fmt.Errorf("validation failed")
				},
			},
			origin:      "https://any.com",
			expectError: true,
		},
		{
			name: "AllowOriginFunc should ignore AllowOrigins",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
				AllowOriginFunc: func(origin string) (bool, error) {
					return origin == "https://specific.com", nil
				},
			},
			origin:        "https://any.com",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			if tt.expectError {
				assert.Error(t, err, "Expected error from custom function")
				return
			}

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectAllowed {
				assert.Equal(t, tt.origin, allowOrigin, "Origin should be allowed")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}
		})
	}
}

func TestCORS_SubdomainMatching(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectAllowed bool
	}{
		{
			name: "subdomain wildcard should match subdomains via regex pattern",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "https://api.example.com",
			expectAllowed: true, // Pattern matching converts * to .* regex
		},
		{
			name: "exact match should work",
			config: CORSConfig{
				AllowOrigins: []string{"https://api.example.com"},
			},
			origin:        "https://api.example.com",
			expectAllowed: true,
		},
		{
			name: "different schemes should not match",
			config: CORSConfig{
				AllowOrigins: []string{"http://api.example.com"},
			},
			origin:        "https://api.example.com",
			expectAllowed: false,
		},
		{
			name: "wildcard should match any scheme via regex pattern",
			config: CORSConfig{
				AllowOrigins: []string{"*://api.example.com"},
			},
			origin:        "https://api.example.com",
			expectAllowed: true, // Pattern matching converts * to .* regex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectAllowed {
				assert.NotEmpty(t, allowOrigin, "Origin should be allowed")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}
		})
	}
}

func TestCORS_PreflightRequests(t *testing.T) {
	tests := []struct {
		name            string
		config          CORSConfig
		origin          string
		requestMethod   string
		requestHeaders  string
		expectedStatus  int
		expectedHeaders map[string]string
	}{
		{
			name: "valid preflight should return 204",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:         "https://example.com",
			requestMethod:  "POST",
			expectedStatus: http.StatusNoContent,
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:  "https://example.com",
				wo.HeaderAccessControlAllowMethods: "GET,HEAD,PUT,PATCH,POST,DELETE",
			},
		},
		{
			name: "preflight with custom allowed headers",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
				AllowHeaders: []string{"Content-Type", "Authorization"},
			},
			origin:         "https://example.com",
			requestMethod:  "POST",
			expectedStatus: http.StatusNoContent,
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:  "https://example.com",
				wo.HeaderAccessControlAllowMethods: "GET,HEAD,PUT,PATCH,POST,DELETE",
				wo.HeaderAccessControlAllowHeaders: "Content-Type,Authorization",
			},
		},
		{
			name: "preflight should echo requested headers when AllowHeaders is empty",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:         "https://example.com",
			requestMethod:  "POST",
			requestHeaders: "Content-Type,Authorization",
			expectedStatus: http.StatusNoContent,
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:  "https://example.com",
				wo.HeaderAccessControlAllowMethods: "GET,HEAD,PUT,PATCH,POST,DELETE",
				wo.HeaderAccessControlAllowHeaders: "Content-Type,Authorization",
			},
		},
		{
			name: "preflight with max age",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
				MaxAge:       3600,
			},
			origin:         "https://example.com",
			requestMethod:  "POST",
			expectedStatus: http.StatusNoContent,
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:  "https://example.com",
				wo.HeaderAccessControlAllowMethods: "GET,HEAD,PUT,PATCH,POST,DELETE",
				wo.HeaderAccessControlMaxAge:       "3600",
			},
		},
		{
			name: "preflight with negative max age",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
				MaxAge:       -1,
			},
			origin:         "https://example.com",
			requestMethod:  "POST",
			expectedStatus: http.StatusNoContent,
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:  "https://example.com",
				wo.HeaderAccessControlAllowMethods: "GET,HEAD,PUT,PATCH,POST,DELETE",
				wo.HeaderAccessControlMaxAge:       "0",
			},
		},
		{
			name: "invalid origin preflight should return 204 without headers",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:         "https://malicious.com",
			requestMethod:  "POST",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}
			if tt.requestMethod != "" {
				headers[wo.HeaderAccessControlRequestMethod] = tt.requestMethod
			}
			if tt.requestHeaders != "" {
				headers[wo.HeaderAccessControlRequestHeaders] = tt.requestHeaders
			}

			event := newCORSTestEvent("OPTIONS", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, event.Response().Status, "Expected status code")

			responseHeaders := event.Response().Header()
			for header, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, responseHeaders.Get(header), "Header %s should match expected value", header)
			}

			// Vary headers should always be set for CORS
			assert.Equal(t, wo.HeaderOrigin, responseHeaders.Get(wo.HeaderVary), "Vary header should include Origin")

			// Preflight specific vary headers are only set when origin is allowed
			if tt.expectedStatus == http.StatusNoContent && len(tt.expectedHeaders) > 0 {
				varyHeader := responseHeaders[wo.HeaderVary]
				assert.Contains(t, varyHeader, wo.HeaderAccessControlRequestMethod, "Vary should include Access-Control-Request-Method")
				assert.Contains(t, varyHeader, wo.HeaderAccessControlRequestHeaders, "Vary should include Access-Control-Request-Headers")
			}
		})
	}
}

func TestCORS_SimpleRequests(t *testing.T) {
	tests := []struct {
		name             string
		config           CORSConfig
		origin           string
		method           string
		expectedHeaders  map[string]string
		expectNextCalled bool
	}{
		{
			name: "simple request should set CORS headers and call next",
			config: CORSConfig{
				AllowOrigins:  []string{"https://example.com"},
				ExposeHeaders: []string{"X-Custom-Header"},
			},
			origin: "https://example.com",
			method: "GET",
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:   "https://example.com",
				wo.HeaderAccessControlExposeHeaders: "X-Custom-Header",
			},
			expectNextCalled: true,
		},
		{
			name: "simple request without allowed origin should call next",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:           "https://malicious.com",
			method:           "GET",
			expectNextCalled: true,
		},
		{
			name: "simple request with credentials",
			config: CORSConfig{
				AllowOrigins:     []string{"https://example.com"},
				AllowCredentials: true,
			},
			origin: "https://example.com",
			method: "GET",
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin:      "https://example.com",
				wo.HeaderAccessControlAllowCredentials: "true",
			},
			expectNextCalled: true,
		},
		{
			name: "simple request without exposed headers should not set expose header",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin: "https://example.com",
			method: "GET",
			expectedHeaders: map[string]string{
				wo.HeaderAccessControlAllowOrigin: "https://example.com",
			},
			expectNextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newTestCORSEvent(tt.method, "http://example.com/api", headers)

			middleware := CORS[*testCORSEvent](tt.config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectNextCalled, event.nextCalled, "Next() should be called as expected")

			responseHeaders := event.Response().Header()
			for header, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, responseHeaders.Get(header), "Header %s should match expected value", header)
			}

			// Vary header should always be set
			assert.Equal(t, wo.HeaderOrigin, responseHeaders.Get(wo.HeaderVary), "Vary header should include Origin")
		})
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	tests := []struct {
		name           string
		config         CORSConfig
		method         string
		expectNext     bool
		expectedStatus int
	}{
		{
			name:           "simple request without origin should call next",
			config:         CORSConfig{},
			method:         "GET",
			expectNext:     true,
			expectedStatus: 0, // No status written yet
		},
		{
			name:           "preflight request without origin should return 204",
			config:         CORSConfig{},
			method:         "OPTIONS",
			expectNext:     false,
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newTestCORSEvent(tt.method, "http://example.com/api", nil)

			middleware := CORS[*testCORSEvent](tt.config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectNext, event.nextCalled, "Next() should be called as expected")
			assert.Equal(t, tt.expectedStatus, event.Response().Status, "Expected status code")

			// No CORS headers should be set without Origin header
			assert.Empty(t, event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin))
		})
	}
}

func TestCORS_MaxAge(t *testing.T) {
	tests := []struct {
		name            string
		config          CORSConfig
		expectedMaxAge  string
		expectHeaderSet bool
	}{
		{
			name: "positive max age should set header",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
				MaxAge:       3600,
			},
			expectedMaxAge:  "3600",
			expectHeaderSet: true,
		},
		{
			name: "zero max age should not set header",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
				MaxAge:       0,
			},
			expectHeaderSet: false,
		},
		{
			name: "negative max age should set to 0",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
				MaxAge:       -1,
			},
			expectedMaxAge:  "0",
			expectHeaderSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: "https://example.com",
			}

			event := newCORSTestEvent("OPTIONS", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			maxAgeHeader := event.Response().Header().Get(wo.HeaderAccessControlMaxAge)
			if tt.expectHeaderSet {
				assert.Equal(t, tt.expectedMaxAge, maxAgeHeader, "Max-Age header should match expected value")
			} else {
				assert.Empty(t, maxAgeHeader, "Max-Age header should not be set")
			}
		})
	}
}

func TestCORS_Skipper(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		skippers      []Skipper[*testCORSEvent]
		origin        string
		expectHeaders bool
		expectNext    bool
	}{
		{
			name: "no skippers should set headers",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			skippers:      []Skipper[*testCORSEvent]{},
			origin:        "https://example.com",
			expectHeaders: true,
			expectNext:    true,
		},
		{
			name: "skipper returning true should skip CORS",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			skippers: []Skipper[*testCORSEvent]{
				func(e *testCORSEvent) bool { return true },
			},
			origin:        "https://example.com",
			expectHeaders: false,
			expectNext:    true,
		},
		{
			name: "skipper returning false should process CORS",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			skippers: []Skipper[*testCORSEvent]{
				func(e *testCORSEvent) bool { return false },
			},
			origin:        "https://example.com",
			expectHeaders: true,
			expectNext:    true,
		},
		{
			name: "prefix skipper should skip CORS for matching path",
			config: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			skippers: []Skipper[*testCORSEvent]{
				PrefixPathSkipper[*testCORSEvent]("/api/"),
			},
			origin:        "https://example.com",
			expectHeaders: false,
			expectNext:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newTestCORSEvent("GET", "http://example.com/api/users", headers)

			middleware := CORS[*testCORSEvent](tt.config, tt.skippers...)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectNext, event.nextCalled, "Next() should be called as expected")

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectHeaders {
				assert.NotEmpty(t, allowOrigin, "CORS headers should be set")
			} else {
				assert.Empty(t, allowOrigin, "CORS headers should not be set")
			}
		})
	}
}

func TestCORS_LongOriginHandling(t *testing.T) {
	tests := []struct {
		name          string
		config        CORSConfig
		origin        string
		expectAllowed bool
	}{
		{
			name: "very long origin should be rejected for regex",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "https://" + strings.Repeat("a", 300) + ".example.com",
			expectAllowed: false,
		},
		{
			name: "origin without protocol should be rejected",
			config: CORSConfig{
				AllowOrigins: []string{"https://*.example.com"},
			},
			origin:        "not-a-real-origin",
			expectAllowed: false,
		},
		{
			name: "malformed origin should be handled gracefully",
			config: CORSConfig{
				AllowOrigins: []string{"https://example.com"},
			},
			origin:        "https://",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderOrigin: tt.origin,
			}

			event := newCORSTestEvent("GET", "http://example.com/api", headers)
			middleware := CORS[*wo.Event](tt.config)
			err := middleware(event)

			assert.NoError(t, err)

			allowOrigin := event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin)
			if tt.expectAllowed {
				assert.NotEmpty(t, allowOrigin, "Origin should be allowed")
			} else {
				assert.Empty(t, allowOrigin, "Origin should not be allowed")
			}
		})
	}
}

func TestCORS_InvalidPatterns(t *testing.T) {
	// Test that invalid regex patterns don't crash the middleware
	config := CORSConfig{
		AllowOrigins: []string{
			"https://[invalid",      // Invalid regex
			"https://example.com",   // Valid origin
			"https://*.example.com", // Valid pattern
		},
	}

	headers := map[string]string{
		wo.HeaderOrigin: "https://api.example.com",
	}

	event := newCORSTestEvent("GET", "http://example.com/api", headers)
	middleware := CORS[*wo.Event](config)
	err := middleware(event)

	assert.NoError(t, err)
	// Should still work with valid patterns
	assert.Equal(t, "https://api.example.com", event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin))
}

// Test helper function matchScheme
func TestMatchScheme(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		pattern  string
		expected bool
	}{
		{
			name:     "matching schemes",
			domain:   "https://example.com",
			pattern:  "https://*.example.com",
			expected: true,
		},
		{
			name:     "different schemes",
			domain:   "http://example.com",
			pattern:  "https://*.example.com",
			expected: false,
		},
		{
			name:     "domain without scheme",
			domain:   "example.com",
			pattern:  "https://example.com",
			expected: false,
		},
		{
			name:     "pattern without scheme",
			domain:   "https://example.com",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "both without scheme",
			domain:   "example.com",
			pattern:  "example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchScheme(tt.domain, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test helper function matchSubdomain
func TestMatchSubdomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			domain:   "https://api.example.com",
			pattern:  "https://api.example.com",
			expected: false, // matchSubdomain only handles wildcards
		},
		{
			name:     "wildcard match",
			domain:   "https://api.example.com",
			pattern:  "https://*.example.com",
			expected: true,
		},
		{
			name:     "subdomain wildcard",
			domain:   "https://v1.api.example.com",
			pattern:  "https://*.example.com",
			expected: true,
		},
		{
			name:     "different domain",
			domain:   "https://api.malicious.com",
			pattern:  "https://*.example.com",
			expected: false,
		},
		{
			name:     "different schemes",
			domain:   "http://api.example.com",
			pattern:  "https://*.example.com",
			expected: false,
		},
		{
			name:     "no scheme in domain",
			domain:   "api.example.com",
			pattern:  "https://*.example.com",
			expected: false,
		},
		{
			name:     "no scheme in pattern",
			domain:   "https://api.example.com",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "very long domain",
			domain:   "https://" + strings.Repeat("a", 300) + ".example.com",
			pattern:  "https://*.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchSubdomain(tt.domain, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCORS_DefaultBehaviorIntegration(t *testing.T) {
	// Test that the middleware works with default configuration in a realistic scenario
	config := CORSConfig{}

	headers := map[string]string{
		wo.HeaderOrigin: "https://myapp.com",
	}

	// Test simple request
	event := newCORSTestEvent("GET", "http://example.com/api/users", headers)
	middleware := CORS[*wo.Event](config)
	err := middleware(event)

	assert.NoError(t, err)
	assert.Equal(t, "*", event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin))
	assert.Equal(t, wo.HeaderOrigin, event.Response().Header().Get(wo.HeaderVary))

	// Test preflight request
	event = newCORSTestEvent("OPTIONS", "http://example.com/api/users", headers)
	event.Request().Header.Set(wo.HeaderAccessControlRequestMethod, "POST")
	err = middleware(event)

	assert.NoError(t, err)
	assert.Equal(t, "*", event.Response().Header().Get(wo.HeaderAccessControlAllowOrigin))
	assert.Equal(t, "GET,HEAD,PUT,PATCH,POST,DELETE", event.Response().Header().Get(wo.HeaderAccessControlAllowMethods))
	assert.Equal(t, http.StatusNoContent, event.Response().Status)
}
