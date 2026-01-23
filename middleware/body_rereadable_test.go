package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

func newBodyRereadableEvent(body io.Reader) *wo.Event {
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(rec, req)

	return e
}

func Test_BodyRereadable_Skipper(t *testing.T) {
	content := "test content that should be rereadable"

	// Create a skipper that always skips
	skipper := func(e *wo.Event) bool {
		return true
	}

	middleware := BodyRereadable[*wo.Event](skipper)
	e := newBodyRereadableEvent(strings.NewReader(content))

	err := middleware(e)
	require.NoError(t, err)

	// The body should remain unchanged since middleware was skipped
	result, err := io.ReadAll(e.Request().Body)
	require.NoError(t, err)
	require.Equal(t, content, string(result))
}

func Test_BodyRereadable_BasicFunctionality(t *testing.T) {
	content := "test content for rereading"
	middleware := BodyRereadable[*wo.Event]()
	e := newBodyRereadableEvent(strings.NewReader(content))

	// Simulate handler that would run after middleware
	handler := func(e *wo.Event) error {
		// During handler execution, the body should be rereadable
		body := e.Request().Body

		// The body should be wrapped with rereadableReadCloser
		_, isRereadable := body.(*rereadableReadCloser)
		require.True(t, isRereadable)

		// Should be able to read the body
		result, err := io.ReadAll(body)
		require.NoError(t, err)
		require.Equal(t, content, string(result))

		// Should be able to read again
		result2, err := io.ReadAll(body)
		require.NoError(t, err)
		require.Equal(t, content, string(result2))

		return nil
	}

	// Apply middleware first
	err := middleware(e)
	require.NoError(t, err)

	// The middleware cleans up after itself, so we need to test during execution
	// Let's create a test that simulates the middleware behavior without defer cleanup
	middlewareNoCleanup := func(e *wo.Event) error {
		skip := ChainSkipper[*wo.Event]()
		if skip(e) {
			return e.Next()
		}

		// wrap the request body to allow multiple reads
		read := &rereadableReadCloser{}
		read.Reset(e.Request().Body)
		e.Request().Body = read

		// Call handler
		return handler(e)
	}

	e2 := newBodyRereadableEvent(strings.NewReader(content))
	err = middlewareNoCleanup(e2)
	require.NoError(t, err)
}

func Test_BodyRereadable_MultipleReads(t *testing.T) {
	content := "test content for multiple reads"

	// Test the rereadable functionality directly
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Read multiple times
	for i := 0; i < 3; i++ {
		result, err := io.ReadAll(rereadable)
		require.NoError(t, err, "Read %d should succeed", i)
		require.Equal(t, content, string(result), "Read %d should return original content", i)
	}
}

func Test_BodyRereadable_PartialReads(t *testing.T) {
	content := "abcdefghij"

	// Test the rereadable functionality directly
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	buffer := make([]byte, 3)

	// First partial read
	n, err := rereadable.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "abc", string(buffer[:n]))

	// Second partial read
	n, err = rereadable.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "def", string(buffer[:n]))

	// Third partial read
	n, err = rereadable.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "ghi", string(buffer[:n]))

	// Final partial read
	n, err = rereadable.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "j", string(buffer[:n]))

	// Should return EOF
	n, err = rereadable.Read(buffer)
	require.Equal(t, 0, n)
	require.Equal(t, io.EOF, err)

	// Now read again - should get the full content
	fullResult, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(fullResult))
}

func Test_RereadableReadCloser(t *testing.T) {
	content := "test"

	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// read multiple times
	for i := 0; i < 3; i++ {
		result, err := io.ReadAll(rereadable)
		if err != nil {
			t.Fatalf("[read:%d] %v", i, err)
		}
		if str := string(result); str != content {
			t.Fatalf("[read:%d] Expected %q, got %q", i, content, result)
		}
	}
}

func Test_RereadableReadCloser_Read_ZeroBuffer(t *testing.T) {
	content := "test content"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Reading with zero-length buffer should not advance the reader
	buffer := make([]byte, 0)
	n, err := rereadable.Read(buffer)
	require.Equal(t, 0, n)
	require.NoError(t, err)

	// Should still be able to read the full content
	result, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result))
}

func Test_RereadableReadCloser_Read_EmptySource(t *testing.T) {
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("")),
	}

	buffer := make([]byte, 10)
	n, err := rereadable.Read(buffer)
	require.Equal(t, 0, n)
	require.Equal(t, io.EOF, err)

	// Should be able to "read" again and get EOF
	n, err = rereadable.Read(buffer)
	require.Equal(t, 0, n)
	require.Equal(t, io.EOF, err)
}

func Test_RereadableReadCloser_Reread_Manual(t *testing.T) {
	content := "test content"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Read partial content
	buffer := make([]byte, 4)
	n, err := rereadable.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, "test", string(buffer[:n]))

	// Read the rest to trigger EOF and populate the copy fully
	rest, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, " content", string(rest))

	// Manually call Reread - this should work now that we have full content buffered
	rereadable.Reread()

	// Should be able to read from the beginning again
	result, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result))
}

