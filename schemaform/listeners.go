package schemaform

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Ported from clicky-ui json-schema-form-listeners.ts / -listener-paths.ts.
//
// `x-on-change` listeners: a property names what happens to its siblings when
// its value is, or becomes, something. `x-on-load` listeners: an object names
// the state its properties start in. The state actions (hide/show,
// enable/disable, require/optional, patch) are DERIVED — re-evaluated from the
// current value every time. The value actions (reset/set) are TRANSITIONAL —
// they fire when the source field is committed.

// ChangeActions names what a listener does to sibling fields.
type ChangeActions struct {
	Hide     []string                   `json:"hide,omitempty"`
	Show     []string                   `json:"show,omitempty"`
	Enable   []string                   `json:"enable,omitempty"`
	Disable  []string                   `json:"disable,omitempty"`
	Require  []string                   `json:"require,omitempty"`
	Optional []string                   `json:"optional,omitempty"`
	Reset    []string                   `json:"reset,omitempty"`
	Set      map[string]any             `json:"set,omitempty"`
	Patch    map[string]json.RawMessage `json:"patch,omitempty"`
}

// Listener is one `x-on-change` / `x-on-load` entry: its actions apply while
// When holds (always, when omitted) and Else applies while it does not.
type Listener struct {
	ChangeActions
	When *Condition     `json:"when,omitempty"`
	Else *ChangeActions `json:"else,omitempty"`
}

// Condition is the if-style predicate (const/enum/not/properties/required) on
// the listener's subject, plus an optional CEL Expr; both must hold.
type Condition struct {
	Predicate *Schema
	Expr      string
	HasExpr   bool
}

func (c *Condition) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("when must be an object: %w", err)
	}
	if raw, ok := keys["expr"]; ok {
		if err := json.Unmarshal(raw, &c.Expr); err != nil {
			return fmt.Errorf("when.expr must be a string: %w", err)
		}
		c.HasExpr = true
		delete(keys, "expr")
	}
	if len(keys) == 0 {
		return nil
	}
	rest, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	c.Predicate = &Schema{}
	return json.Unmarshal(rest, c.Predicate)
}

// ExprInput is what a `when.expr` sees: the source key and its value, the
// object holding it (self) and the form's whole value (root).
type ExprInput struct {
	Expr  string
	Key   string
	Value any
	Self  map[string]any
	Root  map[string]any
}

// Evaluator evaluates a `when.expr`; it must return a bool.
type Evaluator func(ExprInput) (any, error)

type listenerSource struct {
	label string
	key   string
	load  bool
}

type boundListener struct {
	listener Listener
	source   listenerSource
}

type listenerOptions struct {
	root     map[string]any
	evaluate Evaluator
	// topLevel makes root follow the object being edited.
	topLevel bool
}

var patchDenied = map[string]string{
	"required":    "use require/optional",
	"default":     "a patch changes presentation, not values; use set/reset",
	"const":       "a patch changes presentation, not values; use set/reset",
	"x-on-change": "listeners cannot be patched in",
	"x-on-load":   "listeners cannot be patched in",
}

func init() {
	for _, k := range []string{"type", "properties", "items", "additionalProperties", "patternProperties",
		"allOf", "anyOf", "oneOf", "if", "then", "else", "not", "$ref", "$defs"} {
		patchDenied[k] = "it changes the field's shape; use an allOf branch"
	}
}

// applyListenerState is Effective plus the state actions of every listener,
// evaluated against the current value: the object's x-on-load listeners, then
// each property's x-on-change listeners. Property schemas are copied, never
// mutated.
func applyListenerState(schema *Schema, value map[string]any, opts listenerOptions) (Properties, error) {
	props := Effective(schema, value)
	known := declaredKeys(schema)
	bound := loadListenersOf(schema, value)
	for _, key := range props.Keys {
		for i, l := range props.Map[key].OnChange {
			bound = append(bound, boundListener{listener: l, source: changeSource(key, i)})
		}
	}
	for _, b := range bound {
		actions, err := b.selectActions(value, known, opts)
		if err != nil {
			return props, err
		}
		if err := applyStateActions(&props, value, actions); err != nil {
			return props, fmt.Errorf("%s: %w", b.source.label, err)
		}
	}
	return props, nil
}

