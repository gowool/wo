package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

func newBodyLimitEvent(body io.Reader, contentLength int64) *wo.Event {
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.ContentLength = contentLength

	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(wo.NewResponse(rec), req)

	return e
}

func Test_BodyLimitConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      BodyLimitConfig
		expected int64
	}{
		{
			name:     "zero limit should set default",
			cfg:      BodyLimitConfig{Limit: 0},
			expected: maxBodySize,
		},
		{
			name:     "non-zero limit should remain unchanged",
			cfg:      BodyLimitConfig{Limit: 1024},
			expected: 1024,
		},
		{
			name:     "negative limit should remain unchanged",
			cfg:      BodyLimitConfig{Limit: -1},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.SetDefaults()
			require.Equal(t, tt.expected, tt.cfg.Limit)
		})
	}
}

func Test_BodyLimit_ContentLength_Check(t *testing.T) {
	tests := []struct {
		name        string
		limit       int64
		contentLen  int64
		shouldError bool
	}{
		{
			name:        "content length within limit",
			limit:       1024,
			contentLen:  512,
			shouldError: false,
		},
		{
			name:        "content length exactly at limit",
			limit:       1024,
			contentLen:  1024,
			shouldError: false,
		},
		{
			name:        "content length exceeds limit",
			limit:       1024,
			contentLen:  2048,
			shouldError: true,
		},
		{
			name:        "negative limit disables checking",
			limit:       -1,
			contentLen:  999999,
			shouldError: false,
		},
		{
			name:        "zero limit disables checking",
			limit:       0,
			contentLen:  999999,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := BodyLimitConfig{Limit: tt.limit}
			middleware := BodyLimit[*wo.Event](cfg)

			e := newBodyLimitEvent(nil, tt.contentLen)
			err := middleware(e)

			if tt.shouldError {
				require.Error(t, err)
				require.ErrorIs(t, err, wo.ErrStatusRequestEntityTooLarge)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_BodyLimit_Reading_Body(t *testing.T) {
	tests := []struct {
		name                string
		limit               int64
		bodyContent         string
		contentLength       int64 // Set to -1 to trigger content length check bypass
		shouldMiddlewareErr bool
		shouldReadErr       bool
		expectError         string
	}{
		{
			name:                "small body within limit",
			limit:               1024,
			bodyContent:         strings.Repeat("a", 512),
			contentLength:       512,
			shouldMiddlewareErr: false,
			shouldReadErr:       false,
		},
		{
			name:                "body exactly at limit",
			limit:               1024,
			bodyContent:         strings.Repeat("b", 1024),
			contentLength:       1024,
			shouldMiddlewareErr: false,
			shouldReadErr:       false,
		},
		{
			name:                "content length exceeds limit",
			limit:               512,
			bodyContent:         strings.Repeat("c", 1024),
			contentLength:       1024,
			shouldMiddlewareErr: true,
			expectError:         "request entity too large",
		},
		{
			name:                "body exceeds limit during read (unknown content length)",
			limit:               512,
			bodyContent:         strings.Repeat("d", 1024),
			contentLength:       -1, // Unknown content length
			shouldMiddlewareErr: false,
			shouldReadErr:       true,
			expectError:         "request entity too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := BodyLimitConfig{Limit: tt.limit}
			middleware := BodyLimit[*wo.Event](cfg)

			body := strings.NewReader(tt.bodyContent)
			e := newBodyLimitEvent(body, tt.contentLength)

			// Apply middleware
			err := middleware(e)

			if tt.shouldMiddlewareErr {
				require.Error(t, err)
				require.ErrorIs(t, err, wo.ErrStatusRequestEntityTooLarge)
				return // Skip reading test if middleware failed
			} else {
				require.NoError(t, err)
			}

			// Try to read the body
			result, readErr := io.ReadAll(e.Request().Body)

			if tt.shouldReadErr {
				require.Error(t, readErr)
				require.ErrorIs(t, readErr, wo.ErrStatusRequestEntityTooLarge)
			} else {
				require.NoError(t, readErr)
				require.Equal(t, tt.bodyContent, string(result))
			}
		})
	}
}

func Test_BodyLimit_Skipper(t *testing.T) {
	cfg := BodyLimitConfig{Limit: 1} // Very small limit

	// Create a skipper that always skips
	skipper := func(e *wo.Event) bool {
		return true
	}

	middleware := BodyLimit[*wo.Event](cfg, skipper)

	// Large body that would normally exceed limit
	largeBody := strings.Repeat("x", 1024)
	e := newBodyLimitEvent(strings.NewReader(largeBody), int64(len(largeBody)))

	err := middleware(e)
	require.NoError(t, err)

	// Should be able to read the full body since middleware was skipped
	result, readErr := io.ReadAll(e.Request().Body)
	require.NoError(t, readErr)
	require.Equal(t, largeBody, string(result))
}

func Test_limitedReader_Read(t *testing.T) {
	tests := []struct {
		name            string
		limit           int64
		content         string
		readBufferSize  int
		shouldError     bool
		expectedContent string
	}{
		{
			name:            "read within limit in one go",
			limit:           1024,
			content:         "hello world",
			readBufferSize:  100,
			shouldError:     false,
			expectedContent: "hello world",
		},
		{
			name:            "multiple reads within limit",
			limit:           10,
			content:         "abcdefghij",
			readBufferSize:  3,
			shouldError:     false,
			expectedContent: "abcdefghij",
		},
		{
			name:            "exceed limit on second read",
			limit:           5,
			content:         "abcdef",
			readBufferSize:  3,
			shouldError:     true,
			expectedContent: "abcdef", // Returns data read before limit check
		},
		{
			name:            "exactly at limit",
			limit:           10,
			content:         "abcdefghij",
			readBufferSize:  5,
			shouldError:     false,
			expectedContent: "abcdefghij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceReader := strings.NewReader(tt.content)
			lr := &limitedReader{
				ReadCloser: io.NopCloser(sourceReader),
				limit:      tt.limit,
			}

			buffer := make([]byte, tt.readBufferSize)
			var allReadData []byte
			var lastErr error

			// Read until EOF or error
			for {
				n, err := lr.Read(buffer)
				if n > 0 {
					allReadData = append(allReadData, buffer[:n]...)
				}
				if err != nil {
					lastErr = err
					break
				}
				// Safety check to prevent infinite loops
				if len(allReadData) > len(tt.content)+10 {
					t.Fatal("Read too much data, potential infinite loop")
				}
			}

			require.Equal(t, tt.expectedContent, string(allReadData))

			if tt.shouldError {
				require.Error(t, lastErr)
				require.ErrorIs(t, lastErr, wo.ErrStatusRequestEntityTooLarge)
			} else {
				require.Error(t, lastErr) // Should be EOF
				require.Equal(t, io.EOF, lastErr)
			}
		})
	}
}

func Test_limitedReader_Reread(t *testing.T) {
	// Test with a reader that supports Reread
	rereadableContent := "test content"
	rereadableReader := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(rereadableContent)),
	}

	lr := &limitedReader{
		ReadCloser: rereadableReader,
		limit:      int64(len(rereadableContent)),
	}

	// Read some data
	buffer := make([]byte, 4)
	n, err := lr.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 4, n)

	// Call Reread
	lr.Reread()

	// Should be able to read from the beginning again
	n, err = lr.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, "test", string(buffer[:n]))

	// Test with a reader that doesn't support Reread
	regularReader := io.NopCloser(strings.NewReader("regular"))
	lr2 := &limitedReader{
		ReadCloser: regularReader,
		limit:      7,
	}

	// This should not panic
	lr2.Reread()
}

