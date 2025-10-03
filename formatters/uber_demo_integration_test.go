package formatters

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Helper functions stringPtr, intPtr, floatPtr, boolPtr, timePtr
// are defined in pointer_formatting_test.go and reused here.

// Additional helper functions not in pointer_formatting_test.go
func int64Ptr(i int64) *int64    { return &i }
func float64Ptr(f float64) *float64 { return floatPtr(f) }

// NestedStruct demonstrates nested structure handling
type NestedStruct struct {
	ID          int     `json:"id" pretty:"label=ID"`
	Name        string  `json:"name" pretty:"label=Name"`
	Description *string `json:"description,omitempty" pretty:"label=Description,omitempty"`
	Active      bool    `json:"active" pretty:"label=Active"`
}

// Address demonstrates deeply nested maps
type Address struct {
	Street  string                 `json:"street" pretty:"label=Street"`
	City    string                 `json:"city" pretty:"label=City"`
	State   string                 `json:"state" pretty:"label=State"`
	ZipCode string                 `json:"zip_code" pretty:"label=Zip Code"`
	Country string                 `json:"country" pretty:"label=Country"`
	Coords  map[string]interface{} `json:"coordinates,omitempty" pretty:"label=Coordinates,omitempty"`
}

// TableRow demonstrates table formatting
type TableRow struct {
	ID        int     `json:"id" pretty:"label=ID"`
	Product   string  `json:"product" pretty:"label=Product"`
	Quantity  int     `json:"quantity" pretty:"label=Quantity"`
	Price     float64 `json:"price" pretty:"label=Price,format=currency"`
	Subtotal  float64 `json:"subtotal" pretty:"label=Subtotal,format=currency"`
	OrderDate string  `json:"order_date" pretty:"label=Order Date,format=date"`
}

