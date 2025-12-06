package wo

import (
	"errors"
	"mime/multipart"
	"net/textproto"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SimpleStruct represents a basic struct for binding tests
type SimpleStruct struct {
	Name  string `query:"name"`
	Age   int    `query:"age"`
	Email string `query:"email"`
}

// NestedStruct represents a struct with nested struct field
type NestedStruct struct {
	User    SimpleStruct `query:"user"`
	Company string       `query:"company"`
}

// PointerStruct represents a struct with pointer fields
type PointerStruct struct {
	Name   *string  `query:"name"`
	Age    *int     `query:"age"`
	Active *bool    `query:"active"`
	Score  *float64 `query:"score"`
}

// SliceStruct represents a struct with slice fields
type SliceStruct struct {
	Tags     []string  `query:"tags"`
	Numbers  []int     `query:"numbers"`
	Floats   []float64 `query:"floats"`
	Booleans []bool    `query:"booleans"`
}

// CustomUnmarshaler implements BindUnmarshaler
type CustomUnmarshaler struct {
	Value time.Time
}

func (c *CustomUnmarshaler) UnmarshalParam(param string) error {
	t, err := time.Parse(time.RFC3339, param)
	if err != nil {
		return err
	}
	c.Value = t
	return nil
}

// CustomMultipleUnmarshaler implements BindMultipleUnmarshaler
type CustomMultipleUnmarshaler struct {
	Values []string
}

func (c *CustomMultipleUnmarshaler) UnmarshalParams(params []string) error {
	c.Values = params
	return nil
}

// TextUnmarshaler implements encoding.TextUnmarshaler
type TextUnmarshaler struct {
	Value string
}

func (t *TextUnmarshaler) UnmarshalText(text []byte) error {
	t.Value = string(text)
	return nil
}

// StructWithUnmarshalers combines different unmarshaler types
type StructWithUnmarshalers struct {
	Custom   CustomUnmarshaler         `query:"custom"`
	Multiple CustomMultipleUnmarshaler `query:"multiple"`
	Text     TextUnmarshaler           `query:"text"`
	Normal   string                    `query:"normal"`
}

// StructWithFiles tests file binding
type StructWithFiles struct {
	Document *multipart.FileHeader   `form:"document"`
	Images   []*multipart.FileHeader `form:"images"`
	Avatar   *multipart.FileHeader   `form:"avatar"`
	Name     string                  `form:"name"`
}

// AnonymousStructField tests anonymous struct field handling
type AnonymousStructField struct {
	*SimpleStruct        // No tag for anonymous embedded struct
	Extra         string `query:"extra"`
}

// TestBindData_SimpleStruct tests binding to a simple struct
func TestBindData_SimpleStruct(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string][]string
		expected SimpleStruct
		wantErr  bool
	}{
		{
			name: "valid binding",
			data: map[string][]string{
				"name":  {"John Doe"},
				"age":   {"30"},
				"email": {"john@example.com"},
			},
			expected: SimpleStruct{
				Name:  "John Doe",
				Age:   30,
				Email: "john@example.com",
			},
			wantErr: false,
		},
		{
			name: "partial binding",
			data: map[string][]string{
				"name": {"Jane Doe"},
				"age":  {"25"},
			},
			expected: SimpleStruct{
				Name: "Jane Doe",
				Age:  25,
			},
			wantErr: false,
		},
		{
			name:     "empty data",
			data:     map[string][]string{},
			expected: SimpleStruct{},
			wantErr:  false,
		},
		{
			name: "invalid integer",
			data: map[string][]string{
				"name": {"Invalid"},
				"age":  {"not-a-number"},
			},
			expected: SimpleStruct{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result SimpleStruct
			err := BindData(&result, tt.data, "query", nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestBindData_CaseInsensitive tests case-insensitive binding
func TestBindData_CaseInsensitive(t *testing.T) {
	data := map[string][]string{
		"NAME":  {"John Doe"},
		"Age":   {"30"},
		"EMAIL": {"john@example.com"},
	}

	var result SimpleStruct
	err := BindData(&result, data, "query", nil)

	assert.NoError(t, err)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, 30, result.Age)
	assert.Equal(t, "john@example.com", result.Email)
}

// TestBindData_PointerFields tests binding to pointer fields
func TestBindData_PointerFields(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string][]string
		expected PointerStruct
	}{
		{
			name: "valid pointers",
			data: map[string][]string{
				"name":   {"John"},
				"age":    {"30"},
				"active": {"true"},
				"score":  {"95.5"},
			},
			expected: PointerStruct{
				Name:   stringPtr("John"),
				Age:    intPtr(30),
				Active: boolPtr(true),
				Score:  float64Ptr(95.5),
			},
		},
		{
			name: "empty values become zero values",
			data: map[string][]string{
				"name":   {""},
				"age":    {""},
				"active": {""},
				"score":  {""},
			},
			expected: PointerStruct{
				Name:   stringPtr(""),
				Age:    intPtr(0),
				Active: boolPtr(false),
				Score:  float64Ptr(0.0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result PointerStruct
			err := BindData(&result, tt.data, "query", nil)

			assert.NoError(t, err)
			if tt.expected.Name != nil {
				require.NotNil(t, result.Name)
				assert.Equal(t, *tt.expected.Name, *result.Name)
			}
			if tt.expected.Age != nil {
				require.NotNil(t, result.Age)
				assert.Equal(t, *tt.expected.Age, *result.Age)
			}
			if tt.expected.Active != nil {
				require.NotNil(t, result.Active)
				assert.Equal(t, *tt.expected.Active, *result.Active)
			}
			if tt.expected.Score != nil {
				require.NotNil(t, result.Score)
				assert.Equal(t, *tt.expected.Score, *result.Score)
			}
		})
	}
}

// TestBindData_SliceFields tests binding to slice fields
func TestBindData_SliceFields(t *testing.T) {
	data := map[string][]string{
		"tags":     {"go", "web", "framework"},
		"numbers":  {"1", "2", "3"},
		"floats":   {"1.1", "2.2", "3.3"},
		"booleans": {"true", "false", "true"},
	}

	var result SliceStruct
	err := BindData(&result, data, "query", nil)

	assert.NoError(t, err)
	assert.Equal(t, []string{"go", "web", "framework"}, result.Tags)
	assert.Equal(t, []int{1, 2, 3}, result.Numbers)
	assert.Equal(t, []float64{1.1, 2.2, 3.3}, result.Floats)
	assert.Equal(t, []bool{true, false, true}, result.Booleans)
}

// TestBindData_NestedStruct tests binding to nested structs
func TestBindData_NestedStruct(t *testing.T) {
	// Note: The binding implementation doesn't support dot notation like "user.name"
	// It only binds to nested structs that have no explicit tag
	data := map[string][]string{
		"name":    {"John Doe"},
		"age":     {"30"},
		"email":   {"john@example.com"},
		"company": {"Acme Corp"},
	}

	var result NestedStruct
	err := BindData(&result, data, "query", nil)

	assert.NoError(t, err)
	// Only the company field should be set because User has no explicit tag
	assert.Equal(t, "", result.User.Name)
	assert.Equal(t, 0, result.User.Age)
	assert.Equal(t, "", result.User.Email)
	assert.Equal(t, "Acme Corp", result.Company)
}

// TestBindData_CustomUnmarshaler tests custom unmarshaler implementations
func TestBindData_CustomUnmarshaler(t *testing.T) {
	customTime := "2023-12-01T10:00:00Z"
	multipleValues := []string{"value1", "value2", "value3"}
	textValue := "hello world"

	data := map[string][]string{
		"custom":   {customTime},
		"multiple": multipleValues,
		"text":     {textValue},
		"normal":   {"normal value"},
	}

	var result StructWithUnmarshalers
	err := BindData(&result, data, "query", nil)

	assert.NoError(t, err)

	// Test custom unmarshaler
	parsedTime, err := time.Parse(time.RFC3339, customTime)
	assert.NoError(t, err)
	assert.Equal(t, parsedTime, result.Custom.Value)

	// Test multiple unmarshaler
	assert.Equal(t, multipleValues, result.Multiple.Values)

	// Test text unmarshaler
	assert.Equal(t, textValue, result.Text.Value)

	// Test normal field
	assert.Equal(t, "normal value", result.Normal)
}

// TestBindData_MapBinding tests binding to map types
func TestBindData_MapBinding(t *testing.T) {
	t.Run("map[string]string", func(t *testing.T) {
		data := map[string][]string{
			"key1": {"value1"},
			"key2": {"value2"},
		}

		var result map[string]string
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"key1": "value1",
			"key2": "value2",
		}, result)
	})

	t.Run("map[string][]string", func(t *testing.T) {
		data := map[string][]string{
			"key1": {"value1", "value2"},
			"key2": {"value3"},
		}

		var result map[string][]string
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		data := map[string][]string{
			"key1": {"value1"},
			"key2": {"value2"},
		}

		var result map[string]interface{}
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}, result)
	})

	t.Run("unsupported map type", func(t *testing.T) {
		data := map[string][]string{
			"key1": {"value1"},
		}

		var result map[string]int
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err) // Should not error, just not bind
		assert.Nil(t, result)
	})
}

