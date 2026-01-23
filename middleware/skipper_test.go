package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gowool/wo"
)

// newSkipperTestEvent creates a test event for skipper testing purposes
func newSkipperTestEvent() *wo.Event {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// newSkipperTestEventWithRequest creates a test event with specified HTTP request
func newSkipperTestEventWithRequest(method, path string) *wo.Event {
	req := httptest.NewRequest(method, "http://example.com"+path, nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

func TestChainSkipper(t *testing.T) {
	tests := []struct {
		name     string
		skippers []Skipper[*wo.Event]
		want     bool
	}{
		{
			name:     "empty skippers should return false",
			skippers: []Skipper[*wo.Event]{},
			want:     false,
		},
		{
			name: "one skipper returning false",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return false },
			},
			want: false,
		},
		{
			name: "one skipper returning true",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return true },
			},
			want: true,
		},
		{
			name: "multiple skippers all returning false",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return false },
			},
			want: false,
		},
		{
			name: "multiple skippers with first returning true",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return true },
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return false },
			},
			want: true,
		},
		{
			name: "multiple skippers with middle returning true",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return true },
				func(e *wo.Event) bool { return false },
			},
			want: true,
		},
		{
			name: "multiple skippers with last returning true",
			skippers: []Skipper[*wo.Event]{
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return false },
				func(e *wo.Event) bool { return true },
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := ChainSkipper(tt.skippers...)
			resolver := newSkipperTestEvent()
			assert.Equal(t, tt.want, skipper(resolver))
		})
	}
}

func TestPrefixPathSkipper(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		prefixes []string
		want     bool
	}{
		{
			name:     "empty prefixes should return false",
			method:   "GET",
			path:     "/api/users",
			prefixes: []string{},
			want:     false,
		},
		{
			name:     "matching prefix without method",
			method:   "GET",
			path:     "/api/users",
			prefixes: []string{"/api"},
			want:     true,
		},
		{
			name:     "matching prefix with correct method",
			method:   "GET",
			path:     "/api/users",
			prefixes: []string{"GET /api"},
			want:     true,
		},
		{
			name:     "matching prefix with wrong method",
			method:   "POST",
			path:     "/api/users",
			prefixes: []string{"GET /api"},
			want:     false,
		},
		{
			name:     "non-matching prefix",
			method:   "GET",
			path:     "/users",
			prefixes: []string{"/api"},
			want:     false,
		},
		{
			name:     "case insensitive matching - method",
			method:   "get",
			path:     "/api/users",
			prefixes: []string{"GET /api"},
			want:     true,
		},
		{
			name:     "case insensitive matching - path",
			method:   "GET",
			path:     "/API/users",
			prefixes: []string{"/api"},
			want:     true,
		},
		{
			name:     "case insensitive matching - prefix",
			method:   "GET",
			path:     "/api/users",
			prefixes: []string{"/API"},
			want:     true,
		},
		{
			name:     "multiple prefixes - first matches",
			method:   "GET",
			path:     "/api/users",
			prefixes: []string{"/api", "/auth", "/admin"},
			want:     true,
		},
		{
			name:     "multiple prefixes - middle matches",
			method:   "GET",
			path:     "/auth/login",
			prefixes: []string{"/api", "/auth", "/admin"},
			want:     true,
		},
		{
			name:     "multiple prefixes - last matches",
			method:   "GET",
			path:     "/admin/dashboard",
			prefixes: []string{"/api", "/auth", "/admin"},
			want:     true,
		},
		{
			name:     "multiple prefixes - none match",
			method:   "GET",
			path:     "/public/home",
			prefixes: []string{"/api", "/auth", "/admin"},
			want:     false,
		},
		{
			name:     "nested path matching",
			method:   "GET",
			path:     "/api/v1/users/123",
			prefixes: []string{"/api/v1"},
			want:     true,
		},
		{
			name:     "exact match also works as prefix",
			method:   "GET",
			path:     "/api",
			prefixes: []string{"/api"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := PrefixPathSkipper[*wo.Event](tt.prefixes...)
			resolver := newSkipperTestEventWithRequest(tt.method, tt.path)
			assert.Equal(t, tt.want, skipper(resolver))
		})
	}
}

