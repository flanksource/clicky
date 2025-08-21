# PDF Widgets with fpdf Integration

This package provides a comprehensive PDF widget system using [go-pdf/fpdf](https://pkg.go.dev/github.com/go-pdf/fpdf) with full support for `api.Class` and Tailwind-based styling.

## Features

### Widget System
- **Text Widget**: Rich text rendering with full styling support
- **Table Widget**: Tables with header/row styling and auto-sizing
- **Box Widget**: Rectangles with positioned labels and borders
- **Image Widget**: Image rendering with placeholder fallback
- **GridLayout**: Grid-based layouts with spanning support

### Styling Support
- **api.Class Integration**: Full support for the normalized Class structure
- **Tailwind Parsing**: Automatic parsing of Tailwind utility classes via `api.ResolveStyles()`
- **Font Properties**: Size, weight, style, decoration
- **Colors**: Foreground and background colors
- **Padding**: All sides independently configurable
- **Borders**: Line styles, widths, and colors

## Quick Start

```go
package main

import (
    "github.com/flanksource/clicky/api"
    "github.com/flanksource/clicky/formatters/pdf"
)

func main() {
    // Create builder
    builder := pdf.NewBuilder()
    builder.AddPage()
    
    // Create styled text widget
    textWidget := pdf.Text{
        Text: api.Text{
            Content: "Hello, World!",
            Class: api.Class{
                Font: &api.Font{
                    Size: 1.5,
                    Bold: true,
                },
                Foreground: &api.Color{Hex: "#2563eb"},
                Padding: &api.Padding{
                    Top: 1.0,
                    Bottom: 1.0,
                },
            },
        },
    }
    
    // Draw widget
    err := builder.DrawWidget(textWidget)
    if err != nil {
        panic(err)
    }
    
    // Generate PDF
    pdfData, err := builder.Output()
    if err != nil {
        panic(err)
    }
    
    // Save to file
    os.WriteFile("output.pdf", pdfData, 0644)
}
```

## Using Tailwind Classes

You can use Tailwind utility classes by resolving them to `api.Class`:

```go
// Convert Tailwind classes to api.Class
tailwindClasses := "text-blue-600 font-bold text-lg p-4 bg-gray-100"
resolvedClass := api.ResolveStyles(tailwindClasses)

textWidget := pdf.Text{
    Text: api.Text{
        Content: "Styled with Tailwind classes",
        Class:   resolvedClass,
    },
}
```

## Supported Tailwind Utilities

### Colors
- `text-{color}-{shade}` → `Class.Foreground`
- `bg-{color}-{shade}` → `Class.Background`

### Typography
- `font-bold`, `font-semibold`, `font-medium` → `Class.Font.Bold`
- `italic`, `not-italic` → `Class.Font.Italic`
- `underline`, `line-through` → `Class.Font.Underline/Strikethrough`
- `text-xs` through `text-9xl` → `Class.Font.Size`

### Spacing
- `p-{value}` → all sides padding
- `px-{value}`, `py-{value}` → horizontal/vertical padding
- `pt-{value}`, `pr-{value}`, `pb-{value}`, `pl-{value}` → individual sides

### Opacity
- `opacity-50`, `opacity-75`, etc. → `Class.Font.Faint`

## Widget Examples

### Table Widget
```go
tableWidget := pdf.Table{
    Headers: []string{"Name", "Age", "City"},
    Rows: [][]any{
        {"Alice", 30, "New York"},
        {"Bob", 25, "Los Angeles"},
    },
    HeaderStyle: api.Class{
        Font: &api.Font{Bold: true},
        Background: &api.Color{Hex: "#f0f0f0"},
    },
    RowStyle: api.Class{
        Font: &api.Font{Size: 0.9},
    },
}
```

### Box Widget with Labels
```go
boxWidget := pdf.Box{
    Rectangle: api.Rectangle{Width: 100, Height: 50},
    Labels: []pdf.Label{
        {
            Text: api.Text{
                Content: "Centered Label",
                Class: api.Class{
                    Font: &api.Font{Bold: true},
                },
            },
            Positionable: pdf.Positionable{
                Position: &pdf.LabelPosition{
                    Vertical:   pdf.VerticalCenter,
                    Horizontal: pdf.HorizontalCenter,
                },
            },
        },
    },
}
```

### Image Widget
```go
imageWidget := pdf.Image{
    Source:  "path/to/image.jpg", // Or URL
    AltText: "Description",
    Width:   floatPtr(80),
    Height:  floatPtr(60),
}
```

## Architecture

### Core Components

1. **Builder** (`builder.go`): Main PDF document builder with fpdf integration
2. **StyleConverter** (`style.go`): Converts `api.Class` to fpdf styling
3. **Widgets** (`text.go`, `table.go`, `box.go`, `image.go`): Individual widget implementations
4. **Layout** (`layout.go`): Grid layout system

### Key Features

- **State Management**: Automatic save/restore of styling state
- **Position Tracking**: Automatic position management for widgets
- **Page Management**: Automatic page breaks and multi-page support
- **Font Metrics**: Accurate text measurement for layout calculations
- **Error Handling**: Graceful fallbacks for missing resources

## Integration with Existing System

This package integrates seamlessly with the existing formatter system:

- Uses the same `api.Class` structure as other formatters
- Leverages `api.ResolveStyles()` for Tailwind parsing
- Compatible with existing `api.Text` and styling patterns
- Can be used as a drop-in replacement for legacy PDF generation

## Testing

Run tests with:
```bash
go test ./formatters/pdf/... -v
```

Generate a test PDF with:
```bash
SAVE_TEST_PDF=1 go test ./formatters/pdf/... -v
```

See `example_test.go` for comprehensive usage examples.