// TestBindData_FileBinding tests file binding functionality
func TestBindData_FileBinding(t *testing.T) {
	// Create mock file headers
	file1 := &multipart.FileHeader{
		Filename: "document.pdf",
		Header:   make(textproto.MIMEHeader),
	}
	file1.Header.Set("Content-Type", "application/pdf")

	file2 := &multipart.FileHeader{
		Filename: "image.jpg",
		Header:   make(textproto.MIMEHeader),
	}
	file2.Header.Set("Content-Type", "image/jpeg")

	file3 := &multipart.FileHeader{
		Filename: "avatar.png",
		Header:   make(textproto.MIMEHeader),
	}
	file3.Header.Set("Content-Type", "image/png")

	data := map[string][]string{
		"name": {"John Doe"},
	}

	files := map[string][]*multipart.FileHeader{
		"document": {file1},
		"images":   {file2, file3},
		"avatar":   {file3},
	}

	var result StructWithFiles
	err := BindData(&result, data, "form", files)

	assert.NoError(t, err)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, file1, result.Document)
	assert.Equal(t, []*multipart.FileHeader{file2, file3}, result.Images)
	assert.Equal(t, file3, result.Avatar)
}

// TestBindData_FileBindingErrors tests file binding error cases
func TestBindData_FileBindingErrors(t *testing.T) {
	t.Run("non-existent file field", func(t *testing.T) {
		var result StructWithFiles
		err := BindData(&result, map[string][]string{}, "form", map[string][]*multipart.FileHeader{})

		assert.NoError(t, err)
		assert.Nil(t, result.Document)
		assert.Nil(t, result.Images)
		assert.Nil(t, result.Avatar)
	})

	t.Run("binding to struct FileHeader (should error)", func(t *testing.T) {
		type InvalidFileStruct struct {
			File multipart.FileHeader `form:"file"` // Should be pointer
		}

		file := &multipart.FileHeader{
			Filename: "test.txt",
		}

		files := map[string][]*multipart.FileHeader{
			"file": {file},
		}

		var result InvalidFileStruct
		err := BindData(&result, map[string][]string{}, "form", files)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "binding to multipart.FileHeader struct is not supported")
	})
}

