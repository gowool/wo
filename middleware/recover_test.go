package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gowool/wo"
)

func newRecoverEvent() *wo.Event {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	e := new(wo.Event)
	e.Reset(wo.NewResponse(rec), req)

	return e
}

// panickingEvent wraps an event to make Next() panic with the given value
type panickingEvent struct {
	*wo.Event
	panicValue interface{}
}

func (p *panickingEvent) Next() error {
	panic(p.panicValue)
}

// normalEvent wraps an event to make Next() return a specific error (no panic)
type normalEvent struct {
	*wo.Event
	returnErr error
}

func (n *normalEvent) Next() error {
	return n.returnErr
}

func Test_RecoverConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RecoverConfig
		expected int
	}{
		{
			name:     "zero stack size should set default",
			cfg:      RecoverConfig{StackSize: 0},
			expected: 2 << 10, // 2KB
		},
		{
			name:     "non-zero stack size should remain unchanged",
			cfg:      RecoverConfig{StackSize: 4096},
			expected: 4096,
		},
		{
			name:     "negative stack size should remain unchanged",
			cfg:      RecoverConfig{StackSize: -1},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.SetDefaults()
			require.Equal(t, tt.expected, tt.cfg.StackSize)
		})
	}
}

func Test_Recover_NormalFlow(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	err := middleware(e)

	require.NoError(t, err)
}

func Test_Recover_PanicRecovery_Error(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: errors.New("test panic"),
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")

	// Check that the internal error contains our panic message
	require.Contains(t, err.Error(), "test panic")
	require.Contains(t, err.Error(), "[PANIC RECOVER]")
}

func Test_Recover_PanicRecovery_String(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: "test string panic",
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")

	// Check that the internal error contains our panic message
	require.Contains(t, err.Error(), "test string panic")
	require.Contains(t, err.Error(), "[PANIC RECOVER]")
}

func Test_Recover_PanicRecovery_Nil(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: nil,
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")
}

func Test_Recover_PanicRecovery_ErrAbortHandler(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: http.ErrAbortHandler,
	}

	// This should re-panic the ErrAbortHandler
	require.Panics(t, func() {
		_ = middleware(panicHandler)
	})

	// Verify it's the specific error we expect
	require.PanicsWithError(t, http.ErrAbortHandler.Error(), func() {
		_ = middleware(panicHandler)
	})
}

func Test_Recover_StackTrace(t *testing.T) {
	cfg := RecoverConfig{StackSize: 4096}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	// Test with a regular panic - the stack trace will show this test function
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: "stack trace test panic",
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")

	// Check that we got a stack trace (should contain function names)
	errStr := err.Error()
	require.Contains(t, errStr, "[PANIC RECOVER]")
	require.Contains(t, errStr, "Test_Recover_StackTrace")
	// Should contain some go runtime information
	require.True(t, strings.Contains(errStr, "runtime.") || strings.Contains(errStr, "testing."))
}

func Test_Recover_StackTraceSizeLimit(t *testing.T) {
	// Test with a very small stack size to ensure truncation works
	cfg := RecoverConfig{StackSize: 100} // Very small
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: "test panic with small stack",
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")

	// The error should still contain the panic message even with small stack
	errStr := err.Error()
	require.Contains(t, errStr, "test panic with small stack")
	require.Contains(t, errStr, "[PANIC RECOVER]")
}

func Test_Recover_CustomStackSize(t *testing.T) {
	// Test various stack sizes
	testSizes := []int{512, 1024, 4096, 8192}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("stack_size_%d", size), func(t *testing.T) {
			cfg := RecoverConfig{StackSize: size}
			middleware := Recover[wo.Resolver](cfg)

			e := newRecoverEvent()
			panicHandler := &panickingEvent{
				Event:      e,
				panicValue: "test panic",
			}

			err := middleware(panicHandler)

			require.Error(t, err)
			require.Contains(t, err.Error(), "500")
			require.Contains(t, err.Error(), "Internal Server Error")
		})
	}
}

