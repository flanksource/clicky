# Uber Demo - Comprehensive Clicky Formatting Example

This example demonstrates all supported field types and formatting options in Clicky.

## Overview

The uber demo showcases:
- **Primitive types**: string, int, float, bool (pointer and non-pointer variants)
- **Formatted fields**: currency, dates (RFC3339, Unix timestamps)
- **Slices**: primitives, structs, pointers, mixed nil elements
- **Maps**: simple, nested, with struct/pointer values
- **Nested structures**: single and multi-level nesting
- **Table formatting**: slice of structs rendered as tables
- **Tree structures**: hierarchical data representation
- **Complex combinations**: maps of slices, slices of maps, deeply nested structures

## Usage

### Basic Usage

```bash
# Run with default (pretty) format
go run main.go

# Specify output format
go run main.go --format <format>

# Save to file
go run main.go --format json --output output.json

# Disable colors (pretty format only)
go run main.go --no-color
```

### Subcommands

```bash
go run . all                # full showcase (default)
go run . icons              # icon set
go run . colors             # tailwind colors
go run . styles             # text styles
go run . types              # data type handling
go run . tables             # table examples
go run . table-provider     # TableProvider interface
go run . trees              # filesystem tree
go run . tasks              # task progress
go run . task-prompt        # interactive task prompt
go run . task-ui            # web task UI
go run . nil-handling       # zero/nil rendering
go run . components         # CodeBlock, badges, buttons, collapsibles
go run . collapsible-rows   # row-level detail expansion
go run . column-builder     # column builder API
go run . stack-trace        # parsed Java stack trace with source resolution
```

### Supported Formats

| Format | Flag | Description |
|--------|------|-------------|
| **Pretty** | `--format pretty` | Colorized terminal output with tables (default) |
| **JSON** | `--format json` | JSON with proper indentation |
| **YAML** | `--format yaml` | YAML format |
| **CSV** | `--format csv` | Comma-separated values (flattened) |
| **HTML** | `--format html` | HTML with Tailwind CSS and Grid.js tables |
| **Markdown** | `--format markdown` | Markdown format |
| **PDF** | `--format pdf` | PDF output (requires --output or saves to uber_demo.pdf) |

### Examples

```bash
# Pretty format with colors
go run main.go

# JSON output
go run main.go --format json

# YAML output
go run main.go --format yaml

# CSV output
go run main.go --format csv

# HTML output to file
go run main.go --format html --output demo.html

# Markdown output
go run main.go --format markdown

# Pretty format without colors
go run main.go --no-color

# Save JSON to file
go run main.go --format json --output output.json
```

## Data Structure

### Primitive Types

The demo includes all primitive types in both pointer and non-pointer forms:

```go
type UberDemo struct {
    // Non-pointer primitives
    StringField string
    IntField    int
    FloatField  float64
    BoolField   bool

    // Pointer primitives (can be nil)
    StringPtr *string
    IntPtr    *int
    FloatPtr  *float64
    BoolPtr   *bool

    // Nil pointer examples
    NilString *string
    NilInt    *int
    // ...
}
```

### Formatted Fields

Fields with special formatting using `pretty` tags:

```go
Currency       float64 `pretty:"format=currency"`     // $1234.56
DateRFC3339    string  `pretty:"format=date"`         // 2024-01-15 10:30:00
DateUnixInt    int64   `pretty:"format=date"`         // Converted from timestamp
```

### Slices

Various slice types including primitives, pointers, and structs:

```go
StringSlice    []string              // Simple string slice
IntPtrSlice    []*int                // Pointer slice
MixedNilSlice  []*string             // Mix of values and nils
NestedSlice    []NestedStruct        // Slice of structs
```

### Maps

Different map configurations:

```go
StringMap   map[string]string                      // Simple map
NestedMap   map[string]map[string]interface{}     // Nested maps
StructMap   map[string]NestedStruct               // Map with struct values
PointerMap  map[string]*NestedStruct              // Map with pointer values
NilValueMap map[string]*string                    // Map with nil values
```

### Table Data

Slices formatted as tables using the `format=table` tag:

```go
Orders []TableRow `pretty:"format=table"`
```

Rendered as:
```
┌──────┬─────────────┬──────────┬────────┬──────────┬─────────────────────┐
│ id   │ product     │ quantity │ price  │ subtotal │ order_date          │
├──────┼─────────────┼──────────┼────────┼──────────┼─────────────────────┤
│ 1001 │ Widget Pro  │ 5        │ $29.99 │ $149.95  │ 2024-01-15 10:30:00 │
│ 1002 │ Gadget Plus │ 3        │ $49.99 │ $149.97  │ 2024-01-16 14:20:00 │
│ 1003 │ Tool Master │ 10       │ $19.99 │ $199.90  │ 2024-01-17 09:15:00 │
└──────┴─────────────┴──────────┴────────┴──────────┴─────────────────────┘
```

### Tree Structures

Hierarchical data using the `format=tree` tag:

```go
FileSystem *TreeNode `pretty:"format=tree"`
```

### Nested Structures

Complex nested data:

```go
// Deeply nested map structure
DeepNested map[string]map[string]map[string]interface{}

// Map of slices
CategoryProducts map[string][]string

// Slice of maps
ConfigList []map[string]interface{}
```

## Key Features Demonstrated

1. **Nil Handling**: Proper handling of nil pointers, nil slices, and nil map values
2. **Type Formatting**: Currency ($), dates (various formats), custom rendering
3. **Nested Data**: Multi-level nested maps and structs
4. **Table Formatting**: Automatic table generation from slices
5. **Pointer Safety**: Dereferencing pointers safely across all formatters
6. **Mixed Data**: Combinations of different data types
7. **Empty Values**: Empty slices, empty maps, zero values

## Output Examples

### Pretty Format
Colorized terminal output with proper formatting, tables, and structure preservation.

### JSON Format
Standard JSON with all values serialized appropriately.

### YAML Format
YAML with proper nesting and structure.

### CSV Format
Flattened single-row CSV (for struct) or multi-row (for slice).

### HTML Format
Full HTML page with:
- Tailwind CSS styling
- Grid.js interactive tables
- Iconify icons support
- Responsive layout

### Markdown Format
Markdown with bold labels and structured data.

## Use Cases

This demo is useful for:
- **Testing**: Validating formatter consistency across all formats
- **Documentation**: Understanding supported field types and tags
- **Debugging**: Seeing how different data types are handled
- **Learning**: Reference implementation for using Clicky formatters
- **Development**: Testing new formatter features

## Related Examples

- See `formatters/*_test.go` for unit tests
- See main Clicky documentation for detailed tag reference
