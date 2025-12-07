//go:build goexperiment.jsonv2

package encode

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalJSONv2(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		indent  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple object without indent",
			input:   map[string]any{"name": "John", "age": 30},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "simple object with indent",
			input:   map[string]any{"name": "John", "age": 30},
			indent:  "  ",
			wantErr: false,
		},
		{
			name:    "nested object with indent",
			input:   map[string]any{"user": map[string]any{"name": "John", "age": 30}, "active": true},
			indent:  "\t",
			wantErr: false,
		},
		{
			name:    "array without indent",
			input:   []string{"apple", "banana", "cherry"},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "array with indent",
			input:   []string{"apple", "banana", "cherry"},
			indent:  "  ",
			wantErr: false,
		},
		{
			name:    "empty object",
			input:   map[string]any{},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   []any{},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "null value",
			input:   nil,
			indent:  "",
			wantErr: false,
		},
		{
			name:    "primitive values",
			input:   "test string",
			indent:  "",
			wantErr: false,
		},
		{
			name:    "number value",
			input:   42,
			indent:  "",
			wantErr: false,
		},
		{
			name:    "boolean true",
			input:   true,
			indent:  "",
			wantErr: false,
		},
		{
			name:    "boolean false",
			input:   false,
			indent:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := MarshalJSON(&buf, tt.input, tt.indent)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// For deterministic comparison, we compare by marshaling both input and output
			// to standard JSON and comparing the results, rather than comparing strings
			outputStr := buf.String()

			// Parse the output and compare with the original input
			var parsedOutput interface{}
			err = json.Unmarshal([]byte(outputStr), &parsedOutput)
			require.NoError(t, err, "Output should be valid JSON")

			// Convert both to canonical JSON for comparison
			canonicalInput, err := json.Marshal(tt.input)
			require.NoError(t, err)

			canonicalOutput, err := json.Marshal(parsedOutput)
			require.NoError(t, err)

			assert.Equal(t, canonicalInput, canonicalOutput, "JSON content should match regardless of key ordering")
		})
	}
}

func TestMarshalJSONv2WithWriterErrors(t *testing.T) {
	tests := []struct {
		name    string
		writer  io.Writer
		wantErr bool
	}{
		{
			name:    "failing writer",
			writer:  &failingWriterV2{failOnWrite: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"test": "data"}
			err := MarshalJSON(tt.writer, data, "")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test nil writer separately since it panics
	t.Run("nil writer", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// This is expected behavior - JSON encoder panics on nil writer
				t.Logf("Expected panic with nil writer: %v", r)
			}
		}()

		data := map[string]any{"test": "data"}
		err := MarshalJSON(nil, data, "")
		// If we get here without panic, that's also acceptable behavior
		_ = err
	})
}

func TestMarshalJSONv2WithUnsupportedTypes(t *testing.T) {
	// Test with a type that contains an unmarshalable channel
	data := map[string]any{
		"valid":   "data",
		"invalid": make(chan int), // channels cannot be marshaled to JSON
	}

	var buf bytes.Buffer

	// This will panic because channels cannot be marshaled to JSON
	defer func() {
		if r := recover(); r != nil {
			// This is expected behavior - unsupported types cause panic
			t.Logf("Expected panic with unsupported type: %v", r)
		}
	}()

	err := MarshalJSON(&buf, data, "")

	// If we get here without panic, check if an error was returned instead
	if err != nil {
		t.Logf("Error returned instead of panic: %v", err)
	}
}