func applyStateActions(props *Properties, value map[string]any, actions ChangeActions) error {
	flags := []struct {
		targets []string
		apply   func(*Schema)
	}{
		{actions.Hide, func(s *Schema) { s.Hidden = true }},
		{actions.Show, func(s *Schema) { s.Hidden = false }},
		{actions.Disable, func(s *Schema) { s.Disabled = true }},
		{actions.Enable, func(s *Schema) { s.Disabled = false }},
	}
	for _, flag := range flags {
		for _, target := range flag.targets {
			apply := flag.apply
			if err := patchTarget(props, value, target, func(leaf *Schema) (*Schema, error) {
				cp := *leaf
				apply(&cp)
				return &cp, nil
			}); err != nil {
				return err
			}
		}
	}
	for _, target := range actions.Require {
		if _, ok := props.Map[target]; ok {
			props.require(target)
		}
	}
	for _, target := range actions.Optional {
		props.optional(target)
	}
	for _, target := range sortedKeys(actions.Patch) {
		raw := actions.Patch[target]
		if err := patchTarget(props, value, target, func(leaf *Schema) (*Schema, error) {
			return overlay(leaf, raw)
		}); err != nil {
			return err
		}
	}
	return nil
}

func loadListenersOf(schema *Schema, value map[string]any) []boundListener {
	bound := loadListenersAt(schema.OnLoad, "#/x-on-load")
	for i, clause := range schema.AllOf {
		if !isConditional(clause) {
			bound = append(bound, loadListenersAt(clause.OnLoad, fmt.Sprintf("#/allOf/%d/x-on-load", i))...)
			continue
		}
		branch, name := clause.Else, "else"
		if matchesIf(clause.If, value) {
			branch, name = clause.Then, "then"
		}
		if branch != nil {
			bound = append(bound, loadListenersAt(branch.OnLoad, fmt.Sprintf("#/allOf/%d/%s/x-on-load", i, name))...)
		}
	}
	return bound
}

func loadListenersAt(listeners []Listener, pointer string) []boundListener {
	out := make([]boundListener, len(listeners))
	for i, l := range listeners {
		out[i] = boundListener{listener: l, source: listenerSource{label: fmt.Sprintf("x-on-load at %s/%d", pointer, i), load: true}}
	}
	return out
}

func changeSource(key string, index int) listenerSource {
	return listenerSource{label: fmt.Sprintf("x-on-change on %q [%d]", key, index), key: key}
}

// applyChangeEffects commits commit.value at commit.target and runs the value
// actions (reset/set) of that key's listeners against the post-edit object. A
// target whose value changes fires its own listeners in turn; each key fires at
// most once per commit. It returns the new object and the keys the actions
// changed.
func applyChangeEffects(schema *Schema, value map[string]any, commit valueChange, opts listenerOptions) (map[string]any, []string, error) {
	current := shallowCopy(value)
	writeKey(current, commit.target, commit.value)
	known := declaredKeys(schema)
	fired := map[string]bool{}
	queue := []string{commit.target}
	var changed []string
	for len(queue) > 0 {
		source := queue[0]
		queue = queue[1:]
		if fired[source] {
			continue
		}
		fired[source] = true
		props := Effective(schema, current)
		prop := props.Map[source]
		if prop == nil {
			continue
		}
		evalOpts := opts
		if opts.topLevel {
			evalOpts.root = current
		}
		for i, l := range prop.OnChange {
			bound := boundListener{listener: l, source: changeSource(source, i)}
			actions, err := bound.selectActions(current, known, evalOpts)
			if err != nil {
				return value, nil, err
			}
			for _, change := range valueChanges(actions, props) {
				old, had := current[change.target]
				if had == (change.value != nil) && jsonEqual(old, change.value) {
					continue
				}
				current = shallowCopy(current)
				writeKey(current, change.target, change.value)
				queue = append(queue, change.target)
				changed = append(changed, change.target)
			}
		}
	}
	return current, changed, nil
}

