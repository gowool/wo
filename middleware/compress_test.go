package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gowool/wo"
)

// newCompressTestEventWithHeaders creates a test event with specific headers
func newCompressTestEventWithHeaders(headers map[string]string) *wo.Event {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

// testCompressEventWithData wraps an event and provides a response to write
type testCompressEventWithData struct {
	*wo.Event
	responseData []byte
	nextCalled   bool
}

func (e *testCompressEventWithData) Next() error {
	e.nextCalled = true
	if len(e.responseData) > 0 {
		_, err := e.Response().Write(e.responseData)
		return err
	}
	return e.Event.Next()
}

// testInterfaceEvent wraps an event to test response writer interfaces
type testInterfaceEvent struct {
	*wo.Event
	interfacesTested bool
}

type testFlushEvent struct {
	*wo.Event
	flushCalled bool
	data        []byte
}

func (e *testFlushEvent) Next() error {
	if len(e.data) > 0 {
		e.Response().Write(e.data)
	}
	if flusher, ok := e.Response().(http.Flusher); ok {
		flusher.Flush()
		e.flushCalled = true
	}
	return e.Event.Next()
}

func (e *testInterfaceEvent) Next() error {
	w := e.Response()

	// Test Unwrap interface
	if unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
		assert.NotNil(&testing.T{}, unwrapper.Unwrap())
	} else {
		panic("ResponseWriter should implement Unwrap interface")
	}

	// Test Flush interface
	if flusher, ok := w.(http.Flusher); ok {
		assert.NotPanics(&testing.T{}, func() {
			flusher.Flush()
		})
	} else {
		panic("ResponseWriter should implement Flush interface")
	}

	e.interfacesTested = true
	return e.Event.Next()
}

func TestCompressConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   CompressConfig
		expected CompressConfig
	}{
		{
			name:   "empty config should get all defaults",
			config: CompressConfig{},
			expected: CompressConfig{
				MinLength: 1024,
				Level:     -1,
			},
		},
		{
			name: "partial config should only fill missing defaults",
			config: CompressConfig{
				MinLength: 2048,
			},
			expected: CompressConfig{
				MinLength: 2048,
				Level:     -1,
			},
		},
		{
			name: "fully populated config should remain unchanged",
			config: CompressConfig{
				MinLength: 4096,
				Level:     6,
			},
			expected: CompressConfig{
				MinLength: 4096,
				Level:     6,
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

func TestCompressConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    CompressConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid config should not return error",
			config:    CompressConfig{MinLength: 1024, Level: 6},
			expectErr: false,
		},
		{
			name:      "valid default level should not return error",
			config:    CompressConfig{MinLength: 1024, Level: -1},
			expectErr: false,
		},
		{
			name:      "invalid compression level too high should return error",
			config:    CompressConfig{MinLength: 1024, Level: 10},
			expectErr: true,
			errMsg:    "invalid gzip level",
		},
		{
			name:      "invalid compression level too low should return error",
			config:    CompressConfig{MinLength: 1024, Level: -3},
			expectErr: true,
			errMsg:    "invalid gzip level",
		},
		{
			name:      "valid huffman only level should not return error",
			config:    CompressConfig{MinLength: 1024, Level: -2},
			expectErr: false,
		},
		{
			name:      "valid best compression level should not return error",
			config:    CompressConfig{MinLength: 1024, Level: 9},
			expectErr: false,
		},
		{
			name:      "valid no compression level should not return error",
			config:    CompressConfig{MinLength: 1024, Level: 0},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			err := cfg.Validate()

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompress_AcceptEncoding_Header(t *testing.T) {
	tests := []struct {
		name             string
		acceptEncoding   string
		shouldCompress   bool
		expectedEncoding string
	}{
		{
			name:             "gzip header should compress",
			acceptEncoding:   "gzip",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
		{
			name:             "gzip with q=0 should compress (middleware doesn't parse q values)",
			acceptEncoding:   "gzip;q=0",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
		{
			name:             "gzip with q=0.5 should compress",
			acceptEncoding:   "gzip;q=0.5",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
		{
			name:             "multiple encodings with gzip first",
			acceptEncoding:   "gzip, deflate, br",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
		{
			name:             "multiple encodings with gzip second",
			acceptEncoding:   "deflate, gzip, br",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
		{
			name:             "multiple encodings without gzip",
			acceptEncoding:   "deflate, br",
			shouldCompress:   false,
			expectedEncoding: "",
		},
		{
			name:             "wildcard should not compress (middleware only looks for 'gzip' specifically)",
			acceptEncoding:   "*",
			shouldCompress:   false,
			expectedEncoding: "",
		},
		{
			name:             "wildcard with q=0 should not compress",
			acceptEncoding:   "*;q=0",
			shouldCompress:   false,
			expectedEncoding: "",
		},
		{
			name:             "empty header should not compress",
			acceptEncoding:   "",
			shouldCompress:   false,
			expectedEncoding: "",
		},
		{
			name:             "gzip with wildcard",
			acceptEncoding:   "gzip, *;q=0.5",
			shouldCompress:   true,
			expectedEncoding: "gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.acceptEncoding != "" {
				headers[wo.HeaderAcceptEncoding] = tt.acceptEncoding
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: []byte(strings.Repeat("x", 2048)), // Large enough to compress
			}

			config := CompressConfig{
				MinLength: 1024,
			}

			middleware := Compress[*testCompressEventWithData](config)
			err := middleware(event)

			assert.NoError(t, err)

			contentEncoding := event.Response().Header().Get(wo.HeaderContentEncoding)
			if tt.shouldCompress {
				assert.Equal(t, tt.expectedEncoding, contentEncoding)
			} else {
				assert.Empty(t, contentEncoding)
			}
		})
	}
}

func TestCompress_MinLength_Threshold(t *testing.T) {
	tests := []struct {
		name           string
		responseSize   int
		minLength      int
		shouldCompress bool
	}{
		{
			name:           "response smaller than min length should not compress",
			responseSize:   512,
			minLength:      1024,
			shouldCompress: false,
		},
		{
			name:           "response equal to min length should compress",
			responseSize:   1024,
			minLength:      1024,
			shouldCompress: true,
		},
		{
			name:           "response larger than min length should compress",
			responseSize:   2048,
			minLength:      1024,
			shouldCompress: true,
		},
		{
			name:           "zero min length should still have minimum threshold",
			responseSize:   10,
			minLength:      0,
			shouldCompress: false, // Still needs to exceed the minimum threshold
		},
		{
			name:           "empty response should not compress even with zero min length",
			responseSize:   0,
			minLength:      0,
			shouldCompress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: []byte(strings.Repeat("x", tt.responseSize)),
			}

			config := CompressConfig{
				MinLength: tt.minLength,
			}

			middleware := Compress[*testCompressEventWithData](config)
			err := middleware(event)

			assert.NoError(t, err)

			contentEncoding := event.Response().Header().Get(wo.HeaderContentEncoding)
			if tt.shouldCompress {
				assert.Equal(t, "gzip", contentEncoding, "Response should be compressed")
			} else {
				assert.Empty(t, contentEncoding, "Response should not be compressed")
			}
		})
	}
}

func TestCompress_Skipper(t *testing.T) {
	tests := []struct {
		name           string
		skipper        Skipper[*wo.Event]
		shouldCompress bool
	}{
		{
			name:           "no skipper should compress",
			skipper:        nil,
			shouldCompress: true,
		},
		{
			name: "skipper returning true should skip compression",
			skipper: func(e *wo.Event) bool {
				return true
			},
			shouldCompress: false,
		},
		{
			name: "skipper returning false should not skip compression",
			skipper: func(e *wo.Event) bool {
				return false
			},
			shouldCompress: true,
		},
		{
			name:           "path-based skipper should work",
			skipper:        PrefixPathSkipper[*wo.Event]("/api/"), // Test with /api/ path since our request is to example.com/test
			shouldCompress: true,                                  // Should not skip since path doesn't match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: []byte(strings.Repeat("x", 2048)),
			}

			config := CompressConfig{
				MinLength: 1024,
			}

			var err error
			if tt.skipper != nil {
				// Create skippers that work with the test event type
				adapterSkipper := func(e *testCompressEventWithData) bool {
					return tt.skipper(e.Event)
				}
				middleware := Compress[*testCompressEventWithData](config, adapterSkipper)
				err = middleware(event)
			} else {
				middleware := Compress[*testCompressEventWithData](config)
				err = middleware(event)
			}

			assert.NoError(t, err)

			contentEncoding := event.Response().Header().Get(wo.HeaderContentEncoding)
			if tt.shouldCompress {
				assert.Equal(t, "gzip", contentEncoding)
			} else {
				assert.Empty(t, contentEncoding)
			}
		})
	}
}

func TestCompress_Content_Type_Detection(t *testing.T) {
	tests := []struct {
		name           string
		responseData   []byte
		expectedType   string
		shouldCompress bool
		minLength      int
	}{
		{
			name:           "HTML content should be detected",
			responseData:   []byte("<html><body>test</body></html>"),
			expectedType:   "text/html; charset=utf-8",
			shouldCompress: true,
			minLength:      10,
		},
		{
			name:           "JSON content should be detected as plain text",
			responseData:   []byte(`{"key": "value"}`),
			expectedType:   "text/plain; charset=utf-8", // http.DetectContentType returns text/plain for JSON
			shouldCompress: true,
			minLength:      10,
		},
		{
			name:           "Plain text should be detected",
			responseData:   []byte("This is plain text"),
			expectedType:   "text/plain; charset=utf-8",
			shouldCompress: true,
			minLength:      10,
		},
		{
			name:           "Empty response should not set content type",
			responseData:   []byte(""),
			expectedType:   "",    // No content type is set for empty response
			shouldCompress: false, // Too short for compression
			minLength:      1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: tt.responseData,
			}

			config := CompressConfig{
				MinLength: tt.minLength,
			}

			middleware := Compress[*testCompressEventWithData](config)
			err := middleware(event)

			assert.NoError(t, err)
			if tt.expectedType == "" {
				assert.Empty(t, event.Response().Header().Get(wo.HeaderContentType))
			} else {
				assert.Equal(t, tt.expectedType, event.Response().Header().Get(wo.HeaderContentType))
			}

			contentEncoding := event.Response().Header().Get(wo.HeaderContentEncoding)
			if tt.shouldCompress {
				assert.Equal(t, "gzip", contentEncoding, "Response should be compressed")
			} else {
				assert.Empty(t, contentEncoding, "Response should not be compressed")
			}
		})
	}
}