// TestBindData_AnonymousStruct tests anonymous struct field handling
func TestBindData_AnonymousStruct(t *testing.T) {
	t.Run("valid anonymous struct without tags", func(t *testing.T) {
		// Note: The anonymous struct field needs to be initialized first
		data := map[string][]string{
			"name":  {"John Doe"},
			"age":   {"30"},
			"email": {"john@example.com"},
			"extra": {"some extra data"},
		}

		var result AnonymousStructField
		result.SimpleStruct = &SimpleStruct{}
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err)
		// The SimpleStruct fields should be bound because the anonymous field has no tag
		assert.Equal(t, "John Doe", result.Name)
		assert.Equal(t, 30, result.Age)
		assert.Equal(t, "john@example.com", result.Email)
		assert.Equal(t, "some extra data", result.Extra)
	})

	t.Run("anonymous struct with tags (should error)", func(t *testing.T) {
		type AnonymousWithTags struct {
			*SimpleStruct `query:"user"`
			Extra         string `query:"extra"`
		}

		data := map[string][]string{
			"name": {"John"},
		}

		var result AnonymousWithTags
		result.SimpleStruct = &SimpleStruct{}
		err := BindData(&result, data, "query", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query/param/form tags are not allowed with anonymous struct field")
	})

	t.Run("nested struct without explicit tags", func(t *testing.T) {
		// This shows how nested structs actually work - they bind when they have no explicit tag
		type NestedStructNoTag struct {
			Inner SimpleStruct
			Extra string `query:"extra"`
		}

		data := map[string][]string{
			"name":  {"John Doe"},
			"age":   {"30"},
			"email": {"john@example.com"},
			"extra": {"some extra data"},
		}

		var result NestedStructNoTag
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err)
		assert.Equal(t, "John Doe", result.Inner.Name)
		assert.Equal(t, 30, result.Inner.Age)
		assert.Equal(t, "john@example.com", result.Inner.Email)
		assert.Equal(t, "some extra data", result.Extra)
	})
}

