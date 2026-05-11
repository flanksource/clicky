package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/task"
	taskui "github.com/flanksource/clicky/task/ui"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Helper functions for creating pointers
func stringPtr(s string) *string     { return &s }
func intPtr(i int) *int              { return &i }
func int64Ptr(i int64) *int64        { return &i }
func float64Ptr(f float64) *float64  { return &f }
func boolPtr(b bool) *bool           { return &b }
func timePtr(t time.Time) *time.Time { return &t }
func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }

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

func (c ColorExample) PrettyRow(_ any) map[string]api.Text {
	return map[string]api.Text{
		"color_name": c.ColorName,
		"text_50":    c.Text50,
		"text_100":   c.Text100,
		"text_200":   c.Text200,
		"text_300":   c.Text300,
		"text_400":   c.Text400,
		"text_500":   c.Text500,
		"text_600":   c.Text600,
		"text_700":   c.Text700,
		"text_800":   c.Text800,
		"text_900":   c.Text900,
	}
}

// TextStyleExample demonstrates text transformations and styles
type TextStyleExample struct {
	StyleName api.Text `json:"style" pretty:"label=Style"`
	Example   api.Text `json:"example" pretty:"label=Example"`
}

func (t TextStyleExample) PrettyRow(_ any) map[string]api.Text {
	return map[string]api.Text{
		"style":   t.StyleName,
		"example": t.Example,
	}
}

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
	FileSystem *clicky.FileTreeNode `json:"file_system,omitempty" pretty:"label=File System,format=tree,omitempty"`

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

	// ==================== NEW TYPE SHOWCASES ====================
	// Standalone Code value with auto-detected language (vs CodeBlock helper)
	SampleCode api.Code `json:"sample_code" pretty:"label=Sample Code"`

	// LabelBadge two-part pills (label + value)
	LabelBadges api.TextList `json:"label_badges" pretty:"label=Label Badges,format=list"`

	// Link / LinkCommand documentation hyperlinks
	DocLinks api.TextList `json:"doc_links" pretty:"label=Documentation Links,format=list"`

	// Parsed Java stack trace with source resolution
	StackTrace api.StackTrace `json:"stack_trace" pretty:"label=Java Stack Trace"`
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

		// Tree structure - shows actual filesystem from current directory
		FileSystem: clicky.NewFileSystem(".", clicky.WithMaxDepth(2)),

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

		// New type showcases
		SampleCode:  createSampleCode(),
		LabelBadges: createLabelBadges(),
		DocLinks:    createDocLinks(),
		StackTrace:  createSampleStackTrace(),
	}

	return &demo
}

// createSampleCode demonstrates api.Code with auto-detected language.
func createSampleCode() api.Code {
	return api.NewCode(`package main

import "fmt"

func main() {
	fmt.Println("Hello, Clicky!")
}`, "")
}

// createLabelBadges demonstrates api.LabelBadge variants.
func createLabelBadges() api.TextList {
	return api.TextList{
		api.LabelBadge{Label: "env", Value: "prod", Color: "bg-blue-100", TextColor: "text-blue-800"},
		api.LabelBadge{Label: "version", Value: "v1.21.7", Color: "bg-green-100", TextColor: "text-green-800", Shape: "pill"},
		api.LabelBadge{Label: "status", Value: "healthy", Color: "bg-emerald-100", TextColor: "text-emerald-800", Icon: "mdi:check-circle"},
		api.LabelBadge{Label: "region", Value: "us-east-1", Color: "bg-slate-100", TextColor: "text-slate-700", Shape: "square"},
	}
}

// createDocLinks demonstrates clicky.Link with browser-target variants and clicky.LinkCommand.
func createDocLinks() api.TextList {
	return api.TextList{
		clicky.Link("https://docs.flanksource.com").AddText("Flanksource Docs", "text-blue-600 underline"),
		clicky.Link("https://github.com/flanksource/clicky").
			WithTarget(api.LinkTargetTab).
			AddText("Clicky on GitHub", "text-blue-600 underline"),
		clicky.LinkCommand("uber-demo stack-trace").
			AddText("Run stack-trace demo", "text-purple-600 underline"),
	}
}

// javaSampleTrace mirrors api.stacktrace_test.go's javaSample so the demo
// exercises the same parser path the unit tests cover.
const javaSampleTrace = `javax.persistence.PersistenceException: deadlock victim
    at com.example.admin.pas.dal.ActivityDal.findNextPendingActivity(ActivityDal.java:241)
    at com.example.admin.pas.bll.ClientBll.getNextPendingActivityExecutorBll(ClientBll.java:305)
    at java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:628)
Caused by: com.microsoft.sqlserver.jdbc.SQLServerException: deadlock victim
    ... 65 more`

// javaNullPointerTrace exercises Native and Unknown Source frame variants
// alongside a regular caused-by chain. Used as the no-resolver showcase.
const javaNullPointerTrace = `java.lang.NullPointerException: Cannot invoke "String.length()" because "s" is null
    at com.example.app.UserService.validate(UserService.java:88)
    at com.example.app.UserController.register(UserController.java:42)
    at sun.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
    at sun.reflect.NativeMethodAccessorImpl.invoke(Unknown Source)
    at java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1149)
Caused by: java.lang.IllegalStateException: missing username
    at com.example.app.RequestParser.parse(RequestParser.java:17)`

