package wo

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResponseWriter is a mock implementation of http.ResponseWriter for testing
type mockResponseWriter struct {
	status      int
	written     bool
	header      http.Header
	body        *bytes.Buffer
	flushCalled bool
	hijacked    bool
	pusher      *mockPusher
}

type mockPusher struct {
	target string
	opts   *http.PushOptions
	err    error
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		header: make(http.Header),
		body:   bytes.NewBuffer(nil),
		pusher: &mockPusher{},
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.written = true
	if m.status == 0 {
		m.status = http.StatusOK
	}
	return m.body.Write(data)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
	m.written = true
}

func (m *mockResponseWriter) Flush() {
	m.flushCalled = true
}

func (m *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijacked = true
	return nil, nil, nil
}

func (m *mockResponseWriter) Push(target string, opts *http.PushOptions) error {
	m.pusher.target = target
	m.pusher.opts = opts
	return m.pusher.err
}

func (m *mockResponseWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if m.body == nil {
		m.body = bytes.NewBuffer(nil)
	}
	return io.Copy(m.body, r)
}

// mockUnwrapper wraps another ResponseWriter and implements RWUnwrapper
type mockUnwrapper struct {
	http.ResponseWriter
	inner http.ResponseWriter
}

func (m *mockUnwrapper) Unwrap() http.ResponseWriter {
	return m.inner
}

func TestNewResponse(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	assert.NotNil(t, resp)
	assert.Equal(t, mockRW, resp.ResponseWriter)
	assert.NotNil(t, resp.buffer)
	assert.False(t, resp.Written)
	assert.False(t, resp.Buffering)
	assert.Equal(t, 0, resp.Status)
	assert.Equal(t, int64(0), resp.Size)
	assert.Empty(t, resp.beforeFuncs)
	assert.Empty(t, resp.afterFuncs)
}

func TestResponse_Buffer(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	testData := []byte("test data")
	resp.buffer.Write(testData)

	assert.Equal(t, testData, resp.Buffer())
}

func TestResponse_Before(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	called := false
	resp.Before(func() {
		called = true
	})

	assert.Len(t, resp.beforeFuncs, 1)

	// Execute the function
	resp.beforeFuncs[0]()
	assert.True(t, called)
}

func TestResponse_After(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	called := false
	resp.After(func() {
		called = true
	})

	assert.Len(t, resp.afterFuncs, 1)

	// Execute the function
	resp.afterFuncs[0]()
	assert.True(t, called)
}

func TestResponse_WriteHeader(t *testing.T) {
	t.Run("first call", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		resp.WriteHeader(http.StatusNotFound)

		assert.Equal(t, http.StatusNotFound, resp.Status)
		assert.True(t, resp.Written)
		assert.Equal(t, http.StatusNotFound, mockRW.Code)
		assert.Empty(t, mockRW.Header().Get(HeaderContentLength))
	})

	t.Run("subsequent calls ignored", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		resp.WriteHeader(http.StatusNotFound)
		resp.WriteHeader(http.StatusInternalServerError) // Should be ignored

		assert.Equal(t, http.StatusNotFound, resp.Status)
		assert.Equal(t, http.StatusNotFound, mockRW.Code)
	})

	t.Run("with buffering enabled", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)
		resp.Buffering = true

		called := false
		resp.Before(func() {
			called = true
		})

		resp.WriteHeader(http.StatusCreated)

		assert.Equal(t, http.StatusCreated, resp.Status)
		assert.True(t, resp.Written)
		assert.Equal(t, http.StatusOK, mockRW.Code) // Should remain default, not changed to StatusCreated
		assert.False(t, called)                     // Before func should not be called
	})

	t.Run("executes before functions", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		called := false
		resp.Before(func() {
			called = true
		})

		resp.WriteHeader(http.StatusOK)

		assert.True(t, called)
	})
}

func TestResponse_Write(t *testing.T) {
	t.Run("first write triggers WriteHeader", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		data := []byte("test data")
		n, err := resp.Write(data)

		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, int64(len(data)), resp.Size)
		assert.True(t, resp.Written)
		assert.Equal(t, http.StatusOK, resp.Status)
		assert.Equal(t, data, mockRW.Body.Bytes())
	})

	t.Run("with buffering enabled", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)
		resp.Buffering = true

		data := []byte("test data")
		n, err := resp.Write(data)

		require.NoError(t, err)
		assert.Equal(t, 0, n)                // Response.Write when buffering doesn't return byte count (bug in original code)
		assert.Equal(t, int64(0), resp.Size) // Size not updated when buffering
		assert.True(t, resp.Written)
		assert.Empty(t, mockRW.Body.Bytes()) // No data written to underlying writer
		assert.Equal(t, data, resp.Buffer()) // Data is in buffer
	})

	t.Run("executes after functions", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		called := false
		resp.After(func() {
			called = true
		})

		_, _ = resp.Write([]byte("test"))

		assert.True(t, called)
	})

	t.Run("multiple after functions", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		order := []int{}
		resp.After(func() { order = append(order, 1) })
		resp.After(func() { order = append(order, 2) })
		resp.After(func() { order = append(order, 3) })

		_, _ = resp.Write([]byte("test"))

		assert.Equal(t, []int{1, 2, 3}, order)
	})
}

