package entity

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/flanksource/clicky/api"
)

// withEntityIDs wraps each EntityItem in the slice to include a _id field in JSON output.
func withEntityIDs[T EntityItem](items []T) []entityWithID[T] {
	result := make([]entityWithID[T], len(items))
	for i, item := range items {
		result[i] = entityWithID[T]{ID: item.GetID(), Inner: item}
	}
	return result
}

type entityWithID[T EntityItem] struct {
	ID    string `json:"_id"`
	Inner T
}

func (e entityWithID[T]) GetID() string   { return e.ID }
func (e entityWithID[T]) GetName() string { return e.Inner.GetName() }

func (e entityWithID[T]) Columns() []api.ColumnDef {
	withID := func(columns []api.ColumnDef) []api.ColumnDef {
		columns = api.MustMergeSortableColumns(reflect.TypeFor[T](), columns)
		for _, column := range columns {
			if column.Name == "_id" {
				return columns
			}
		}
		return append([]api.ColumnDef{api.Column("_id").Hidden().Build()}, columns...)
	}

	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return withID(tp.Columns())
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		columns := columnsFromPrettyRow(prettyRow)
		if len(columns) > 0 {
			return withID(columns)
		}
	}

	columns, _, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return withID(columns)
}

func (e entityWithID[T]) Row() map[string]any {
	withID := func(row map[string]any) map[string]any {
		next := make(map[string]any, len(row)+1)
		next["_id"] = e.ID
		for key, value := range row {
			next[key] = value
		}
		return next
	}

	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return withID(tp.Row())
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		row := make(map[string]any, len(prettyRow))
		for key, value := range prettyRow {
			row[key] = value
		}
		return withID(row)
	}

	_, row, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return withID(row)
}

func (e entityWithID[T]) PrettyRow(opts interface{}) map[string]api.Text {
	if row, ok := entityPrettyRow(any(e.Inner), opts); ok {
		next := make(map[string]api.Text, len(row)+1)
		next["_id"] = api.Text{Content: e.ID}
		for key, value := range row {
			next[key] = value
		}
		return next
	}

	rowData := e.Row()
	if len(rowData) == 0 {
		return nil
	}

	row := make(map[string]api.Text)
	order := 1
	for _, col := range e.Columns() {
		if col.Hidden {
			continue
		}
		cell := api.Text{Style: fmt.Sprintf("order-%d %s", order, col.Style)}
		if value, ok := rowData[col.Name]; ok {
			cell = addPrettyRowValue(cell, value)
		}
		row[col.DisplayLabel()] = cell
		order++
	}
	return row
}

func entityPrettyRow(inner any, opts interface{}) (map[string]api.Text, bool) {
	if pr, ok := inner.(api.PrettyRow); ok {
		if row := pr.PrettyRow(opts); len(row) > 0 {
			return row, true
		}
	}
	return nil, false
}

func columnsFromPrettyRow(prettyRow map[string]api.Text) []api.ColumnDef {
	type orderedColumn struct {
		name  string
		style string
		order int
	}

	columns := make([]orderedColumn, 0, len(prettyRow))
	for name, text := range prettyRow {
		columns = append(columns, orderedColumn{
			name:  name,
			style: text.Style,
			order: api.ExtractOrderValue(text.Style),
		})
	}

	sort.Slice(columns, func(i, j int) bool {
		if columns[i].order != columns[j].order {
			return columns[i].order < columns[j].order
		}
		return columns[i].name < columns[j].name
	})

	defs := make([]api.ColumnDef, 0, len(columns))
	for _, col := range columns {
		defs = append(defs, api.ColumnDef{
			Name:  col.name,
			Label: col.name,
			Style: col.style,
		})
	}
	return defs
}

func columnsAndRowFromStruct(inner any) ([]api.ColumnDef, map[string]any, bool) {
	parser := api.NewStructParser()
	val := reflect.ValueOf(inner)
	if !val.IsValid() {
		return nil, nil, false
	}

	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil, nil, false
		}
		val = val.Elem()
	}

	fields, err := parser.GetTableFields(val)
	if err != nil {
		return nil, nil, false
	}

	typedRow, err := parser.StructToRow(val)
	if err != nil {
		return nil, nil, false
	}

	columns := make([]api.ColumnDef, 0, len(fields))
	row := make(map[string]any, len(fields))
	for _, field := range fields {
		columns = append(columns, api.ColumnDef{
			Name:          field.Name,
			Label:         field.Label,
			Style:         field.Style,
			HeaderStyle:   field.LabelStyle,
			Type:          field.Type,
			Format:        field.Format,
			SortKey:       field.SortKey,
			FormatOptions: field.FormatOptions,
		})
		if value, ok := typedRow[field.Name]; ok {
			row[field.Name] = value.Value()
		}
	}

	return columns, row, true
}

func addPrettyRowValue(cell api.Text, value any) api.Text {
	switch v := value.(type) {
	case api.PrettyShort:
		return cell.Add(v.PrettyShort())
	case api.Textable:
		return cell.Add(v)
	case api.Pretty:
		return cell.Add(v.Pretty())
	default:
		return cell.Add(api.Human(v))
	}
}

func (e entityWithID[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(e.Inner)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 || data[0] != '{' {
		return data, nil
	}
	idJSON, err := json.Marshal(e.ID)
	if err != nil {
		return nil, err
	}
	prefix := []byte(`{"_id":`)
	prefix = append(prefix, idJSON...)
	prefix = append(prefix, ',')
	return append(prefix, data[1:]...), nil
}
