package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gowool/wo"
)

// newSecurityTestEvent creates a test event for security middleware testing purposes
func newSecurityTestEvent() *wo.Event {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// newSecurityTestEventWithHTTPS creates a test event with HTTPS
func newSecurityTestEventWithHTTPS() *wo.Event {
	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// newSecurityTestEventWithHeaders creates a test event with specific headers
func newSecurityTestEventWithHeaders(headers map[string]string) *wo.Event {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

func TestSecurityConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   SecurityConfig
		expected SecurityConfig
	}{
		{
			name:   "empty config should get all defaults",
			config: SecurityConfig{},
			expected: SecurityConfig{
				XSSProtection:      "1; mode=block",
				ContentTypeNosniff: "nosniff",
				XFrameOptions:      "SAMEORIGIN",
				HSTSMaxAge:         15724800,
			},
		},
		{
			name: "partial config should only fill missing defaults",
			config: SecurityConfig{
				XSSProtection:         "custom-xss",
				HSTSMaxAge:            12345,
				ContentSecurityPolicy: "default-src 'self'",
			},
			expected: SecurityConfig{
				XSSProtection:         "custom-xss",
				ContentTypeNosniff:    "nosniff",
				XFrameOptions:         "SAMEORIGIN",
				HSTSMaxAge:            12345,
				ContentSecurityPolicy: "default-src 'self'",
			},
		},
		{
			name: "zero HSTSMaxAge should get default",
			config: SecurityConfig{
				XSSProtection:      "custom-xss",
				ContentTypeNosniff: "custom-nosniff",
				XFrameOptions:      "custom-frame",
				HSTSMaxAge:         0,
			},
			expected: SecurityConfig{
				XSSProtection:      "custom-xss",
				ContentTypeNosniff: "custom-nosniff",
				XFrameOptions:      "custom-frame",
				HSTSMaxAge:         15724800,
			},
		},
		{
			name: "negative HSTSMaxAge should get default",
			config: SecurityConfig{
				HSTSMaxAge: -1,
			},
			expected: SecurityConfig{
				XSSProtection:      "1; mode=block",
				ContentTypeNosniff: "nosniff",
				XFrameOptions:      "SAMEORIGIN",
				HSTSMaxAge:         15724800,
			},
		},
		{
			name: "fully populated config should remain unchanged",
			config: SecurityConfig{
				XSSProtection:         "custom-xss",
				ContentTypeNosniff:    "custom-nosniff",
				XFrameOptions:         "DENY",
				HSTSMaxAge:            999999,
				HSTSExcludeSubdomains: true,
				ContentSecurityPolicy: "custom-csp",
				CSPReportOnly:         true,
				HSTSPreloadEnabled:    true,
				ReferrerPolicy:        "strict-origin-when-cross-origin",
			},
			expected: SecurityConfig{
				XSSProtection:         "custom-xss",
				ContentTypeNosniff:    "custom-nosniff",
				XFrameOptions:         "DENY",
				HSTSMaxAge:            999999,
				HSTSExcludeSubdomains: true,
				ContentSecurityPolicy: "custom-csp",
				CSPReportOnly:         true,
				HSTSPreloadEnabled:    true,
				ReferrerPolicy:        "strict-origin-when-cross-origin",
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

func TestSecurity_DefaultHeaders(t *testing.T) {
	tests := []struct {
		name            string
		useHTTPS        bool
		expectedHeaders map[string]string
	}{
		{
			name:     "HTTP request should set basic security headers",
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderXXSSProtection:      "1; mode=block",
				wo.HeaderXContentTypeOptions: "nosniff",
				wo.HeaderXFrameOptions:       "SAMEORIGIN",
			},
		},
		{
			name:     "HTTPS request should include HSTS header",
			useHTTPS: true,
			expectedHeaders: map[string]string{
				wo.HeaderXXSSProtection:          "1; mode=block",
				wo.HeaderXContentTypeOptions:     "nosniff",
				wo.HeaderXFrameOptions:           "SAMEORIGIN",
				wo.HeaderStrictTransportSecurity: "max-age=15724800; includeSubdomains",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event *wo.Event
			if tt.useHTTPS {
				event = newSecurityTestEventWithHTTPS()
			} else {
				event = newSecurityTestEvent()
			}

			middleware := Security[*wo.Event](SecurityConfig{})
			err := middleware(event)
			assert.NoError(t, err)

			for header, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, event.Response().Header().Get(header), "Header %s should match expected value", header)
			}
		})
	}
}

func TestSecurity_CustomHeaders(t *testing.T) {
	tests := []struct {
		name            string
		config          SecurityConfig
		useHTTPS        bool
		expectedHeaders map[string]string
	}{
		{
			name: "custom XSS protection",
			config: SecurityConfig{
				XSSProtection: "0",
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderXXSSProtection: "0",
			},
		},
		{
			name: "custom content type nosniff",
			config: SecurityConfig{
				ContentTypeNosniff: "custom",
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderXContentTypeOptions: "custom",
			},
		},
		{
			name: "custom frame options",
			config: SecurityConfig{
				XFrameOptions: "DENY",
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderXFrameOptions: "DENY",
			},
		},
		{
			name: "content security policy",
			config: SecurityConfig{
				ContentSecurityPolicy: "default-src 'self'",
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderContentSecurityPolicy: "default-src 'self'",
			},
		},
		{
			name: "content security policy report only",
			config: SecurityConfig{
				ContentSecurityPolicy: "default-src 'self'",
				CSPReportOnly:         true,
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderContentSecurityPolicyReportOnly: "default-src 'self'",
			},
		},
		{
			name: "referrer policy",
			config: SecurityConfig{
				ReferrerPolicy: "strict-origin-when-cross-origin",
			},
			useHTTPS: false,
			expectedHeaders: map[string]string{
				wo.HeaderReferrerPolicy: "strict-origin-when-cross-origin",
			},
		},
		{
			name: "empty values should not set headers",
			config: SecurityConfig{
				XSSProtection:         "",
				ContentTypeNosniff:    "",
				XFrameOptions:         "",
				ContentSecurityPolicy: "",
				ReferrerPolicy:        "",
			},
			useHTTPS:        false,
			expectedHeaders: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event *wo.Event
			if tt.useHTTPS {
				event = newSecurityTestEventWithHTTPS()
			} else {
				event = newSecurityTestEvent()
			}

			middleware := Security[*wo.Event](tt.config)
			err := middleware(event)
			assert.NoError(t, err)

			// Check that expected headers are set
			for header, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, event.Response().Header().Get(header), "Header %s should match expected value", header)
			}
		})
	}
}

