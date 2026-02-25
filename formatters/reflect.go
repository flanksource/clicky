package formatters

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flanksource/clicky/api"
)

// unwrapElement dereferences pointers and interfaces to get the underlying concrete value.
// Returns an error if a nil pointer is encountered during unwrapping.
func unwrapElement(val reflect.Value) (reflect.Value, error) {
	val, isNil := api.SafeDerefPointer(val)
	if isNil {
		return reflect.Value{}, fmt.Errorf("cannot convert slice with nil pointer element")
	}

	if val.Kind() == reflect.Interface && !val.IsNil() {
		val = val.Elem()
	}

	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return reflect.Value{}, fmt.Errorf("cannot convert slice with nil pointer element")
		}
		val = val.Elem()
	}
	return val, nil
}

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
	firstElem, _ = api.SafeDerefPointer(firstElem)

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
		elem, isNil := api.SafeDerefPointer(elem)
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

	// If no elements were collected, return empty slice with inner element type
	if len(flattened) == 0 {
		innerType := firstElem.Type().Elem()
		return reflect.MakeSlice(reflect.SliceOf(innerType), 0, 0)
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
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if strings.EqualFold(field.Name, name) {
			return val.Field(i)
		}
	}

	return reflect.Value{}
}
