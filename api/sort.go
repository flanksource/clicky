package api

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type sortableField struct {
	field reflect.StructField
	key   string
}

// MergeSortableColumns enriches provider-defined columns with public sort keys
// declared on the concrete row struct. Provider metadata wins when both seams
// declare a key for the same column.
func MergeSortableColumns(rowType reflect.Type, columns []ColumnDef) ([]ColumnDef, error) {
	fields, err := sortableFields(rowType)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return append([]ColumnDef(nil), columns...), validateSortKeys(columns)
	}

	merged := append([]ColumnDef(nil), columns...)
	matchedColumns := make(map[int]string, len(fields))
	for _, sortable := range fields {
		field := sortable.field
		jsonName := jsonFieldName(field)
		label := ParsePrettyTagWithName(jsonName, field.Tag.Get("pretty")).Label
		var matches []int
		for i, column := range merged {
			if column.Name == field.Name || column.Name == jsonName || column.DisplayLabel() == label {
				matches = append(matches, i)
			}
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("sort field %s with key %q does not match a table column", field.Name, sortable.key)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("sort field %s with key %q ambiguously matches %d table columns", field.Name, sortable.key, len(matches))
		}

		index := matches[0]
		if previous, exists := matchedColumns[index]; exists {
			return nil, fmt.Errorf("sort fields %s and %s match table column %q", previous, field.Name, merged[index].Name)
		}
		matchedColumns[index] = field.Name
		if merged[index].SortKey == "" {
			merged[index].SortKey = sortable.key
		}
	}

	if err := validateSortKeys(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// MustMergeSortableColumns is MergeSortableColumns for rendering paths whose
// schema cannot return an error. Invalid sortable metadata is a registration
// bug and therefore fails immediately.
func MustMergeSortableColumns(rowType reflect.Type, columns []ColumnDef) []ColumnDef {
	merged, err := MergeSortableColumns(rowType, columns)
	if err != nil {
		panic(err)
	}
	return merged
}

// SortableKeysForType returns the public sort keys exposed by a row type after
// merging struct tags with an optional TableProvider schema.
func SortableKeysForType(rowType reflect.Type) ([]string, error) {
	rowType = dereferenceType(rowType)
	if rowType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("sortable row must be a struct, got %s", rowType)
	}

	var columns []ColumnDef
	if provider, ok := tableProviderForType(rowType); ok {
		columns = provider.Columns()
	} else {
		fields, err := NewStructParser().GetTableFields(reflect.New(rowType).Elem())
		if err != nil {
			return nil, err
		}
		columns = make([]ColumnDef, 0, len(fields))
		for _, field := range fields {
			columns = append(columns, ColumnDef{Name: field.Name, Label: field.Label, SortKey: field.SortKey})
		}
	}

	merged, err := MergeSortableColumns(rowType, columns)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(merged))
	for _, column := range merged {
		if column.SortKey != "" {
			keys = append(keys, column.SortKey)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func sortableFields(rowType reflect.Type) ([]sortableField, error) {
	rowType = dereferenceType(rowType)
	if rowType.Kind() != reflect.Struct {
		return nil, nil
	}

	var fields []sortableField
	for i := 0; i < rowType.NumField(); i++ {
		field := rowType.Field(i)
		if !field.IsExported() {
			continue
		}
		if key, exists := field.Tag.Lookup("sort"); exists {
			key = strings.TrimSpace(key)
			if key == "" {
				return nil, fmt.Errorf("sort field %s declares an empty key", field.Name)
			}
			fields = append(fields, sortableField{field: field, key: key})
			continue
		}
		if field.Anonymous {
			nested, err := sortableFields(field.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
		}
	}
	return fields, nil
}

func validateSortKeys(columns []ColumnDef) error {
	seen := make(map[string]string)
	for _, column := range columns {
		if column.SortKey == "" {
			continue
		}
		if strings.TrimSpace(column.SortKey) != column.SortKey || strings.ContainsAny(column.SortKey, " \t\r\n") {
			return fmt.Errorf("table column %q has invalid public sort key %q", column.Name, column.SortKey)
		}
		if previous, exists := seen[column.SortKey]; exists {
			return fmt.Errorf("table columns %q and %q declare duplicate sort key %q", previous, column.Name, column.SortKey)
		}
		seen[column.SortKey] = column.Name
	}
	return nil
}

func tableProviderForType(rowType reflect.Type) (TableProvider, bool) {
	rowType = dereferenceType(rowType)
	valueType := reflect.TypeOf((*TableProvider)(nil)).Elem()
	if rowType.Implements(valueType) {
		return reflect.New(rowType).Elem().Interface().(TableProvider), true
	}
	pointerType := reflect.PointerTo(rowType)
	if pointerType.Implements(valueType) {
		return reflect.New(rowType).Interface().(TableProvider), true
	}
	return nil, false
}

func dereferenceType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value
}