// fakeJavaSourceResolver returns synthetic source lines for com.example.* frames
// so the rendered trace shows the focal-line highlight without needing on-disk
// Java sources. Production code would point at a real source tree or decompiler.
func fakeJavaSourceResolver() clicky.SourceResolverFunc {
	snippets := map[string][]string{
		"com.example.admin.pas.dal.ActivityDal": {
			"public Activity findNextPendingActivity() {",
			"    Query q = em.createNamedQuery(\"Activity.findNextPending\");",
			"    q.setLockMode(LockModeType.PESSIMISTIC_WRITE);",
			"    return q.getSingleResult();",
			"}",
		},
		"com.example.admin.pas.bll.ClientBll": {
			"public Executor getNextPendingActivityExecutorBll() {",
			"    Activity next = activityDal.findNextPendingActivity();",
			"    return executorFactory.create(next);",
			"}",
		},
	}
	return func(_ context.Context, frame api.StackFrame, _ int) ([]string, int, string, bool) {
		lines, ok := snippets[frame.Class]
		if !ok {
			return nil, 0, "", false
		}
		// Place the focal line in the middle of the snippet for a visible highlight.
		startLine := frame.Line - len(lines)/2
		if startLine < 1 {
			startLine = 1
		}
		return lines, startLine, "java", true
	}
}

// createSampleStackTrace parses the fixture and attaches a fake source resolver.
func createSampleStackTrace() api.StackTrace {
	return clicky.StackTrace(javaSampleTrace,
		clicky.WithStackInclude("com.example."),
		clicky.WithStackContext(2),
		clicky.WithSourceResolver(fakeJavaSourceResolver()),
	)
}

// createIconsShowcase creates a comprehensive showcase of all available icons
func createIconsShowcase() (items []IconShowcase) {
	for k, v := range icons.All {
		items = append(items, IconShowcase{
			Icon: api.Text{}.Add(v.WithStyle("text-xl text-green-500")),
			Name: k,
		})
	}
	return items
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
	sampleText := "The Quick Brown FOX Jumps Over The-Lazy-Dog"

	examples := []TextStyleExample{}
	for _, style := range []string{
		"uppercase", "lowercase", "capitalize", "normal-case",
		"text-left", "text-center", "text-right", "text-justify",
		"font-thin", "font-light", "font-normal", "font-medium", "font-semibold", "font-bold",
		"underline", "line-through", "italic",
		"opacity-25", "opacity-50", "opacity-75",
		"uppercase font-bold text-green-600 underline",
		"lowercase italic text-purple-700 opacity-75",
		"max-w-[5ch] truncate",
		"max-w-[5ch]",
		"max-w-[5]",
		"max-w-[5] truncate-prefix",
		"max-w-[5] truncate-suffix",
		"max-w-[5ch] truncate", "max-w-[5ch] text-ellipsis", "max-w-[5ch] text-clip",
		"text-xs", "text-sm", "text-base", "text-lg", "text-xl", "text-2xl", "text-3xl",
	} {
		examples = append(examples, TextStyleExample{
			StyleName: api.Text{Content: style},
			Example:   api.Text{Content: sampleText, Style: style},
		})
	}
	return examples

}

// NilHandlingExample demonstrates how zero/nil/empty values are rendered
type NilHandlingExample struct {
	Label       string     `json:"label" pretty:"label=Field"`
	UUID        uuid.UUID  `json:"uuid" pretty:"label=uuid.UUID"`
	UUIDPtr     *uuid.UUID `json:"uuid_ptr,omitempty" pretty:"label=*uuid.UUID,omitempty"`
	TimeVal     time.Time  `json:"time_val" pretty:"label=time.Time"`
	TimePtr     *time.Time `json:"time_ptr,omitempty" pretty:"label=*time.Time,omitempty"`
	DateStr     string     `json:"date_str" pretty:"label=Date String,format=date"`
	DateInt     int64      `json:"date_int" pretty:"label=Date Unix,format=date"`
	StringPtr   *string    `json:"string_ptr,omitempty" pretty:"label=*string,omitempty"`
	IntPtr      *int       `json:"int_ptr,omitempty" pretty:"label=*int,omitempty"`
	EmptyStruct zeroImpl   `json:"empty_struct" pretty:"label=IsZero() struct"`
	NilStruct   nilImpl    `json:"nil_struct" pretty:"label=IsNil() struct"`
}

type zeroImpl struct{ zero bool }

func (z zeroImpl) IsZero() bool { return z.zero }

type nilImpl struct{ isNil bool }

func (n nilImpl) IsNil() bool { return n.isNil }

func createNilHandlingShowcase() []NilHandlingExample {
	now := time.Now()
	id := uuid.New()
	return []NilHandlingExample{
		{
			Label:       "All populated",
			UUID:        id,
			UUIDPtr:     uuidPtr(id),
			TimeVal:     now,
			TimePtr:     timePtr(now),
			DateStr:     now.Format(time.RFC3339),
			DateInt:     now.Unix(),
			StringPtr:   stringPtr("world"),
			IntPtr:      intPtr(99),
			EmptyStruct: zeroImpl{zero: false},
			NilStruct:   nilImpl{isNil: false},
		},
		{
			Label:       "All zero/nil/empty",
			UUID:        uuid.UUID{},
			UUIDPtr:     nil,
			TimeVal:     time.Time{},
			TimePtr:     nil,
			DateStr:     "",
			DateInt:     0,
			StringPtr:   nil,
			IntPtr:      nil,
			EmptyStruct: zeroImpl{zero: true},
			NilStruct:   nilImpl{isNil: true},
		},
		{
			Label:       "Mixed",
			UUID:        id,
			UUIDPtr:     nil,
			TimeVal:     now,
			TimePtr:     nil,
			DateStr:     now.Format(time.RFC3339),
			DateInt:     0,
			StringPtr:   nil,
			IntPtr:      nil,
			EmptyStruct: zeroImpl{zero: true},
			NilStruct:   nilImpl{isNil: false},
		},
	}
}

// NilHandlingOptions for showing nil handling showcase
type NilHandlingOptions struct{}

// AllOptions are options for showcase commands
type AllOptions struct {
	IncludeIcons  bool `flag:"icons" help:"Include icons showcase" default:"true"`
	IncludeColors bool `flag:"colors" help:"Include colors showcase" default:"true"`
	IncludeStyles bool `flag:"styles" help:"Include text styles showcase" default:"true"`
	IncludeTypes  bool `flag:"types" help:"Include data types showcase" default:"true"`
}