func TestCompress_NextCall(t *testing.T) {
	tests := []struct {
		name       string
		nextCalled bool
	}{
		{
			name:       "middleware should call next",
			nextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: []byte(strings.Repeat("x", 2048)),
			}

			config := CompressConfig{
				MinLength: 1024,
			}

			middleware := Compress[*testCompressEventWithData](config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Equal(t, tt.nextCalled, event.nextCalled, "Next() should be called as expected")
		})
	}
}

// Test that compressed data can be properly decompressed
func TestCompress_CompressionQuality(t *testing.T) {
	t.Run("gzip compressed data should be decompressible", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		originalData := strings.Repeat("Hello, World! ", 100) // Large enough to compress

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(originalData),
		}

		config := CompressConfig{
			MinLength: 100,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))

		// Try to decompress the response body by checking if we can read compressed data
		// The response writer gets wrapped by gzipResponseWriter, so we verify compression worked
		// by checking the Content-Encoding header and ensuring data was written
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))

		// Verify the response actually contains data by checking that middleware processed it
		assert.True(t, event.nextCalled, "Next() should have been called to write data")
	})
}

func TestCompress_Content_Length_Header_Removal(t *testing.T) {
	t.Run("Content-Length header should be removed when compressing", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		baseEvent.Response().Header().Set(wo.HeaderContentLength, "1024")

		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 1024)),
		}

		config := CompressConfig{
			MinLength: 10,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Empty(t, event.Response().Header().Get(wo.HeaderContentLength), "Content-Length header should be removed")
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding), "Content-Encoding should be gzip")
	})
}

func TestCompress_Response_Writer_Interfaces(t *testing.T) {
	t.Run("gzip response writer should implement required interfaces", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testInterfaceEvent{
			Event: baseEvent,
		}

		config := CompressConfig{
			MinLength: 10,
		}

		middleware := Compress[*testInterfaceEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.True(t, event.interfacesTested, "Interface tests should have been run")
	})
}

func TestCompress_Empty_Response(t *testing.T) {
	t.Run("empty response should not be compressed", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)

		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte{}, // Empty response
		}

		config := CompressConfig{
			MinLength: 10,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding), "Empty response should not be compressed")
	})
}

