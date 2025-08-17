package clicky

import (
	"testing"
	"time"

	"github.com/flanksource/clicky/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for creating pointer values
func stringPtr(s string) *string                 { return &s }
func intPtr(i int) *int                          { return &i }
func boolPtr(b bool) *bool                       { return &b }
func floatPtr(f float64) *float64                { return &f }
func timePtr(t time.Time) *time.Time             { return &t }
func durationPtr(d time.Duration) *time.Duration { return &d }

// Test structures for various pointer scenarios

// BasicPointerStruct tests basic pointer types
type BasicPointerStruct struct {
	StringPtr   *string        `json:"string_ptr" pretty:"label=String Pointer"`
	IntPtr      *int           `json:"int_ptr" pretty:"label=Integer Pointer"`
	BoolPtr     *bool          `json:"bool_ptr" pretty:"label=Boolean Pointer"`
	FloatPtr    *float64       `json:"float_ptr" pretty:"label=Float Pointer"`
	TimePtr     *time.Time     `json:"time_ptr" pretty:"label=Time Pointer"`
	DurationPtr *time.Duration `json:"duration_ptr" pretty:"label=Duration Pointer"`
}

// OptionalFieldStruct tests optional pointer fields with omitempty
type OptionalFieldStruct struct {
	Required *string `json:"required" pretty:"label=Required"`
	Optional *string `json:"optional,omitempty" pretty:"label=Optional,omitempty"`
	NilField *string `json:"nil_field,omitempty" pretty:"label=Nil Field,omitempty"`
}

// SlicePointerStruct tests slices of pointers
type SlicePointerStruct struct {
	StringSlice  []*string             `json:"string_slice" pretty:"label=String Slice"`
	IntSlice     []*int                `json:"int_slice" pretty:"label=Int Slice"`
	StructSlice  []*BasicPointerStruct `json:"struct_slice" pretty:"label=Struct Slice,table"`
	EmptySlice   []*string             `json:"empty_slice" pretty:"label=Empty Slice"`
	NilOnlySlice []*string             `json:"nil_only_slice" pretty:"label=Nil Only Slice"`
}

// MapPointerStruct tests maps with pointer values
type MapPointerStruct struct {
	StringMap   map[string]*string             `json:"string_map" pretty:"label=String Map"`
	IntMap      map[string]*int                `json:"int_map" pretty:"label=Int Map"`
	StructMap   map[string]*BasicPointerStruct `json:"struct_map" pretty:"label=Struct Map"`
	NilValueMap map[string]*string             `json:"nil_value_map" pretty:"label=Nil Value Map"`
	NestedMap   map[string]map[string]*int     `json:"nested_map" pretty:"label=Nested Map"`
}

// DoublePointerStruct tests double pointers
type DoublePointerStruct struct {
	DoubleString **string `json:"double_string" pretty:"label=Double String Pointer"`
	DoubleInt    **int    `json:"double_int" pretty:"label=Double Int Pointer"`
	DoubleBool   **bool   `json:"double_bool" pretty:"label=Double Bool Pointer"`
}

// CircularRefStruct tests circular references
type CircularRefStruct struct {
	Name   *string            `json:"name" pretty:"label=Name"`
	Parent *CircularRefStruct `json:"parent,omitempty" pretty:"label=Parent,omitempty"`
	Child  *CircularRefStruct `json:"child,omitempty" pretty:"label=Child,omitempty"`
}

// ComplexNestedStruct tests complex nested pointer scenarios
type ComplexNestedStruct struct {
	ID       *int                         `json:"id" pretty:"label=ID"`
	Name     *string                      `json:"name" pretty:"label=Name"`
	Details  *OptionalFieldStruct         `json:"details" pretty:"label=Details"`
	Items    []*SlicePointerStruct        `json:"items" pretty:"label=Items,table"`
	Metadata map[string]*MapPointerStruct `json:"metadata" pretty:"label=Metadata"`
}

// TestBasicPointerFormatting tests basic pointer handling
func TestBasicPointerFormatting(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("all pointers populated", func(t *testing.T) {
		now := time.Now()
		duration := 5 * time.Minute

		s := &BasicPointerStruct{
			StringPtr:   stringPtr("Hello World"),
			IntPtr:      intPtr(42),
			BoolPtr:     boolPtr(true),
			FloatPtr:    floatPtr(3.14159),
			TimePtr:     timePtr(now),
			DurationPtr: durationPtr(duration),
		}

		// Test with PrettyParser
		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "Hello World")
		assert.Contains(t, result, "42")
		assert.Contains(t, result, "true")
		assert.Contains(t, result, "3.14159")

		// Test with FormatManager
		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Hello World"`)
		assert.Contains(t, jsonResult, `42`)
	})

	t.Run("nil pointers", func(t *testing.T) {
		s := &BasicPointerStruct{
			StringPtr:   nil,
			IntPtr:      nil,
			BoolPtr:     nil,
			FloatPtr:    nil,
			TimePtr:     nil,
			DurationPtr: nil,
		}

		// Test with PrettyParser
		result, err := parser.Parse(s)
		require.NoError(t, err)
		// Should contain null representations
		assert.Contains(t, result, "null")

		// Test JSON format
		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"string_ptr"`)
		assert.Contains(t, jsonResult, `"int_ptr"`)
		assert.Contains(t, jsonResult, `"bool_ptr"`)
		assert.Contains(t, jsonResult, `null`)
	})

	t.Run("mixed nil and non-nil", func(t *testing.T) {
		s := &BasicPointerStruct{
			StringPtr:   stringPtr("Present"),
			IntPtr:      nil,
			BoolPtr:     boolPtr(false),
			FloatPtr:    nil,
			TimePtr:     timePtr(time.Now()),
			DurationPtr: nil,
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "Present")
		assert.Contains(t, result, "false")
		assert.Contains(t, result, "null")
	})
}

// TestSliceOfPointers tests slice of pointer handling
func TestSliceOfPointers(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("populated slice", func(t *testing.T) {
		s := &SlicePointerStruct{
			StringSlice: []*string{
				stringPtr("First"),
				stringPtr("Second"),
				stringPtr("Third"),
			},
			IntSlice: []*int{
				intPtr(1),
				intPtr(2),
				intPtr(3),
			},
			EmptySlice:   []*string{},
			NilOnlySlice: []*string{nil, nil, nil},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "First")
		assert.Contains(t, result, "Second")
		assert.Contains(t, result, "Third")

		// Test JSON format
		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"First"`)
		assert.Contains(t, jsonResult, `"Second"`)
		assert.Contains(t, jsonResult, `"Third"`)
		assert.Contains(t, jsonResult, `1`)
		assert.Contains(t, jsonResult, `2`)
		assert.Contains(t, jsonResult, `3`)
		assert.Contains(t, jsonResult, `"empty_slice"`)
		assert.Contains(t, jsonResult, `[]`)
		assert.Contains(t, jsonResult, `null`)
	})

	t.Run("mixed nil in slice", func(t *testing.T) {
		s := &SlicePointerStruct{
			StringSlice: []*string{
				stringPtr("First"),
				nil,
				stringPtr("Third"),
				nil,
			},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "First")
		assert.Contains(t, result, "Third")

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"First"`)
		assert.Contains(t, jsonResult, `"Third"`)
		assert.Contains(t, jsonResult, `null`)
	})

	t.Run("slice of struct pointers", func(t *testing.T) {
		s := &SlicePointerStruct{
			StructSlice: []*BasicPointerStruct{
				{
					StringPtr: stringPtr("Item 1"),
					IntPtr:    intPtr(1),
				},
				nil,
				{
					StringPtr: stringPtr("Item 3"),
					IntPtr:    intPtr(3),
				},
			},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "Item 1")
		assert.Contains(t, result, "Item 3")
	})
}

// TestMapWithPointerValues tests maps with pointer values
func TestMapWithPointerValues(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("string pointer map", func(t *testing.T) {
		s := &MapPointerStruct{
			StringMap: map[string]*string{
				"key1": stringPtr("value1"),
				"key2": stringPtr("value2"),
				"key3": nil,
			},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "key1")
		assert.Contains(t, result, "value1")
		assert.Contains(t, result, "key2")
		assert.Contains(t, result, "value2")
		assert.Contains(t, result, "key3")

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"value1"`)
		assert.Contains(t, jsonResult, `"value2"`)
		assert.Contains(t, jsonResult, `"key3"`)
		assert.Contains(t, jsonResult, `null`)
	})

	t.Run("struct pointer map", func(t *testing.T) {
		s := &MapPointerStruct{
			StructMap: map[string]*BasicPointerStruct{
				"first": {
					StringPtr: stringPtr("First Item"),
					IntPtr:    intPtr(100),
				},
				"second": nil,
				"third": {
					StringPtr: nil,
					IntPtr:    intPtr(300),
				},
			},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "First Item")
		assert.Contains(t, result, "100")
		assert.Contains(t, result, "300")
	})

	t.Run("nested map with pointers", func(t *testing.T) {
		s := &MapPointerStruct{
			NestedMap: map[string]map[string]*int{
				"outer1": {
					"inner1": intPtr(11),
					"inner2": intPtr(12),
					"inner3": nil,
				},
				"outer2": {
					"inner1": nil,
					"inner2": intPtr(22),
				},
				"outer3": nil,
			},
		}

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `11`)
		assert.Contains(t, jsonResult, `12`)
		assert.Contains(t, jsonResult, `22`)
		assert.Contains(t, jsonResult, `"outer3"`)
		assert.Contains(t, jsonResult, `null`)
	})
}