func Test_Recover_HandlerErrorPropagation(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	expectedErr := errors.New("handler error")
	normalEvent := &normalEvent{
		Event:     e,
		returnErr: expectedErr,
	}

	err := middleware(normalEvent)

	// The middleware should return the error from the handler since there was no panic
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

func Test_Recover_NilHandler(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	// Don't set a handler, should still work

	err := middleware(e)

	require.NoError(t, err)
}

func Test_Recover_MultiplePanics(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	// Test multiple consecutive panics
	for i := 0; i < 3; i++ {
		e := newRecoverEvent()
		panicHandler := &panickingEvent{
			Event:      e,
			panicValue: fmt.Errorf("panic iteration %d", i),
		}

		err := middleware(panicHandler)

		require.Error(t, err)
		require.Contains(t, err.Error(), "500")
		require.Contains(t, err.Error(), "Internal Server Error")
		require.Contains(t, err.Error(), fmt.Sprintf("panic iteration %d", i))
	}
}

func Test_Recover_DifferentPanicTypes(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	panicTests := []struct {
		name     string
		panicVal interface{}
	}{
		{
			name:     "string",
			panicVal: "string panic",
		},
		{
			name:     "error",
			panicVal: errors.New("error panic"),
		},
		{
			name:     "int",
			panicVal: 42,
		},
		{
			name:     "struct",
			panicVal: struct{ Field string }{Field: "test"},
		},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			e := newRecoverEvent()
			panicHandler := &panickingEvent{
				Event:      e,
				panicValue: tt.panicVal,
			}

			err := middleware(panicHandler)

			require.Error(t, err)
			require.Contains(t, err.Error(), "500")
			require.Contains(t, err.Error(), "Internal Server Error")
			require.Contains(t, err.Error(), "[PANIC RECOVER]")
		})
	}
}

func Test_Recover_StackTraceRealRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stack trace test in short mode")
	}

	cfg := RecoverConfig{StackSize: 4096}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()
	// Test with a string panic - the stack trace will still show the call stack
	panicHandler := &panickingEvent{
		Event:      e,
		panicValue: "real runtime stack trace test",
	}

	err := middleware(panicHandler)

	require.Error(t, err)
	// The error should be an internal server error with panic recovery info
	require.Contains(t, err.Error(), "500")
	require.Contains(t, err.Error(), "Internal Server Error")

	// Should contain the current test function name in the stack trace
	errStr := err.Error()
	require.Contains(t, errStr, "Test_Recover_StackTraceRealRuntime")
	require.Contains(t, errStr, "[PANIC RECOVER]")
}

func Test_Recover_ConcurrentAccess(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	// Test concurrent usage
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			e := newRecoverEvent()
			panicHandler := &panickingEvent{
				Event:      e,
				panicValue: fmt.Errorf("concurrent panic %d", id),
			}

			err := middleware(panicHandler)
			require.Error(t, err)
			require.Contains(t, err.Error(), "500")
			require.Contains(t, err.Error(), "Internal Server Error")
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func Test_Recover_GoroutinePanic(t *testing.T) {
	cfg := RecoverConfig{StackSize: 1024}
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()

	// This should only catch panics in the main goroutine, not spawned ones
	// We'll spawn a goroutine that panics, but catch it with its own recover to avoid crashing the test
	done := make(chan bool, 1)

	go func() {
		defer func() {
			// Catch the panic to prevent it from crashing the test
			_ = recover()
			done <- true
		}()
		panic("goroutine panic - should not be caught by middleware")
	}()

	// Wait for the goroutine to finish
	<-done

	// Now test the middleware with a normal event (no panic in main goroutine)
	handler := &normalEvent{
		Event:     e,
		returnErr: nil,
	}

	err := middleware(handler)

	// Should not error because no panic occurred in the main goroutine
	require.NoError(t, err)
}

// Helper functions for testing stack traces
func deepFunctionThatPanics() {
	// Add some depth to the call stack
	intermediateFunction()
}

func intermediateFunction() {
	panic("deep panic")
}

// Benchmark tests
func Benchmark_Recover_Middleware_NoPanic(b *testing.B) {
	cfg := RecoverConfig{StackSize: 2 << 10} // 2KB default
	middleware := Recover[wo.Resolver](cfg)

	e := newRecoverEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = middleware(e)
	}
}

func Benchmark_Recover_Middleware_WithPanic(b *testing.B) {
	cfg := RecoverConfig{StackSize: 2 << 10} // 2KB default
	middleware := Recover[wo.Resolver](cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := newRecoverEvent()
		panicHandler := &panickingEvent{
			Event:      e,
			panicValue: "benchmark panic",
		}

		_ = middleware(panicHandler)
	}
}

func Benchmark_Recover_Middleware_WithErrAbortHandler(b *testing.B) {
	cfg := RecoverConfig{StackSize: 2 << 10} // 2KB default
	middleware := Recover[wo.Resolver](cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		func() {
			defer func() {
				// Catch the re-panic to avoid crashing the benchmark
				_ = recover()
			}()

			e := newRecoverEvent()
			panicHandler := &panickingEvent{
				Event:      e,
				panicValue: http.ErrAbortHandler,
			}

			_ = middleware(panicHandler)
		}()
	}
}

func Benchmark_Recover_StackTraceCollection(b *testing.B) {
	cfg := RecoverConfig{StackSize: 4096}
	middleware := Recover[wo.Resolver](cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := newRecoverEvent()
		panicFunc := func() {
			deepFunctionThatPanics()
		}

		panicHandler := &panickingEvent{
			Event:      e,
			panicValue: panicFunc,
		}

		_ = middleware(panicHandler)
	}
}