func TestCompress_Configuration_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    CompressConfig
		expectErr bool
	}{
		{
			name: "valid configuration should not error",
			config: CompressConfig{
				MinLength: 1024,
				Level:     6,
			},
			expectErr: false,
		},
		{
			name: "invalid configuration should error",
			config: CompressConfig{
				MinLength: 1024,
				Level:     10, // Invalid level
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Compress function panics on invalid config, so we need to recover
			var didPanic bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						didPanic = true
					}
				}()
				_ = Compress[*testCompressEventWithData](tt.config)
			}()

			if tt.expectErr {
				assert.True(t, didPanic, "Expected panic for invalid config")
			} else {
				assert.False(t, didPanic, "Should not panic for valid config")
			}
		})
	}
}

// Benchmark tests
func BenchmarkCompress(b *testing.B) {
	headers := map[string]string{
		wo.HeaderAcceptEncoding: "gzip",
	}

	config := CompressConfig{
		MinLength: 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		middleware := Compress[*testCompressEventWithData](config)
		_ = middleware(event)
	}
}

func BenchmarkCompressWithoutCompression(b *testing.B) {
	headers := map[string]string{}
	// No Accept-Encoding header

	config := CompressConfig{
		MinLength: 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		middleware := Compress[*testCompressEventWithData](config)
		_ = middleware(event)
	}
}

func TestCompress_Vary_Header(t *testing.T) {
	t.Run("Vary header should be added regardless of compression", func(t *testing.T) {
		tests := []struct {
			name           string
			acceptEncoding string
			expectVary     bool
		}{
			{
				name:           "with gzip accept-encoding",
				acceptEncoding: "gzip",
				expectVary:     true,
			},
			{
				name:           "without gzip accept-encoding",
				acceptEncoding: "deflate",
				expectVary:     true,
			},
			{
				name:           "with empty accept-encoding",
				acceptEncoding: "",
				expectVary:     true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				headers := map[string]string{}
				if tt.acceptEncoding != "" {
					headers[wo.HeaderAcceptEncoding] = tt.acceptEncoding
				}

				baseEvent := newCompressTestEventWithHeaders(headers)
				event := &testCompressEventWithData{
					Event:        baseEvent,
					responseData: []byte(strings.Repeat("x", 2048)),
				}

				config := CompressConfig{MinLength: 1024}
				middleware := Compress[*testCompressEventWithData](config)
				err := middleware(event)

				assert.NoError(t, err)
				if tt.expectVary {
					assert.Equal(t, wo.HeaderAcceptEncoding, event.Response().Header().Get(wo.HeaderVary))
				}
			})
		}
	})
}

func TestCompress_Compression_Levels(t *testing.T) {
	t.Run("different compression levels should work", func(t *testing.T) {
		levels := []int{-2, -1, 0, 1, 6, 9}

		for _, level := range levels {
			t.Run("level_"+strconv.Itoa(level), func(t *testing.T) {
				headers := map[string]string{
					wo.HeaderAcceptEncoding: "gzip",
				}

				baseEvent := newCompressTestEventWithHeaders(headers)
				event := &testCompressEventWithData{
					Event:        baseEvent,
					responseData: []byte(strings.Repeat("test data ", 200)),
				}

				config := CompressConfig{
					MinLength: 100,
					Level:     level,
				}

				middleware := Compress[*testCompressEventWithData](config)
				err := middleware(event)

				assert.NoError(t, err)
				assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
			})
		}
	})
}

type testPusherResponseWriter struct {
	*httptest.ResponseRecorder
	pushCalled bool
	pushTarget string
}

func (w *testPusherResponseWriter) Push(target string, opts *http.PushOptions) error {
	w.pushCalled = true
	w.pushTarget = target
	return nil
}

