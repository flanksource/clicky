# Clicky Pretty Formatter

A sophisticated struct formatter that uses reflection and struct tags to create beautiful, styled output with lipgloss. Now with integrated MCP (Model Context Protocol) support and TaskManager for concurrent task execution.

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

### 🤖 MCP (Model Context Protocol) Integration
- Expose CLI commands as MCP tools for AI assistants
- Integrated with TaskManager for visual progress tracking
- Configurable tool exposure and security settings
- Support for Claude Desktop, Cursor, and other MCP clients

### 📊 Task Manager
- Concurrent task execution with visual progress bars
- Retry logic with exponential backoff
- Timeout support and cancellation
- Beautiful Lipgloss-styled output

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

## MCP (Model Context Protocol) Integration

Clicky includes built-in MCP support, allowing you to expose your CLI commands as tools for AI assistants.

### Adding MCP to Your CLI

```go
package main

import (
    "github.com/flanksource/clicky/mcp"
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "myapp",
        Short: "My CLI application",
    }
    
    // Add your application commands
    rootCmd.AddCommand(myCommand1())
    rootCmd.AddCommand(myCommand2())
    
    // Add MCP server functionality
    rootCmd.AddCommand(mcp.NewCommand())
    
    rootCmd.Execute()
}
```

### Running as MCP Server

```bash
# Start MCP server with default configuration
myapp mcp serve

# Auto-expose all commands
myapp mcp serve --auto-expose

# View configuration
myapp mcp config
```

### Integration with AI Assistants

Configure Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "myapp": {
      "command": "/path/to/myapp",
      "args": ["mcp", "serve"]
    }
  }
}
```

### MCP with TaskManager

When MCP tools are executed, they automatically use Clicky's TaskManager for:
- Visual progress tracking with progress bars
- Concurrent execution control
- Retry logic with exponential backoff
- Timeout protection
- Beautiful styled output

## Task Manager

Clicky includes a powerful TaskManager for concurrent task execution with visual feedback.

### Basic Usage

```go
package main

import (
    "github.com/flanksource/clicky"
)

func main() {
    tm := clicky.NewTaskManager()
    
    // Start a task with progress tracking
    task := tm.Start("Processing data",
        clicky.WithTimeout(30 * time.Second),
        clicky.WithFunc(func(t *clicky.Task) error {
            t.SetProgress(0, 100)
            
            for i := 0; i <= 100; i++ {
                // Simulate work
                time.Sleep(100 * time.Millisecond)
                t.SetProgress(i, 100)
                
                if i % 20 == 0 {
                    t.Infof("Processed %d%%", i)
                }
            }
            
            t.Success()
            return nil
        }),
    )
    
    // Wait for all tasks to complete
    tm.Wait()
}
```

### Advanced Features

```go
// Configure retry behavior
tm.SetRetryConfig(clicky.RetryConfig{
    MaxRetries:      3,
    BaseDelay:       1 * time.Second,
    MaxDelay:        30 * time.Second,
    BackoffFactor:   2.0,
    JitterFactor:    0.1,
    RetryableErrors: []string{"timeout", "temporary"},
})

// Set concurrency limit
tm.SetMaxConcurrent(5)

// Enable verbose logging
tm.SetVerbose(true)
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