func Test_limitedReader_Error_Propagation(t *testing.T) {
	// Test that underlying reader errors are properly propagated
	expectedErr := errors.New("underlying reader error")

	errorReader := &errorReadCloser{err: expectedErr}
	lr := &limitedReader{
		ReadCloser: errorReader,
		limit:      1024,
	}

	buffer := make([]byte, 10)
	_, err := lr.Read(buffer)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
}

func Test_BodyLimit_Integration(t *testing.T) {
	// Integration test that simulates a real HTTP request scenario
	cfg := BodyLimitConfig{Limit: 100}
	middleware := BodyLimit[*wo.Event](cfg)

	// Create a request with a body larger than the limit
	largeBody := strings.Repeat("data", 30)                  // 120 bytes, exceeds 100 byte limit
	e := newBodyLimitEvent(strings.NewReader(largeBody), -1) // Unknown content length

	// Apply middleware - should not error since content length is unknown
	err := middleware(e)
	require.NoError(t, err)

	// Simulate reading the body in a handler
	bodyBuffer := &bytes.Buffer{}
	_, err = io.Copy(bodyBuffer, e.Request().Body)

	// Should get an error because body exceeds limit during read
	require.Error(t, err)
	require.ErrorIs(t, err, wo.ErrStatusRequestEntityTooLarge)
}

func Test_BodyLimit_Zero_ContentLength(t *testing.T) {
	// Test behavior when ContentLength is 0 (unknown size)
	cfg := BodyLimitConfig{Limit: 10}
	middleware := BodyLimit[*wo.Event](cfg)

	// Create request with unknown content length (-1)
	body := strings.NewReader("some content")
	e := newBodyLimitEvent(body, -1)

	err := middleware(e)
	require.NoError(t, err)

	// The body reader should be replaced with limitedReader
	_, isLimitedReader := e.Request().Body.(*limitedReader)
	require.True(t, isLimitedReader)
}

// Helper types for testing

type errorReadCloser struct {
	err error
}

func (erc *errorReadCloser) Read(p []byte) (n int, error error) {
	return 0, erc.err
}

func (erc *errorReadCloser) Close() error {
	return erc.err
}

func Benchmark_BodyLimit_Middleware_Apply(b *testing.B) {
	cfg := BodyLimitConfig{Limit: 1024 * 1024} // 1MB
	middleware := BodyLimit[*wo.Event](cfg)

	body := strings.NewReader("small body")
	e := newBodyLimitEvent(body, int64(body.Len()))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = middleware(e)
	}
}

func Benchmark_limitedReader_Read_WithinLimit(b *testing.B) {
	content := strings.Repeat("x", 1024)
	source := io.NopCloser(strings.NewReader(content))

	lr := &limitedReader{
		ReadCloser: source,
		limit:      2048, // 2KB limit, content is within limit
	}

	buffer := make([]byte, 64) // Small buffer to require multiple reads

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lr.totalRead = 0 // Reset for each iteration
		_, _ = lr.Read(buffer)
	}
}