// UberDemo demonstrates all supported field types and formatting options
type UberDemo struct {
	// ==================== PRIMITIVE TYPES ====================
	// Non-pointer primitives
	StringField string  `json:"string_field" pretty:"label=String Field"`
	IntField    int     `json:"int_field" pretty:"label=Integer Field"`
	FloatField  float64 `json:"float_field" pretty:"label=Float Field"`
	BoolField   bool    `json:"bool_field" pretty:"label=Boolean Field"`

	// Pointer primitives (can be nil)
	StringPtr *string  `json:"string_ptr,omitempty" pretty:"label=String Pointer,omitempty"`
	IntPtr    *int     `json:"int_ptr,omitempty" pretty:"label=Integer Pointer,omitempty"`
	FloatPtr  *float64 `json:"float_ptr,omitempty" pretty:"label=Float Pointer,omitempty"`
	BoolPtr   *bool    `json:"bool_ptr,omitempty" pretty:"label=Boolean Pointer,omitempty"`

	// Nil pointer examples
	NilString *string  `json:"nil_string,omitempty" pretty:"label=Nil String,omitempty"`
	NilInt    *int     `json:"nil_int,omitempty" pretty:"label=Nil Integer,omitempty"`
	NilFloat  *float64 `json:"nil_float,omitempty" pretty:"label=Nil Float,omitempty"`

	// ==================== FORMATTED FIELDS ====================
	Currency       float64  `json:"currency" pretty:"label=Currency,format=currency"`
	CurrencyPtr    *float64 `json:"currency_ptr,omitempty" pretty:"label=Currency Pointer,format=currency,omitempty"`
	DateRFC3339    string   `json:"date_rfc3339" pretty:"label=Date (RFC3339),format=date"`
	DateUnixInt    int64    `json:"date_unix_int" pretty:"label=Date (Unix Int),format=date"`
	DateUnixString string   `json:"date_unix_string" pretty:"label=Date (Unix String),format=date"`
	DateUnixFloat  float64  `json:"date_unix_float" pretty:"label=Date (Unix Float),format=date"`

	// ==================== SLICES ====================
	// Primitive slices
	StringSlice []string  `json:"string_slice" pretty:"label=String Slice"`
	IntSlice    []int     `json:"int_slice" pretty:"label=Integer Slice"`
	FloatSlice  []float64 `json:"float_slice" pretty:"label=Float Slice"`
	BoolSlice   []bool    `json:"bool_slice" pretty:"label=Boolean Slice"`

	// Pointer slices
	StringPtrSlice []*string `json:"string_ptr_slice" pretty:"label=String Pointer Slice"`
	IntPtrSlice    []*int    `json:"int_ptr_slice" pretty:"label=Integer Pointer Slice"`
	MixedNilSlice  []*string `json:"mixed_nil_slice" pretty:"label=Mixed Nil Slice"`

	// Struct slices
	NestedSlice []NestedStruct `json:"nested_slice" pretty:"label=Nested Struct Slice"`

	// Empty and nil slices
	EmptySlice []string `json:"empty_slice,omitempty" pretty:"label=Empty Slice,omitempty"`
	NilSlice   []string `json:"nil_slice,omitempty" pretty:"label=Nil Slice,omitempty"`

	// ==================== MAPS ====================
	// Simple maps
	StringMap map[string]string `json:"string_map" pretty:"label=String Map"`
	IntMap    map[string]int    `json:"int_map" pretty:"label=Integer Map"`

	// Nested maps
	NestedMap map[string]map[string]interface{} `json:"nested_map" pretty:"label=Nested Map"`

	// Maps with struct values
	StructMap map[string]NestedStruct `json:"struct_map" pretty:"label=Struct Map"`

	// Maps with pointer values
	PointerMap map[string]*NestedStruct `json:"pointer_map" pretty:"label=Pointer Map"`

	// Map with nil values
	NilValueMap map[string]*string `json:"nil_value_map" pretty:"label=Nil Value Map"`

	// Empty maps
	EmptyMap map[string]string `json:"empty_map,omitempty" pretty:"label=Empty Map,omitempty"`

	// ==================== NESTED STRUCTURES ====================
	// Non-pointer nested struct
	Nested NestedStruct `json:"nested" pretty:"label=Nested Struct"`

	// Pointer nested struct
	NestedPtr *NestedStruct `json:"nested_ptr,omitempty" pretty:"label=Nested Struct Pointer,omitempty"`

	// Nil nested struct
	NilNested *NestedStruct `json:"nil_nested,omitempty" pretty:"label=Nil Nested Struct,omitempty"`

	// Complex nested structure with map
	AddressInfo Address `json:"address_info" pretty:"label=Address Information"`

	// ==================== TABLE DATA ====================
	// Table formatted slice
	Orders []TableRow `json:"orders" pretty:"label=Orders,format=table"`

	// ==================== TREE STRUCTURE ====================
	// Tree formatted data
	FileSystem *api.SimpleTreeNode `json:"file_system,omitempty" pretty:"label=File System,format=tree,omitempty"`

	// ==================== MIXED COMPLEX DATA ====================
	// Map of slices
	CategoryProducts map[string][]string `json:"category_products" pretty:"label=Category Products"`

	// Slice of maps
	ConfigList []map[string]interface{} `json:"config_list" pretty:"label=Configuration List"`

	// Deeply nested structure
	DeepNested map[string]map[string]map[string]interface{} `json:"deep_nested" pretty:"label=Deeply Nested Map"`
}