func TestSuffixPathSkipper(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		suffixes []string
		want     bool
	}{
		{
			name:     "empty suffixes should return false",
			method:   "GET",
			path:     "/users.json",
			suffixes: []string{},
			want:     false,
		},
		{
			name:     "matching suffix without method",
			method:   "GET",
			path:     "/users.json",
			suffixes: []string{".json"},
			want:     true,
		},
		{
			name:     "matching suffix with correct method",
			method:   "GET",
			path:     "/users.json",
			suffixes: []string{"GET .json"},
			want:     true,
		},
		{
			name:     "matching suffix with wrong method",
			method:   "POST",
			path:     "/users.json",
			suffixes: []string{"GET .json"},
			want:     false,
		},
		{
			name:     "non-matching suffix",
			method:   "GET",
			path:     "/users.xml",
			suffixes: []string{".json"},
			want:     false,
		},
		{
			name:     "case insensitive matching - method",
			method:   "get",
			path:     "/users.json",
			suffixes: []string{"GET .json"},
			want:     true,
		},
		{
			name:     "case insensitive matching - path",
			method:   "GET",
			path:     "/users.JSON",
			suffixes: []string{".json"},
			want:     true,
		},
		{
			name:     "case insensitive matching - suffix",
			method:   "GET",
			path:     "/users.json",
			suffixes: []string{".JSON"},
			want:     true,
		},
		{
			name:     "multiple suffixes - first matches",
			method:   "GET",
			path:     "/users.json",
			suffixes: []string{".json", ".xml", ".html"},
			want:     true,
		},
		{
			name:     "multiple suffixes - middle matches",
			method:   "GET",
			path:     "/users.xml",
			suffixes: []string{".json", ".xml", ".html"},
			want:     true,
		},
		{
			name:     "multiple suffixes - last matches",
			method:   "GET",
			path:     "/users.html",
			suffixes: []string{".json", ".xml", ".html"},
			want:     true,
		},
		{
			name:     "multiple suffixes - none match",
			method:   "GET",
			path:     "/users.txt",
			suffixes: []string{".json", ".xml", ".html"},
			want:     false,
		},
		{
			name:     "path with multiple dots",
			method:   "GET",
			path:     "/api/v1/users.json",
			suffixes: []string{".json"},
			want:     true,
		},
		{
			name:     "exact match",
			method:   "GET",
			path:     "/.json",
			suffixes: []string{".json"},
			want:     true,
		},
		{
			name:     "empty path",
			method:   "GET",
			path:     "",
			suffixes: []string{".json"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := SuffixPathSkipper[*wo.Event](tt.suffixes...)
			resolver := newSkipperTestEventWithRequest(tt.method, tt.path)
			assert.Equal(t, tt.want, skipper(resolver))
		})
	}
}

func TestEqualPathSkipper(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		paths  []string
		want   bool
	}{
		{
			name:   "empty paths should return false",
			method: "GET",
			path:   "/users",
			paths:  []string{},
			want:   false,
		},
		{
			name:   "exact match without method",
			method: "GET",
			path:   "/users",
			paths:  []string{"/users"},
			want:   true,
		},
		{
			name:   "exact match with correct method",
			method: "GET",
			path:   "/users",
			paths:  []string{"GET /users"},
			want:   true,
		},
		{
			name:   "exact match with wrong method",
			method: "POST",
			path:   "/users",
			paths:  []string{"GET /users"},
			want:   false,
		},
		{
			name:   "non-matching path",
			method: "GET",
			path:   "/users",
			paths:  []string{"/api/users"},
			want:   false,
		},
		{
			name:   "case sensitive method matching",
			method: "get",
			path:   "/users",
			paths:  []string{"GET /users"},
			want:   false, // EqualPathSkipper is case sensitive for methods
		},
		{
			name:   "case insensitive matching - path",
			method: "GET",
			path:   "/USERS",
			paths:  []string{"/users"},
			want:   true,
		},
		{
			name:   "case insensitive matching - paths array",
			method: "GET",
			path:   "/users",
			paths:  []string{"/USERS"},
			want:   true,
		},
		{
			name:   "multiple paths - first matches",
			method: "GET",
			path:   "/users",
			paths:  []string{"/users", "/auth", "/admin"},
			want:   true,
		},
		{
			name:   "multiple paths - middle matches",
			method: "GET",
			path:   "/auth",
			paths:  []string{"/users", "/auth", "/admin"},
			want:   true,
		},
		{
			name:   "multiple paths - last matches",
			method: "GET",
			path:   "/admin",
			paths:  []string{"/users", "/auth", "/admin"},
			want:   true,
		},
		{
			name:   "multiple paths - none match",
			method: "GET",
			path:   "/public",
			paths:  []string{"/users", "/auth", "/admin"},
			want:   false,
		},
		{
			name:   "empty path match",
			method: "GET",
			path:   "",
			paths:  []string{""},
			want:   true,
		},
		{
			name:   "root path match",
			method: "GET",
			path:   "/",
			paths:  []string{"/"},
			want:   true,
		},
		{
			name:   "subdirectory should not match parent",
			method: "GET",
			path:   "/api/users",
			paths:  []string{"/api"},
			want:   false,
		},
		{
			name:   "parent should not match subdirectory",
			method: "GET",
			path:   "/api",
			paths:  []string{"/api/users"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := EqualPathSkipper[*wo.Event](tt.paths...)
			resolver := newSkipperTestEventWithRequest(tt.method, tt.path)
			assert.Equal(t, tt.want, skipper(resolver))
		})
	}
}

