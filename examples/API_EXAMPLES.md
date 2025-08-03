# SchemaFormatter API Examples

The new `SchemaFormatter` provides a clean API for formatting structured data using YAML schemas. Here are examples of how to use it:

## Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "clicky"
)

func main() {
    // Load schema from YAML file
    formatter, err := clicky.LoadSchemaFromYAML("order-schema.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Format a single file
    options := clicky.FormatOptions{
        Format:  "pretty",
        NoColor: false,
        Verbose: true,
    }
    
    result, err := formatter.FormatFile("order-data.json", options)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(result)
}
```

## Format Multiple Files

```go
// Format multiple files at once
files := []string{"order1.json", "order2.yaml", "order3.json"}

options := clicky.FormatOptions{
    Format:  "html",
    Output:  "reports/",  // Output directory
    NoColor: false,
    Verbose: true,
}

err := formatter.FormatFiles(files, options)
if err != nil {
    log.Fatal(err)
}
```

## Different Output Formats

```go
// Pretty terminal output with colors
options := clicky.FormatOptions{Format: "pretty", NoColor: false}
result, _ := formatter.FormatFile("data.json", options)

// HTML with Tailwind CSS
options = clicky.FormatOptions{Format: "html"}
htmlResult, _ := formatter.FormatFile("data.json", options)

// CSV (extracts table data)
options = clicky.FormatOptions{Format: "csv"}
csvResult, _ := formatter.FormatFile("data.json", options)
```

## Schema Definition Example

```yaml
# order-schema.yaml
fields:
  - name: "id"
    type: "string"
    format: "color"
    color_options:
      blue: "*"
  
  - name: "status"
    type: "string"
    format: "color"
    color_options:
      green: "completed"
      yellow: "processing"
      red: "cancelled"
  
  - name: "total_amount"
    type: "float"
    format: "currency"
  
  - name: "items"
    type: "array"
    format: "table"
    format_options:
      sort: "line_total"
      dir: "desc"
```

## Features

### Automatic Heuristics
The schema formatter automatically enhances your schema based on field names and data:

- Fields with "date", "time", "created" → `format: "date"`
- Fields with "price", "amount", "total" → `format: "currency"`
- Fields with "item", "list", "entries" → `format: "table"`
- Status/priority fields → automatic color coding

### Nested Structure Support
- Structs are rendered recursively with proper indentation
- Maps are converted and formatted as nested structures
- Arrays can be displayed as formatted tables

### Multiple Output Formats
- **Pretty**: Colored terminal output with tables
- **HTML**: Responsive design with Tailwind CSS
- **CSV**: Extracts table data from structures
- **JSON/YAML**: Standard serialization formats

### File Output Options
```go
// Output to specific file
options.Output = "output.html"

// Output to directory (auto-generates filenames)
options.Output = "reports/"

// Output with placeholders
options.Output = "reports/{name}.{format}"

// Output to stdout
options.Output = ""
```