// IconsOptions for showing just icons
type IconsOptions struct{}

// ColorsOptions for showing just colors
type ColorsOptions struct{}

// StylesOptions for showing just text styles
type StylesOptions struct{}

// TypesOptions for showing just data types
type TypesOptions struct{}

// TablesOptions for showing just table examples
type TablesOptions struct{}

// TasksOptions for showing task progress tracking
type TasksOptions struct {
	Scenario string        `flag:"scenario" help:"Task scenario to run (basic, concurrent, dependencies, all)" default:"basic"`
	NumTasks int           `flag:"tasks" help:"Number of tasks for concurrent scenario" default:"5"`
	NoSleep  bool          `flag:"no-sleep" help:"Skip simulated delays" default:"false"`
	Timeout  time.Duration `flag:"timeout" help:"Overall timeout for all tasks" default:"0"`
}

// TaskPromptOptions configures the interactive task/prompt demo.
type TaskPromptOptions struct {
	NoSleep bool `flag:"no-sleep" help:"Skip simulated delays before the prompt appears" default:"false"`
	Count   int  `flag:"count" help:"Number of options to generate for the select prompts" default:"8"`
}

type TaskPromptResult struct {
	SelectedAction string   `json:"selected_action" pretty:"label=Selected Action"`
	SelectedChecks []string `json:"selected_checks,omitempty" pretty:"label=Selected Checks,omitempty"`
	ChangeTicket   string   `json:"change_ticket,omitempty" pretty:"label=Change Ticket,omitempty"`
	PromptStatus   string   `json:"prompt_status" pretty:"label=Prompt Status"`
	OptionCount    int      `json:"option_count" pretty:"label=Option Count"`
	TerminalNote   string   `json:"terminal_note" pretty:"label=Terminal Handoff"`
}

// ProductTable demonstrates basic table formatting
type ProductTable struct {
	ID          int     `json:"id" pretty:"label=ID"`
	Name        string  `json:"name" pretty:"label=Product Name"`
	Category    string  `json:"category" pretty:"label=Category"`
	Description string  `json:"description" pretty:"label=Description"`
	Price       float64 `json:"price" pretty:"label=Price,format=currency"`
	InStock     bool    `json:"in_stock" pretty:"label=In Stock"`
	Rating      float64 `json:"rating" pretty:"label=Rating"`
}

// EmployeeTable demonstrates the new TableProvider interface with builder-style columns
type EmployeeTable struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Department string  `json:"department"`
	Salary     float64 `json:"salary"`
	HireDate   string  `json:"hire_date"`
	Active     bool    `json:"active"`
}

// Columns implements api.TableProvider - defines column schema in display order
func (EmployeeTable) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("id").Label("ID").Build(),
		api.Column("name").Label("Name").Style("font-semibold").Build(),
		api.Column("department").Label("Department").Build(),
		api.Column("salary").Label("Salary").Format("currency").Style("text-green-600").Build(),
		api.Column("hire_date").Label("Hire Date").Build(),
		api.Column("status").Label("Status").Build(),
	}
}

// Rows implements api.TableProvider - returns raw data for this item
func (e EmployeeTable) Row() map[string]any {
	statusIcon := icons.Cross.WithStyle("text-red-500")
	statusText := "Inactive"
	statusStyle := "text-red-600"
	if e.Active {
		statusIcon = icons.Check.WithStyle("text-green-500")
		statusText = "Active"
		statusStyle = "text-green-600"
	}

	return map[string]any{
		"id":         e.ID,
		"name":       e.Name,
		"department": api.Text{}.Add(icons.Folder.WithStyle("text-blue-500")).Add(api.Text{Content: e.Department}),
		"salary":     e.Salary,
		"hire_date":  e.HireDate,
		"status":     api.Text{}.Add(statusIcon).Add(api.Text{Content: statusText, Style: statusStyle}),
	}
}

// SalesTable demonstrates table with various formatted fields
type SalesTable struct {
	OrderID   int     `json:"order_id" pretty:"label=Order ID"`
	Customer  string  `json:"customer" pretty:"label=Customer"`
	Product   string  `json:"product" pretty:"label=Product"`
	Quantity  int     `json:"quantity" pretty:"label=Qty"`
	UnitPrice float64 `json:"unit_price" pretty:"label=Unit Price,format=currency"`
	Total     float64 `json:"total" pretty:"label=Total,format=currency"`
	OrderDate string  `json:"order_date" pretty:"label=Order Date,format=date"`
	Status    string  `json:"status" pretty:"label=Status"`
}

// showAll displays all showcases
func showAll(opts AllOptions) (any, error) {
	demo := createDemoData()

	if !opts.IncludeIcons {
		demo.IconsTable = nil
	}
	if !opts.IncludeColors {
		demo.ColorsTable = nil
	}
	if !opts.IncludeStyles {
		demo.TextStylesTable = nil
	}
	if !opts.IncludeTypes {
		// Clear all type demo fields except FileSystem (which is now a tree demo)
		demo.StringField = ""
		demo.IntField = 0
		demo.Orders = nil
	}

	return demo, nil
}

// showIcons displays icon showcase
func showIcons(opts IconsOptions) (any, error) {
	return createIconsShowcase(), nil
}

// showColors displays color showcase
func showColors(opts ColorsOptions) (any, error) {
	return createColorsShowcase(), nil
}

// showStyles displays text styles showcase
func showStyles(opts StylesOptions) (any, error) {
	return createTextStylesShowcase(), nil
}

// showTypes displays data types showcase
func showTypes(opts TypesOptions) (any, error) {
	demo := createDemoData()
	// Clear showcases, keep only type demos
	demo.IconsTable = nil
	demo.ColorsTable = nil
	demo.TextStylesTable = nil
	return demo, nil
}