// TestBindData_ErrorConditions tests various error conditions
func TestBindData_ErrorConditions(t *testing.T) {
	t.Run("nil destination", func(t *testing.T) {
		err := BindData(nil, map[string][]string{}, "query", nil)
		assert.NoError(t, err) // Should not error, just return
	})

	t.Run("non-struct destination", func(t *testing.T) {
		var result string
		err := BindData(&result, map[string][]string{"key": {"value"}}, "form", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "binding element must be a struct")
	})

	t.Run("param/query/header with non-struct (should not error)", func(t *testing.T) {
		var result string
		err := BindData(&result, map[string][]string{"key": {"value"}}, "query", nil)

		assert.NoError(t, err) // Should not error for these tags
	})

	t.Run("unsupported field type", func(t *testing.T) {
		type UnsupportedStruct struct {
			Field chan int `query:"field"`
		}

		data := map[string][]string{
			"field": {"value"},
		}

		var result UnsupportedStruct
		err := BindData(&result, data, "query", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown type")
	})

	t.Run("unsettable field", func(t *testing.T) {
		type UnsettableStruct struct {
			unexported string `query:"unexported"`
		}

		data := map[string][]string{
			"unexported": {"value"},
		}

		var result UnsettableStruct
		err := BindData(&result, data, "query", nil)

		assert.NoError(t, err) // Should not error, just skip unsettable field
		assert.Empty(t, result.unexported)
	})
}

// ErrorUnmarshaler is a type that always returns an error
type ErrorUnmarshaler struct{}

func (e *ErrorUnmarshaler) UnmarshalParam(param string) error {
	return errors.New("custom unmarshal error")
}

// ErrorMultipleUnmarshaler is a type that always returns an error for multiple params
type ErrorMultipleUnmarshaler struct{}

func (e *ErrorMultipleUnmarshaler) UnmarshalParams(params []string) error {
	return errors.New("custom multiple unmarshal error")
}

// ErrorTextUnmarshaler is a type that always returns an error for text unmarshaling
type ErrorTextUnmarshaler struct{}

func (e *ErrorTextUnmarshaler) UnmarshalText(text []byte) error {
	return errors.New("custom text unmarshal error")
}

// TestBindData_CustomUnmarshalerErrors tests error cases for custom unmarshalers
func TestBindData_CustomUnmarshalerErrors(t *testing.T) {
	t.Run("BindUnmarshaler error", func(t *testing.T) {
		type TestStruct struct {
			Field ErrorUnmarshaler `query:"field"`
		}

		data := map[string][]string{
			"field": {"value"},
		}

		var result TestStruct
		err := BindData(&result, data, "query", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom unmarshal error")
	})

	t.Run("BindMultipleUnmarshaler error", func(t *testing.T) {
		type TestStruct struct {
			Field ErrorMultipleUnmarshaler `query:"field"`
		}

		data := map[string][]string{
			"field": {"value1", "value2"},
		}

		var result TestStruct
		err := BindData(&result, data, "query", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom multiple unmarshal error")
	})

	t.Run("TextUnmarshaler error", func(t *testing.T) {
		type TestStruct struct {
			Field ErrorTextUnmarshaler `query:"field"`
		}

		data := map[string][]string{
			"field": {"value"},
		}

		var result TestStruct
		err := BindData(&result, data, "query", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom text unmarshal error")
	})
}

// TestBindData_NumericConversions tests numeric type conversions
func TestBindData_NumericConversions(t *testing.T) {
	type NumericStruct struct {
		Int     int     `query:"int"`
		Int8    int8    `query:"int8"`
		Int16   int16   `query:"int16"`
		Int32   int32   `query:"int32"`
		Int64   int64   `query:"int64"`
		Uint    uint    `query:"uint"`
		Uint8   uint8   `query:"uint8"`
		Uint16  uint16  `query:"uint16"`
		Uint32  uint32  `query:"uint32"`
		Uint64  uint64  `query:"uint64"`
		Float32 float32 `query:"float32"`
		Float64 float64 `query:"float64"`
		Bool    bool    `query:"bool"`
	}

	data := map[string][]string{
		"int":     {"-42"},
		"int8":    {"-128"},
		"int16":   {"-32768"},
		"int32":   {"-2147483648"},
		"int64":   {"-9223372036854775808"},
		"uint":    {"42"},
		"uint8":   {"255"},
		"uint16":  {"65535"},
		"uint32":  {"4294967295"},
		"uint64":  {"18446744073709551615"},
		"float32": {"3.14"},
		"float64": {"3.14159265359"},
		"bool":    {"true"},
	}

	var result NumericStruct
	err := BindData(&result, data, "query", nil)

	assert.NoError(t, err)
	assert.Equal(t, -42, result.Int)
	assert.Equal(t, int8(-128), result.Int8)
	assert.Equal(t, int16(-32768), result.Int16)
	assert.Equal(t, int32(-2147483648), result.Int32)
	assert.Equal(t, int64(-9223372036854775808), result.Int64)
	assert.Equal(t, uint(42), result.Uint)
	assert.Equal(t, uint8(255), result.Uint8)
	assert.Equal(t, uint16(65535), result.Uint16)
	assert.Equal(t, uint32(4294967295), result.Uint32)
	assert.Equal(t, uint64(18446744073709551615), result.Uint64)
	assert.Equal(t, float32(3.14), result.Float32)
	assert.Equal(t, float64(3.14159265359), result.Float64)
	assert.True(t, result.Bool)
}

// TestIsFieldMultipartFile tests the isFieldMultipartFile function
func TestIsFieldMultipartFile(t *testing.T) {
	tests := []struct {
		name        string
		fieldType   reflect.Type
		expectedOK  bool
		expectedErr bool
	}{
		{
			name:        "FileHeader pointer",
			fieldType:   reflect.TypeOf(&multipart.FileHeader{}),
			expectedOK:  true,
			expectedErr: false,
		},
		{
			name:        "FileHeader slice",
			fieldType:   reflect.TypeOf([]multipart.FileHeader(nil)),
			expectedOK:  true,
			expectedErr: false,
		},
		{
			name:        "FileHeader pointer slice",
			fieldType:   reflect.TypeOf([]*multipart.FileHeader(nil)),
			expectedOK:  true,
			expectedErr: false,
		},
		{
			name:        "FileHeader struct (should error)",
			fieldType:   reflect.TypeOf(multipart.FileHeader{}),
			expectedOK:  true,
			expectedErr: true,
		},
		{
			name:        "unsupported type",
			fieldType:   reflect.TypeOf(""),
			expectedOK:  false,
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := isFieldMultipartFile(tt.fieldType)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedOK, ok)
		})
	}
}

// TestSetMultipartFileHeaderTypes tests the setMultipartFileHeaderTypes function
func TestSetMultipartFileHeaderTypes(t *testing.T) {
	file1 := &multipart.FileHeader{Filename: "file1.txt"}
	file2 := &multipart.FileHeader{Filename: "file2.txt"}
	files := map[string][]*multipart.FileHeader{
		"files": {file1, file2},
	}

	t.Run("FileHeader pointer slice", func(t *testing.T) {
		var headers []*multipart.FileHeader
		field := reflect.ValueOf(&headers).Elem()
		result := setMultipartFileHeaderTypes(field, "files", files)

		assert.True(t, result)
		assert.Equal(t, []*multipart.FileHeader{file1, file2}, headers)
	})

	t.Run("FileHeader slice", func(t *testing.T) {
		var headers []multipart.FileHeader
		field := reflect.ValueOf(&headers).Elem()
		result := setMultipartFileHeaderTypes(field, "files", files)

		assert.True(t, result)
		assert.Len(t, headers, 2)
		assert.Equal(t, "file1.txt", headers[0].Filename)
		assert.Equal(t, "file2.txt", headers[1].Filename)
	})

	t.Run("FileHeader pointer", func(t *testing.T) {
		var header *multipart.FileHeader
		field := reflect.ValueOf(&header).Elem()
		result := setMultipartFileHeaderTypes(field, "files", files)

		assert.True(t, result)
		assert.Equal(t, file1, header)
	})

	t.Run("unsupported type", func(t *testing.T) {
		var s string
		field := reflect.ValueOf(&s).Elem()
		result := setMultipartFileHeaderTypes(field, "files", files)

		assert.False(t, result)
	})

	t.Run("non-existent field", func(t *testing.T) {
		var header *multipart.FileHeader
		field := reflect.ValueOf(&header).Elem()
		result := setMultipartFileHeaderTypes(field, "nonexistent", files)

		assert.False(t, result)
	})
}

// TestSetWithProperType tests the setWithProperType function
func TestSetWithProperType(t *testing.T) {
	tests := []struct {
		name        string
		valueKind   reflect.Kind
		value       string
		setupField  func() reflect.Value
		expectedErr bool
		verify      func(t *testing.T, field reflect.Value)
	}{
		{
			name:      "int",
			valueKind: reflect.Int,
			value:     "42",
			setupField: func() reflect.Value {
				return reflect.New(reflect.TypeOf(0)).Elem()
			},
			expectedErr: false,
			verify: func(t *testing.T, field reflect.Value) {
				assert.Equal(t, int64(42), field.Int())
			},
		},
		{
			name:      "string",
			valueKind: reflect.String,
			value:     "hello",
			setupField: func() reflect.Value {
				return reflect.New(reflect.TypeOf("")).Elem()
			},
			expectedErr: false,
			verify: func(t *testing.T, field reflect.Value) {
				assert.Equal(t, "hello", field.String())
			},
		},
		{
			name:      "bool",
			valueKind: reflect.Bool,
			value:     "true",
			setupField: func() reflect.Value {
				return reflect.New(reflect.TypeOf(false)).Elem()
			},
			expectedErr: false,
			verify: func(t *testing.T, field reflect.Value) {
				assert.True(t, field.Bool())
			},
		},
		{
			name:      "float64",
			valueKind: reflect.Float64,
			value:     "3.14",
			setupField: func() reflect.Value {
				return reflect.New(reflect.TypeOf(0.0)).Elem()
			},
			expectedErr: false,
			verify: func(t *testing.T, field reflect.Value) {
				assert.Equal(t, 3.14, field.Float())
			},
		},
		{
			name:      "pointer to int",
			valueKind: reflect.Ptr,
			value:     "42",
			setupField: func() reflect.Value {
				// Create a pointer to int and return the pointer value
				var intVar *int
				return reflect.ValueOf(&intVar).Elem() // Get the pointer field itself
			},
			expectedErr: false,
			verify: func(t *testing.T, field reflect.Value) {
				// After calling setWithProperType, the pointer should be set to point to 42
				if field.IsNil() {
					t.Error("Expected field to be non-nil after setting")
				} else {
					assert.Equal(t, int64(42), field.Elem().Int())
				}
			},
		},
		{
			name:      "invalid int",
			valueKind: reflect.Int,
			value:     "not-a-number",
			setupField: func() reflect.Value {
				return reflect.New(reflect.TypeOf(0)).Elem()
			},
			expectedErr: true,
			verify:      nil,
		},
		{
			name:      "unknown type",
			valueKind: reflect.Chan,
			value:     "value",
			setupField: func() reflect.Value {
				ch := make(chan int)
				return reflect.ValueOf(&ch).Elem()
			},
			expectedErr: true,
			verify:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := tt.setupField()
			err := setWithProperType(tt.valueKind, tt.value, field)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.verify != nil {
					tt.verify(t, field)
				}
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func float64Ptr(f float64) *float64 {
	return &f
}

// TestUnmarshalInputToField tests the unmarshalInputToField function
func TestUnmarshalInputToField(t *testing.T) {
	t.Run("BindUnmarshaler", func(t *testing.T) {
		custom := &CustomUnmarshaler{}
		field := reflect.ValueOf(custom)
		testTime := "2023-12-01T10:00:00Z"

		ok, err := unmarshalInputToField(reflect.Ptr, testTime, field)

		assert.True(t, ok)
		assert.NoError(t, err)

		expectedTime, _ := time.Parse(time.RFC3339, testTime)
		assert.Equal(t, expectedTime, custom.Value)
	})

	t.Run("TextUnmarshaler", func(t *testing.T) {
		text := &TextUnmarshaler{}
		field := reflect.ValueOf(text)
		testValue := "hello world"

		ok, err := unmarshalInputToField(reflect.Ptr, testValue, field)

		assert.True(t, ok)
		assert.NoError(t, err)

		assert.Equal(t, testValue, text.Value)
	})

	t.Run("non-unmarshaler", func(t *testing.T) {
		s := &struct{}{}
		field := reflect.ValueOf(s)

		ok, err := unmarshalInputToField(reflect.Ptr, "value", field)

		assert.False(t, ok)
		assert.NoError(t, err)
	})
}

// TestUnmarshalInputsToField tests the unmarshalInputsToField function
func TestUnmarshalInputsToField(t *testing.T) {
	t.Run("BindMultipleUnmarshaler", func(t *testing.T) {
		multiple := &CustomMultipleUnmarshaler{}
		field := reflect.ValueOf(multiple)
		values := []string{"value1", "value2", "value3"}

		ok, err := unmarshalInputsToField(reflect.Ptr, values, field)

		assert.True(t, ok)
		assert.NoError(t, err)

		assert.Equal(t, values, multiple.Values)
	})

	t.Run("non-multiple-unmarshaler", func(t *testing.T) {
		s := &struct{}{}
		field := reflect.ValueOf(s)
		values := []string{"value1", "value2"}

		ok, err := unmarshalInputsToField(reflect.Ptr, values, field)

		assert.False(t, ok)
		assert.NoError(t, err)
	})
}

// TestSetIntField tests integer field setting
func TestSetIntField(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		bitSize  int
		expected int64
		wantErr  bool
	}{
		{"valid int", "42", 32, 42, false},
		{"valid negative", "-42", 32, -42, false},
		{"empty string", "", 32, 0, false},
		{"invalid int", "not-a-number", 32, 0, true},
		{"overflow int8", "128", 8, 0, true}, // 128 overflows int8
		{"max int64", "9223372036854775807", 64, 9223372036854775807, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.New(reflect.TypeOf(int64(0))).Elem()
			err := setIntField(tt.value, tt.bitSize, field)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, field.Int())
			}
		})
	}
}

// TestSetUintField tests unsigned integer field setting
func TestSetUintField(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		bitSize  int
		expected uint64
		wantErr  bool
	}{
		{"valid uint", "42", 32, 42, false},
		{"empty string", "", 32, 0, false},
		{"invalid uint", "not-a-number", 32, 0, true},
		{"negative uint", "-1", 32, 0, true},
		{"max uint64", "18446744073709551615", 64, 18446744073709551615, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.New(reflect.TypeOf(uint64(0))).Elem()
			err := setUintField(tt.value, tt.bitSize, field)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, field.Uint())
			}
		})
	}
}

