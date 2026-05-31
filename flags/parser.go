package flags

import (
	"fmt"
	"reflect"
	"strings"
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

		// Check for flag tag on this field. The flag tag may carry a
		// comma-delimited shorthand, e.g. `flag:"limit,l"` → --limit | -l.
		flagName, commaShort := splitFlagTag(field.Tag.Get("flag"))
		isArgs := field.Tag.Get("args") == "true"
		isStdin := field.Tag.Get("stdin") == "true"

		// Skip fields that don't have flag, args, or stdin tags
		if (flagName == "" || flagName == "-") && !isArgs && !isStdin {
			continue
		}

		// An explicit short tag takes precedence over the comma-derived alias.
		shortFlag := field.Tag.Get("short")
		if shortFlag == "" {
			shortFlag = commaShort
		}
		if l := len([]rune(shortFlag)); l > 1 {
			return fmt.Errorf("field %s: flag shorthand %q must be a single character", field.Name, shortFlag)
		}

		// Extract flag metadata
		info := FieldInfo{
			FieldName:    field.Name,
			FieldPath:    currentPath,
			FieldType:    field.Type,
			FlagName:     flagName,
			Help:         field.Tag.Get("help"),
			DefaultValue: field.Tag.Get("default"),
			ShortFlag:    shortFlag,
			Required:     field.Tag.Get("required") == "true",
			IsStdin:      isStdin,
			IsArgs:       isArgs,
		}

		*fields = append(*fields, info)
	}

	return nil
}

// splitFlagTag splits a flag tag into its long name and optional single-char
// shorthand, e.g. "limit,l" → ("limit", "l"). Whitespace around each part is
// trimmed. A tag with no comma yields an empty shorthand.
func splitFlagTag(tag string) (name, short string) {
	long, rest, found := strings.Cut(tag, ",")
	if !found {
		return strings.TrimSpace(long), ""
	}
	return strings.TrimSpace(long), strings.TrimSpace(rest)
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