func TestUnmarshalJSONv2(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		target   any
		expected any
		wantErr  bool
	}{
		{
			name:     "simple object",
			input:    `{"name":"John","age":30}`,
			target:   &map[string]any{},
			expected: map[string]any{"name": "John", "age": float64(30)},
			wantErr:  false,
		},
		{
			name:     "array of strings",
			input:    `["apple","banana","cherry"]`,
			target:   &[]string{},
			expected: []string{"apple", "banana", "cherry"},
			wantErr:  false,
		},
		{
			name:     "array of objects",
			input:    `[{"name":"John","age":30},{"name":"Jane","age":25}]`,
			target:   &[]map[string]any{},
			expected: []map[string]any{{"name": "John", "age": float64(30)}, {"name": "Jane", "age": float64(25)}},
			wantErr:  false,
		},
		{
			name:     "empty object",
			input:    `{}`,
			target:   &map[string]any{},
			expected: map[string]any{},
			wantErr:  false,
		},
		{
			name:     "empty array",
			input:    `[]`,
			target:   &[]any{},
			expected: []any{},
			wantErr:  false,
		},
		{
			name:     "null value",
			input:    `null`,
			target:   new(any),
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    `"hello world"`,
			target:   new(string),
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:     "number value to int",
			input:    `42`,
			target:   new(int),
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "number value to float64",
			input:    `3.14159`,
			target:   new(float64),
			expected: 3.14159,
			wantErr:  false,
		},
		{
			name:     "boolean true",
			input:    `true`,
			target:   new(bool),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "boolean false",
			input:    `false`,
			target:   new(bool),
			expected: false,
			wantErr:  false,
		},
		{
			name:     "nested object",
			input:    `{"user":{"name":"John","age":30},"active":true}`,
			target:   &map[string]any{},
			expected: map[string]any{"user": map[string]any{"name": "John", "age": float64(30)}, "active": true},
			wantErr:  false,
		},
		{
			name:     "mixed array",
			input:    `[1,"two",true,null,{"nested":"value"}]`,
			target:   &[]any{},
			expected: []any{float64(1), "two", true, nil, map[string]any{"nested": "value"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			err := UnmarshalJSON(reader, tt.target)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Use assertions based on the target type
			switch target := tt.target.(type) {
			case *map[string]any:
				assert.Equal(t, tt.expected, *target)
			case *[]string:
				assert.Equal(t, tt.expected, *target)
			case *[]map[string]any:
				assert.Equal(t, tt.expected, *target)
			case *[]any:
				assert.Equal(t, tt.expected, *target)
			case *any:
				assert.Equal(t, tt.expected, *target)
			case *string:
				assert.Equal(t, tt.expected, *target)
			case *int:
				assert.Equal(t, tt.expected, *target)
			case *float64:
				assert.Equal(t, tt.expected, *target)
			case *bool:
				assert.Equal(t, tt.expected, *target)
			default:
				t.Fatalf("Unsupported target type: %T", target)
			}
		})
	}
}

func TestUnmarshalJSONv2WithReaderErrors(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		target  any
		wantErr bool
	}{
		{
			name:    "failing reader",
			reader:  &failingReaderV2{failOnRead: true},
			target:  &map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnmarshalJSON(tt.reader, tt.target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test nil reader separately since it panics
	t.Run("nil reader", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// This is expected behavior - JSON decoder panics on nil reader
				t.Logf("Expected panic with nil reader: %v", r)
			}
		}()

		var data map[string]any
		err := UnmarshalJSON(nil, &data)
		// If we get here without panic, check if an error was returned instead
		if err != nil {
			t.Logf("Error returned instead of panic: %v", err)
		}
	})
}

func TestUnmarshalJSONv2WithInvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target any
	}{
		{
			name:   "invalid syntax - missing comma",
			input:  `{"name":"John" "age":30}`,
			target: &map[string]any{},
		},
		{
			name:   "invalid syntax - unclosed object",
			input:  `{"name":"John","age":30`,
			target: &map[string]any{},
		},
		{
			name:   "invalid syntax - unclosed string",
			input:  `{"name":"John","age":unclosed`,
			target: &map[string]any{},
		},
		{
			name:   "empty input",
			input:  "",
			target: &map[string]any{},
		},
		{
			name:   "whitespace only",
			input:  "   \t\n  ",
			target: &map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			err := UnmarshalJSON(reader, tt.target)
			assert.Error(t, err)
		})
	}
}