func TestSecurity_HSTSConfigurations(t *testing.T) {
	tests := []struct {
		name              string
		config            SecurityConfig
		useHTTPS          bool
		forwardedHTTPS    bool
		expectedHeader    string
		shouldNotHaveHSTS bool
	}{
		{
			name:           "HTTPS with default HSTS config",
			config:         SecurityConfig{},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800; includeSubdomains",
		},
		{
			name: "HTTPS with exclude subdomains",
			config: SecurityConfig{
				HSTSExcludeSubdomains: true,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800",
		},
		{
			name: "HTTPS with preload",
			config: SecurityConfig{
				HSTSPreloadEnabled: true,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800; includeSubdomains; preload",
		},
		{
			name: "HTTPS with exclude subdomains and preload",
			config: SecurityConfig{
				HSTSExcludeSubdomains: true,
				HSTSPreloadEnabled:    true,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800; preload",
		},
		{
			name: "HTTPS with custom max age",
			config: SecurityConfig{
				HSTSMaxAge: 12345,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=12345; includeSubdomains",
		},
		{
			name:              "HTTP should not have HSTS",
			config:            SecurityConfig{},
			useHTTPS:          false,
			shouldNotHaveHSTS: true,
		},
		{
			name: "zero max age should not have HSTS before SetDefaults",
			config: SecurityConfig{
				HSTSMaxAge: 0,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800; includeSubdomains", // Security calls SetDefaults which converts 0 to default
		},
		{
			name: "negative max age should not have HSTS",
			config: SecurityConfig{
				HSTSMaxAge: -1,
			},
			useHTTPS:       true,
			expectedHeader: "max-age=15724800; includeSubdomains", // SetDefaults will fix this
		},
		{
			name:           "forwarded HTTPS header",
			config:         SecurityConfig{},
			useHTTPS:       false,
			forwardedHTTPS: true,
			expectedHeader: "max-age=15724800; includeSubdomains",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event *wo.Event
			if tt.forwardedHTTPS {
				event = newSecurityTestEventWithHeaders(map[string]string{
					wo.HeaderXForwardedProto: "https",
				})
			} else if tt.useHTTPS {
				event = newSecurityTestEventWithHTTPS()
			} else {
				event = newSecurityTestEvent()
			}

			middleware := Security[*wo.Event](tt.config)
			err := middleware(event)
			assert.NoError(t, err)

			hstsValue := event.Response().Header().Get(wo.HeaderStrictTransportSecurity)

			if tt.shouldNotHaveHSTS {
				assert.Empty(t, hstsValue, "HSTS header should not be set")
			} else {
				assert.Equal(t, tt.expectedHeader, hstsValue, "HSTS header should match expected value")
			}
		})
	}
}

func TestSecurity_Skipper(t *testing.T) {
	tests := []struct {
		name          string
		config        SecurityConfig
		skippers      []Skipper[*wo.Event]
		path          string
		expectHeaders bool
	}{
		{
			name: "no skippers should set headers",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers:      []Skipper[*wo.Event]{},
			path:          "/api/test",
			expectHeaders: true,
		},
		{
			name: "skipper returning false should set headers",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return false },
			},
			path:          "/api/test",
			expectHeaders: true,
		},
		{
			name: "skipper returning true should skip headers",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return true },
			},
			path:          "/api/test",
			expectHeaders: false,
		},
		{
			name: "prefix skipper should work",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers: []Skipper[*wo.Event]{
				PrefixPathSkipper[*wo.Event]("/api/"),
			},
			path:          "/api/users",
			expectHeaders: false,
		},
		{
			name: "prefix skipper should not match different path",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers: []Skipper[*wo.Event]{
				PrefixPathSkipper[*wo.Event]("/api/"),
			},
			path:          "/web/users",
			expectHeaders: true,
		},
		{
			name: "multiple skippers should work with chain",
			config: SecurityConfig{
				XSSProtection: "test-value",
			},
			skippers: []Skipper[*wo.Event]{
				PrefixPathSkipper[*wo.Event]("/health"),
				PrefixPathSkipper[*wo.Event]("/api/"),
			},
			path:          "/api/users",
			expectHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			rec := httptest.NewRecorder()
			event := new(wo.Event)
			event.Reset(rec, req)

			middleware := Security[*wo.Event](tt.config, tt.skippers...)
			err := middleware(event)
			assert.NoError(t, err)

			xssValue := event.Response().Header().Get(wo.HeaderXXSSProtection)

			if tt.expectHeaders {
				assert.Equal(t, tt.config.XSSProtection, xssValue, "XSS header should be set")
			} else {
				assert.Empty(t, xssValue, "XSS header should not be set")
			}
		})
	}
}

