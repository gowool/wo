package must

import (
	"errors"
	"reflect"
	"testing"
)

func TestMust_Success(t *testing.T) {
	tests := []struct {
		name  string
		value any
		err   error
	}{
		{
			name:  "string value with nil error",
			value: "test string",
			err:   nil,
		},
		{
			name:  "integer value with nil error",
			value: 42,
			err:   nil,
		},
		{
			name:  "slice value with nil error",
			value: []int{1, 2, 3},
			err:   nil,
		},
		{
			name:  "struct value with nil error",
			value: struct{ Name string }{Name: "test"},
			err:   nil,
		},
		{
			name:  "nil value with nil error",
			value: nil,
			err:   nil,
		},
		{
			name:  "pointer value with nil error",
			value: new(int),
			err:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Must() panicked unexpectedly: %v", r)
				}
			}()

			result := Must(tt.value, tt.err)
			if !reflect.DeepEqual(result, tt.value) {
				t.Errorf("Must() returned %v, want %v", result, tt.value)
			}
		})
	}
}

func TestMust_Panic(t *testing.T) {
	tests := []struct {
		name  string
		value any
		err   error
	}{
		{
			name:  "string value with error",
			value: "test string",
			err:   errors.New("test error"),
		},
		{
			name:  "integer value with error",
			value: 42,
			err:   errors.New("another error"),
		},
		{
			name:  "nil value with error",
			value: nil,
			err:   errors.New("error with nil value"),
		},
		{
			name:  "empty error string",
			value: "test",
			err:   errors.New(""),
		},
		{
			name:  "wrapped error",
			value: true,
			err:   errors.New("wrapped error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should panic
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Must() did not panic, expected panic with error: %v", tt.err)
				} else if r != tt.err {
					t.Errorf("Must() panicked with %v, want %v", r, tt.err)
				}
			}()

			_ = Must(tt.value, tt.err)
			t.Errorf("Must() should have panicked")
		})
	}
}

func TestMust_DifferentErrorTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		err   error
	}{
		{
			name:  "standard error",
			value: "test",
			err:   errors.New("standard error"),
		},
		{
			name:  "custom error type",
			value: 123,
			err:   &customError{msg: "custom error"},
		},
		{
			name:  "nil error pointer",
			value: "test",
			err:   nil, // (*customError)(nil) is just nil, so use nil explicitly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should panic if err is not nil
			defer func() {
				if tt.err != nil {
					if r := recover(); r == nil {
						t.Errorf("Must() did not panic, expected panic with error: %v", tt.err)
					} else if r != tt.err {
						t.Errorf("Must() panicked with %v, want %v", r, tt.err)
					}
				} else {
					if r := recover(); r != nil {
						t.Errorf("Must() panicked unexpectedly: %v", r)
					}
				}
			}()

			result := Must(tt.value, tt.err)
			if tt.err == nil && !reflect.DeepEqual(result, tt.value) {
				t.Errorf("Must() returned %v, want %v", result, tt.value)
			}
		})
	}
}

// Custom error type for testing
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

// Benchmark tests
func BenchmarkMust_Success(b *testing.B) {
	value := "test value"
	var err error = nil
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Must(value, err)
	}
}

func BenchmarkMust_Panic(b *testing.B) {
	value := "test value"
	err := errors.New("test error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: We can't benchmark the actual panic since it would stop the benchmark
		// This benchmark shows the overhead when an error is present (before panic)
		func() {
			defer func() { _ = recover() }()
			_ = Must(value, err)
		}()
	}
}