// TestDoublePointers tests double pointer handling
func TestDoublePointers(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("non-nil double pointers", func(t *testing.T) {
		strVal := "Hello"
		strPtr := &strVal
		intVal := 42
		intPtr := &intVal
		boolVal := true
		boolPtr := &boolVal

		s := &DoublePointerStruct{
			DoubleString: &strPtr,
			DoubleInt:    &intPtr,
			DoubleBool:   &boolPtr,
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "Hello")
		assert.Contains(t, result, "42")
		assert.Contains(t, result, "true")

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Hello"`)
		assert.Contains(t, jsonResult, `42`)
		assert.Contains(t, jsonResult, `true`)
	})

	t.Run("nil at different levels", func(t *testing.T) {
		var nilBoolPtr *bool = nil

		s := &DoublePointerStruct{
			DoubleString: nil,         // nil at first level
			DoubleInt:    nil,         // nil at first level
			DoubleBool:   &nilBoolPtr, // nil at second level
		}

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"double_string"`)
		assert.Contains(t, jsonResult, `"double_int"`)
		assert.Contains(t, jsonResult, `"double_bool"`)
		assert.Contains(t, jsonResult, `null`)
	})
}

// TestCircularReferences tests circular reference handling
func TestCircularReferences(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("parent-child circular reference", func(t *testing.T) {
		parent := &CircularRefStruct{
			Name: stringPtr("Parent"),
		}
		child := &CircularRefStruct{
			Name:   stringPtr("Child"),
			Parent: parent,
		}
		parent.Child = child

		// This should not cause infinite recursion
		// Most formatters should detect and handle circular refs
		result, err := parser.Parse(parent)
		require.NoError(t, err)
		assert.Contains(t, result, "Parent")
		assert.Contains(t, result, "Child")
	})

	t.Run("self-reference", func(t *testing.T) {
		self := &CircularRefStruct{
			Name: stringPtr("Self"),
		}
		self.Parent = self // Self-reference

		// Should handle without infinite recursion
		jsonResult, err := manager.JSON(self)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Self"`)
	})
}