// showTables displays various table examples
func showTables(opts TablesOptions) (any, error) {
	now := time.Now()

	products := []ProductTable{
		{ID: 1, Name: "Laptop Pro 15", Category: "Electronics", Price: 1299.99, InStock: true, Rating: 4.8},
		{ID: 2, Name: "Wireless Mouse", Category: "Accessories", Price: 29.99, InStock: true, Rating: 4.5,
			Description: "Format: USB\n Color: Orange",
		},
		{ID: 3, Name: "USB-C Hub", Category: "Accessories", Price: 49.99, InStock: false, Rating: 4.2},
		{ID: 4, Name: "Monitor 27\"", Category: "Electronics", Price: 349.99, InStock: true, Rating: 4.7},
		{ID: 5, Name: "Mechanical Keyboard", Category: "Accessories", Price: 149.99, InStock: true, Rating: 4.9},
	}

	employees := []EmployeeTable{
		{ID: 101, Name: "Alice Johnson", Department: "Engineering", Salary: 95000.00, HireDate: "2020-03-15", Active: true},
		{ID: 102, Name: "Bob Smith", Department: "Sales", Salary: 75000.00, HireDate: "2019-07-22", Active: true},
		{ID: 103, Name: "Carol Williams", Department: "Marketing", Salary: 68000.00, HireDate: "2021-01-10", Active: false},
		{ID: 104, Name: "David Brown", Department: "Engineering", Salary: 102000.00, HireDate: "2018-11-05", Active: true},
		{ID: 105, Name: "Eve Davis", Department: "HR", Salary: 62000.00, HireDate: "2022-06-18", Active: true},
	}

	sales := []SalesTable{
		{
			OrderID:   1001,
			Customer:  "Acme Corp",
			Product:   "Widget Pro",
			Quantity:  50,
			UnitPrice: 29.99,
			Total:     1499.50,
			OrderDate: now.AddDate(0, 0, -5).Format(time.RFC3339),
			Status:    "Shipped",
		},
		{
			OrderID:   1002,
			Customer:  "Tech Solutions Inc",
			Product:   "Gadget Plus",
			Quantity:  25,
			UnitPrice: 49.99,
			Total:     1249.75,
			OrderDate: now.AddDate(0, 0, -3).Format(time.RFC3339),
			Status:    "Processing",
		},
		{
			OrderID:   1003,
			Customer:  "Global Industries",
			Product:   "Tool Master",
			Quantity:  100,
			UnitPrice: 19.99,
			Total:     1999.00,
			OrderDate: now.AddDate(0, 0, -1).Format(time.RFC3339),
			Status:    "Delivered",
		},
	}

	result := struct {
		Products  []ProductTable  `json:"products" pretty:"label=Product Catalog,format=table"`
		Employees []EmployeeTable `json:"employees" pretty:"label=Employee Directory,format=table"`
		Sales     []SalesTable    `json:"sales" pretty:"label=Recent Sales,format=table"`
	}{
		Products:  products,
		Employees: employees,
		Sales:     sales,
	}

	return result, nil
}

// TableProviderOptions for demonstrating the new TableProvider interface
type TableProviderOptions struct {
	Single bool `flag:"single" help:"Show single item table instead of slice"`
}

// showTableProvider demonstrates the new TableProvider interface with NewTableFrom
func showTableProvider(opts TableProviderOptions) (any, error) {
	employees := []EmployeeTable{
		{ID: 101, Name: "Alice Johnson", Department: "Engineering", Salary: 95000, HireDate: "2020-03-15", Active: true},
		{ID: 102, Name: "Bob Smith", Department: "Sales", Salary: 75000, HireDate: "2019-07-22", Active: true},
		{ID: 103, Name: "Carol Williams", Department: "Marketing", Salary: 68000, HireDate: "2021-01-10", Active: false},
	}

	if opts.Single {
		// Demonstrate single item as table
		return api.NewTableFrom([]EmployeeTable{employees[0]}), nil
	}

	// Demonstrate slice as table
	return api.NewTableFrom(employees), nil
}

// showTrees displays tree structure examples
func showTrees(opts clicky.FileTreeOptions) (any, error) {
	return clicky.NewFileSystem(".", clicky.WithMaxDepth(opts.MaxDepth)), nil
}

// showTasks displays task progress tracking examples
func showTasks(opts TasksOptions) (any, error) {
	sleep := func(d time.Duration) {
		if !opts.NoSleep {
			time.Sleep(d)
		}
	}

	var taskOpts []task.Option
	if opts.Timeout > 0 {
		taskOpts = append(taskOpts, task.WithTimeout(opts.Timeout))
	}

	fmt.Fprintln(os.Stderr, "[stderr] starting")
	fmt.Fprintln(os.Stdout, "[stdout] starting")

	switch opts.Scenario {
	case "basic":
		runBasicTasks(sleep, taskOpts...)
	case "concurrent":
		runConcurrentTasks(opts.NumTasks, sleep, taskOpts...)
	case "dependencies":
		runDependencyTasks(sleep, taskOpts...)
	case "all":
		runBasicTasks(sleep, taskOpts...)
		runConcurrentTasks(opts.NumTasks, sleep, taskOpts...)
		runDependencyTasks(sleep, taskOpts...)
	default:
		runBasicTasks(sleep, taskOpts...)
	}

	task.Wait()

	fmt.Fprintln(os.Stderr, "[stderr] stopping")
	fmt.Fprintln(os.Stdout, "[stdout] stopping")

	return nil, nil
}