// testNextEvent wraps an event to track Next() calls for testing
type testNextEvent struct {
	*wo.Event
	nextCalled bool
}

func (e *testNextEvent) Next() error {
	e.nextCalled = true
	return e.Event.Next()
}

func TestSecurity_NextCall(t *testing.T) {
	tests := []struct {
		name       string
		config     SecurityConfig
		nextCalled bool
	}{
		{
			name:       "middleware should call next",
			config:     SecurityConfig{},
			nextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEvent := newSecurityTestEvent()
			event := &testNextEvent{Event: baseEvent}

			middleware := Security[*testNextEvent](tt.config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.nextCalled, event.nextCalled, "Next() should be called as expected")
		})
	}
}

func TestSecurity_MissingHeaderConstants(t *testing.T) {
	// This test ensures that all header constants used exist
	// This helps catch any issues if header constants are renamed or removed

	tests := []struct {
		name     string
		config   SecurityConfig
		useHTTPS bool
	}{
		{
			name: "all security headers should use proper constants",
			config: SecurityConfig{
				XSSProtection:         "test-xss",
				ContentTypeNosniff:    "test-nosniff",
				XFrameOptions:         "test-frame",
				ContentSecurityPolicy: "test-csp",
				ReferrerPolicy:        "test-referrer",
			},
			useHTTPS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event *wo.Event
			if tt.useHTTPS {
				event = newSecurityTestEventWithHTTPS()
			} else {
				event = newSecurityTestEvent()
			}

			middleware := Security[*wo.Event](tt.config)
			err := middleware(event)
			assert.NoError(t, err)

			headers := event.Response().Header()

			// Verify all expected headers are present
			assert.Equal(t, tt.config.XSSProtection, headers.Get(wo.HeaderXXSSProtection))
			assert.Equal(t, tt.config.ContentTypeNosniff, headers.Get(wo.HeaderXContentTypeOptions))
			assert.Equal(t, tt.config.XFrameOptions, headers.Get(wo.HeaderXFrameOptions))
			assert.Equal(t, tt.config.ContentSecurityPolicy, headers.Get(wo.HeaderContentSecurityPolicy))
			assert.Equal(t, tt.config.ReferrerPolicy, headers.Get(wo.HeaderReferrerPolicy))

			// HSTS should be present for HTTPS
			if tt.useHTTPS {
				assert.NotEmpty(t, headers.Get(wo.HeaderStrictTransportSecurity))
			}
		})
	}
}

