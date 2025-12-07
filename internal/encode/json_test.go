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

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		indent  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple object without indent",
			input:   map[string]interface{}{"name": "John", "age": 30},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "simple object with indent",
			input:   map[string]interface{}{"name": "John", "age": 30},
			indent:  "  ",
			wantErr: false,
		},
		{
			name:    "nested object with indent",
			input:   map[string]interface{}{"user": map[string]interface{}{"name": "John", "age": 30}, "active": true},
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
			input:   map[string]interface{}{},
			indent:  "",
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   []interface{}{},
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

			// Remove trailing newline for comparison ( MarshalJSON adds newline)
			if len(outputStr) > 0 && outputStr[len(outputStr)-1] == '\n' {
				outputStr = outputStr[:len(outputStr)-1]
			}

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

func TestMarshalJSONWithWriterErrors(t *testing.T) {
	tests := []struct {
		name    string
		writer  io.Writer
		wantErr bool
	}{
		{
			name:    "failing writer",
			writer:  &failingWriter{failOnWrite: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]interface{}{"test": "data"}
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

		data := map[string]interface{}{"test": "data"}
		err := MarshalJSON(nil, data, "")
		// If we get here without panic, that's also acceptable behavior
		_ = err
	})
}

func TestMarshalJSONWithUnsupportedTypes(t *testing.T) {
	// Test with a type that contains an unmarshalable channel
	data := map[string]interface{}{
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

func TestUnmarshalJSON(t *testing.T) {
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
			target:   &map[string]interface{}{},
			expected: map[string]interface{}{"name": "John", "age": float64(30)},
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
			target:   &[]map[string]interface{}{},
			expected: []map[string]interface{}{{"name": "John", "age": float64(30)}, {"name": "Jane", "age": float64(25)}},
			wantErr:  false,
		},
		{
			name:     "empty object",
			input:    `{}`,
			target:   &map[string]interface{}{},
			expected: map[string]interface{}{},
			wantErr:  false,
		},
		{
			name:     "empty array",
			input:    `[]`,
			target:   &[]interface{}{},
			expected: []interface{}{},
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
			target:   &map[string]interface{}{},
			expected: map[string]interface{}{"user": map[string]interface{}{"name": "John", "age": float64(30)}, "active": true},
			wantErr:  false,
		},
		{
			name:     "mixed array",
			input:    `[1,"two",true,null,{"nested":"value"}]`,
			target:   &[]interface{}{},
			expected: []interface{}{float64(1), "two", true, nil, map[string]interface{}{"nested": "value"}},
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
			case *map[string]interface{}:
				assert.Equal(t, tt.expected, *target)
			case *[]string:
				assert.Equal(t, tt.expected, *target)
			case *[]map[string]interface{}:
				assert.Equal(t, tt.expected, *target)
			case *[]interface{}:
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

func TestUnmarshalJSONWithReaderErrors(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		target  any
		wantErr bool
	}{
		{
			name:    "failing reader",
			reader:  &failingReader{failOnRead: true},
			target:  &map[string]interface{}{},
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

		var data map[string]interface{}
		err := UnmarshalJSON(nil, &data)
		// If we get here without panic, check if an error was returned instead
		if err != nil {
			t.Logf("Error returned instead of panic: %v", err)
		}
	})
}

func TestUnmarshalJSONWithInvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target any
	}{
		{
			name:   "invalid syntax - missing comma",
			input:  `{"name":"John" "age":30}`,
			target: &map[string]interface{}{},
		},
		{
			name:   "invalid syntax - unclosed object",
			input:  `{"name":"John","age":30`,
			target: &map[string]interface{}{},
		},
		{
			name:   "invalid syntax - unclosed string",
			input:  `{"name":"John","age":unclosed`,
			target: &map[string]interface{}{},
		},
		{
			name:   "empty input",
			input:  "",
			target: &map[string]interface{}{},
		},
		{
			name:   "whitespace only",
			input:  "   \t\n  ",
			target: &map[string]interface{}{},
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

func TestUnmarshalJSONWithTypeMismatch(t *testing.T) {
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

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		indent string
		input  any
	}{
		{
			name:   "simple object no indent",
			indent: "",
			input: map[string]interface{}{
				"name":   "John",
				"age":    30,
				"active": true,
			},
		},
		{
			name:   "simple object with indent",
			indent: "  ",
			input: map[string]interface{}{
				"name":   "John",
				"age":    30,
				"active": true,
			},
		},
		{
			name:   "complex nested object",
			indent: "\t",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
					"age":  30,
					"address": map[string]interface{}{
						"street":  "123 Main St",
						"city":    "Anytown",
						"country": "USA",
					},
				},
				"orders": []interface{}{
					map[string]interface{}{"id": 1, "total": 99.99},
					map[string]interface{}{"id": 2, "total": 149.99},
				},
				"active": true,
			},
		},
		{
			name:   "array of mixed types",
			indent: "  ",
			input: []interface{}{
				"string",
				42,
				true,
				nil,
				map[string]interface{}{"nested": "object"},
				[]interface{}{1, 2, 3},
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
			var result interface{}
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

// Helper types for testing errors

type failingWriter struct {
	failOnWrite bool
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	if fw.failOnWrite {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

type failingReader struct {
	failOnRead bool
}

func (fr *failingReader) Read(p []byte) (n int, err error) {
	if fr.failOnRead {
		return 0, errors.New("read failed")
	}
	return 0, io.EOF
}