func showTaskPrompt(opts TaskPromptOptions) (any, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil, fmt.Errorf("task-prompt requires an interactive terminal")
	}

	optionCount := max(opts.Count, 3)
	actions := buildRolloutActions(optionCount)
	checks := buildValidationChecks(optionCount)

	sleep := func(d time.Duration) {
		if !opts.NoSleep {
			time.Sleep(d)
		}
	}

	releaseWatch := make(chan struct{})

	task.StartTask("Collect rollout snapshot", func(ctx flanksourceContext.Context, t *task.Task) (map[string]any, error) {
		steps := []string{
			"Inspecting cluster state",
			"Checking canary pods",
			"Verifying recent error rates",
		}
		for i, step := range steps {
			sleep(180 * time.Millisecond)
			t.SetProgress(i+1, len(steps))
			t.Infof("%s", step)
		}
		t.Success()
		return map[string]any{"region": "us-east-1", "healthy_pods": 12}, nil
	})

	task.StartTask("Watch operator gate", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		stages := []string{
			"Streaming canary metrics",
			"Tailing ingress logs",
			"Waiting for operator prompt",
		}
		for i, stage := range stages {
			sleep(150 * time.Millisecond)
			t.SetDescription(stage)
			t.SetProgress(i+1, len(stages))
		}

		<-releaseWatch
		t.SetDescription("Operator responded")
		t.Success()
		return "gate released", nil
	})

	sleep(350 * time.Millisecond)

	result := TaskPromptResult{
		SelectedAction: "operator cancelled",
		PromptStatus:   "selection cancelled",
		OptionCount:    optionCount,
		TerminalNote:   "Task rendering stops before prompt UI takes over the ANSI terminal.",
	}

	action, ok := clicky.PromptSelect(actions, clicky.PromptSelectOptions[string]{
		Title: "Choose the next rollout action",
	})

	if ok {
		result.SelectedAction = action
		result.PromptStatus = "action selected"

		selectedChecks, checksOK := clicky.PromptMultiSelect(checks, clicky.PromptMultiSelectOptions[string]{
			Title:   "Choose the rollout checks to keep running",
			Limit:   min(4, len(checks)),
			Ordered: true,
		})

		if checksOK {
			result.SelectedChecks = selectedChecks
			result.PromptStatus = "checks selected"

			ticket, ticketOK := clicky.PromptText(clicky.PromptTextOptions{
				Title:       "Enter the change ticket",
				Placeholder: "e.g. REL-1234",
				Validate: func(value string) error {
					if strings.TrimSpace(value) == "" {
						return fmt.Errorf("change ticket is required")
					}
					return nil
				},
			})
			if ticketOK {
				result.ChangeTicket = ticket
				result.PromptStatus = "confirmed"
			} else {
				result.PromptStatus = "ticket entry cancelled"
			}
		} else {
			result.PromptStatus = "check selection cancelled"
		}
	}

	close(releaseWatch)
	task.Wait()

	return result, nil
}

func buildRolloutActions(count int) []string {
	base := []string{
		"Promote canary",
		"Hold rollout",
		"Abort release",
		"Roll back deploy",
		"Pause traffic shift",
		"Scale canary up",
		"Scale canary down",
		"Open incident bridge",
		"Notify API owners",
		"Trigger smoke suite",
	}

	return buildPromptOptions(base, count, "Action")
}

func buildValidationChecks(count int) []string {
	base := []string{
		"Tail ingress error logs",
		"Watch saturation dashboard",
		"Run synthetic checkout probe",
		"Verify canary pod restarts",
		"Inspect downstream queue depth",
		"Track p95 latency panel",
		"Confirm feature flag rollout",
		"Ping on-call database owner",
		"Check payment gateway health",
		"Review deployment event stream",
	}

	return buildPromptOptions(base, count, "Check")
}

func buildPromptOptions(base []string, count int, prefix string) []string {
	options := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if i < len(base) {
			options = append(options, base[i])
			continue
		}
		options = append(options, fmt.Sprintf("%s %02d", prefix, i+1))
	}
	return options
}

func runBasicTasks(sleep func(time.Duration), opts ...task.Option) {
	// Task that succeeds and returns a string result
	task.StartTask("Download dependencies", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		t.Tracef("Resolving dependency graph")
		t.Debugf("Cache directory: /tmp/deps-cache")
		t.Infof("Starting dependency download")
		for i := 1; i <= 5; i++ {
			sleep(150 * time.Millisecond)
			t.SetProgress(i, 5)
			t.Infof("Downloaded package %d of 5", i)
		}
		t.Tracef("Verifying checksums")
		t.Debugf("All checksums valid")
		t.Success()
		return "5 packages downloaded", nil
	}, opts...)

	// Task that ends with a warning and returns a map
	task.StartTask("Build project", func(ctx flanksourceContext.Context, t *task.Task) (map[string]any, error) {
		steps := []string{"Parsing", "Type checking", "Compiling", "Linking"}
		for i, step := range steps {
			sleep(200 * time.Millisecond)
			t.SetProgress(i+1, len(steps))
			t.Debugf("Step %d: %s", i+1, step)
			t.Infof("%s...", step)
		}
		t.Warnf("Found 2 deprecated API calls")
		t.Warnf("Build artifact is 15%% larger than previous")
		t.Warning()
		return map[string]any{"warnings": 2, "artifact_size": "48MB"}, nil
	}, opts...)

	// Task that succeeds and returns an int
	task.StartTask("Run tests", func(ctx flanksourceContext.Context, t *task.Task) (int, error) {
		t.Debugf("Discovering test files")
		for i := 1; i <= 8; i++ {
			sleep(100 * time.Millisecond)
			t.SetProgress(i, 8)
			if i == 5 {
				t.Warnf("Test %d was slow (2.3s)", i)
			}
			t.Infof("Test %d passed", i)
		}
		t.Tracef("Generating coverage report")
		t.Success()
		return 8, nil
	}, opts...)

	// Task that fails with an error
	task.StartTask("Publish artifacts", func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
		t.Infof("Authenticating with registry")
		sleep(200 * time.Millisecond)
		t.SetProgress(1, 3)
		t.Infof("Uploading artifact...")
		sleep(300 * time.Millisecond)
		t.SetProgress(2, 3)
		t.Errorf("Connection reset by peer")
		t.Warnf("Retrying upload...")
		sleep(200 * time.Millisecond)
		t.Errorf("Registry returned 503 Service Unavailable")
		return t.FailedWithError(fmt.Errorf("failed to publish after 2 attempts"))
	}, opts...)
}

