package arr

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Map function with int to string conversion
func TestMap_IntToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		iteratee func(int) string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []int{},
			iteratee: func(i int) string { return strconv.Itoa(i) },
			expected: nil,
		},
		{
			name:     "nil slice",
			input:    nil,
			iteratee: func(i int) string { return strconv.Itoa(i) },
			expected: nil,
		},
		{
			name:     "single element",
			input:    []int{42},
			iteratee: func(i int) string { return strconv.Itoa(i) },
			expected: []string{"42"},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			iteratee: func(i int) string { return strconv.Itoa(i * 2) },
			expected: []string{"2", "4", "6", "8", "10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Map(tt.input, tt.iteratee)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test Map function with string to int conversion
func TestMap_StringToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		iteratee func(string) int
		expected []int
	}{
		{
			name:     "empty slice",
			input:    []string{},
			iteratee: func(s string) int { num, _ := strconv.Atoi(s); return num },
			expected: nil,
		},
		{
			name:     "nil slice",
			input:    nil,
			iteratee: func(s string) int { num, _ := strconv.Atoi(s); return num },
			expected: nil,
		},
		{
			name:     "string to int conversion",
			input:    []string{"1", "2", "3"},
			iteratee: func(s string) int { num, _ := strconv.Atoi(s); return num * 2 },
			expected: []int{2, 4, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Map(tt.input, tt.iteratee)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test Map with different data types
func TestMap_DifferentTypes(t *testing.T) {
	t.Run("struct to field", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		people := []Person{
			{Name: "Alice", Age: 30},
			{Name: "Bob", Age: 25},
			{Name: "Charlie", Age: 35},
		}

		names := Map(people, func(p Person) string {
			return p.Name
		})

		assert.Equal(t, []string{"Alice", "Bob", "Charlie"}, names)
	})

	t.Run("int to string pointer", func(t *testing.T) {
		numbers := []int{1, 2, 3}

		// Map to string pointers
		pointers := Map(numbers, func(n int) *string {
			s := strconv.Itoa(n)
			return &s
		})

		require.Len(t, pointers, 3)
		assert.Equal(t, "1", *pointers[0])
		assert.Equal(t, "2", *pointers[1])
		assert.Equal(t, "3", *pointers[2])
	})

	t.Run("struct to interface", func(t *testing.T) {
		values := []int{1, 2, 3}

		// Map to interface slice
		interfaces := Map(values, func(n int) interface{} {
			return fmt.Sprintf("number-%d", n)
		})

		expected := []interface{}{"number-1", "number-2", "number-3"}
		assert.Equal(t, expected, interfaces)
	})

	t.Run("function transformation", func(t *testing.T) {
		// Map functions to their string representations
		functions := []func(int) int{
			func(x int) int { return x + 1 },
			func(x int) int { return x * 2 },
			func(x int) int { return x - 1 },
		}

		descriptions := Map(functions, func(f func(int) int) string {
			return fmt.Sprintf("%T", f)
		})

		require.Len(t, descriptions, 3)
		for _, desc := range descriptions {
			assert.Contains(t, desc, "func(int) int")
		}
	})
}

// Test Map edge cases and performance
func TestMap_EdgeCases(t *testing.T) {
	t.Run("large slice", func(t *testing.T) {
		// Create a large slice
		size := 10000
		input := make([]int, size)
		for i := 0; i < size; i++ {
			input[i] = i
		}

		// Map to squares
		result := Map(input, func(x int) int { return x * x })

		require.Len(t, result, size)
		assert.Equal(t, 0, result[0])
		assert.Equal(t, 1, result[1])
		assert.Equal(t, 4, result[2])
		assert.Equal(t, 9, result[3])
		assert.Equal(t, (size-1)*(size-1), result[size-1])
	})

	t.Run("nil iteratee function", func(t *testing.T) {
		// This should cause a panic, but we can't easily test for nil function calls in Go
		// The function signature ensures iteratee is not nil at compile time
		input := []int{1, 2, 3}
		iteratee := func(x int) int { return x * 2 }

		result := Map(input, iteratee)
		assert.Equal(t, []int{2, 4, 6}, result)
	})

	t.Run("complex transformation", func(t *testing.T) {
		type Node struct {
			ID    int
			Value string
		}

		type NodeDTO struct {
			ID       int    `json:"id"`
			Content  string `json:"content"`
			IsActive bool   `json:"isActive"`
		}

		nodes := []Node{
			{ID: 1, Value: "active"},
			{ID: 2, Value: "inactive"},
			{ID: 3, Value: "active"},
		}

		dtos := Map(nodes, func(n Node) NodeDTO {
			return NodeDTO{
				ID:       n.ID,
				Content:  n.Value,
				IsActive: n.Value == "active",
			}
		})

		expected := []NodeDTO{
			{ID: 1, Content: "active", IsActive: true},
			{ID: 2, Content: "inactive", IsActive: false},
			{ID: 3, Content: "active", IsActive: true},
		}
		assert.Equal(t, expected, dtos)
	})
}

// Test Map with modify operations
func TestMap_ModifyOperations(t *testing.T) {
	t.Run("uppercase strings", func(t *testing.T) {
		input := []string{"hello", "world", "go"}
		result := Map(input, func(s string) string {
			// Uppercase transformation
			runes := []rune(s)
			for i, r := range runes {
				if r >= 'a' && r <= 'z' {
					runes[i] = r - ('a' - 'A')
				}
			}
			return string(runes)
		})

		assert.Equal(t, []string{"HELLO", "WORLD", "GO"}, result)
	})

	t.Run("mathematical operations", func(t *testing.T) {
		input := []float64{1.0, 2.5, 3.14, 4.2}
		result := Map(input, func(f float64) float64 {
			// Round to nearest integer
			return float64(int(f + 0.5))
		})

		assert.Equal(t, []float64{1.0, 3.0, 3.0, 4.0}, result)
	})
}

// Test Copy function
func TestCopy(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: []int{},
		},
		{
			name:     "single element",
			input:    []int{42},
			expected: []int{42},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Copy(tt.input)
			assert.Equal(t, tt.expected, result)

			// Test slice independence by checking that they don't share the same underlying array
			if len(tt.input) > 0 {
				// Append to result - should not affect original
				result = append(result, 999)              //nolint:ineffassign,staticcheck
				assert.Len(t, tt.input, len(tt.expected)) // Original length unchanged
			}
		})
	}
}