// createDemoData creates a comprehensive demo dataset
func createDemoData() *UberDemo {
	now := time.Now()
	unixNow := now.Unix()

	return &UberDemo{
		// Primitive types
		StringField: "Hello, Clicky!",
		IntField:    42,
		FloatField:  3.14159,
		BoolField:   true,

		// Pointer primitives
		StringPtr: stringPtr("Pointer String"),
		IntPtr:    intPtr(100),
		FloatPtr:  float64Ptr(99.99),
		BoolPtr:   boolPtr(false),

		// Nil pointers (explicitly nil)
		NilString: nil,
		NilInt:    nil,
		NilFloat:  nil,

		// Formatted fields
		Currency:       1234.56,
		CurrencyPtr:    float64Ptr(789.01),
		DateRFC3339:    now.Format(time.RFC3339),
		DateUnixInt:    unixNow,
		DateUnixString: fmt.Sprintf("%d", unixNow),
		DateUnixFloat:  float64(unixNow) + 0.5,

		// Slices
		StringSlice: []string{"alpha", "beta", "gamma", "delta"},
		IntSlice:    []int{1, 2, 3, 5, 8, 13, 21},
		FloatSlice:  []float64{1.1, 2.2, 3.3, 4.4},
		BoolSlice:   []bool{true, false, true, true, false},

		// Pointer slices
		StringPtrSlice: []*string{
			stringPtr("first"),
			stringPtr("second"),
			stringPtr("third"),
		},
		IntPtrSlice: []*int{
			intPtr(10),
			intPtr(20),
			intPtr(30),
		},
		MixedNilSlice: []*string{
			stringPtr("value1"),
			nil,
			stringPtr("value3"),
			nil,
			stringPtr("value5"),
		},

		// Struct slices
		NestedSlice: []NestedStruct{
			{ID: 1, Name: "First Item", Description: stringPtr("Description for first"), Active: true},
			{ID: 2, Name: "Second Item", Description: nil, Active: false},
			{ID: 3, Name: "Third Item", Description: stringPtr("Description for third"), Active: true},
		},

		// Empty and nil slices
		EmptySlice: []string{},
		NilSlice:   nil,

		// Maps
		StringMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		IntMap: map[string]int{
			"count":  42,
			"total":  1000,
			"active": 17,
		},

		// Nested maps
		NestedMap: map[string]map[string]interface{}{
			"database": {
				"host":     "localhost",
				"port":     5432,
				"name":     "mydb",
				"ssl":      true,
				"timeout":  30,
			},
			"cache": {
				"type":    "redis",
				"ttl":     3600,
				"enabled": true,
			},
		},

		// Struct map
		StructMap: map[string]NestedStruct{
			"item1": {ID: 101, Name: "Map Item 1", Description: stringPtr("First map item"), Active: true},
			"item2": {ID: 102, Name: "Map Item 2", Description: nil, Active: false},
		},

		// Pointer map
		PointerMap: map[string]*NestedStruct{
			"ptr1": &NestedStruct{ID: 201, Name: "Pointer Item 1", Description: stringPtr("First pointer"), Active: true},
			"ptr2": nil,
			"ptr3": &NestedStruct{ID: 203, Name: "Pointer Item 3", Description: nil, Active: true},
		},

		// Nil value map
		NilValueMap: map[string]*string{
			"key1": stringPtr("has value"),
			"key2": nil,
			"key3": stringPtr("also has value"),
			"key4": nil,
		},

		// Empty map
		EmptyMap: map[string]string{},

		// Nested structures
		Nested: NestedStruct{
			ID:          1,
			Name:        "Nested Structure",
			Description: stringPtr("This is a nested struct"),
			Active:      true,
		},
		NestedPtr: &NestedStruct{
			ID:          2,
			Name:        "Nested Pointer",
			Description: stringPtr("This is a nested pointer struct"),
			Active:      false,
		},
		NilNested: nil,

		// Address with coordinates
		AddressInfo: Address{
			Street:  "123 Main Street",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "USA",
			Coords: map[string]interface{}{
				"latitude":  37.7749,
				"longitude": -122.4194,
				"elevation": 52.0,
			},
		},

		// Table data
		Orders: []TableRow{
			{
				ID:        1001,
				Product:   "Widget Pro",
				Quantity:  5,
				Price:     29.99,
				Subtotal:  149.95,
				OrderDate: "2024-01-15T10:30:00Z",
			},
			{
				ID:        1002,
				Product:   "Gadget Plus",
				Quantity:  3,
				Price:     49.99,
				Subtotal:  149.97,
				OrderDate: "2024-01-16T14:20:00Z",
			},
			{
				ID:        1003,
				Product:   "Tool Master",
				Quantity:  10,
				Price:     19.99,
				Subtotal:  199.90,
				OrderDate: "2024-01-17T09:15:00Z",
			},
		},

		// Tree structure
		FileSystem: &api.SimpleTreeNode{
			Label: "root",
			Children: []api.TreeNode{
				&api.SimpleTreeNode{
					Label: "home",
					Children: []api.TreeNode{
						&api.SimpleTreeNode{
							Label: "user",
							Children: []api.TreeNode{
								&api.SimpleTreeNode{Label: "documents (10 files)"},
								&api.SimpleTreeNode{Label: "downloads (5 files)"},
								&api.SimpleTreeNode{Label: "pictures (150 files)"},
							},
						},
					},
				},
				&api.SimpleTreeNode{
					Label: "etc",
					Children: []api.TreeNode{
						&api.SimpleTreeNode{Label: "config (system config)"},
						&api.SimpleTreeNode{Label: "hosts (network config)"},
					},
				},
				&api.SimpleTreeNode{
					Label: "var",
					Children: []api.TreeNode{
						&api.SimpleTreeNode{
							Label: "log",
							Children: []api.TreeNode{
								&api.SimpleTreeNode{Label: "system.log (2.5 MB)"},
								&api.SimpleTreeNode{Label: "error.log (150 KB)"},
							},
						},
					},
				},
			},
		},

		// Mixed complex data
		CategoryProducts: map[string][]string{
			"Electronics": {"Laptop", "Phone", "Tablet"},
			"Books":       {"Fiction", "Non-Fiction", "Technical"},
			"Clothing":    {"Shirts", "Pants", "Shoes"},
		},

		ConfigList: []map[string]interface{}{
			{
				"name":    "config1",
				"enabled": true,
				"value":   100,
			},
			{
				"name":    "config2",
				"enabled": false,
				"value":   200,
			},
		},

		DeepNested: map[string]map[string]map[string]interface{}{
			"level1": {
				"level2a": {
					"level3a": "deep value A",
					"level3b": 42,
				},
				"level2b": {
					"level3c": true,
					"level3d": 3.14,
				},
			},
		},
	}
}