func writeKey(object map[string]any, key string, v any) {
	if v == nil {
		delete(object, key)
		return
	}
	object[key] = v
}

type valueChange struct {
	target string
	value  any
}

func valueChanges(actions ChangeActions, props Properties) []valueChange {
	var out []valueChange
	for _, target := range actions.Reset {
		var fallback any
		if prop := props.Map[target]; prop != nil && prop.Default != nil {
			fallback = cloneValue(prop.Default)
		}
		out = append(out, valueChange{target, fallback})
	}
	for _, target := range sortedKeys(actions.Set) {
		out = append(out, valueChange{target, cloneValue(actions.Set[target])})
	}
	return out
}

// selectActions validates both branches of a listener (so a schema error
// surfaces whichever branch is live) and returns the one that applies now.
func (b boundListener) selectActions(self map[string]any, known map[string]bool, opts listenerOptions) (ChangeActions, error) {
	l, source := b.listener, b.source
	if err := validateActions(source, l.ChangeActions, known); err != nil {
		return ChangeActions{}, err
	}
	if l.Else != nil {
		if err := validateActions(source, *l.Else, known); err != nil {
			return ChangeActions{}, err
		}
	}
	holds, err := listenerHolds(l, source, self, opts)
	if err != nil {
		return ChangeActions{}, err
	}
	if holds {
		return l.ChangeActions, nil
	}
	if l.Else != nil {
		return *l.Else, nil
	}
	return ChangeActions{}, nil
}

func listenerHolds(l Listener, source listenerSource, self map[string]any, opts listenerOptions) (bool, error) {
	if l.When == nil {
		return true, nil
	}
	var subject any = self
	present := true
	if !source.load {
		subject, present = self[source.key]
	}
	if l.When.Predicate == nil && !l.When.HasExpr {
		return false, fmt.Errorf("%s: when states nothing it can evaluate", source.label)
	}
	if l.When.Predicate != nil {
		holds, decided := evaluatePredicate(l.When.Predicate, subject, present)
		if !decided {
			return false, fmt.Errorf("%s: when states nothing it can evaluate", source.label)
		}
		if !holds {
			return false, nil
		}
	}
	if !l.When.HasExpr {
		return true, nil
	}
	if opts.evaluate == nil {
		return false, fmt.Errorf("%s: when.expr %q needs an expression evaluator", source.label, l.When.Expr)
	}
	result, err := opts.evaluate(ExprInput{Expr: l.When.Expr, Key: source.key, Value: subject, Self: self, Root: opts.root})
	if err != nil {
		return false, fmt.Errorf("%s: when.expr %q: %w", source.label, l.When.Expr, err)
	}
	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("%s: expression evaluator returned %T for %q, expected bool", source.label, result, l.When.Expr)
	}
	return b, nil
}

func validateActions(source listenerSource, actions ChangeActions, known map[string]bool) error {
	label := source.label
	setKeys := sortedKeys(actions.Set)
	local := map[string][]string{"require": actions.Require, "optional": actions.Optional, "reset": actions.Reset, "set": setKeys}
	for _, name := range []string{"require", "optional", "reset", "set"} {
		if source.load && len(local[name]) > 0 {
			return fmt.Errorf("%s: %s is not allowed in x-on-load, which only derives state (hide/show/enable/disable/patch)", label, name)
		}
		for _, target := range local[name] {
			if isPathTarget(target) {
				return fmt.Errorf("%s: %s %q cannot take a path; value and required actions act on sibling keys only", label, name, target)
			}
		}
	}
	all := [][]string{actions.Hide, actions.Show, actions.Enable, actions.Disable, actions.Require, actions.Optional, actions.Reset, setKeys}
	for _, targets := range all {
		for _, target := range targets {
			if err := validateTarget(label, target, known); err != nil {
				return err
			}
		}
	}
	pairs := []struct {
		a, b        string
		left, right []string
	}{
		{"hide", "show", actions.Hide, actions.Show},
		{"enable", "disable", actions.Enable, actions.Disable},
		{"require", "optional", actions.Require, actions.Optional},
		{"reset", "set", actions.Reset, setKeys},
	}
	for _, p := range pairs {
		for _, target := range p.left {
			if contains(p.right, target) {
				return fmt.Errorf("%s: both %s and %s %q", label, p.a, p.b, target)
			}
		}
	}
	return validatePatch(label, actions, known)
}

