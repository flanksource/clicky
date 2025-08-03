# Clicky Schema-Based CLI

This directory contains a command-line tool that uses YAML schema files to format structured data with customizable styling and layouts.

## Features

- **Schema-driven formatting**: Define field types, colors, and formatting in YAML
- **Heuristic enhancement**: Automatically infer formatting based on field names and values
- **Multiple output formats**: pretty, HTML, CSV, JSON, YAML, Markdown
- **Recursive struct rendering**: Nested structures are displayed with proper indentation
- **Enhanced table formatting**: Wider columns and better table layout
- **Color coding**: Support for conditional colors in both terminal and HTML output

## Usage

```bash
# Build the tool
cd cmd/clicky
go build

# Basic usage
./clicky --schema ../../examples/order-schema.yaml ../../examples/example-data.json

# Specify output format
./clicky --schema ../../examples/order-schema.yaml --format html ../../examples/example-data.json

# Output to file
./clicky --schema ../../examples/order-schema.yaml --format html --output output.html ../../examples/example-data.json

# Process multiple files
./clicky --schema ../../examples/order-schema.yaml --format pretty data1.json data2.yaml

# Output to directory (generates separate files)
./clicky --schema ../../examples/order-schema.yaml --format html --output reports/ *.json
```

## Schema Format

The schema is defined in YAML and describes how to format each field:

```yaml
fields:
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
    table_options:
      title: "Order Items"
      fields:
        - name: "name"
          type: "string"
        - name: "unit_price"
          type: "float"
          format: "currency"
```

## Field Types

- `string`: Text values
- `int`: Integer numbers
- `float`: Floating-point numbers
- `boolean`: True/false values
- `date`: Date/time values
- `struct`: Nested objects (rendered recursively)
- `array`: Lists (can be formatted as tables)

## Format Options

- `color`: Apply conditional colors based on values
- `currency`: Format as currency (e.g., $123.45)
- `date`: Format date values
- `float`: Format floating-point numbers with specified precision
- `table`: Format arrays as tables

## Color Options

Colors can be applied conditionally:

```yaml
color_options:
  green: "completed"        # Exact match
  red: "failed"
  yellow: ">=50"           # Numeric comparison
  blue: "<100"
```

## Heuristic Features

The tool automatically enhances schemas based on field names and values:

- **Date fields**: Fields containing "date", "time", "created", "updated" → `format: "date"`
- **Currency fields**: Fields containing "price", "cost", "amount", "total" → `format: "currency"`
- **Table fields**: Array fields containing "item", "list", "entries" → `format: "table"`
- **Status fields**: Automatic color coding for status, priority, level fields
- **Score fields**: Numeric ranges for rating/score fields

## Examples

See the `examples/` directory for:
- `order-schema.yaml`: Complete schema for e-commerce orders
- `example-data.json`: Sample order data
- `clicky-pipe.go`: Alternative pipe-based tool

## Output Formats

### Pretty (Terminal)
- Colored output with proper indentation
- 2-column summary layout
- Tables with borders and alignment
- Wider columns for better readability

### HTML
- Tailwind CSS styling
- Responsive 2-column grid layout
- Color-coded values
- Summary displayed first, then tables

### CSV
- Extracts table data from nested structures
- Uses first table found in the schema

### Other Formats
- JSON: Pretty-printed JSON
- YAML: Formatted YAML output
- Markdown: Markdown tables and formatting