// TestUberDemoNoPointerAddresses tests that no format outputs pointer addresses
func TestUberDemoNoPointerAddresses(t *testing.T) {
	manager := NewFormatManager()
	demo := createDemoData()

	// Regex to detect pointer addresses like 0x14000123abc
	pointerRegex := regexp.MustCompile(`0x[0-9a-fA-F]+`)

	testCases := []struct {
		name   string
		format func(interface{}) (string, error)
	}{
		{"JSON", manager.JSON},
		{"YAML", manager.YAML},
		{"CSV", manager.CSV},
		{"Markdown", manager.Markdown},
		{"Pretty", manager.Pretty},
		// HTML is excluded from pointer check as it may contain valid hex colors
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.format(demo)
			require.NoError(t, err, "Format %s should not error", tc.name)
			assert.NotEmpty(t, output, "Format %s should produce output", tc.name)

			// Check for pointer addresses
			matches := pointerRegex.FindAllString(output, -1)
			if len(matches) > 0 {
				t.Errorf("Format %s contains pointer addresses: %v", tc.name, matches)
				// Show context around first match
				if idx := strings.Index(output, matches[0]); idx >= 0 {
					start := max(0, idx-50)
					end := min(len(output), idx+50)
					t.Logf("Context: ...%s...", output[start:end])
				}
			}
		})
	}
}