func Test_RereadableReadCloser_Reread_BeforeAnyRead(t *testing.T) {
	content := "test content"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Call Reread before any reads - should not panic
	rereadable.Reread()

	// Should be able to read normally
	result, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result))
}

func Test_RereadableReadCloser_Reread_AfterCompleteRead(t *testing.T) {
	content := "test content"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Read everything
	result, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result))

	// Reread should work even after complete read (Read method automatically calls it)
	result2, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result2))
}

func Test_RereadableReadCloser_Reset(t *testing.T) {
	content1 := "first content"
	content2 := "second content"

	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content1)),
	}

	// Read first content
	result1, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content1, string(result1))

	// Reset with new content
	rereadable.Reset(io.NopCloser(strings.NewReader(content2)))

	// Should read the new content
	result2, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content2, string(result2))

	// Should be able to read the new content again
	result3, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content2, string(result3))
}

func Test_RereadableReadCloser_Reset_WithNil(t *testing.T) {
	content := "test content"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Read some content
	_, err := io.ReadAll(rereadable)
	require.NoError(t, err)

	// Reset with nil
	rereadable.Reset(nil)

	// Should handle gracefully
	require.Nil(t, rereadable.ReadCloser)
	require.Nil(t, rereadable.copy)
	require.Nil(t, rereadable.active)
}

func Test_RereadableReadCloser_Error_Propagation(t *testing.T) {
	expectedErr := errors.New("underlying reader error")

	errorReader := &rereadableErrorReadCloser{err: expectedErr}
	rereadable := &rereadableReadCloser{
		ReadCloser: errorReader,
	}

	buffer := make([]byte, 10)
	_, err := rereadable.Read(buffer)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
}

func Test_RereadableReadCloser_LargeContent(t *testing.T) {
	// Test with content larger than typical buffer sizes
	content := strings.Repeat("x", 10000) // 10KB
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// Read in small chunks to test buffering behavior
	buffer := make([]byte, 256)
	var allRead []byte

	for {
		n, err := rereadable.Read(buffer)
		if n > 0 {
			allRead = append(allRead, buffer[:n]...)
		}
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	require.Equal(t, content, string(allRead))

	// Should be able to read the full content again
	result, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(result))
}

func Test_BodyRereadable_Integration(t *testing.T) {
	// Integration test that simulates a real HTTP request scenario
	// This tests the core rereadable functionality that the middleware enables

	content := "request body content that needs to be read multiple times"
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	// First consumer reads the body
	firstRead, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(firstRead))

	// Second consumer should also be able to read the body
	secondRead, err := io.ReadAll(rereadable)
	require.NoError(t, err)
	require.Equal(t, content, string(secondRead))
}

func Test_BodyRereadable_EmptyBody(t *testing.T) {
	middleware := BodyRereadable[*wo.Event]()
	e := newBodyRereadableEvent(strings.NewReader(""))

	err := middleware(e)
	require.NoError(t, err)

	// Should handle empty body gracefully
	result, err := io.ReadAll(e.Request().Body)
	require.NoError(t, err)
	require.Equal(t, "", string(result))

	// Should be able to read again
	result2, err := io.ReadAll(e.Request().Body)
	require.NoError(t, err)
	require.Equal(t, "", string(result2))
}

func Test_BodyRereadable_NilBody(t *testing.T) {
	middleware := BodyRereadable[*wo.Event]()
	e := newBodyRereadableEvent(nil)

	err := middleware(e)
	require.NoError(t, err)

	// Should handle nil body gracefully
	require.NotNil(t, e.Request().Body)

	// Reading should work (though with nil body, it's essentially EOF)
	result, err := io.ReadAll(e.Request().Body)
	require.NoError(t, err)
	require.Equal(t, "", string(result))
}

// Helper types for testing

type rereadableErrorReadCloser struct {
	err error
}

func (erc *rereadableErrorReadCloser) Read(p []byte) (n int, error error) {
	return 0, erc.err
}

func (erc *rereadableErrorReadCloser) Close() error {
	return erc.err
}

// Benchmark tests

func Benchmark_BodyRereadable_Middleware_Apply(b *testing.B) {
	middleware := BodyRereadable[*wo.Event]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := newBodyRereadableEvent(strings.NewReader("test body content"))
		_ = middleware(e)
	}
}

func Benchmark_RereadableReadCloser_FirstRead(b *testing.B) {
	content := strings.Repeat("x", 1024)
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	buffer := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rereadable.Reset(io.NopCloser(strings.NewReader(content)))
		_, _ = rereadable.Read(buffer)
	}
}

func Benchmark_RereadableReadCloser_SecondRead(b *testing.B) {
	content := strings.Repeat("x", 1024)
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	buffer := make([]byte, 1024)
	// First read to populate the buffer
	_, _ = io.ReadAll(rereadable)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rereadable.Read(buffer)
		rereadable.Reread()
	}
}

func Benchmark_RereadableReadCloser_ReadAll(b *testing.B) {
	content := strings.Repeat("x", 4096)
	rereadable := &rereadableReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader(content)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rereadable.Reset(io.NopCloser(strings.NewReader(content)))
		_, _ = io.ReadAll(rereadable)
	}
}