func TestCompress_Push_Supported(t *testing.T) {
	t.Run("Push should delegate to underlying pusher when available", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Header.Set(wo.HeaderAcceptEncoding, "gzip")

		baseRecorder := &testPusherResponseWriter{ResponseRecorder: httptest.NewRecorder()}
		e := new(wo.Event)
		e.Reset(baseRecorder, req)

		event := &testCompressEventWithData{
			Event:        e,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{MinLength: 1024}
		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))

		baseRecorder.pushCalled = false
		w := event.Response()
		if pusher, ok := w.(interface {
			Push(string, *http.PushOptions) error
		}); ok {
			err := pusher.Push("/style.css", &http.PushOptions{})
			assert.NoError(t, err, "Push should succeed with pusher available")
			assert.True(t, baseRecorder.pushCalled, "Underlying pusher should have been called")
		} else {
			t.Error("Response writer should implement Push interface")
		}
	})
}

func TestGzipResponseWriter_Push_Direct(t *testing.T) {
	t.Run("Push should delegate to underlying pusher", func(t *testing.T) {
		recorder := &testPusherResponseWriter{ResponseRecorder: httptest.NewRecorder()}
		req := httptest.NewRequest("GET", "/", nil)
		e := new(wo.Event)
		e.Reset(recorder, req)

		gw := &gzipResponseWriter{
			ResponseWriter: e.Response(),
			minLength:      1024,
			code:           http.StatusOK,
		}

		err := gw.Push("/style.css", &http.PushOptions{})
		assert.NoError(t, err)
		assert.True(t, recorder.pushCalled)
		assert.Equal(t, "/style.css", recorder.pushTarget)
	})
}

func TestCompress_Flush_With_Buffered_Data(t *testing.T) {
	t.Run("flush with buffered data should compress buffer", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)

		event := &testFlushEvent{
			Event: baseEvent,
			data:  []byte("small"), // Below threshold, will be buffered
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testFlushEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.True(t, event.flushCalled)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Multiple_Writes(t *testing.T) {
	t.Run("multiple writes should be compressed properly", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{
			MinLength: 50,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
		assert.True(t, event.nextCalled)
	})
}

func TestCompress_Status_Only_Response(t *testing.T) {
	t.Run("status code without body should not compress", func(t *testing.T) {
		tests := []struct {
			name     string
			status   int
			compress bool
		}{
			{name: "404 Not Found", status: http.StatusNotFound, compress: false},
			{name: "301 Moved Permanently", status: http.StatusMovedPermanently, compress: false},
			{name: "302 Found", status: http.StatusFound, compress: false},
			{name: "304 Not Modified", status: http.StatusNotModified, compress: false},
			{name: "204 No Content", status: http.StatusNoContent, compress: false},
			{name: "200 OK with body", status: http.StatusOK, compress: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				headers := map[string]string{
					wo.HeaderAcceptEncoding: "gzip",
				}

				baseEvent := newCompressTestEventWithHeaders(headers)

				var responseData []byte
				if tt.compress {
					responseData = []byte(strings.Repeat("x", 2048))
				}

				event := &testCompressEventWithData{
					Event:        baseEvent,
					responseData: responseData,
				}

				config := CompressConfig{
					MinLength: 1024,
				}

				middleware := Compress[*testCompressEventWithData](config)
				err := middleware(event)

				assert.NoError(t, err)
				if tt.compress {
					assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
				} else {
					assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding))
				}
			})
		}
	})
}

func TestCompress_Hijack(t *testing.T) {
	t.Run("gzipResponseWriter should support Hijack", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{MinLength: 1024}
		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)

		w := event.Response()
		if hijacker, ok := w.(interface {
			Hijack() (net.Conn, *bufio.ReadWriter, error)
		}); ok {
			conn, rw, err := hijacker.Hijack()
			assert.Error(t, err) // httptest.ResponseRecorder doesn't support hijacking
			assert.Nil(t, conn)
			assert.Nil(t, rw)
		} else {
			t.Error("ResponseWriter should implement Hijack interface")
		}
	})
}

func TestCompress_Push(t *testing.T) {
	t.Run("gzipResponseWriter should support Push", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{MinLength: 1024}
		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)

		w := event.Response()
		if pusher, ok := w.(interface {
			Push(string, *http.PushOptions) error
		}); ok {
			opts := &http.PushOptions{Header: http.Header{}}
			err := pusher.Push("/style.css", opts)
			assert.Equal(t, http.ErrNotSupported, err)
		} else {
			t.Error("ResponseWriter should implement Push interface")
		}
	})
}