// TestUberDemoNestedStructsFormatted tests that nested structs are properly formatted
func TestUberDemoNestedStructsFormatted(t *testing.T) {
	manager := NewFormatManager()
	demo := createDemoData()

	t.Run("Pretty format nested structs in slices", func(t *testing.T) {
		output, err := manager.Pretty(demo)
		require.NoError(t, err)

		// Should contain field names from JSON tags, not Go field names
		assert.Contains(t, output, "id:")
		assert.Contains(t, output, "name:")
		assert.Contains(t, output, "description:")
		assert.Contains(t, output, "active:")

		// Should NOT contain Go struct field names in raw format
		assert.NotContains(t, output, "ID:")
		assert.NotContains(t, output, "Name:")
	})

	t.Run("JSON format nested structs in slices", func(t *testing.T) {
		output, err := manager.JSON(demo)
		require.NoError(t, err)

		// Should contain proper JSON field names
		assert.Contains(t, output, `"id"`)
		assert.Contains(t, output, `"name"`)
		assert.Contains(t, output, `"description"`)

		// Should properly format nested structs in NestedSlice
		assert.Contains(t, output, `"nested_slice"`)
		assert.Contains(t, output, `"First Item"`)
		assert.Contains(t, output, `"Second Item"`)
	})

	t.Run("YAML format nested structs in maps", func(t *testing.T) {
		output, err := manager.YAML(demo)
		require.NoError(t, err)

		// Should contain YAML formatted nested structs
		assert.Contains(t, output, "struct_map:")
		assert.Contains(t, output, "Map Item 1")
		assert.Contains(t, output, "Map Item 2")
	})
}

// TestUberDemoPointerDereferencing tests that pointers are properly dereferenced
func TestUberDemoPointerDereferencing(t *testing.T) {
	manager := NewFormatManager()
	demo := createDemoData()

	t.Run("Pointer fields show values not addresses", func(t *testing.T) {
		output, err := manager.Pretty(demo)
		require.NoError(t, err)

		// Pointer primitives should show their values
		assert.Contains(t, output, "Pointer String")
		assert.Contains(t, output, "100")
		assert.Contains(t, output, "99.99")

		// Nil pointers should show null
		assert.Contains(t, output, "null")
	})

	t.Run("Nested struct pointer fields dereferenced", func(t *testing.T) {
		output, err := manager.Pretty(demo)
		require.NoError(t, err)

		// Description field in nested structs should be dereferenced
		assert.Contains(t, output, "Description for first")
		assert.Contains(t, output, "Description for third")
	})

	t.Run("Pointer slices properly formatted", func(t *testing.T) {
		output, err := manager.JSON(demo)
		require.NoError(t, err)

		// String pointer slice should have dereferenced values
		assert.Contains(t, output, `"first"`)
		assert.Contains(t, output, `"second"`)
		assert.Contains(t, output, `"third"`)

		// Int pointer slice should have dereferenced values
		assert.Contains(t, output, `10`)
		assert.Contains(t, output, `20`)
		assert.Contains(t, output, `30`)
	})
}

// TestUberDemoFormattingPreserved tests that special formatting is preserved
func TestUberDemoFormattingPreserved(t *testing.T) {
	manager := NewFormatManager()
	demo := createDemoData()

	t.Run("Currency formatting preserved", func(t *testing.T) {
		output, err := manager.Pretty(demo)
		require.NoError(t, err)

		// Currency fields should be formatted with $ symbol
		assert.Contains(t, output, "$1234.56")
		assert.Contains(t, output, "$789.01")
	})

	t.Run("Date formatting preserved", func(t *testing.T) {
		output, err := manager.Pretty(demo)
		require.NoError(t, err)

		// Date fields should be formatted (not as raw Unix timestamps)
		// Should not contain raw Unix timestamp numbers as-is
		assert.NotContains(t, output, fmt.Sprintf("date_unix_int: %d", demo.DateUnixInt))
	})
}

// Helper function for min/max (Go 1.21+)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
