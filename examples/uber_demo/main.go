package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
)

// Helper functions for creating pointers
func stringPtr(s string) *string     { return &s }
func intPtr(i int) *int              { return &i }
func int64Ptr(i int64) *int64        { return &i }
func float64Ptr(f float64) *float64  { return &f }
func boolPtr(b bool) *bool           { return &b }
func timePtr(t time.Time) *time.Time { return &t }

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

// IconShowcase demonstrates all available icons
type IconShowcase struct {
	Icon        api.Text `json:"icon" pretty:"label=Icon"`
	Name        string   `json:"name" pretty:"label=Name"`
	Description string   `json:"description" pretty:"label=Description"`
}

func (i IconShowcase) PrettyRow(_ any) map[string]api.Text {
	return map[string]api.Text{
		"icon":        i.Icon,
		"name":        clicky.Text(i.Name),
		"description": clicky.Text(i.Description),
	}
}

// ColorExample demonstrates Tailwind color styles
type ColorExample struct {
	ColorName api.Text `json:"color_name" pretty:"label=Color Name"`
	Text50    api.Text `json:"text_50" pretty:"label=50"`
	Text100   api.Text `json:"text_100" pretty:"label=100"`
	Text200   api.Text `json:"text_200" pretty:"label=200"`
	Text300   api.Text `json:"text_300" pretty:"label=300"`
	Text400   api.Text `json:"text_400" pretty:"label=400"`
	Text500   api.Text `json:"text_500" pretty:"label=500"`
	Text600   api.Text `json:"text_600" pretty:"label=600"`
	Text700   api.Text `json:"text_700" pretty:"label=700"`
	Text800   api.Text `json:"text_800" pretty:"label=800"`
	Text900   api.Text `json:"text_900" pretty:"label=900"`
}

// TextStyleExample demonstrates text transformations and styles
type TextStyleExample struct {
	StyleName   api.Text `json:"style_name" pretty:"label=Style"`
	Example     api.Text `json:"example" pretty:"label=Example"`
	Description string   `json:"description" pretty:"label=Description"`
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

	// ==================== STYLE SHOWCASES ====================
	// All available icons
	IconsTable []IconShowcase `json:"icons_table" pretty:"label=Available Icons,format=table"`

	// Tailwind color examples
	ColorsTable []ColorExample `json:"colors_table" pretty:"label=Tailwind Colors,format=table"`

	// Text style examples
	TextStylesTable []TextStyleExample `json:"text_styles_table" pretty:"label=Text Styles & Transformations,format=table"`
}

// createDemoData creates a comprehensive demo dataset
func createDemoData() *UberDemo {
	now := time.Now()
	unixNow := now.Unix()

	demo := UberDemo{
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
				"host":    "localhost",
				"port":    5432,
				"name":    "mydb",
				"ssl":     true,
				"timeout": 30,
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

		// Icons showcase
		IconsTable: createIconsShowcase(),

		// Colors showcase
		ColorsTable: createColorsShowcase(),

		// Text styles showcase
		TextStylesTable: createTextStylesShowcase(),
	}

	return &demo
}