// TestComplexNestedStructures tests complex nested pointer scenarios
func TestComplexNestedStructures(t *testing.T) {
	parser := NewPrettyParser()
	manager := formatters.NewFormatManager()

	t.Run("fully populated complex struct", func(t *testing.T) {
		s := &ComplexNestedStruct{
			ID:   intPtr(1),
			Name: stringPtr("Complex Test"),
			Details: &OptionalFieldStruct{
				Required: stringPtr("Required Value"),
				Optional: stringPtr("Optional Value"),
				NilField: nil,
			},
			Items: []*SlicePointerStruct{
				{
					StringSlice: []*string{stringPtr("Item1"), stringPtr("Item2")},
					IntSlice:    []*int{intPtr(10), intPtr(20)},
				},
				nil,
			},
			Metadata: map[string]*MapPointerStruct{
				"meta1": {
					StringMap: map[string]*string{
						"key": stringPtr("value"),
					},
				},
				"meta2": nil,
			},
		}

		result, err := parser.Parse(s)
		require.NoError(t, err)
		assert.Contains(t, result, "Complex Test")
		assert.Contains(t, result, "Required Value")
		assert.Contains(t, result, "Optional Value")
		assert.Contains(t, result, "Item1")
		assert.Contains(t, result, "Item2")
	})

	t.Run("sparse complex struct", func(t *testing.T) {
		s := &ComplexNestedStruct{
			ID:       intPtr(2),
			Name:     nil,
			Details:  nil,
			Items:    []*SlicePointerStruct{},
			Metadata: map[string]*MapPointerStruct{},
		}

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `2`)
		assert.Contains(t, jsonResult, `"name"`)
		assert.Contains(t, jsonResult, `"details"`)
		assert.Contains(t, jsonResult, `"items"`)
		assert.Contains(t, jsonResult, `[]`)
		assert.Contains(t, jsonResult, `"metadata"`)
		assert.Contains(t, jsonResult, `{}`)
	})
}

