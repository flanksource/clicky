# Clicky Pretty Formatter

A sophisticated struct formatter that uses reflection and struct tags to create beautiful, styled output with lipgloss.

## Features

### 🎨 Struct Tag-Based Formatting
- Parse `pretty` struct tags to control formatting
- Support for various format types: `currency`, `date`, `float`, `color`, `table`
- Conditional coloring based on field values
- Table formatting with sorting capabilities

### 🌈 Lipgloss Integration
- Beautiful styled output using [Charmbracelet Lipgloss](https://github.com/charmbracelet/lipgloss)
- Customizable color themes
- Support for colored and non-colored output
- Professional table rendering with borders

### 🔧 Lenient JSON Parsing
- Parse JSON with comments (`// comment`)
- Handle trailing commas gracefully
- Support quoted JSON strings
- Fallback to string representation for invalid JSON

## Quick Start

```go
package main

import (
    "fmt"
    "clicky"
)

type Invoice struct {
    ID         string        `json:"id"`
    Items      []InvoiceItem `json:"items" pretty:"table,sort=amount,dir=desc"`
    Total      float64       `json:"total" pretty:"currency"`
    CreatedAt  string        `json:"created_at" pretty:"date,format=epoch"`
    CustomerID string        `json:"customer_id"`
    Status     string        `json:"status" pretty:"color,green=paid,red=unpaid,blue=pending"`
}

type InvoiceItem struct {
    ID          string  `json:"id" pretty:"hide"`
    Description string  `json:"description"`
    Amount      float64 `json:"amount" pretty:"currency"`
    Quantity    float64 `json:"quantity" pretty:"float,digits=2"`
    Total       float64 `json:"total" pretty:"currency"`
}

func main() {
    parser := clicky.NewPrettyParser()

    invoice := Invoice{
        ID:         "INV-001",
        Total:      125.50,
        CreatedAt:  "1640995200",
        CustomerID: "CUST-123",
        Status:     "paid",
        Items: []InvoiceItem{
            {
                ID:          "ITEM-001",
                Description: "Web Development",
                Amount:      100.0,
                Quantity:    2.5,
                Total:       250.0,
            },
        },
    }

    result, _ := parser.Parse(invoice)
    fmt.Println(result)
}
```

## Pretty Tag Syntax

### Basic Formats
- `pretty:"currency"` - Format as currency ($123.45)
- `pretty:"date"` - Format as date (2006-01-02 15:04:05)
- `pretty:"date,format=epoch"` - Parse epoch timestamp
- `pretty:"float,digits=2"` - Format float with 2 decimal places
- `pretty:"hide"` - Hide field from output

### Color Formatting
- `pretty:"color,green=paid,red=unpaid"` - Conditional coloring
- `pretty:"color,green=>0,red=<0"` - Numeric conditions
- `pretty:"color,green=>=100,yellow=<100"` - Range conditions

### Table Formatting
- `pretty:"table"` - Render slice as table
- `pretty:"table,sort=amount"` - Sort by field
- `pretty:"table,sort=amount,dir=desc"` - Sort descending

## API Reference

### PrettyParser

```go
type PrettyParser struct {
    Theme   Theme
    NoColor bool
}

// Create new parser with default theme
parser := NewPrettyParser()

// Parse any struct
result, err := parser.Parse(data)

// Disable colors
parser.NoColor = true
```

### FormatManager

```go
fm := NewFormatManager()

// Various output formats
pretty, _ := fm.Pretty(data)
json, _ := fm.JSON(data)
yaml, _ := fm.YAML(data)

// Control styling
fm.SetNoColor(true)
fm.SetTheme(customTheme)
```

### JSON Parsing

```go
// Lenient JSON parsing
data, err := ParseJSON([]byte(`{"name": "test", /* comment */ "value": 42,}`))
```

### Themes

```go
theme := Theme{
    Primary:   lipgloss.Color("#8A2BE2"),
    Secondary: lipgloss.Color("#4169E1"),
    Success:   lipgloss.Color("#32CD32"),
    Warning:   lipgloss.Color("#FFD700"),
    Error:     lipgloss.Color("#FF6347"),
    Info:      lipgloss.Color("#00CED1"),
    Muted:     lipgloss.Color("#808080"),
}

parser.Theme = theme
```

## Examples

### Currency Formatting
```go
type Product struct {
    Name  string  `json:"name"`
    Price float64 `json:"price" pretty:"currency"`
}
```
Output: `Price: $29.99`

### Date Formatting
```go
type Event struct {
    Name string `json:"name"`
    Date string `json:"date" pretty:"date,format=epoch"`
}
```
Output: `Date: 2024-01-01 12:00:00`

### Conditional Colors
```go
type Status struct {
    State  string `json:"state" pretty:"color,green=success,red=error"`
    Score  int    `json:"score" pretty:"color,green=>=80,yellow=>=60,red=<60"`
}
```

### Table with Sorting
```go
type Report struct {
    Items []Item `json:"items" pretty:"table,sort=priority,dir=desc"`
}
```

## Output Examples

### Formatted Invoice
```
id: INV-001
┌─────────────────┬────────┬──────────┬─────────┐
│ description     │ amount │ quantity │ total   │
├─────────────────┼────────┼──────────┼─────────┤
│ Web Development │ $100   │ 2.50     │ $250.00 │
│ Consulting      │ $50    │ 1.00     │ $50.00  │
└─────────────────┴────────┴──────────┴─────────┘
total: $125.50
created_at: 2022-01-01 02:00:00
customer_id: CUST-123
status: paid (green)
```

## Dependencies

- `github.com/charmbracelet/lipgloss` - Terminal styling
- `gopkg.in/yaml.v3` - YAML support
- `github.com/spf13/pflag` - CLI flag support

## Testing

Run the comprehensive test suite:
```bash
go test -v
```

## Contributing

The implementation is designed to be extensible. Key areas for contribution:

1. **New Format Types**: Add support for additional format types
2. **Enhanced Table Features**: Column width control, cell alignment
3. **Advanced Color Logic**: More sophisticated conditional coloring
4. **Export Formats**: HTML, PDF, Excel formatters
5. **Performance**: Optimize reflection usage for large structures

## License

This implementation follows Go best practices and is designed for production use.
