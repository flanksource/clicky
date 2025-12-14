package formatters

import (
	"reflect"
	"strings"
)

// FlattenSlice flattens a slice of slices into a single-level slice.
// If the input is not a slice of slices, it returns the input unchanged.
// This allows safe use on any slice without pre-checking.
func FlattenSlice(val reflect.Value) reflect.Value {
	// Check if input is a slice or array
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return val
	}

	// Empty slice - return as-is
	if val.Len() == 0 {
		return val
	}

	// Get the first element to check if this is a slice of slices
	firstElem := val.Index(0)
	firstElem, _ = safeDerefPointer(firstElem)

	// Dereference interface to get underlying concrete type
	if firstElem.Kind() == reflect.Interface && !firstElem.IsNil() {
		firstElem = firstElem.Elem()
	}

	// Not a slice of slices - return input unchanged
	if firstElem.Kind() != reflect.Slice && firstElem.Kind() != reflect.Array {
		return val
	}

	// It's a slice of slices - flatten it
	var flattened []reflect.Value
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elem, isNil := safeDerefPointer(elem)
		if isNil {
			continue // Skip nil outer elements
		}

		// Dereference interface
		if elem.Kind() == reflect.Interface && !elem.IsNil() {
			elem = elem.Elem()
		}

		// Iterate inner slice and collect all elements
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
			for j := 0; j < elem.Len(); j++ {
				innerElem := elem.Index(j)
				flattened = append(flattened, innerElem)
			}
		}
	}

	// If no elements were collected, return empty slice of same type as input
	if len(flattened) == 0 {
		return reflect.MakeSlice(val.Type(), 0, 0)
	}

	// Create a new slice with the flattened elements
	// Determine the element type from the first flattened element
	elemType := flattened[0].Type()
	newSlice := reflect.MakeSlice(reflect.SliceOf(elemType), len(flattened), len(flattened))
	for i, elem := range flattened {
		newSlice.Index(i).Set(elem)
	}

	return newSlice
}

// ToSlice converts variadic any arguments to a slice of type T if all elements implement T.
// It handles:
// - Single slice argument: []T or []any where elements are T
// - Multiple arguments: each implementing T
// - Nested slices: flattens one level if first arg is a slice
func ToSlice[T any](data ...any) ([]T, bool) {
	if len(data) == 0 {
		return nil, false
	}

	var result []T

	// Case 1: Single argument that is already a slice
	if len(data) == 1 {
		val := reflect.ValueOf(data[0])
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			// It's a slice, try to convert each element
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if elem.CanInterface() {
					if typed, ok := elem.Interface().(T); ok {
						result = append(result, typed)

					} else {

						return nil, false // Not all elements are T
					}
				} else {

					return nil, false
				}
			}
			return result, len(result) > 0
		}
	}

	// Case 2: Multiple arguments or single non-slice argument
	for _, item := range data {
		// Check if this item is a slice (nested slice case)
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			// Flatten one level
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if elem.CanInterface() {
					if typed, ok := elem.Interface().(T); ok {
						result = append(result, typed)
					} else {
						return nil, false
					}
				} else {
					return nil, false
				}
			}
		} else {
			// Single item
			if typed, ok := item.(T); ok {
				result = append(result, typed)
			} else {
				return nil, false
			}
		}
	}

	return result, len(result) > 0
}

// processSliceElement handles slice elements that might be nil pointers
func processSliceElement(elem reflect.Value) (reflect.Value, bool) {
	// If it's a pointer, dereference it safely
	if elem.Kind() == reflect.Ptr {
		if elem.IsNil() {
			return reflect.Value{}, true // Nil element
		}
		return elem.Elem(), false
	}

	return elem, false // Not a pointer
}

// safeDerefPointer safely dereferences a pointer value, returning the dereferenced value and whether it was nil
func safeDerefPointer(val reflect.Value) (reflect.Value, bool) {
	if val.Kind() != reflect.Ptr {
		return val, false // Not a pointer, return as-is
	}

	if val.IsNil() {
		return reflect.Value{}, true // Nil pointer
	}

	return val.Elem(), false // Dereferenced value
}

// isEmptyValue checks if a reflect.Value is considered empty
func isEmptyValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Interface:
		if v.IsNil() {
			return true
		}
		// For interface{}, check the underlying value
		return isEmptyValue(v.Elem())
	case reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

// GetFieldValueCaseInsensitive tries to find a field by name with different casing
func GetFieldValueCaseInsensitive(val reflect.Value, name string) reflect.Value {
	if val.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	typ := val.Type()
	// Try exact match first
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == name {
			return val.Field(i)
		}
	}

	// Try case-insensitive match
	lowerName := strings.ToLower(name)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if strings.EqualFold(field.Name, lowerName) {
			return val.Field(i)
		}
	}

	return reflect.Value{}
}