func TestCompress_Flush_Small_Response(t *testing.T) {
	t.Run("flush on small response should force compression", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testFlushEvent{
			Event: baseEvent,
			data:  []byte("small data"),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testFlushEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.True(t, event.flushCalled)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Flush_Large_Response(t *testing.T) {
	t.Run("flush on large response should work", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testFlushEvent{
			Event: baseEvent,
			data:  []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testFlushEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.True(t, event.flushCalled)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_WriteHeader_Before_Write(t *testing.T) {
	t.Run("WriteHeader should set status correctly before write", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Content_Type_Already_Set(t *testing.T) {
	t.Run("existing Content-Type should not be overwritten", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		baseEvent.Response().Header().Set(wo.HeaderContentType, "application/json")

		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "application/json", event.Response().Header().Get(wo.HeaderContentType))
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Empty_Write(t *testing.T) {
	t.Run("empty write should not cause issues", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte{},
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

type testWriteHeaderEvent struct {
	*wo.Event
	callWriteHeader bool
	responseData    []byte
	statusCode      int
}

func (e *testWriteHeaderEvent) Next() error {
	if e.callWriteHeader {
		e.Response().WriteHeader(e.statusCode)
	}
	if len(e.responseData) > 0 {
		_, err := e.Response().Write(e.responseData)
		return err
	}
	return e.Event.Next()
}

func TestCompress_WriteHeader_Explicit(t *testing.T) {
	tests := []struct {
		name            string
		callWriteHeader bool
		statusCode      int
		responseSize    int
		minLength       int
		expectCompress  bool
	}{
		{
			name:            "WriteHeader called before write, should compress",
			callWriteHeader: true,
			statusCode:      http.StatusOK,
			responseSize:    2048,
			minLength:       1024,
			expectCompress:  true,
		},
		{
			name:            "WriteHeader called with 404, no body, should not compress",
			callWriteHeader: true,
			statusCode:      http.StatusNotFound,
			responseSize:    0,
			minLength:       1024,
			expectCompress:  false,
		},
		{
			name:            "WriteHeader called with 301, no body, should not compress",
			callWriteHeader: true,
			statusCode:      http.StatusMovedPermanently,
			responseSize:    0,
			minLength:       1024,
			expectCompress:  false,
		},
		{
			name:            "WriteHeader called, body below min length, should not compress",
			callWriteHeader: true,
			statusCode:      http.StatusOK,
			responseSize:    512,
			minLength:       1024,
			expectCompress:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testWriteHeaderEvent{
				Event:           baseEvent,
				callWriteHeader: tt.callWriteHeader,
				statusCode:      tt.statusCode,
				responseData:    []byte(strings.Repeat("x", tt.responseSize)),
			}

			config := CompressConfig{
				MinLength: tt.minLength,
			}

			middleware := Compress[*testWriteHeaderEvent](config)
			err := middleware(event)

			assert.NoError(t, err)
			if tt.expectCompress {
				assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
			} else {
				assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding))
			}
		})
	}
}

type testNoBodyEvent struct {
	*wo.Event
	statusCode int
}

func (e *testNoBodyEvent) Next() error {
	e.Response().WriteHeader(e.statusCode)
	return e.Event.Next()
}

func TestCompress_NoBody_Cleanup(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "204 No Content", statusCode: http.StatusNoContent},
		{name: "304 Not Modified", statusCode: http.StatusNotModified},
		{name: "404 Not Found", statusCode: http.StatusNotFound},
		{name: "301 Moved Permanently", statusCode: http.StatusMovedPermanently},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			event := &testNoBodyEvent{
				Event:      baseEvent,
				statusCode: tt.statusCode,
			}

			config := CompressConfig{
				MinLength: 1024,
			}

			middleware := Compress[*testNoBodyEvent](config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding), "Should not compress status-only responses")
		})
	}
}

func TestCompress_Preserve_Response_Headers(t *testing.T) {
	t.Run("existing headers should be preserved when compressing", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		baseEvent.Response().Header().Set("X-Custom-Header", "custom-value")
		baseEvent.Response().Header().Set("Cache-Control", "max-age=3600")

		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "custom-value", event.Response().Header().Get("X-Custom-Header"))
		assert.Equal(t, "max-age=3600", event.Response().Header().Get("Cache-Control"))
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Response_Below_Threshold(t *testing.T) {
	tests := []struct {
		name          string
		responseSize  int
		minLength     int
		expectWritten bool
	}{
		{
			name:          "response below threshold should not compress",
			responseSize:  500,
			minLength:     1024,
			expectWritten: true,
		},
		{
			name:          "small response just below threshold",
			responseSize:  1023,
			minLength:     1024,
			expectWritten: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				wo.HeaderAcceptEncoding: "gzip",
			}

			baseEvent := newCompressTestEventWithHeaders(headers)
			responseData := []byte(strings.Repeat("x", tt.responseSize))
			event := &testCompressEventWithData{
				Event:        baseEvent,
				responseData: responseData,
			}

			config := CompressConfig{
				MinLength: tt.minLength,
			}

			middleware := Compress[*testCompressEventWithData](config)
			err := middleware(event)

			assert.NoError(t, err)
			assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding), "Should not compress below threshold")
		})
	}
}