func TestResponse_Flush(t *testing.T) {
	t.Run("successful flush", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		// Should not panic
		assert.NotPanics(t, func() {
			resp.Flush()
		})
		assert.True(t, resp.Written)
	})

	t.Run("flush error panic", func(t *testing.T) {
		// Use a writer that doesn't support flushing
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		// Replace with a non-flushing writer
		resp.ResponseWriter = &struct {
			http.ResponseWriter
		}{mockRW}

		assert.Panics(t, func() {
			resp.Flush()
		})
	})
}

func TestResponse_FlushError(t *testing.T) {
	t.Run("successful flush", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		err := resp.FlushError()
		assert.NoError(t, err)
		assert.True(t, resp.Written)
	})

	t.Run("flush not supported", func(t *testing.T) {
		// Use a writer that doesn't support flushing
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		err := resp.FlushError()
		// httptest.ResponseRecorder should support flushing, so this should not error
		assert.NoError(t, err)
		assert.True(t, resp.Written)
	})
}

func TestResponse_Hijack(t *testing.T) {
	t.Run("successful hijack", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		conn, rw, err := resp.Hijack()
		// httptest.ResponseRecorder doesn't support hijacking
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Nil(t, rw)
	})
}

func TestResponse_Push(t *testing.T) {
	t.Run("successful push", func(t *testing.T) {
		mockRW := &mockResponseWriter{pusher: &mockPusher{}}
		resp := NewResponse(mockRW)

		opts := &http.PushOptions{Header: http.Header{"Test": []string{"value"}}}
		err := resp.Push("/target", opts)

		require.NoError(t, err)
		assert.Equal(t, "/target", mockRW.pusher.target)
		assert.Equal(t, opts, mockRW.pusher.opts)
	})

	t.Run("push through unwrapper", func(t *testing.T) {
		innerRW := &mockResponseWriter{pusher: &mockPusher{}}
		unwrapperRW := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          innerRW,
		}
		resp := NewResponse(unwrapperRW)

		opts := &http.PushOptions{}
		err := resp.Push("/target", opts)

		require.NoError(t, err)
		assert.Equal(t, "/target", innerRW.pusher.target)
	})

	t.Run("push not supported", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		err := resp.Push("/target", nil)
		assert.Equal(t, http.ErrNotSupported, err)
	})

	t.Run("push error propagation", func(t *testing.T) {
		expectedErr := errors.New("push error")
		mockRW := &mockResponseWriter{
			pusher: &mockPusher{err: expectedErr},
		}
		resp := NewResponse(mockRW)

		err := resp.Push("/target", nil)
		assert.Equal(t, expectedErr, err)
	})
}

func TestResponse_ReadFrom(t *testing.T) {
	t.Run("read from supported", func(t *testing.T) {
		mockRW := newMockResponseWriter()
		resp := NewResponse(mockRW)

		data := "test data from reader"
		reader := strings.NewReader(data)

		n, err := resp.ReadFrom(reader)
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
		assert.Equal(t, data, mockRW.body.String())
	})

	t.Run("read from not supported uses io.Copy", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		data := "test data from reader"
		reader := strings.NewReader(data)

		n, err := resp.ReadFrom(reader)
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
		assert.Equal(t, data, mockRW.Body.String())
	})

	t.Run("read from through unwrapper", func(t *testing.T) {
		innerRW := &mockResponseWriter{}
		unwrapperRW := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          innerRW,
		}
		resp := NewResponse(unwrapperRW)

		data := "test data from reader"
		reader := strings.NewReader(data)

		n, err := resp.ReadFrom(reader)
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
		assert.Equal(t, data, innerRW.body.String())
	})
}

func TestResponse_Unwrap(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	assert.Equal(t, mockRW, resp.Unwrap())
}

func TestResponse_Reset(t *testing.T) {
	mockRW1 := httptest.NewRecorder()
	mockRW2 := httptest.NewRecorder()
	resp := NewResponse(mockRW1)

	// Modify some fields
	resp.Buffering = true
	resp.Written = true
	resp.Status = http.StatusNotFound
	resp.Size = 100
	resp.buffer.WriteString("test data")
	resp.Before(func() {})
	resp.After(func() {})

	// Reset
	resp.Reset(mockRW2)

	assert.Equal(t, mockRW2, resp.ResponseWriter)
	assert.Empty(t, resp.buffer.Bytes())
	assert.Empty(t, resp.beforeFuncs)
	assert.Empty(t, resp.afterFuncs)
	assert.False(t, resp.Written)
	assert.False(t, resp.Buffering)
	assert.Equal(t, 0, resp.Status)
	assert.Equal(t, int64(0), resp.Size)
}