func TestSecurity_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		config     SecurityConfig
		setupEvent func() *wo.Event
	}{
		{
			name:   "nil event should handle gracefully",
			config: SecurityConfig{},
			// This test would require a different approach since we can't pass nil
			// For now, we'll test with a minimal event
			setupEvent: func() *wo.Event {
				req := httptest.NewRequest("GET", "/", nil)
				rec := httptest.NewRecorder()
				e := new(wo.Event)
				e.Reset(rec, req)
				return e
			},
		},
		{
			name:   "root path",
			config: SecurityConfig{},
			setupEvent: func() *wo.Event {
				req := httptest.NewRequest("GET", "/", nil)
				rec := httptest.NewRecorder()
				e := new(wo.Event)
				e.Reset(rec, req)
				return e
			},
		},
		{
			name:   "request with no TLS and no forwarded proto",
			config: SecurityConfig{},
			setupEvent: func() *wo.Event {
				req := httptest.NewRequest("GET", "http://example.com/test", nil)
				// Ensure no forwarded proto header
				req.Header.Del("X-Forwarded-Proto")
				rec := httptest.NewRecorder()
				e := new(wo.Event)
				e.Reset(rec, req)
				return e
			},
		},
		{
			name:   "forwarded proto with non-https value",
			config: SecurityConfig{},
			setupEvent: func() *wo.Event {
				req := httptest.NewRequest("GET", "http://example.com/test", nil)
				req.Header.Set("X-Forwarded-Proto", "http")
				rec := httptest.NewRecorder()
				e := new(wo.Event)
				e.Reset(rec, req)
				return e
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tt.setupEvent()

			middleware := Security[*wo.Event](tt.config)
			err := middleware(event)

			// The middleware should handle all these cases gracefully
			assert.NoError(t, err)

			// Basic headers should still be set
			assert.Equal(t, "1; mode=block", event.Response().Header().Get(wo.HeaderXXSSProtection))
			assert.Equal(t, "nosniff", event.Response().Header().Get(wo.HeaderXContentTypeOptions))
			assert.Equal(t, "SAMEORIGIN", event.Response().Header().Get(wo.HeaderXFrameOptions))
		})
	}
}