// createIconsShowcase creates a comprehensive showcase of all available icons
func createIconsShowcase() []IconShowcase {
	return []IconShowcase{
		{Icon: api.Text{}.Add(icons.AI), Name: "AI", Description: "Artificial Intelligence / Robot"},
		{Icon: api.Text{}.Add(icons.ArrowDown), Name: "ArrowDown", Description: "Down arrow"},
		{Icon: api.Text{}.Add(icons.ArrowLeft), Name: "ArrowLeft", Description: "Left arrow"},
		{Icon: api.Text{}.Add(icons.ArrowRight), Name: "ArrowRight", Description: "Right arrow"},
		{Icon: api.Text{}.Add(icons.ArrowUp), Name: "ArrowUp", Description: "Up arrow"},
		{Icon: api.Text{}.Add(icons.Check), Name: "Check / Pass / Success", Description: "Success indicator"},
		{Icon: api.Text{}.Add(icons.ChevronDown), Name: "ChevronDown", Description: "Chevron down"},
		{Icon: api.Text{}.Add(icons.ChevronLeft), Name: "ChevronLeft", Description: "Chevron left"},
		{Icon: api.Text{}.Add(icons.ChevronRight), Name: "ChevronRight", Description: "Chevron right"},
		{Icon: api.Text{}.Add(icons.ChevronUp), Name: "ChevronUp", Description: "Chevron up"},
		{Icon: api.Text{}.Add(icons.CI), Name: "CI", Description: "Continuous Integration"},
		{Icon: api.Text{}.Add(icons.Clean), Name: "Clean", Description: "Cleaning / Broom"},
		{Icon: api.Text{}.Add(icons.Cloud), Name: "Cloud", Description: "Cloud computing"},
		{Icon: api.Text{}.Add(icons.Config), Name: "Config", Description: "Configuration / Settings"},
		{Icon: api.Text{}.Add(icons.Cost), Name: "Cost", Description: "Money / Dollar sign"},
		{Icon: api.Text{}.Add(icons.Cross), Name: "Cross / Error / Fail", Description: "Error indicator"},
		{Icon: api.Text{}.Add(icons.Database), Name: "Database / DB", Description: "Database"},
		{Icon: api.Text{}.Add(icons.Debug), Name: "Debug", Description: "Debugging / Bug"},
		{Icon: api.Text{}.Add(icons.Docs), Name: "Docs", Description: "Documentation"},
		{Icon: api.Text{}.Add(icons.Exclamation), Name: "Exclamation", Description: "Exclamation mark"},
		{Icon: api.Text{}.Add(icons.Feat), Name: "Feat", Description: "New feature / Sparkles"},
		{Icon: api.Text{}.Add(icons.File), Name: "File", Description: "File document"},
		{Icon: api.Text{}.Add(icons.Fix), Name: "Fix", Description: "Bug fix"},
		{Icon: api.Text{}.Add(icons.Folder), Name: "Folder", Description: "Folder / Directory"},
		{Icon: api.Text{}.Add(icons.Golang), Name: "Golang", Description: "Go programming language"},
		{Icon: api.Text{}.Add(icons.Heart), Name: "Heart", Description: "Heart / Love"},
		{Icon: api.Text{}.Add(icons.Http), Name: "Http", Description: "HTTP / Web"},
		{Icon: api.Text{}.Add(icons.Idea), Name: "Idea", Description: "Light bulb / Idea"},
		{Icon: api.Text{}.Add(icons.Info), Name: "Info", Description: "Information"},
		{Icon: api.Text{}.Add(icons.Java), Name: "Java", Description: "Java programming language"},
		{Icon: api.Text{}.Add(icons.JS), Name: "JS", Description: "JavaScript"},
		{Icon: api.Text{}.Add(icons.Key), Name: "Key", Description: "Key / Access"},
		{Icon: api.Text{}.Add(icons.Kubernetes), Name: "Kubernetes", Description: "Kubernetes"},
		{Icon: api.Text{}.Add(icons.Lambda), Name: "Lambda", Description: "Lambda function"},
		{Icon: api.Text{}.Add(icons.Launch), Name: "Launch", Description: "Launch / Party"},
		{Icon: api.Text{}.Add(icons.Link), Name: "Link", Description: "Link / Chain"},
		{Icon: api.Text{}.Add(icons.Lock), Name: "Lock", Description: "Lock / Security"},
		{Icon: api.Text{}.Add(icons.Loop), Name: "Loop", Description: "Loop / Refresh"},
		{Icon: api.Text{}.Add(icons.Math), Name: "Math", Description: "Mathematics / Calculator"},
		{Icon: api.Text{}.Add(icons.Method), Name: "Method", Description: "Function / Method"},
		{Icon: api.Text{}.Add(icons.Monitor), Name: "Monitor", Description: "Computer monitor"},
		{Icon: api.Text{}.Add(icons.Network), Name: "Network", Description: "Network / Globe"},
		{Icon: api.Text{}.Add(icons.Package), Name: "Package", Description: "Package / Box"},
		{Icon: api.Text{}.Add(icons.Pause), Name: "Pause", Description: "Pause"},
		{Icon: api.Text{}.Add(icons.Pending), Name: "Pending", Description: "Pending / Hourglass"},
		{Icon: api.Text{}.Add(icons.Performance), Name: "Performance", Description: "Performance / Lightning"},
		{Icon: api.Text{}.Add(icons.Play), Name: "Play / Start", Description: "Play button"},
		{Icon: api.Text{}.Add(icons.Plugin), Name: "Plugin", Description: "Plugin / Puzzle piece"},
		{Icon: api.Text{}.Add(icons.Python), Name: "Python", Description: "Python programming language"},
		{Icon: api.Text{}.Add(icons.Refactor), Name: "Refactor", Description: "Code refactoring"},
		{Icon: api.Text{}.Add(icons.Reload), Name: "Reload", Description: "Reload / Refresh"},
		{Icon: api.Text{}.Add(icons.Robot), Name: "Robot", Description: "Robot"},
		{Icon: api.Text{}.Add(icons.Rocket), Name: "Rocket", Description: "Rocket"},
		{Icon: api.Text{}.Add(icons.Search), Name: "Search", Description: "Search / Magnifying glass"},
		{Icon: api.Text{}.Add(icons.Skip), Name: "Skip", Description: "Skip"},
		{Icon: api.Text{}.Add(icons.Star), Name: "Star", Description: "Star"},
		{Icon: api.Text{}.Add(icons.Stop), Name: "Stop", Description: "Stop sign"},
		{Icon: api.Text{}.Add(icons.Style), Name: "Style", Description: "Style / Palette"},
		{Icon: api.Text{}.Add(icons.Table), Name: "Table", Description: "Table / Clipboard"},
		{Icon: api.Text{}.Add(icons.Target), Name: "Target", Description: "Target / Bullseye"},
		{Icon: api.Text{}.Add(icons.Test), Name: "Test", Description: "Testing / Beaker"},
		{Icon: api.Text{}.Add(icons.TS), Name: "TS", Description: "TypeScript"},
		{Icon: api.Text{}.Add(icons.Type), Name: "Type", Description: "Type / Class"},
		{Icon: api.Text{}.Add(icons.Unknown), Name: "Unknown", Description: "Unknown / Question mark"},
		{Icon: api.Text{}.Add(icons.Variable), Name: "Variable", Description: "Variable"},
		{Icon: api.Text{}.Add(icons.Warning), Name: "Warning", Description: "Warning sign"},
		{Icon: api.Text{}.Add(icons.Wrench), Name: "Wrench", Description: "Wrench / Tool"},
		{Icon: api.Text{}.Add(icons.Zombie), Name: "Zombie", Description: "Zombie / Skull"},
	}
}

