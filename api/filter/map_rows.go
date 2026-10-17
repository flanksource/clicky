package filter

import (
	"fmt"
	"regexp"

	"github.com/google/cel-go/cel"
)

var celIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type MapPredicate struct {
	program cel.Program
}

func CompileMapPredicate(keys []string, expression string) (*MapPredicate, error) {
	if expression == "" {
		return nil, nil
	}
	declarations := []cel.EnvOption{cel.Variable("row", cel.MapType(cel.StringType, cel.DynType))}
	for _, key := range keys {
		if key != "row" && celIdentifier.MatchString(key) {
			declarations = append(declarations, cel.Variable(key, cel.DynType))
		}
	}
	env, err := cel.NewEnv(declarations...)
	if err != nil {
		return nil, fmt.Errorf("create row filter: %w", err)
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile row filter %q: %w", expression, issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("row filter %q must return a boolean", expression)
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("create row filter program: %w", err)
	}
	return &MapPredicate{program: program}, nil
}

func (predicate *MapPredicate) Match(row map[string]any) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	variables := make(map[string]any, len(row)+1)
	variables["row"] = row
	for key, value := range row {
		if key != "row" && celIdentifier.MatchString(key) {
			variables[key] = value
		}
	}
	value, _, err := predicate.program.Eval(variables)
	if err != nil {
		return false, err
	}
	match, ok := value.Value().(bool)
	if !ok {
		return false, fmt.Errorf("row filter returned %T, expected boolean", value.Value())
	}
	return match, nil
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
