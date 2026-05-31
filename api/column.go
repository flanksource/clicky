package api

import "fmt"

// ColumnDef defines a table column's schema and display properties.
// Order is determined by array position when returned from TableProvider.Columns().
type ColumnDef struct {
	Name          string
	Label         string
	Style         string
	HeaderStyle   string
	Type          string
	Format        string
	FormatOptions map[string]string
	MaxWidth      int
	Hidden        bool
}

// DisplayLabel returns Label if set, otherwise prettifies Name.
func (c ColumnDef) DisplayLabel() string {
	if c.Label != "" {
		return c.Label
	}
	return PrettifyFieldName(c.Name)
}

// ColumnBuilder provides a fluent API for constructing ColumnDef instances.
type ColumnBuilder struct {
	col ColumnDef
}

// Column creates a new column builder with the given field name.
func Column(name string) *ColumnBuilder {
	return &ColumnBuilder{col: ColumnDef{Name: name}}
}

// Label sets the display label for the column header.
func (b *ColumnBuilder) Label(label string) *ColumnBuilder {
	b.col.Label = label
	return b
}

// Style sets the Tailwind CSS classes for cell content.
func (b *ColumnBuilder) Style(style string) *ColumnBuilder {
	b.col.Style = style
	return b
}

// HeaderStyle sets the Tailwind CSS classes for the column header.
func (b *ColumnBuilder) HeaderStyle(style string) *ColumnBuilder {
	b.col.HeaderStyle = style
	return b
}

// Type sets the data type hint (string, int, float, date, etc.).
func (b *ColumnBuilder) Type(typ string) *ColumnBuilder {
	b.col.Type = typ
	return b
}

// Format sets the format type (currency, date, bytes, etc.).
func (b *ColumnBuilder) Format(format string) *ColumnBuilder {
	b.col.Format = format
	return b
}

// FormatOption adds a single format option key-value pair.
func (b *ColumnBuilder) FormatOption(key, value string) *ColumnBuilder {
	if b.col.FormatOptions == nil {
		b.col.FormatOptions = make(map[string]string)
	}
	b.col.FormatOptions[key] = value
	return b
}

// MaxWidth sets the maximum display width in characters.
func (b *ColumnBuilder) MaxWidth(width int) *ColumnBuilder {
	b.col.MaxWidth = width
	return b
}

// Hidden marks the column as hidden from display.
func (b *ColumnBuilder) Hidden() *ColumnBuilder {
	b.col.Hidden = true
	return b
}

// Build returns the constructed ColumnDef.
func (b *ColumnBuilder) Build() ColumnDef {
	return b.col
}

// TableProvider is the interface for types that can represent themselves as table rows.
// Column order is determined by the slice order returned from Columns().
type TableProvider interface {
	// Columns returns the column schema in display order.
	Columns() []ColumnDef

	// Row returns the raw data for this item as a map of column name to value.
	// Values are rendered using Text{}.Add(value).
	Row() map[string]any
}

// DetailProvider is an optional interface for TableProvider types that supply
// expandable detail content per row. Return nil to indicate no detail.
type DetailProvider interface {
	RowDetail() Textable
}

// NewTableFrom creates a TextTable from a slice of TableProvider items.
func NewTableFrom[T TableProvider](items []T) TextTable {
	table := TextTable{}
	if len(items) == 0 {
		return table
	}

	// Get column definitions from first item
	columns := items[0].Columns()

	// Build headers and field names from column definitions
	for _, col := range columns {
		if col.Hidden {
			continue
		}
		table.Headers = append(table.Headers, Text{Content: col.DisplayLabel(), Style: col.HeaderStyle})
		table.FieldNames = append(table.FieldNames, col.Name)

		style := col.Style
		if col.MaxWidth > 0 {
			style = fmt.Sprintf("%s max-w-[%dch] truncate", style, col.MaxWidth)
		}

		// Convert ColumnDef to PrettyField for Columns slice
		table.Columns = append(table.Columns, PrettyField{
			Name:          col.Name,
			Label:         col.DisplayLabel(),
			Style:         style,
			LabelStyle:    col.HeaderStyle,
			Type:          col.Type,
			Format:        col.Format,
			FormatOptions: col.FormatOptions,
		})
	}

	// Build rows and collect detail content
	var hasDetail bool
	for _, item := range items {
		rowData := item.Row()
		row := TableRow{}

		for _, col := range columns {
			if val, exists := rowData[col.Name]; exists && col.Hidden {
				row[col.Name] = TypedValue{Textable: Text{}.Add(convertToTextable(val))}
				continue
			}
			if col.Hidden {
				continue
			}
			if val, exists := rowData[col.Name]; exists {
				style := col.Style
				if col.MaxWidth > 0 {
					style = fmt.Sprintf("%s max-w-[%dch] truncate", style, col.MaxWidth)
				}
				text := Text{Style: style}.Add(convertToTextable(val))
				row[col.Name] = TypedValue{Textable: text}
			} else {
				row[col.Name] = TypedValue{Textable: Text{}}
			}
		}

		table.Rows = append(table.Rows, row)

		if dp, ok := any(item).(DetailProvider); ok {
			detail := dp.RowDetail()
			table.RowDetail = append(table.RowDetail, detail)
			if detail != nil {
				hasDetail = true
			}
		}
	}

	// Clear RowDetail if no items actually provided detail content
	if !hasDetail {
		table.RowDetail = nil
	}

	return table
}

// convertToTextable converts any value to a Textable for table cells.
func convertToTextable(val any) Textable {
	if val == nil {
		return Text{}
	}

	switch v := val.(type) {
	case PrettyShort:
		return v.PrettyShort()
	case Textable:
		return v
	case Pretty:
		return v.Pretty()
	default:
		return Human(v)
	}
}
