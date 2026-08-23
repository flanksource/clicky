package entity

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"

	"github.com/flanksource/clicky/api"
)

// SortDirection is the normalized direction delivered to an entity list
// callback after request validation.
type SortDirection string

const (
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

// SortOptions is embedded in an entity's list options to receive the validated
// public sort key selected by the caller.
type SortOptions struct {
	Key       string
	Direction SortDirection
}

// SetSort implements SortCarrier.
func (o *SortOptions) SetSort(value SortOptions) {
	*o = value
}

// SortCarrier is implemented by list options that accept normalized sorting.
type SortCarrier interface {
	SetSort(SortOptions)
}

// SortSpec enables server-side sorting for an entity list operation.
type SortSpec struct {
	Default SortOptions
	keys    map[string]struct{}
}

// Keys returns the response struct's public sort keys in stable order.
func (s SortSpec) Keys() []string {
	keys := make([]string, 0, len(s.keys))
	for key := range s.keys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func prepareSortSpec[T EntityItem, ListOpts any](spec *SortSpec) *SortSpec {
	if spec == nil {
		return nil
	}
	var opts ListOpts
	if _, ok := any(&opts).(SortCarrier); !ok {
		panic(fmt.Sprintf("clicky sortable entity options %s must implement SortCarrier", reflect.TypeFor[ListOpts]()))
	}

	keys, err := api.SortableKeysForType(reflect.TypeFor[T]())
	if err != nil {
		panic(fmt.Sprintf("clicky sortable entity response %s: %v", reflect.TypeFor[T](), err))
	}
	if len(keys) == 0 {
		panic(fmt.Sprintf("clicky sortable entity response %s declares no sort tags or column sort keys", reflect.TypeFor[T]()))
	}

	prepared := &SortSpec{Default: spec.Default, keys: make(map[string]struct{}, len(keys))}
	for _, key := range keys {
		prepared.keys[key] = struct{}{}
	}
	if prepared.Default.Key == "" {
		panic("clicky sortable entity default sort key is required")
	}
	if _, ok := prepared.keys[prepared.Default.Key]; !ok {
		panic(fmt.Sprintf("clicky sortable entity default key %q is not declared by its response columns", prepared.Default.Key))
	}
	if prepared.Default.Direction == "" {
		prepared.Default.Direction = SortDirectionAsc
	}
	if !validSortDirection(prepared.Default.Direction) {
		panic(fmt.Sprintf("clicky sortable entity default direction %q is invalid", prepared.Default.Direction))
	}
	return prepared
}

func resolveSortOptions(flagMap map[string]string, spec *SortSpec) (SortOptions, error) {
	if spec == nil {
		return SortOptions{}, nil
	}
	key, hasKey := flagMap["sort"]
	direction, hasDirection := flagMap["order"]
	if !hasKey && !hasDirection {
		return spec.Default, nil
	}
	if !hasKey {
		return SortOptions{}, NewStatusError(http.StatusBadRequest, "invalid_sort", "order requires sort")
	}
	if _, ok := spec.keys[key]; !ok {
		return SortOptions{}, NewStatusErrorf(http.StatusBadRequest, "invalid_sort", "unsupported sort key %q", key)
	}
	if !hasDirection {
		return SortOptions{Key: key, Direction: SortDirectionAsc}, nil
	}
	normalizedDirection := SortDirection(direction)
	if !validSortDirection(normalizedDirection) {
		return SortOptions{}, NewStatusErrorf(http.StatusBadRequest, "invalid_sort", "unsupported sort direction %q", direction)
	}
	return SortOptions{Key: key, Direction: normalizedDirection}, nil
}

func validSortDirection(direction SortDirection) bool {
	return direction == SortDirectionAsc || direction == SortDirectionDesc
}

func resolveSortableEntityOpts[T any](flagMap map[string]string, filters []Filter[T], spec *SortSpec) (T, error) {
	opts, err := resolveEntityOpts(flagMap, filters)
	if err != nil {
		return opts, err
	}
	if spec == nil {
		return opts, nil
	}
	value, err := resolveSortOptions(flagMap, spec)
	if err != nil {
		return opts, err
	}
	carrier, ok := any(&opts).(SortCarrier)
	if !ok {
		panic(fmt.Sprintf("clicky sortable entity options %s must implement SortCarrier", reflect.TypeFor[T]()))
	}
	carrier.SetSort(value)
	return opts, nil
}
