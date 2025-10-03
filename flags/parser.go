package flags

import (
	"fmt"
	"reflect"
)

// ParseStructFields recursively parses struct fields including embedded structs
// and returns a flat list of all fields with flag tags
func ParseStructFields(structType reflect.Type) ([]FieldInfo, error) {
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct type, got %s", structType.Kind())
	}

	var fields []FieldInfo
	err := parseStructFieldsRecursive(structType, nil, &fields)
	return fields, err
}

// parseStructFieldsRecursive recursively processes struct fields
// fieldPath tracks the indices needed to navigate from root to current field
func parseStructFieldsRecursive(structType reflect.Type, fieldPath []int, fields *[]FieldInfo) error {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		currentPath := append([]int{}, fieldPath...)
		currentPath = append(currentPath, i)

		// Handle embedded/anonymous struct fields
		if field.Anonymous {
			fieldType := field.Type
			// Dereference pointer if needed
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			if fieldType.Kind() == reflect.Struct {
				// Recursively process embedded struct
				if err := parseStructFieldsRecursive(fieldType, currentPath, fields); err != nil {
					return err
				}
				continue
			}
		}

		// Check for flag tag on this field
		flagName := field.Tag.Get("flag")
		if flagName == "" {
			continue
		}

		// Extract flag metadata
		info := FieldInfo{
			FieldName:    field.Name,
			FieldPath:    currentPath,
			FieldType:    field.Type,
			FlagName:     flagName,
			Help:         field.Tag.Get("help"),
			DefaultValue: field.Tag.Get("default"),
			ShortFlag:    field.Tag.Get("short"),
			Required:     field.Tag.Get("required") == "true",
			IsStdin:      field.Tag.Get("stdin") == "true",
		}

		*fields = append(*fields, info)
	}

	return nil
}

// GetFieldByPath navigates through embedded structs using field indices
func GetFieldByPath(structValue reflect.Value, fieldPath []int) reflect.Value {
	current := structValue
	for _, index := range fieldPath {
		current = current.Field(index)

		// If we hit an embedded pointer struct, dereference it
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				// Initialize nil embedded struct pointer
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}
	}
	return current
}
