package convert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringToBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []byte{},
		},
		{
			name:     "single character",
			input:    "a",
			expected: []byte{'a'},
		},
		{
			name:     "simple ASCII string",
			input:    "hello world",
			expected: []byte("hello world"),
		},
		{
			name:     "string with spaces",
			input:    "  leading and trailing  ",
			expected: []byte("  leading and trailing  "),
		},
		{
			name:     "string with newlines",
			input:    "line1\nline2\nline3",
			expected: []byte("line1\nline2\nline3"),
		},
		{
			name:     "string with tabs",
			input:    "col1\tcol2\tcol3",
			expected: []byte("col1\tcol2\tcol3"),
		},
		{
			name:     "string with null bytes",
			input:    "before\x00after",
			expected: []byte("before\x00after"),
		},
		{
			name:     "UTF-8 characters",
			input:    "héllo wörld 🌍",
			expected: []byte("héllo wörld 🌍"),
		},
		{
			name:     "JSON-like string",
			input:    `{"key": "value", "number": 123}`,
			expected: []byte(`{"key": "value", "number": 123}`),
		},
		{
			name:     "URL-like string",
			input:    "https://example.com/path?query=value&other=123",
			expected: []byte("https://example.com/path?query=value&other=123"),
		},
		{
			name:     "binary data",
			input:    "\x01\x02\x03\x04\x05",
			expected: []byte("\x01\x02\x03\x04\x05"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test the conversion
			result := StringToBytes(tt.input)

			// Verify the result matches expected bytes
			require.Equal(t, tt.expected, result, "Converted bytes should match expected bytes")

			// Verify the length is preserved
			require.Equal(t, len(tt.input), len(result), "Length should be preserved")

			// Verify the capacity equals the length (as expected from unsafe.Slice)
			require.Equal(t, len(tt.input), cap(result), "Capacity should equal length")
		})
	}
}

func TestStringToBytesMemorySafety(t *testing.T) {
	t.Parallel()

	originalStr := "test string"

	// Convert string to bytes
	bytes := StringToBytes(originalStr)

	// Verify we can safely read the bytes
	require.Equal(t, []byte(originalStr), bytes)

	// Verify that modifying the returned byte slice would affect the string
	// Note: This is undefined behavior in Go, but we test that the conversion
	// doesn't crash and that the byte slice is readable
	for i, b := range bytes {
		require.Equal(t, byte(originalStr[i]), b, "Byte at position %d should match original string", i)
	}
}

func TestStringToBytesEmptyString(t *testing.T) {
	t.Parallel()

	// Test with empty string
	emptyStr := ""
	bytes := StringToBytes(emptyStr)

	// Should return empty slice (may be nil for empty string)
	require.Equal(t, 0, len(bytes), "Length should be 0 for empty string")

	// If not nil, capacity should be 0
	if bytes != nil {
		require.Equal(t, 0, cap(bytes), "Capacity should be 0 for empty string")
	}
}

//func TestStringToBytesLargeString(t *testing.T) {
//	t.Parallel()
//
//	// Create a large string (1MB)
//	largeStr := string(make([]byte, 1024*1024))
//	for i := range largeStr {
//		largeStr = largeStr[:i] + "A" + largeStr[i+1:]
//	}
//
//	// Convert to bytes
//	bytes := StringToBytes(largeStr)
//
//	// Verify length and content
//	require.Equal(t, len(largeStr), len(bytes), "Length should be preserved for large string")
//
//	// Spot check some positions
//	for i := 0; i < len(bytes); i += len(bytes) / 100 {
//		if i < len(bytes) {
//			require.Equal(t, byte('A'), bytes[i], "Character at position %d should be 'A'", i)
//		}
//	}
//}

func TestStringToBytesConsistency(t *testing.T) {
	t.Parallel()

	input := "consistent test string"

	// Convert multiple times and verify consistency
	result1 := StringToBytes(input)
	result2 := StringToBytes(input)
	result3 := StringToBytes(input)

	// All results should be equal in content
	require.Equal(t, result1, result2, "Multiple conversions should produce identical results")
	require.Equal(t, result2, result3, "Multiple conversions should produce identical results")

	// All should match the expected byte slice
	expected := []byte(input)
	require.Equal(t, expected, result1, "Result should match expected byte slice")
}

func BenchmarkStringToBytes(b *testing.B) {
	input := "benchmark test string for performance testing"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StringToBytes(input)
	}
}

func BenchmarkStringToBytesEmpty(b *testing.B) {
	input := ""

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StringToBytes(input)
	}
}

func BenchmarkStringToBytesLarge(b *testing.B) {
	input := string(make([]byte, 1024)) // 1KB string
	for i := range input {
		input = input[:i] + "X" + input[i+1:]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StringToBytes(input)
	}
}