// TestOptionalFields tests omitempty behavior with pointers
func TestOptionalFields(t *testing.T) {
	manager := formatters.NewFormatManager()

	t.Run("with omitempty tags", func(t *testing.T) {
		s := &OptionalFieldStruct{
			Required: stringPtr("Required"),
			Optional: nil, // Should be omitted in JSON
			NilField: nil, // Should be omitted in JSON
		}

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Required"`)
		assert.NotContains(t, jsonResult, `"optional"`)
		assert.NotContains(t, jsonResult, `"nil_field"`)
	})

	t.Run("all fields populated", func(t *testing.T) {
		s := &OptionalFieldStruct{
			Required: stringPtr("Required"),
			Optional: stringPtr("Optional"),
			NilField: stringPtr("Not Nil"),
		}

		jsonResult, err := manager.JSON(s)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Required"`)
		assert.Contains(t, jsonResult, `"Optional"`)
		assert.Contains(t, jsonResult, `"Not Nil"`)
	})
}

// TestEdgeCases tests edge cases in pointer formatting
func TestEdgeCases(t *testing.T) {
	manager := formatters.NewFormatManager()

	t.Run("nil root pointer", func(t *testing.T) {
		var nilStruct *BasicPointerStruct = nil

		// Using manager for nil root test, parser.Parse may not handle nil gracefully

		jsonResult, err := manager.JSON(nilStruct)
		require.NoError(t, err)
		assert.Equal(t, "null", jsonResult)
	})

	t.Run("pointer to empty struct", func(t *testing.T) {
		type EmptyStruct struct{}
		empty := &EmptyStruct{}

		jsonResult, err := manager.JSON(empty)
		require.NoError(t, err)
		assert.Equal(t, "{}", jsonResult)
	})

	t.Run("pointer to anonymous struct", func(t *testing.T) {
		anon := &struct {
			Field *string `json:"field"`
		}{
			Field: stringPtr("Anonymous"),
		}

		jsonResult, err := manager.JSON(anon)
		require.NoError(t, err)
		assert.Contains(t, jsonResult, `"Anonymous"`)
	})

	t.Run("interface containing nil pointer", func(t *testing.T) {
		var ptr *string = nil
		var iface interface{} = ptr

		jsonResult, err := manager.JSON(iface)
		require.NoError(t, err)
		assert.Equal(t, "null", jsonResult)
	})
}

// TestAllFormats tests pointer formatting across all supported formats
func TestAllFormats(t *testing.T) {
	manager := formatters.NewFormatManager()

	s := &BasicPointerStruct{
		StringPtr:   stringPtr("Test String"),
		IntPtr:      intPtr(123),
		BoolPtr:     boolPtr(true),
		FloatPtr:    floatPtr(45.67),
		TimePtr:     timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		DurationPtr: durationPtr(30 * time.Second),
	}

	formats := []string{"json", "yaml", "pretty", "markdown", "csv"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var output string
			var err error

			switch format {
			case "json":
				output, err = manager.JSON(s)
			case "yaml":
				output, err = manager.YAML(s)
			case "pretty":
				output, err = manager.Pretty(s)
			case "markdown":
				output, err = manager.Markdown(s)
			case "csv":
				output, err = manager.CSV(s)
			}

			require.NoError(t, err, "Format %s should not error", format)
			assert.NotEmpty(t, output, "Format %s should produce output", format)

			// All formats should include the actual values
			assert.Contains(t, output, "Test String", "Format %s should contain string value", format)
			assert.Contains(t, output, "123", "Format %s should contain int value", format)
			assert.NotContains(t, output, "0x", "Output should not have pointer addresses")
		})
	}
}