func runConcurrentTasks(numTasks int, sleep func(time.Duration), opts ...task.Option) {
	for i := 1; i <= numTasks; i++ {
		taskNum := i
		task.StartTask(fmt.Sprintf("Process batch %d", taskNum), func(ctx flanksourceContext.Context, t *task.Task) ([]string, error) {
			items := 10
			processed := make([]string, 0, items)
			for j := 1; j <= items; j++ {
				sleep(50 * time.Millisecond)
				t.SetProgress(j, items)
				t.Tracef("Processing item %d/%d", j, items)
				if j%3 == 0 {
					t.Debugf("Checkpoint at item %d", j)
				}
				processed = append(processed, fmt.Sprintf("item-%d", j))
			}
			t.Infof("Processed %d items", items)
			if taskNum%3 == 0 {
				t.Warnf("Batch %d had slow items", taskNum)
				t.Warning()
			} else {
				t.Success()
			}
			return processed, nil
		}, opts...)
	}
}

func runDependencyTasks(sleep func(time.Duration), opts ...task.Option) {
	setup := task.StartTask("Setup environment", func(ctx flanksourceContext.Context, t *task.Task) (bool, error) {
		t.Debugf("Checking prerequisites")
		t.Infof("Installing toolchain")
		sleep(200 * time.Millisecond)
		t.Infof("Configuring network")
		sleep(100 * time.Millisecond)
		t.Infof("Environment ready")
		t.Success()
		return true, nil
	}, opts...)

	build := task.StartTask("Build application", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		for i := 1; i <= 4; i++ {
			sleep(150 * time.Millisecond)
			t.SetProgress(i, 4)
			t.Debugf("Compiling module %d", i)
		}
		t.Tracef("Binary size: 24MB")
		t.Success()
		return "app-v1.2.3", nil
	}, append(opts, task.WithDependencies(setup.GetTask()))...)

	task.StartTask("Deploy application", func(ctx flanksourceContext.Context, t *task.Task) (map[string]string, error) {
		stages := []string{"Upload", "Configure", "Start", "Verify"}
		for i, stage := range stages {
			sleep(200 * time.Millisecond)
			t.SetProgress(i+1, len(stages))
			t.Infof("%s complete", stage)
			if stage == "Configure" {
				t.Debugf("Applied 3 config patches")
			}
		}
		t.Success()
		return map[string]string{"url": "https://app.example.com", "version": "v1.2.3"}, nil
	}, append(opts, task.WithDependencies(build.GetTask()))...)
}

func showNilHandling(opts NilHandlingOptions) (any, error) {
	return createNilHandlingShowcase(), nil
}

// StackTraceOptions for the stack-trace subcommand.
type StackTraceOptions struct {
	Context int `flag:"context" help:"Source context lines per frame" default:"3"`
	Max     int `flag:"max-frames" help:"Maximum frames to render (0 = all)" default:"0"`
}

// StackTraceShowcase renders multiple traces side by side so the demo covers
// resolved vs. unresolved frames and runtime/native frame variants.
type StackTraceShowcase struct {
	WithSource    api.StackTrace `json:"with_source" pretty:"label=With Source Resolution"`
	WithoutSource api.StackTrace `json:"without_source" pretty:"label=Headers Only (no resolver)"`
	NativeFrames  api.StackTrace `json:"native_frames" pretty:"label=Native + Unknown Source frames"`
}

func showStackTrace(opts StackTraceOptions) (any, error) {
	resolvedOpts := []api.StackTraceOption{
		clicky.WithStackInclude("com.example."),
		clicky.WithStackContext(opts.Context),
		clicky.WithSourceResolver(fakeJavaSourceResolver()),
	}
	if opts.Max > 0 {
		resolvedOpts = append(resolvedOpts, clicky.WithMaxStackFrames(opts.Max))
	}

	headersOnlyOpts := []api.StackTraceOption{clicky.WithStackInclude("com.example.")}
	if opts.Max > 0 {
		headersOnlyOpts = append(headersOnlyOpts, clicky.WithMaxStackFrames(opts.Max))
	}

	return StackTraceShowcase{
		WithSource:    clicky.StackTrace(javaSampleTrace, resolvedOpts...),
		WithoutSource: clicky.StackTrace(javaNullPointerTrace, headersOnlyOpts...),
		NativeFrames:  clicky.StackTrace(javaNullPointerTrace, headersOnlyOpts...),
	}, nil
}

// ComponentsOptions for the components showcase command
type ComponentsOptions struct{}

type ComponentsShowcase struct {
	CodeBlocks     api.TextList `json:"-" pretty:"label=Code Blocks,format=list"`
	CollapsedItems api.TextList `json:"-" pretty:"label=Collapsed Sections,format=list"`
	Badges         api.TextList `json:"-" pretty:"label=Badges,format=list"`
	Buttons        api.TextList `json:"-" pretty:"label=Buttons,format=list"`
}

