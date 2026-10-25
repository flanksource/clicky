package filter

import (
	"fmt"
)

type MapPredicate struct {
	predicate Predicate
}

// CompileMapPredicate compiles expression for map rows with the given keys.
// Values are untyped; "row" holds every column so keys with spaces stay addressable.
func CompileMapPredicate(keys []string, expression string) (*MapPredicate, error) {
	if expression == "" {
		return nil, nil
	}
	sample := make(map[string]any, len(keys)+1)
	for _, key := range keys {
		sample[key] = nil
	}
	sample["row"] = map[string]any{}
	predicate, err := Compile(expression, sample)
	if err != nil {
		return nil, fmt.Errorf("compile row filter: %w", err)
	}
	return &MapPredicate{predicate: predicate}, nil
}

func (predicate *MapPredicate) Match(row map[string]any) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	variables := make(map[string]any, len(row)+1)
	for key, value := range row {
		variables[key] = value
	}
	variables["row"] = row
	return predicate.predicate.Match(variables)
}

func MapRows(rows []map[string]any, expression string) ([]map[string]any, error) {
	if expression == "" || len(rows) == 0 {
		return rows, nil
	}
	keys := make([]string, 0, len(rows[0]))
	for key := range rows[0] {
		keys = append(keys, key)
	}
	predicate, err := CompileMapPredicate(keys, expression)
	if err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0, len(rows))
	for index, row := range rows {
		match, err := predicate.Match(row)
		if err != nil {
			return nil, fmt.Errorf("evaluate row filter at row %d: %w", index+1, err)
		}
		if match {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}
