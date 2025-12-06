package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGobCodec(t *testing.T) {
	codec := NewGobCodec()

	assert.NotNil(t, codec)
	assert.IsType(t, GobCodec{}, codec)
}

func TestGobCodec_Encode(t *testing.T) {
	codec := NewGobCodec()

	tests := []struct {
		name     string
		deadline time.Time
		values   map[string]any
		wantErr  bool
	}{
		{
			name:     "basic encoding",
			deadline: time.Now().Add(time.Hour),
			values:   map[string]any{"user_id": 123, "role": "admin"},
			wantErr:  false,
		},
		{
			name:     "empty values",
			deadline: time.Now().Add(time.Minute),
			values:   map[string]any{},
			wantErr:  false,
		},
		{
			name:     "nil values",
			deadline: time.Now().Add(time.Minute),
			values:   nil,
			wantErr:  false,
		},
		{
			name:     "complex values",
			deadline: time.Now().Add(24 * time.Hour),
			values: map[string]any{
				"string": "test",
				"int":    42,
				"float":  3.14,
				"bool":   true,
				"slice":  []string{"a", "b", "c"},
			},
			wantErr: false,
		},
		{
			name:     "zero deadline",
			deadline: time.Time{},
			values:   map[string]any{"test": "value"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := codec.Encode(tt.deadline, tt.values)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Greater(t, len(result), 0)
			}
		})
	}
}

func TestGobCodec_Decode(t *testing.T) {
	codec := NewGobCodec()

	// First encode some test data
	deadline := time.Now().Add(time.Hour)
	values := map[string]any{
		"user_id": 123,
		"role":    "admin",
		"active":  true,
	}
	encoded, err := codec.Encode(deadline, values)
	require.NoError(t, err)

	tests := []struct {
		name      string
		data      []byte
		wantErr   bool
		expectErr string
	}{
		{
			name:    "valid encoded data",
			data:    encoded,
			wantErr: false,
		},
		{
			name:      "empty data",
			data:      []byte{},
			wantErr:   true,
			expectErr: "EOF",
		},
		{
			name:      "nil data",
			data:      nil,
			wantErr:   true,
			expectErr: "EOF",
		},
		{
			name:      "invalid data",
			data:      []byte{0xFF, 0xFE, 0xFD},
			wantErr:   true,
			expectErr: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decodedDeadline, decodedValues, err := codec.Decode(tt.data)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectErr != "" {
					assert.Contains(t, err.Error(), tt.expectErr)
				}
				assert.Equal(t, time.Time{}, decodedDeadline)
				assert.Nil(t, decodedValues)
			} else {
				assert.NoError(t, err)
				assert.True(t, deadline.Equal(decodedDeadline))
				assert.Equal(t, values, decodedValues)
			}
		})
	}
}

func TestGobCodec_EncodeDecodeRoundTrip(t *testing.T) {
	codec := NewGobCodec()

	testCases := []struct {
		name     string
		deadline time.Time
		values   map[string]any
	}{
		{
			name:     "simple session data",
			deadline: time.Now().Add(2 * time.Hour),
			values: map[string]any{
				"user_id": 42,
				"role":    "user",
			},
		},
		{
			name:     "session with complex data",
			deadline: time.Now().Add(30 * time.Minute),
			values: map[string]any{
				"user_id":       999,
				"username":      "john_doe",
				"login_history": []string{"2023-01-01", "2023-01-15"},
				"premium":       true,
				"score":         95.5,
			},
		},
		{
			name:     "empty session",
			deadline: time.Now().Add(5 * time.Minute),
			values:   map[string]any{},
		},
		{
			name:     "zero deadline session",
			deadline: time.Time{},
			values:   map[string]any{"expired": true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode
			encoded, err := codec.Encode(tc.deadline, tc.values)
			require.NoError(t, err)
			assert.NotEmpty(t, encoded)

			// Decode
			decodedDeadline, decodedValues, err := codec.Decode(encoded)
			require.NoError(t, err)

			// Verify
			assert.True(t, tc.deadline.Equal(decodedDeadline), "deadline should match")
			assert.Equal(t, tc.values, decodedValues, "values should match")
		})
	}
}