func showComponents(_ ComponentsOptions) (any, error) {
	return ComponentsShowcase{
		CodeBlocks: api.TextList{
			api.CodeBlock("go", "func main() {\n\tfmt.Println(\"Hello, World!\")\n}"),
			api.CodeBlock("sql", "SELECT u.name, COUNT(o.id) AS orders\nFROM users u\nLEFT JOIN orders o ON o.user_id = u.id\nWHERE u.active = true\nGROUP BY u.name\nORDER BY orders DESC\nLIMIT 10;"),
			api.CodeBlock("json", "{\n  \"name\": \"clicky\",\n  \"version\": \"1.0.0\",\n  \"features\": [\"pretty\", \"html\", \"markdown\"]\n}"),
			api.CodeBlock("yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: mission-control\nspec:\n  replicas: 3"),
		},
		CollapsedItems: api.TextList{
			clicky.Collapsed("Database Configuration", clicky.Map(map[string]string{"host": "db.example.com", "port": "5432", "database": "production", "ssl": "required"})),
			clicky.Collapsed("Build Output", api.CodeBlock("bash", "$ go build ./...\n$ go test ./... -count=1\nok  github.com/flanksource/clicky/api  0.42s\nPASS")),
			clicky.Collapsed("Error Details", api.Text{Content: "Connection refused: dial tcp 10.0.0.1:5432\nRetried 3 times with exponential backoff", Style: "text-red-600"}),
		},
		Badges: api.TextList{
			api.Text{}.Add(api.Badge("Healthy", "bg-green-100 text-green-800")).Add(api.Badge("Warning", "bg-yellow-100 text-yellow-800")).Add(api.Badge("Critical", "bg-red-100 text-red-800")).Add(api.Badge("Unknown", "bg-gray-100 text-gray-800")),
			api.Text{}.Add(api.Badge("v1.2.3", "bg-blue-100 text-blue-800")).Add(api.Badge("stable", "bg-green-100 text-green-800")).Add(api.Badge("amd64", "bg-purple-100 text-purple-800")),
		},
		Buttons: api.TextList{
			api.ButtonGroup{Buttons: []api.Button{
				{Label: "View Docs", Href: "https://docs.flanksource.com"},
				{Label: "Open Dashboard", Href: "https://app.flanksource.com"},
				{Label: "Report Issue", Href: "https://github.com/flanksource/clicky/issues"},
			}},
		},
	}, nil
}

type DetailEmployee struct {
	ID, Name, Department, Email, Notes string
}

func (DetailEmployee) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("id").Label("ID").Build(),
		api.Column("name").Label("Name").Style("font-semibold").Build(),
		api.Column("department").Label("Department").Build(),
		api.Column("email").Label("Email").Build(),
	}
}

func (e DetailEmployee) Row() map[string]any {
	return map[string]any{
		"id": e.ID, "name": e.Name,
		"department": api.Text{}.Add(icons.Folder.WithStyle("text-blue-500")).Add(api.Text{Content: e.Department}),
		"email":      e.Email,
	}
}

func (e DetailEmployee) RowDetail() api.Textable {
	if e.Notes == "" {
		return nil
	}
	return api.Text{Content: e.Notes, Style: "text-gray-600 italic"}
}

type CollapsibleRowOptions struct{}

func showCollapsibleRows(_ CollapsibleRowOptions) (any, error) {
	return api.NewTableFrom([]DetailEmployee{
		{ID: "1", Name: "Alice Johnson", Department: "Engineering", Email: "alice@example.com", Notes: "Tech lead for the platform team. On-call this week."},
		{ID: "2", Name: "Bob Smith", Department: "Sales", Email: "bob@example.com"},
		{ID: "3", Name: "Carol Williams", Department: "Marketing", Email: "carol@example.com", Notes: "Working on Q2 launch campaign."},
		{ID: "4", Name: "David Brown", Department: "Engineering", Email: "david@example.com"},
		{ID: "5", Name: "Eve Davis", Department: "HR", Email: "eve@example.com", Notes: "Coordinating annual review cycle."},
	}), nil
}

type SparseTable struct{ Name, Version, Deprecated, License, Description string }

func (SparseTable) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("name").Label("Package").Style("font-semibold").Build(),
		api.Column("version").Label("Version").Build(),
		api.Column("deprecated").Label("Deprecated").Build(),
		api.Column("license").Label("License").Build(),
		api.Column("description").Label("Description").MaxWidth(40).Build(),
	}
}

func (s SparseTable) Row() map[string]any {
	return map[string]any{"name": s.Name, "version": s.Version, "deprecated": s.Deprecated, "license": s.License, "description": s.Description}
}

type ColumnBuilderOptions struct{}

func showColumnBuilder(_ ColumnBuilderOptions) (any, error) {
	return api.NewTableFrom([]SparseTable{
		{Name: "react", Version: "18.3.1", License: "MIT", Description: "A JavaScript library for building user interfaces"},
		{Name: "lodash", Version: "4.17.21", License: "MIT", Description: "A modern JavaScript utility library"},
		{Name: "express", Version: "4.18.2", License: "MIT", Description: "Fast, unopinionated, minimalist web framework"},
		{Name: "moment", Version: "2.29.4", License: "MIT", Description: "Parse, validate, manipulate, and display dates"},
		{Name: "webpack", Version: "5.88.0", License: "MIT", Description: "A bundler for JavaScript and friends"},
	}), nil
}

type TaskUIOptions struct {
	Port     int    `flag:"port" help:"HTTP server port" default:"8080"`
	NumTasks int    `flag:"tasks" help:"Number of concurrent tasks for batch group" default:"5"`
	Count    int    `flag:"count" help:"Total number of tasks to generate (overrides default groups)" default:"0"`
	Spread   string `flag:"spread" help:"Spread task creation over this duration (e.g. 30s, 1m)" default:""`
}