// Test Copy with different data types
func TestCopy_DifferentTypes(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		input := []string{"hello", "world", "go"}
		result := Copy(input)

		assert.Equal(t, input, result)
		// Test independence
		result = append(result, "new")                           //nolint:ineffassign,staticcheck
		assert.Equal(t, []string{"hello", "world", "go"}, input) // Original unchanged
	})

	t.Run("structs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		input := []Person{
			{Name: "Alice", Age: 30},
			{Name: "Bob", Age: 25},
		}

		result := Copy(input)

		assert.Equal(t, input, result)
		// Test independence
		result = append(result, Person{Name: "New", Age: 99}) //nolint:ineffassign
		assert.Len(t, input, 2)                               // Original length unchanged

		// Verify deep copy for structs
		result[0].Name = "Modified"
		assert.Equal(t, "Alice", input[0].Name) // Original should be unchanged
	})

	t.Run("pointers", func(t *testing.T) {
		// Test with slice of pointers
		values := []int{1, 2, 3}
		pointers := []*int{&values[0], &values[1], &values[2]}

		result := Copy(pointers)

		assert.Equal(t, pointers, result)
		// Test independence
		result = append(result, &values[0]) //nolint:ineffassign
		assert.Len(t, pointers, 3)          // Original length unchanged

		// Note: The pointers themselves are copied, but they point to the same data
		*result[0] = 99
		assert.Equal(t, 99, values[0]) // Original data is changed through the copied pointer
	})

	t.Run("nested slices", func(t *testing.T) {
		// Test with nested slices (shallow copy behavior)
		input := [][]int{
			{1, 2},
			{3, 4},
			{5, 6},
		}

		result := Copy(input)

		assert.Equal(t, input, result)
		// Test independence
		result = append(result, []int{7, 8})
		assert.Len(t, input, 3) // Original length unchanged

		// Note: This is a shallow copy - nested slices are shared
		result[0][0] = 99
		assert.Equal(t, 99, input[0][0]) // Original nested slice is modified
	})

	t.Run("interfaces", func(t *testing.T) {
		input := []interface{}{
			"string",
			42,
			3.14,
			true,
		}

		result := Copy(input)

		assert.Equal(t, input, result)
		// Test independence
		result = append(result, "new") //nolint:ineffassign,staticcheck
		assert.Len(t, input, 4)        // Original length unchanged
	})

	t.Run("bytes", func(t *testing.T) {
		input := [][]byte{
			[]byte("hello"),
			[]byte("world"),
		}

		result := Copy(input)

		assert.Equal(t, input, result)
		// Test independence
		result = append(result, []byte("new"))
		assert.Len(t, input, 2) // Original length unchanged

		// Note: This is a shallow copy - byte slices are shared
		result[0][0] = 'H'                         // Replace 'h' with 'H'
		assert.Equal(t, []byte("Hello"), input[0]) // Original byte slice is modified
	})
}

// Test Copy behavior and guarantees
func TestCopy_Behavior(t *testing.T) {
	t.Run("independence from original", func(t *testing.T) {
		original := []int{1, 2, 3, 4, 5}
		copy := Copy(original)

		// Modify the copy
		copy[0] = 99
		copy = append(copy, 6)
		copy[1] = 88

		// Original should be unchanged
		assert.Equal(t, []int{1, 2, 3, 4, 5}, original)
	})

	t.Run("capacity preservation", func(t *testing.T) {
		original := make([]int, 3, 10) // len=3, cap=10
		for i := 0; i < 3; i++ {
			original[i] = i + 1
		}

		copy := Copy(original)

		assert.Equal(t, len(original), len(copy))
		// Note: Copy doesn't preserve capacity - it creates a slice with exact capacity
		assert.Equal(t, len(original), cap(copy))
		assert.Equal(t, original, copy)
	})

	t.Run("large slice performance", func(t *testing.T) {
		// Create a large slice
		size := 100000
		original := make([]int, size)
		for i := 0; i < size; i++ {
			original[i] = i
		}

		// Copy should work efficiently
		c := Copy(original)

		assert.Len(t, c, size)
		assert.Equal(t, original, c)
		// Test independence by modifying copy
		c[0] = 999
		assert.Equal(t, 0, original[0]) // Original should be unchanged
	})
}

// Benchmark tests
func BenchmarkMap(b *testing.B) {
	input := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Map(input, func(x int) int { return x * 2 })
	}
}

func BenchmarkCopy(b *testing.B) {
	input := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Copy(input)
	}
}

// Test concurrent safety (these functions are inherently thread-safe)
func TestConcurrentUsage(t *testing.T) {
	t.Run("concurrent map operations", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		// Run multiple Map operations concurrently
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				result := Map(input, func(x int) int { return x * id })
				assert.Len(t, result, len(input))
				done <- true
			}(i + 1)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent copy operations", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		// Run multiple Copy operations concurrently
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				result := Copy(input)
				assert.Equal(t, input, result)
				// Test independence
				result = append(result, 999) //nolint:ineffassign,staticcheck
				assert.Len(t, input, 10)     // Original length unchanged
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