// TestSetBoolField tests boolean field setting
func TestSetBoolField(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
		wantErr  bool
	}{
		{"true", "true", true, false},
		{"True", "True", true, false},
		{"1", "1", true, false},
		{"false", "false", false, false},
		{"False", "False", false, false},
		{"0", "0", false, false},
		{"empty", "", false, false},
		{"invalid", "not-a-bool", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.New(reflect.TypeOf(false)).Elem()
			err := setBoolField(tt.value, field)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, field.Bool())
			}
		})
	}
}

// TestSetFloatField tests float field setting
func TestSetFloatField(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		bitSize  int
		expected float64
		wantErr  bool
	}{
		{"valid float", "3.14", 64, 3.14, false},
		{"negative float", "-2.5", 64, -2.5, false},
		{"integer as float", "42", 64, 42.0, false},
		{"empty string", "", 64, 0.0, false},
		{"invalid float", "not-a-number", 64, 0.0, true},
		{"scientific notation", "1.5e2", 64, 150.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflect.New(reflect.TypeOf(float64(0))).Elem()
			err := setFloatField(tt.value, tt.bitSize, field)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, field.Float())
			}
		})
	}
}

// TestBindData_ComplexScenarios tests complex real-world scenarios
func TestBindData_ComplexScenarios(t *testing.T) {
	t.Run("mixed fields with different tags", func(t *testing.T) {
		type ComplexStruct struct {
			// Basic types
			Name   string  `form:"name"`
			Age    int     `form:"age"`
			Active bool    `form:"active"`
			Score  float64 `form:"score"`

			// Pointer types
			Nickname *string `form:"nickname"`
			Height   *int    `form:"height"`

			// Slice types
			Tags    []string `form:"tags"`
			Numbers []int    `form:"numbers"`

			// Nested struct (without tag - will bind automatically)
			Profile struct {
				Bio  string `form:"bio"`
				City string `form:"city"`
			}

			// Custom unmarshaler
			BirthDate CustomUnmarshaler `form:"birthdate"`

			// Text unmarshaler
			Description TextUnmarshaler `form:"description"`
		}

		nick := "johnny"
		height := 180
		birthdate := "1990-01-01T00:00:00Z"

		data := map[string][]string{
			"name":        {"John Doe"},
			"age":         {"30"},
			"active":      {"true"},
			"score":       {"95.5"},
			"nickname":    {nick},
			"height":      {strconv.Itoa(height)},
			"tags":        {"developer", "go", "web"},
			"numbers":     {"1", "2", "3"},
			"bio":         {"Software developer"},
			"city":        {"New York"},
			"birthdate":   {birthdate},
			"description": {"Passionate developer"},
		}

		var result ComplexStruct
		err := BindData(&result, data, "form", nil)

		assert.NoError(t, err)
		assert.Equal(t, "John Doe", result.Name)
		assert.Equal(t, 30, result.Age)
		assert.True(t, result.Active)
		assert.Equal(t, 95.5, result.Score)
		assert.Equal(t, &nick, result.Nickname)
		assert.Equal(t, &height, result.Height)
		assert.Equal(t, []string{"developer", "go", "web"}, result.Tags)
		assert.Equal(t, []int{1, 2, 3}, result.Numbers)
		assert.Equal(t, "Software developer", result.Profile.Bio)
		assert.Equal(t, "New York", result.Profile.City)
		assert.Equal(t, "Passionate developer", result.Description.Value)

		expectedBirthDate, _ := time.Parse(time.RFC3339, birthdate)
		assert.Equal(t, expectedBirthDate, result.BirthDate.Value)
	})

	t.Run("empty slices and pointers", func(t *testing.T) {
		type SparseStruct struct {
			EmptySlice []string `form:"empty_slice"`
			EmptyPtr   *string  `form:"empty_ptr"`
			ZeroSlice  []int    `form:"zero_slice"`
			NotNilPtr  *string  `form:"not_nil_ptr"`
		}

		// Note: Empty slices in map[string][]string are tricky
		// For testing, we'll omit the key entirely to simulate empty slice
		data := map[string][]string{
			"zero_slice":  {"0"},     // Slice with zero value
			"empty_ptr":   {""},      // Empty string for pointer
			"not_nil_ptr": {"value"}, // Non-empty value
		}

		var result SparseStruct
		err := BindData(&result, data, "form", nil)

		assert.NoError(t, err)
		// EmptySlice should remain nil since the key wasn't in the map
		assert.Nil(t, result.EmptySlice)
		assert.NotNil(t, result.EmptyPtr)
		assert.Equal(t, "", *result.EmptyPtr)
		assert.Equal(t, []int{0}, result.ZeroSlice)
		assert.NotNil(t, result.NotNilPtr)
		assert.Equal(t, "value", *result.NotNilPtr)
	})
}