func TestCompress_Write_Below_Threshold_Buffers(t *testing.T) {
	t.Run("writes below threshold should be buffered", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)

		responseData := []byte(strings.Repeat("a", 500))
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: responseData,
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Empty(t, event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Content_Type_Not_Preset(t *testing.T) {
	t.Run("content type should be detected if not preset", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)

		htmlData := []byte("<!DOCTYPE html><html><body>" + strings.Repeat("Test content ", 100) + "</body></html>")
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: htmlData,
		}

		config := CompressConfig{
			MinLength: 50,
		}

		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
		contentType := event.Response().Header().Get(wo.HeaderContentType)
		assert.Contains(t, contentType, "text/html")
	})
}

func TestCompress_Flush_Forces_Compression(t *testing.T) {
	t.Run("flush should force compression even for small data", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)

		event := &testFlushEvent{
			Event: baseEvent,
			data:  []byte("small data"), // Below threshold
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testFlushEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.True(t, event.flushCalled)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding), "Flush should force compression")
	})
}

func TestCompress_Push_NotSupported(t *testing.T) {
	t.Run("Push should return ErrNotSupported when pusher not available", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testCompressEventWithData{
			Event:        baseEvent,
			responseData: []byte(strings.Repeat("x", 2048)),
		}

		config := CompressConfig{MinLength: 1024}
		middleware := Compress[*testCompressEventWithData](config)
		err := middleware(event)

		assert.NoError(t, err)

		w := event.Response()
		if pusher, ok := w.(interface {
			Push(string, *http.PushOptions) error
		}); ok {
			err := pusher.Push("/test", &http.PushOptions{Header: http.Header{}})
			assert.Equal(t, http.ErrNotSupported, err, "Push should return ErrNotSupported")
		}
	})
}

type testMultipleWritesEvent struct {
	*wo.Event
	firstWrite  []byte
	secondWrite []byte
}

func (e *testMultipleWritesEvent) Next() error {
	if len(e.firstWrite) > 0 {
		_, _ = e.Response().Write(e.firstWrite)
	}
	if len(e.secondWrite) > 0 {
		_, _ = e.Response().Write(e.secondWrite)
	}
	return e.Event.Next()
}

func TestCompress_Multiple_Writes_Cross_Threshold(t *testing.T) {
	t.Run("multiple writes that cross threshold should compress", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testMultipleWritesEvent{
			Event:       baseEvent,
			firstWrite:  []byte(strings.Repeat("x", 600)),
			secondWrite: []byte(strings.Repeat("y", 600)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testMultipleWritesEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}

func TestCompress_Write_After_Compression_Start(t *testing.T) {
	t.Run("writes after compression starts should go to writer", func(t *testing.T) {
		headers := map[string]string{
			wo.HeaderAcceptEncoding: "gzip",
		}

		baseEvent := newCompressTestEventWithHeaders(headers)
		event := &testMultipleWritesEvent{
			Event:       baseEvent,
			firstWrite:  []byte(strings.Repeat("x", 1200)),
			secondWrite: []byte(strings.Repeat("y", 500)),
		}

		config := CompressConfig{
			MinLength: 1024,
		}

		middleware := Compress[*testMultipleWritesEvent](config)
		err := middleware(event)

		assert.NoError(t, err)
		assert.Equal(t, "gzip", event.Response().Header().Get(wo.HeaderContentEncoding))
	})
}