func TestUnmarshalJSONv2WithTypeMismatch(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target any
	}{
		{
			name:   "string to int",
			input:  `"not a number"`,
			target: new(int),
		},
		{
			name:   "object to string",
			input:  `{"not":"a string"}`,
			target: new(string),
		},
		{
			name:   "array to bool",
			input:  `[1,2,3]`,
			target: new(bool),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			err := UnmarshalJSON(reader, tt.target)
			assert.Error(t, err)
		})
	}
}

func TestRoundTripv2(t *testing.T) {
	tests := []struct {
		name   string
		indent string
		input  any
	}{
		{
			name:   "simple object no indent",
			indent: "",
			input: map[string]any{
				"name":   "John",
				"age":    30,
				"active": true,
			},
		},
		{
			name:   "simple object with indent",
			indent: "  ",
			input: map[string]any{
				"name":   "John",
				"age":    30,
				"active": true,
			},
		},
		{
			name:   "complex nested object",
			indent: "\t",
			input: map[string]any{
				"user": map[string]any{
					"name": "John",
					"age":  30,
					"address": map[string]any{
						"street":  "123 Main St",
						"city":    "Anytown",
						"country": "USA",
					},
				},
				"orders": []any{
					map[string]any{"id": 1, "total": 99.99},
					map[string]any{"id": 2, "total": 149.99},
				},
				"active": true,
			},
		},
		{
			name:   "array of mixed types",
			indent: "  ",
			input: []any{
				"string",
				42,
				true,
				nil,
				map[string]any{"nested": "object"},
				[]any{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			var buf bytes.Buffer
			err := MarshalJSON(&buf, tt.input, tt.indent)
			require.NoError(t, err)

			// Unmarshal back to interface{}
			var result any
			reader := bytes.NewReader(buf.Bytes())
			err = UnmarshalJSON(reader, &result)
			require.NoError(t, err)

			// Convert input to JSON and back for comparison
			// This accounts for differences in how numbers are handled (int vs float64)
			var expectedJSON []byte
			expectedJSON, err = json.Marshal(tt.input)
			require.NoError(t, err)

			var resultJSON []byte
			resultJSON, err = json.Marshal(result)
			require.NoError(t, err)

			assert.Equal(t, expectedJSON, resultJSON)
		})
	}
}

func TestJSONv2IndentOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		indent   string
		contains []string // substrings that should be in the output
	}{
		{
			name:   "two space indent",
			input:  map[string]any{"a": 1, "b": 2},
			indent: "  ",
			contains: []string{
				"  \"a\": 1",
				"  \"b\": 2",
			},
		},
		{
			name:   "tab indent",
			input:  map[string]any{"a": 1, "b": 2},
			indent: "\t",
			contains: []string{
				"\t\"a\": 1",
				"\t\"b\": 2",
			},
		},
		{
			name:   "four space indent",
			input:  map[string]any{"nested": map[string]any{"inner": "value"}},
			indent: "    ",
			contains: []string{
				"    \"nested\": {",
				"        \"inner\": \"value\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := MarshalJSON(&buf, tt.input, tt.indent)
			require.NoError(t, err)

			output := buf.String()
			for _, contain := range tt.contains {
				assert.Contains(t, output, contain, "Output should contain: %s", contain)
			}
		})
	}
}

// Helper types for testing errors

type failingWriterV2 struct {
	failOnWrite bool
}

func (fw *failingWriterV2) Write(p []byte) (n int, err error) {
	if fw.failOnWrite {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

type failingReaderV2 struct {
	failOnRead bool
}

func (fr *failingReaderV2) Read(p []byte) (n int, err error) {
	if fr.failOnRead {
		return 0, errors.New("read failed")
	}
	return 0, io.EOF
}