// createColorsShowcase creates a showcase of Tailwind color styles
func createColorsShowcase() []ColorExample {
	colors := []string{"red", "orange", "yellow", "green", "blue", "indigo", "purple", "pink", "gray"}
	result := make([]ColorExample, len(colors))

	for i, color := range colors {
		result[i] = ColorExample{
			ColorName: api.Text{Content: color, Style: "font-bold capitalize"},
			Text50:    api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-50", color)},
			Text100:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-100", color)},
			Text200:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-200", color)},
			Text300:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-300", color)},
			Text400:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-400", color)},
			Text500:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-500", color)},
			Text600:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-600", color)},
			Text700:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-700", color)},
			Text800:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-800", color)},
			Text900:   api.Text{Content: "Sample", Style: fmt.Sprintf("text-%s-900", color)},
		}
	}

	return result
}

// createTextStylesShowcase creates a showcase of text styles and transformations
func createTextStylesShowcase() []TextStyleExample {
	sampleText := "The Quick Brown Fox"

	return []TextStyleExample{
		// Text transformations
		{
			StyleName:   api.Text{Content: "UPPERCASE", Style: "font-bold text-blue-600"},
			Example:     api.Text{Content: sampleText, Style: "uppercase"},
			Description: "Converts all text to uppercase letters",
		},
		{
			StyleName:   api.Text{Content: "lowercase", Style: "font-bold text-blue-600"},
			Example:     api.Text{Content: sampleText, Style: "lowercase"},
			Description: "Converts all text to lowercase letters",
		},
		{
			StyleName:   api.Text{Content: "Capitalize", Style: "font-bold text-blue-600"},
			Example:     api.Text{Content: sampleText, Style: "capitalize"},
			Description: "Capitalizes the first letter of each word",
		},

		// Font weights
		{
			StyleName:   api.Text{Content: "Thin", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-thin"},
			Description: "Very light font weight (100)",
		},
		{
			StyleName:   api.Text{Content: "Light", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-light"},
			Description: "Light font weight (300)",
		},
		{
			StyleName:   api.Text{Content: "Normal", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-normal"},
			Description: "Normal font weight (400)",
		},
		{
			StyleName:   api.Text{Content: "Medium", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-medium"},
			Description: "Medium font weight (500)",
		},
		{
			StyleName:   api.Text{Content: "Semibold", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-semibold"},
			Description: "Semibold font weight (600)",
		},
		{
			StyleName:   api.Text{Content: "Bold", Style: "font-bold text-purple-600"},
			Example:     api.Text{Content: sampleText, Style: "font-bold"},
			Description: "Bold font weight (700)",
		},

		// Text decorations
		{
			StyleName:   api.Text{Content: "Underline", Style: "font-bold text-green-600"},
			Example:     api.Text{Content: sampleText, Style: "underline"},
			Description: "Adds an underline to text",
		},
		{
			StyleName:   api.Text{Content: "Line Through", Style: "font-bold text-green-600"},
			Example:     api.Text{Content: sampleText, Style: "line-through"},
			Description: "Adds a strikethrough line",
		},
		{
			StyleName:   api.Text{Content: "Italic", Style: "font-bold text-green-600"},
			Example:     api.Text{Content: sampleText, Style: "italic"},
			Description: "Italicizes the text",
		},

		// Opacity variations
		{
			StyleName:   api.Text{Content: "Opacity 25%", Style: "font-bold text-orange-600"},
			Example:     api.Text{Content: sampleText, Style: "opacity-25"},
			Description: "25% opacity (very faint)",
		},
		{
			StyleName:   api.Text{Content: "Opacity 50%", Style: "font-bold text-orange-600"},
			Example:     api.Text{Content: sampleText, Style: "opacity-50"},
			Description: "50% opacity (semi-transparent)",
		},
		{
			StyleName:   api.Text{Content: "Opacity 75%", Style: "font-bold text-orange-600"},
			Example:     api.Text{Content: sampleText, Style: "opacity-75"},
			Description: "75% opacity (slightly transparent)",
		},

		// Combined styles
		{
			StyleName:   api.Text{Content: "Combined 1", Style: "font-bold text-red-600"},
			Example:     api.Text{Content: sampleText, Style: "uppercase font-bold text-green-600 underline"},
			Description: "Uppercase + Bold + Green + Underline",
		},
		{
			StyleName:   api.Text{Content: "Combined 2", Style: "font-bold text-red-600"},
			Example:     api.Text{Content: sampleText, Style: "lowercase italic text-purple-700 opacity-75"},
			Description: "Lowercase + Italic + Purple + 75% Opacity",
		},
		{
			StyleName:   api.Text{Content: "Combined 3", Style: "font-bold text-red-600"},
			Example:     api.Text{Content: sampleText, Style: "capitalize font-semibold text-blue-500 underline"},
			Description: "Capitalize + Semibold + Blue + Underline",
		},
	}
}

func main() {
	// Parse command-line flags
	format := flag.String("format", "pretty", "Output format (json, yaml, csv, html, pdf, markdown, pretty)")
	flag.Parse()
	clicky.Infof("Pringint %s", *format)

	// Create demo data
	demo := createDemoData()

	t := clicky.Text("")
	for _, icon := range createIconsShowcase() {
		t = t.NewLine().Add(clicky.Text(icon.Name).Space().Add(icon.Icon))
	}

	for _, style := range demo.TextStylesTable {
		t = t.NewLine().Add(style.StyleName)
	}

	for _, color := range demo.ColorsTable {
		t = t.NewLine().Add(color.ColorName)
	}

	os.Stderr.WriteString(t.ANSI() + "\n")
	os.Stderr.WriteString(clicky.MustFormat(demo.ColorsTable, clicky.FormatOptions{Pretty: true}) + "\n")

	clicky.MustPrint(t, clicky.FormatOptions{Format: *format})

}