func showTaskUI(opts TaskUIOptions) (any, error) {
	task.SetNoProgress(true)
	var spread time.Duration
	if opts.Spread != "" {
		if d, err := time.ParseDuration(opts.Spread); err == nil {
			spread = d
		}
	}
	go func() {
		time.Sleep(1 * time.Second)
		if opts.Count > 0 {
			runScaleTasks(opts.Count, spread)
		} else {
			runGroupedTasks(opts.NumTasks, func(d time.Duration) { time.Sleep(d * 3) })
		}
	}()

	handler := taskui.Handler()

	addr := fmt.Sprintf(":%d", opts.Port)
	clicky.Infof("Task UI at http://localhost%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
	return nil, nil
}

func runGroupedTasks(numTasks int, sleep func(time.Duration), _ ...task.Option) {
	build := task.StartGroup[string]("Build Pipeline")
	build.Add("Download dependencies", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		for i := 1; i <= 5; i++ {
			sleep(150 * time.Millisecond)
			t.SetProgress(i, 5)
			t.Infof("Downloaded package %d of 5", i)
		}
		t.Success()
		return "done", nil
	})
	build.Add("Build project", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		for i, step := range []string{"Parsing", "Type checking", "Compiling", "Linking"} {
			sleep(200 * time.Millisecond)
			t.SetProgress(i+1, 4)
			t.Infof("%s...", step)
		}
		t.Warnf("Found 2 deprecated API calls")
		t.Warning()
		return "built with warnings", nil
	})
	build.Add("Run tests", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		for i := 1; i <= 8; i++ {
			sleep(100 * time.Millisecond)
			t.SetProgress(i, 8)
			t.Infof("Test %d passed", i)
		}
		t.Success()
		return "8 tests passed", nil
	})
	build.Add("Publish artifacts", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		t.Infof("Uploading...")
		sleep(300 * time.Millisecond)
		t.SetProgress(1, 3)
		sleep(300 * time.Millisecond)
		t.Errorf("Connection reset by peer")
		t.SetProgress(2, 3)
		sleep(200 * time.Millisecond)
		t.Errorf("Registry returned 503")
		t.FailedWithError(fmt.Errorf("publish failed"))
		return "", fmt.Errorf("publish failed")
	})

	batch := task.StartGroup[string]("Batch Processing")
	for i := 1; i <= numTasks; i++ {
		n := i
		batch.Add(fmt.Sprintf("Process batch %d", n), func(_ flanksourceContext.Context, t *task.Task) (string, error) {
			for j := 1; j <= 10; j++ {
				sleep(50 * time.Millisecond)
				t.SetProgress(j, 10)
			}
			if n%3 == 0 {
				t.Warnf("Batch %d had slow items", n)
				t.Warning()
			} else {
				t.Success()
			}
			return fmt.Sprintf("batch %d done", n), nil
		})
	}
}

func runScaleTasks(count int, duration time.Duration, _ ...task.Option) {
	batchSize := 50
	if count < 100 {
		batchSize = max(1, count/5)
	}
	numBatches := (count + batchSize - 1) / batchSize

	var delay time.Duration
	if duration > 0 && numBatches > 1 {
		delay = duration / time.Duration(numBatches)
	}

	taskNum := 0
	for b := 0; b < numBatches; b++ {
		groupName := fmt.Sprintf("Batch %d", b+1)
		if numBatches == 1 {
			groupName = "Tasks"
		}
		g := task.StartGroup[string](groupName)
		remaining := min(batchSize, count-taskNum)
		for i := 0; i < remaining; i++ {
			taskNum++
			n := taskNum
			g.Add(fmt.Sprintf("Task %d", n), func(_ flanksourceContext.Context, t *task.Task) (string, error) {
				steps := 3 + n%5
				for s := 1; s <= steps; s++ {
					time.Sleep(time.Duration(50+n%100) * time.Millisecond)
					t.SetProgress(s, steps)
					t.Infof("Step %d/%d", s, steps)
				}
				switch n % 10 {
				case 7:
					t.Warnf("Task %d completed with warnings", n)
					t.Warning()
				case 9:
					t.Errorf("Task %d encountered an error", n)
					t.FailedWithError(fmt.Errorf("task %d failed", n))
					return "", fmt.Errorf("failed")
				default:
					t.Success()
				}
				return fmt.Sprintf("task %d done", n), nil
			})
		}
		if delay > 0 && b < numBatches-1 {
			time.Sleep(delay)
		}
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "uber-demo",
		Short: "Comprehensive demonstration of Clicky formatting capabilities",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			clicky.Flags.UseFlags()
		},
		Long: `Uber Demo showcases all Clicky formatting features including:
- All data types (primitives, pointers, slices, maps, nested structs)
- Icons showcase with 50+ icons
- Tailwind color styles (9 colors × 10 shades)
- Text transformations (uppercase, lowercase, capitalize, etc.)
- Font weights, decorations, and combined styles
- Multiple output formats (pretty, json, yaml, html, markdown, csv, pdf)`,
	}

	clicky.AddCommand(rootCmd, AllOptions{}, showAll)
	clicky.AddCommand(rootCmd, IconsOptions{}, showIcons)
	clicky.AddCommand(rootCmd, ColorsOptions{}, showColors)
	clicky.AddCommand(rootCmd, StylesOptions{}, showStyles)
	clicky.AddCommand(rootCmd, TypesOptions{}, showTypes)
	clicky.AddCommand(rootCmd, TablesOptions{}, showTables)
	clicky.AddNamedCommand("table-provider", rootCmd, TableProviderOptions{}, showTableProvider)
	clicky.AddNamedCommand("trees", rootCmd, clicky.FileTreeOptions{}, showTrees)
	clicky.AddNamedCommand("tasks", rootCmd, TasksOptions{}, showTasks)
	clicky.AddNamedCommand("task-prompt", rootCmd, TaskPromptOptions{}, showTaskPrompt)
	clicky.AddNamedCommand("task-ui", rootCmd, TaskUIOptions{}, showTaskUI)
	clicky.AddNamedCommand("nil-handling", rootCmd, NilHandlingOptions{}, showNilHandling)
	clicky.AddNamedCommand("components", rootCmd, ComponentsOptions{}, showComponents)
	clicky.AddNamedCommand("collapsible-rows", rootCmd, CollapsibleRowOptions{}, showCollapsibleRows)
	clicky.AddNamedCommand("column-builder", rootCmd, ColumnBuilderOptions{}, showColumnBuilder)
	clicky.AddNamedCommand("stack-trace", rootCmd, StackTraceOptions{}, showStackTrace)

	clicky.BindAllFlags(rootCmd.PersistentFlags())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
