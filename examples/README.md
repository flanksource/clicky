# Clicky Examples

This directory contains examples showing how to use the clicky library to format JSON data with pretty printing.

## Files

- `clicky-pipe.go` - Main example program that can read JSON from stdin or file
- `example-data.json` - Sample order data with complex nested structures and arrays
- `go.mod` - Module configuration

## Usage

### Format from file (default pretty format)
```bash
go run clicky-pipe.go
```

### Format from stdin
```bash
cat example-data.json | go run clicky-pipe.go
```

### Different output formats
```bash
# Pretty format (default) - styled terminal output with tables
cat example-data.json | go run clicky-pipe.go pretty

# CSV format - extracts table data (items) as CSV
cat example-data.json | go run clicky-pipe.go csv

# HTML format - full HTML page with Tailwind CSS
cat example-data.json | go run clicky-pipe.go html > order.html

# JSON format - clean JSON output
cat example-data.json | go run clicky-pipe.go json

# YAML format
cat example-data.json | go run clicky-pipe.go yaml

# Markdown format
cat example-data.json | go run clicky-pipe.go markdown
```

## Features Demonstrated

### Summary Layout
- **2-column layout** for summary fields (non-table data)
- **Pretty field names** - converts `snake_case` and `camelCase` to "Title Case"
- **Summary first** - shows summary information before tables

### Table Formatting
- **Automatic table detection** using `pretty:"table"` tags
- **Sorting** with `sort=field_name,dir=desc` options
- **Field hiding** using `pretty:"hide"` tags
- **Beautiful table rendering** with borders and alignment

### Conditional Formatting
- **Color coding** based on values (e.g., `pretty:"color,green=completed,red=failed"`)
- **Numeric conditions** (e.g., `pretty:"color,green=>=36,yellow=>=24,red=<24"`)
- **String matching** for status indicators

### Data Type Formatting
- **Currency** formatting with `pretty:"currency"`
- **Date** formatting with `pretty:"date,format=epoch"`
- **Float** precision with `pretty:"float,digits=1"`

### Output Formats
- **Pretty** - Colored terminal output with tables and formatting
- **CSV** - Extracts first table found for spreadsheet import
- **HTML** - Full HTML page with Tailwind CSS styling
- **JSON/YAML/Markdown** - Standard formats for integration

## Example Output

### Pretty Format (Terminal)
Shows a 2-column summary followed by a formatted table:
```
Id: ORD-2024-4567                  Customer: {Acme Corporation...}
Status: processing                 Priority: high
Total Amount: $15750.00            Currency: USD
...

┌─────────────────────────────┬─────────────┬──────────┐
│ name                        │ category    │ quantity │
├─────────────────────────────┼─────────────┼──────────┤
│ Professional Laptop 15-inch │ Electronics │ 5        │
│ USB-C Docking Station       │ Accessories │ 5        │
│ 4K Monitor 27-inch          │ Electronics │ 3        │
└─────────────────────────────┴─────────────┴──────────┘
```

### CSV Format
Extracts just the items table:
```csv
name,category,quantity,unit_price,discount_percent,line_total,warranty_months
Professional Laptop 15-inch,Electronics,5,2800,10,12600,36
USB-C Docking Station,Accessories,5,450,5,2137.5,24
4K Monitor 27-inch,Electronics,3,675,15,1721.25,36
```

### HTML Format
Creates a responsive webpage with Tailwind CSS styling, featuring:
- Summary cards with 2-column responsive grid
- Professional table styling with hover effects
- Mobile-friendly responsive design