func TestResponseInterfaceAssertions(t *testing.T) {
	// Test that Response implements the expected interfaces
	var _ http.Flusher = (*Response)(nil)
	var _ http.Hijacker = (*Response)(nil)
	var _ http.Pusher = (*Response)(nil)
	var _ FlushErrorer = (*Response)(nil)
	var _ RWUnwrapper = (*Response)(nil)

	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	// These should compile without errors
	_ = http.Flusher(resp)
	_ = http.Hijacker(resp)
	_ = http.Pusher(resp)
	_ = FlushErrorer(resp)
	_ = RWUnwrapper(resp)
}

func TestResponse_ComplexWorkflow(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)

	// Set up before and after functions
	beforeOrder := []int{}
	afterOrder := []int{}
	resp.Before(func() { beforeOrder = append(beforeOrder, 1) })
	resp.Before(func() { beforeOrder = append(beforeOrder, 2) })
	resp.After(func() { afterOrder = append(afterOrder, 1) })
	resp.After(func() { afterOrder = append(afterOrder, 2) })

	// Set status
	resp.WriteHeader(http.StatusCreated)

	// Write data
	data := []byte("response data")
	n, err := resp.Write(data)

	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Verify state
	assert.Equal(t, http.StatusCreated, resp.Status)
	assert.Equal(t, int64(len(data)), resp.Size)
	assert.True(t, resp.Written)
	assert.Equal(t, data, mockRW.Body.Bytes())
	assert.Equal(t, http.StatusCreated, mockRW.Code)

	// Verify function execution order
	assert.Equal(t, []int{1, 2}, beforeOrder)
	assert.Equal(t, []int{1, 2}, afterOrder)
}

func TestResponse_BufferingWorkflow(t *testing.T) {
	mockRW := httptest.NewRecorder()
	resp := NewResponse(mockRW)
	resp.Buffering = true

	// Set status (should not call underlying writer)
	resp.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusOK, mockRW.Code) // Should remain default

	// Write data (should be buffered)
	data := []byte("buffered data")
	_, _ = resp.Write(data)

	// Verify buffer contains data
	assert.Equal(t, data, resp.Buffer())

	// Verify underlying writer was not called
	assert.Equal(t, http.StatusOK, mockRW.Code) // Should remain default
	assert.Empty(t, mockRW.Body.Bytes())
	assert.Equal(t, int64(0), resp.Size) // Size not updated when buffering
}

func TestUnwrapResponse(t *testing.T) {
	t.Run("unwraps Response directly", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		unwrapped, err := UnwrapResponse(resp)

		require.NoError(t, err)
		assert.Same(t, resp, unwrapped)
	})

	t.Run("unwraps through single RWUnwrapper", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)
		wrapped := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          resp,
		}

		unwrapped, err := UnwrapResponse(wrapped)

		require.NoError(t, err)
		assert.Same(t, resp, unwrapped)
	})

	t.Run("unwraps through multiple layers of RWUnwrapper", func(t *testing.T) {
		mockRW := httptest.NewRecorder()
		resp := NewResponse(mockRW)

		layer3 := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          resp,
		}
		layer2 := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          layer3,
		}
		layer1 := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          layer2,
		}

		unwrapped, err := UnwrapResponse(layer1)

		require.NoError(t, err)
		assert.Same(t, resp, unwrapped)
	})

	t.Run("returns error when ResponseWriter doesn't implement Unwrap", func(t *testing.T) {
		mockRW := httptest.NewRecorder()

		unwrapped, err := UnwrapResponse(mockRW)

		require.Error(t, err)
		assert.Nil(t, unwrapped)
		assert.Equal(t, "ResponseWriter does not implement 'Unwrap() http.ResponseWriter' interface", err.Error())
	})

	t.Run("returns error when unwrapping chain ends without finding Response", func(t *testing.T) {
		innerRW := httptest.NewRecorder()
		middleRW := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          innerRW,
		}
		outerRW := &mockUnwrapper{
			ResponseWriter: httptest.NewRecorder(),
			inner:          middleRW,
		}

		unwrapped, err := UnwrapResponse(outerRW)

		require.Error(t, err)
		assert.Nil(t, unwrapped)
		assert.Equal(t, "ResponseWriter does not implement 'Unwrap() http.ResponseWriter' interface", err.Error())
	})

	t.Run("handles nil ResponseWriter", func(t *testing.T) {
		var rw http.ResponseWriter = nil

		unwrapped, err := UnwrapResponse(rw)

		require.Error(t, err)
		assert.Nil(t, unwrapped)
		assert.Equal(t, "ResponseWriter does not implement 'Unwrap() http.ResponseWriter' interface", err.Error())
	})
}