func validatePatch(label string, actions ChangeActions, known map[string]bool) error {
	for _, target := range sortedKeys(actions.Patch) {
		if err := validateTarget(label, target, known); err != nil {
			return err
		}
		var keywords map[string]json.RawMessage
		if err := json.Unmarshal(actions.Patch[target], &keywords); err != nil {
			return fmt.Errorf("%s: patch %q must be an object of keywords", label, target)
		}
		for _, keyword := range sortedKeys(keywords) {
			if reason, denied := patchDenied[keyword]; denied {
				return fmt.Errorf("%s: patch %q may not set %s (%s)", label, target, keyword, reason)
			}
		}
		verbs := []struct {
			name    string
			keyword string
			targets []string
		}{
			{"hide", "x-hidden", actions.Hide}, {"show", "x-hidden", actions.Show},
			{"disable", "x-disabled", actions.Disable}, {"enable", "x-disabled", actions.Enable},
		}
		for _, v := range verbs {
			if _, ok := keywords[v.keyword]; ok && contains(v.targets, target) {
				return fmt.Errorf("%s: both %s and patch %s on %q", label, v.name, v.keyword, target)
			}
		}
	}
	return nil
}

func validateTarget(label, target string, known map[string]bool) error {
	if isPathTarget(target) && known[target] {
		return fmt.Errorf("%s: target %q is both a declared key and a path", label, target)
	}
	segments := strings.Split(target, "/")
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("%s: target %q has an empty path segment", label, target)
		}
	}
	if !known[segments[0]] {
		return fmt.Errorf("%s: unknown target %q", label, target)
	}
	return nil
}

func isPathTarget(target string) bool { return strings.Contains(target, "/") }

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// patchTarget rewrites the property a target names. A plain key is replaced by
// apply(prop); a path (`sibling/child/…`) clones the sibling along the way,
// putting each patched child in a trailing unconditional allOf member, and an
// array is crossed into its items without spending a segment. A segment the
// current shape does not declare leaves the sibling untouched.
func patchTarget(props *Properties, value map[string]any, target string, apply func(*Schema) (*Schema, error)) error {
	segments := strings.Split(target, "/")
	prop := props.Map[segments[0]]
	if prop == nil {
		return nil
	}
	patched, err := patchNode(prop, value[segments[0]], segments[1:], apply)
	if err != nil || patched == nil {
		return err
	}
	props.Map[segments[0]] = patched
	return nil
}

func patchNode(node *Schema, nodeValue any, segments []string, apply func(*Schema) (*Schema, error)) (*Schema, error) {
	if len(segments) == 0 {
		return apply(node)
	}
	if node.Items != nil || node.Type.Has("array") {
		if node.Items == nil {
			return nil, nil
		}
		items, err := patchNode(node.Items, nil, segments, apply)
		if err != nil || items == nil {
			return nil, err
		}
		cp := *node
		cp.Items = items
		return &cp, nil
	}
	object, _ := asObject(nodeValue)
	child := Effective(node, object).Map[segments[0]]
	if child == nil {
		return nil, nil
	}
	patched, err := patchNode(child, object[segments[0]], segments[1:], apply)
	if err != nil || patched == nil {
		return nil, err
	}
	cp := *node
	cp.AllOf = append(append([]*Schema(nil), node.AllOf...), &Schema{
		Properties: map[string]*Schema{segments[0]: patched},
		order:      []string{segments[0]},
	})
	return &cp, nil
}