func TestGobCodec_EncodeDecodeConsistency(t *testing.T) {
	codec := NewGobCodec()

	deadline := time.Now().Add(time.Hour)
	values := map[string]any{
		"test_string": "hello world",
		"test_int":    12345,
		"test_bool":   true,
		"test_float":  12.34,
	}

	// Encode the same data multiple times
	var encodings [][]byte
	for i := 0; i < 5; i++ {
		encoded, err := codec.Encode(deadline, values)
		require.NoError(t, err)
		encodings = append(encodings, encoded)
	}

	// All encodings should decode to the same data
	for i, encoded := range encodings {
		t.Run(fmt.Sprintf("decoding_iteration_%d", i), func(t *testing.T) {
			decodedDeadline, decodedValues, err := codec.Decode(encoded)
			require.NoError(t, err)
			assert.True(t, deadline.Equal(decodedDeadline))
			assert.Equal(t, values, decodedValues)
		})
	}
}

func TestGobCodec_EdgeCases(t *testing.T) {
	codec := NewGobCodec()

	t.Run("encoding with unregistered types", func(t *testing.T) {
		// Test with a custom type that gob can't handle
		type CustomType struct {
			Field chan int // gob can't encode channels
		}

		values := map[string]any{
			"custom": CustomType{},
		}

		_, err := codec.Encode(time.Now(), values)
		assert.Error(t, err)
	})

	t.Run("very large values", func(t *testing.T) {
		// Create a large slice
		largeSlice := make([]string, 1000)
		for i := range largeSlice {
			largeSlice[i] = "item_" + string(rune(i))
		}

		values := map[string]any{
			"large_data": largeSlice,
		}

		encoded, err := codec.Encode(time.Now().Add(time.Hour), values)
		require.NoError(t, err)

		decodedDeadline, decodedValues, err := codec.Decode(encoded)
		require.NoError(t, err)
		assert.NotZero(t, decodedDeadline)
		assert.Equal(t, len(largeSlice), len(decodedValues["large_data"].([]string)))
	})

	t.Run("unicode and special characters", func(t *testing.T) {
		values := map[string]any{
			"unicode":  "Hello 世界 🌍",
			"special":  "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			"newlines": "line1\nline2\r\nline3",
			"tabs":     "col1\tcol2\tcol3",
			"quotes":   `'single' and "double" quotes`,
		}

		encoded, err := codec.Encode(time.Now().Add(time.Hour), values)
		require.NoError(t, err)

		_, decodedValues, err := codec.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, values, decodedValues)
	})
}

func TestGobCodec_PartialCorruption(t *testing.T) {
	codec := NewGobCodec()

	// Create valid encoded data
	deadline := time.Now().Add(time.Hour)
	values := map[string]any{"test": "value"}
	encoded, err := codec.Encode(deadline, values)
	require.NoError(t, err)
	require.Greater(t, len(encoded), 10) // Ensure we have enough data to corrupt

	// Test corruption scenarios
	corruptionTests := []struct {
		name     string
		mutateFn func([]byte) []byte
	}{
		{
			name: "truncate end",
			mutateFn: func(b []byte) []byte {
				return b[:len(b)/2]
			},
		},
		{
			name: "truncate beginning",
			mutateFn: func(b []byte) []byte {
				return b[len(b)/2:]
			},
		},
		{
			name: "corrupt magic bytes",
			mutateFn: func(b []byte) []byte {
				result := make([]byte, len(b))
				copy(result, b)
				// Corrupt the first few bytes that contain gob magic numbers
				if len(result) >= 2 {
					result[0] = 0xFF
					result[1] = 0xFF
				}
				return result
			},
		},
		{
			name: "all zeros",
			mutateFn: func(b []byte) []byte {
				return make([]byte, len(b))
			},
		},
	}

	for _, tt := range corruptionTests {
		t.Run(tt.name, func(t *testing.T) {
			corrupted := tt.mutateFn(encoded)

			_, decodedValues, err := codec.Decode(corrupted)

			assert.Error(t, err)
			assert.Nil(t, decodedValues)
		})
	}
}

func TestGobCodec_ConcurrentAccess(t *testing.T) {
	codec := NewGobCodec()

	// Test concurrent encoding and decoding
	const numGoroutines = 10
	const numIterations = 5

	errChan := make(chan error, numGoroutines*2)

	// Concurrent encoding
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numIterations; j++ {
				values := map[string]any{
					"goroutine": id,
					"iteration": j,
					"data":      "test data",
				}
				_, err := codec.Encode(time.Now().Add(time.Hour), values)
				if err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}(i)
	}

	// Concurrent decoding (using pre-encoded data)
	deadline := time.Now().Add(time.Hour)
	values := map[string]any{"concurrent": "test"}
	encoded, err := codec.Encode(deadline, values)
	require.NoError(t, err)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numIterations; j++ {
				_, _, err := codec.Decode(encoded)
				if err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines*2; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}
