// Package celfilter is the CEL engine for github.com/flanksource/clicky/api/filter.
package celfilter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/flanksource/clicky/api/filter"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Adapter struct {
	options []cel.EnvOption
}

// New returns a CEL adapter with the cel-go standard library, the ext
// strings/lists/math/encoders/sets libraries and any extra options.
func New(options ...cel.EnvOption) *Adapter {
	base := []cel.EnvOption{ext.Strings(), ext.Lists(), ext.Math(), ext.Encoders(), ext.Sets()}
	return &Adapter{options: append(base, options...)}
}

// Use makes the CEL adapter the filter engine.
func Use() {
	filter.SetAdapter(New())
}

func (a *Adapter) Validate(expression string) error {
	env, err := cel.NewEnv(a.options...)
	if err != nil {
		return fmt.Errorf("create CEL environment: %w", err)
	}
	if _, issues := env.Parse(expression); issues != nil && issues.Err() != nil {
		return fmt.Errorf("invalid CEL expression %q: %w", expression, issues.Err())
	}
	return nil
}

func (a *Adapter) Compile(expression string, sample map[string]any) (filter.Predicate, error) {
	env, err := cel.NewEnv(append(a.options, declarations(sample)...)...)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile CEL expression %q: %w", expression, issues.Err())
	}
	if output := ast.OutputType(); output != cel.BoolType && output != cel.DynType {
		return nil, fmt.Errorf("CEL expression %q returns %s, expected bool", expression, output)
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("create CEL program: %w", err)
	}
	return predicate{program: program}, nil
}

type predicate struct {
	program cel.Program
}

func (p predicate) Match(variables map[string]any) (bool, error) {
	out, _, err := p.program.Eval(variables)
	if err != nil {
		if strings.Contains(err.Error(), "no such attribute") {
			return false, fmt.Errorf("%w: %w", filter.ErrMissingVariable, err)
		}
		return false, err
	}
	match, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("filter returned %T, expected bool", out.Value())
	}
	return match, nil
}

func declarations(sample map[string]any) []cel.EnvOption {
	decls := make([]cel.EnvOption, 0, len(sample))
	for name, value := range sample {
		if identifier.MatchString(name) {
			decls = append(decls, cel.Variable(name, typeOf(value)))
		}
	}
	return decls
}

func typeOf(value any) *cel.Type {
	switch value.(type) {
	case string:
		return cel.StringType
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return cel.IntType
	case float32, float64:
		return cel.DoubleType
	case bool:
		return cel.BoolType
	case map[string]any:
		return cel.MapType(cel.StringType, cel.DynType)
	default:
		return cel.DynType
	}
}