func TestCheckMethod(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		skip         string
		expectedPath string
		expectedOK   bool
	}{
		{
			name:         "no method specified in skip",
			method:       "GET",
			skip:         "/api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "matching method",
			method:       "GET",
			skip:         "GET /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "non-matching method",
			method:       "POST",
			skip:         "GET /api/users",
			expectedPath: "",
			expectedOK:   false,
		},
		{
			name:         "case sensitive method matching",
			method:       "get",
			skip:         "GET /api/users",
			expectedPath: "",
			expectedOK:   false,
		},
		{
			name:         "different matching method",
			method:       "POST",
			skip:         "POST /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "PUT method matching",
			method:       "PUT",
			skip:         "PUT /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "DELETE method matching",
			method:       "DELETE",
			skip:         "DELETE /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "PATCH method matching",
			method:       "PATCH",
			skip:         "PATCH /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "HEAD method matching",
			method:       "HEAD",
			skip:         "HEAD /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "OPTIONS method matching",
			method:       "OPTIONS",
			skip:         "OPTIONS /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "method with empty path",
			method:       "GET",
			skip:         "GET ",
			expectedPath: "",
			expectedOK:   true,
		},
		{
			name:         "malformed pattern - missing path",
			method:       "GET",
			skip:         "GET",
			expectedPath: "GET",
			expectedOK:   true,
		},
		{
			name:         "malformed pattern - extra spaces",
			method:       "GET",
			skip:         "GET   /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "empty skip string",
			method:       "GET",
			skip:         "",
			expectedPath: "",
			expectedOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := CheckMethod(tt.method, tt.skip)
			assert.Equal(t, tt.expectedPath, path)
			assert.Equal(t, tt.expectedOK, ok)
		})
	}
}

func TestIntegration(t *testing.T) {
	// Test that all skippers work well together in realistic scenarios
	t.Run("complex chain with different skippers", func(t *testing.T) {
		// Create a chain that skips API endpoints for admin users or static files
		skipper := ChainSkipper[*wo.Event](
			// Skip all GET requests to static files
			PrefixPathSkipper[*wo.Event]("GET /static/", "GET /public/"),
			// Skip admin API endpoints
			PrefixPathSkipper[*wo.Event]("/api/admin/"),
			// Skip health check endpoint
			EqualPathSkipper[*wo.Event]("/health", "/ready"),
			// Skip all JSON API calls from specific service
			SuffixPathSkipper[*wo.Event]("POST .json", "PUT .json", "DELETE .json"),
		)

		testCases := []struct {
			name   string
			method string
			path   string
			want   bool
		}{
			{
				name:   "static CSS file should be skipped",
				method: "GET",
				path:   "/static/styles.css",
				want:   true,
			},
			{
				name:   "static JS file should be skipped",
				method: "GET",
				path:   "/static/app.js",
				want:   true,
			},
			{
				name:   "public image should be skipped",
				method: "GET",
				path:   "/public/logo.png",
				want:   true,
			},
			{
				name:   "admin API should be skipped",
				method: "POST",
				path:   "/api/admin/users",
				want:   true,
			},
			{
				name:   "health check should be skipped",
				method: "GET",
				path:   "/health",
				want:   true,
			},
			{
				name:   "readiness check should be skipped",
				method: "GET",
				path:   "/ready",
				want:   true,
			},
			{
				name:   "JSON POST should be skipped",
				method: "POST",
				path:   "/api/users.json",
				want:   true,
			},
			{
				name:   "JSON PUT should be skipped",
				method: "PUT",
				path:   "/api/users/123.json",
				want:   true,
			},
			{
				name:   "JSON DELETE should be skipped",
				method: "DELETE",
				path:   "/api/users/123.json",
				want:   true,
			},
			{
				name:   "regular API call should not be skipped",
				method: "GET",
				path:   "/api/users",
				want:   false,
			},
			{
				name:   "non-static GET should not be skipped",
				method: "GET",
				path:   "/dashboard",
				want:   false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resolver := newSkipperTestEventWithRequest(tc.method, tc.path)
				assert.Equal(t, tc.want, skipper(resolver))
			})
		}
	})
}

// Benchmark tests
func BenchmarkChainSkipper(b *testing.B) {
	skippers := make([]Skipper[*wo.Event], 10)
	for i := range skippers {
		// Half return false, half return true
		if i%2 == 0 {
			skippers[i] = func(e *wo.Event) bool { return false }
		} else {
			skippers[i] = func(e *wo.Event) bool { return true }
		}
	}

	skipper := ChainSkipper(skippers...)
	resolver := newSkipperTestEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		skipper(resolver)
	}
}

func BenchmarkPrefixPathSkipper(b *testing.B) {
	prefixes := []string{"/api", "/auth", "/admin", "/static", "/public"}
	skipper := PrefixPathSkipper[*wo.Event](prefixes...)
	resolver := newSkipperTestEventWithRequest("GET", "/api/users")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		skipper(resolver)
	}
}

func BenchmarkSuffixPathSkipper(b *testing.B) {
	suffixes := []string{".json", ".xml", ".html", ".css", ".js"}
	skipper := SuffixPathSkipper[*wo.Event](suffixes...)
	resolver := newSkipperTestEventWithRequest("GET", "/api/users.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		skipper(resolver)
	}
}

func BenchmarkEqualPathSkipper(b *testing.B) {
	paths := []string{"/health", "/ready", "/metrics", "/status"}
	skipper := EqualPathSkipper[*wo.Event](paths...)
	resolver := newSkipperTestEventWithRequest("GET", "/health")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		skipper(resolver)
	}
}
