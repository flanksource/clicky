package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// Helper functions for creating pointers
func stringPtr(s string) *string       { return &s }
func intPtr(i int) *int                { return &i }
func int64Ptr(i int64) *int64          { return &i }
func float64Ptr(f float64) *float64    { return &f }
func boolPtr(b bool) *bool             { return &b }
func timePtr(t time.Time) *time.Time   { return &t }

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
	Currency       float64 `json:"currency" pretty:"label=Currency,format=currency"`
	CurrencyPtr    *float64 `json:"currency_ptr,omitempty" pretty:"label=Currency Pointer,format=currency,omitempty"`
	DateRFC3339    string  `json:"date_rfc3339" pretty:"label=Date (RFC3339),format=date"`
	DateUnixInt    int64   `json:"date_unix_int" pretty:"label=Date (Unix Int),format=date"`
	DateUnixString string  `json:"date_unix_string" pretty:"label=Date (Unix String),format=date"`
	DateUnixFloat  float64 `json:"date_unix_float" pretty:"label=Date (Unix Float),format=date"`

	// ==================== SLICES ====================
	// Primitive slices
	StringSlice []string  `json:"string_slice" pretty:"label=String Slice"`
	IntSlice    []int     `json:"int_slice" pretty:"label=Integer Slice"`
	FloatSlice  []float64 `json:"float_slice" pretty:"label=Float Slice"`
	BoolSlice   []bool    `json:"bool_slice" pretty:"label=Boolean Slice"`

	// Pointer slices
	StringPtrSlice []*string  `json:"string_ptr_slice" pretty:"label=String Pointer Slice"`
	IntPtrSlice    []*int     `json:"int_ptr_slice" pretty:"label=Integer Pointer Slice"`
	MixedNilSlice  []*string  `json:"mixed_nil_slice" pretty:"label=Mixed Nil Slice"`

	// Struct slices
	NestedSlice []NestedStruct `json:"nested_slice" pretty:"label=Nested Struct Slice"`

	// Empty and nil slices
	EmptySlice []string  `json:"empty_slice,omitempty" pretty:"label=Empty Slice,omitempty"`
	NilSlice   []string  `json:"nil_slice,omitempty" pretty:"label=Nil Slice,omitempty"`

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
			"ptr1": {ID: 201, Name: "Pointer Item 1", Description: stringPtr("First pointer"), Active: true},
			"ptr2": nil,
			"ptr3": {ID: 203, Name: "Pointer Item 3", Description: nil, Active: true},
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

func main() {
	// Parse command-line flags
	format := flag.String("format", "pretty", "Output format (json, yaml, csv, html, pdf, markdown, pretty)")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	output := flag.String("output", "", "Output file (default: stdout)")
	flag.Parse()

	// Create demo data
	demo := createDemoData()

	// Create format manager
	manager := formatters.NewFormatManager()

	// Handle PDF format separately (requires filename)
	if *format == "pdf" {
		outputFile := *output
		if outputFile == "" {
			outputFile = "uber_demo.pdf"
		}
		err := manager.Pdf(demo, outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating PDF: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "PDF written to: %s\n", outputFile)
		return
	}

	// Set NoColor for pretty formatter
	if *format == "pretty" && *noColor {
		manager = formatters.NewFormatManager()
		prettyFormatter := formatters.NewPrettyFormatter()
		prettyFormatter.NoColor = *noColor
		result, err := prettyFormatter.Format(demo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
			os.Exit(1)
		}
		if *output != "" {
			err = os.WriteFile(*output, []byte(result), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Output written to: %s\n", *output)
		} else {
			fmt.Print(result)
		}
		return
	}

	// Format the data
	var result string
	var err error

	switch *format {
	case "json":
		result, err = manager.JSON(demo)
	case "yaml":
		result, err = manager.YAML(demo)
	case "csv":
		result, err = manager.CSV(demo)
	case "html":
		result, err = manager.HTML(demo)
	case "markdown":
		result, err = manager.Markdown(demo)
	case "pretty":
		result, err = manager.Pretty(demo)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", *format)
		fmt.Fprintf(os.Stderr, "Supported formats: json, yaml, csv, html, pdf, markdown, pretty\n")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	// Output result
	if *output != "" {
		err = os.WriteFile(*output, []byte(result), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Output written to: %s\n", *output)
	} else {
		fmt.Print(result)
	}
}
