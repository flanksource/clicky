package schemaform

// Ported from clicky-ui json-schema-form-conditionals.ts.

// Properties is the effective shape of an object: its properties in render
// order and the keys currently required.
type Properties struct {
	Keys     []string
	Map      map[string]*Schema
	Required []string
}

func newProperties(schema *Schema) Properties {
	p := Properties{Map: map[string]*Schema{}}
	for _, k := range schema.PropertyOrder() {
		p.set(k, schema.Properties[k])
	}
	p.Required = append(p.Required, schema.Required...)
	return p
}

func (p *Properties) set(key string, s *Schema) {
	if _, ok := p.Map[key]; !ok {
		p.Keys = append(p.Keys, key)
	}
	p.Map[key] = s
}

func (p *Properties) require(key string) {
	if !p.IsRequired(key) {
		p.Required = append(p.Required, key)
	}
}

func (p *Properties) optional(key string) {
	out := p.Required[:0:0]
	for _, k := range p.Required {
		if k != key {
			out = append(out, k)
		}
	}
	p.Required = out
}

// IsRequired reports whether key is currently required.
func (p Properties) IsRequired(key string) bool {
	for _, k := range p.Required {
		if k == key {
			return true
		}
	}
	return false
}

// evaluatePredicate answers whether an `if.properties[k]` sub-schema holds for
// the value at that key. decided is false when the sub-schema states nothing
// the form can check, so the caller fails closed. present distinguishes an
// absent key from an explicit null.
func evaluatePredicate(sub *Schema, value any, present bool) (holds, decided bool) {
	holds = true
	if sub.HasConst() {
		decided = true
		if !present || !jsonEqual(value, sub.Const) {
			holds = false
		}
	}
	if sub.Enum != nil {
		decided = true
		if !present || !containsJSON(sub.Enum, value) {
			holds = false
		}
	}
	if sub.Not != nil {
		inner, innerDecided := evaluatePredicate(sub.Not, value, present)
		if !innerDecided {
			return false, false
		}
		decided = true
		if inner {
			holds = false
		}
	}
	if sub.Properties != nil || sub.Required != nil {
		decided = true
		object, ok := asObject(value)
		if !ok || !matchesIf(sub, object) {
			holds = false
		}
	}
	return holds && decided, decided
}

func containsJSON(options []any, value any) bool {
	for _, option := range options {
		if jsonEqual(option, value) {
			return true
		}
	}
	return false
}

// matchesIf reports whether the `if` sub-schema holds: every `if.required` key
// is present and every `if.properties[k]` predicate holds. A predicate the form
// cannot evaluate makes the whole `if` false, never true.
func matchesIf(ifSchema *Schema, value map[string]any) bool {
	if ifSchema == nil {
		return false
	}
	for _, k := range ifSchema.Required {
		if _, ok := value[k]; !ok {
			return false
		}
	}
	for _, k := range ifSchema.PropertyOrder() {
		v, present := value[k]
		if holds, decided := evaluatePredicate(ifSchema.Properties[k], v, present); !decided || !holds {
			return false
		}
	}
	return true
}

func isConditional(clause *Schema) bool { return clause.If != nil || clause.Then != nil }

// Effective merges the schema's base properties with each `allOf` member's
// contribution: an if/then member contributes `then` when its `if` matches the
// current value and `else` otherwise; an unconditional member always
// contributes. Later members win on key collision (replacing the property
// wholesale); required is the union.
func Effective(schema *Schema, value map[string]any) Properties {
	p := newProperties(schema)
	merge := func(branch *Schema) {
		if branch == nil {
			return
		}
		for _, k := range branch.PropertyOrder() {
			p.set(k, branch.Properties[k])
		}
		for _, k := range branch.Required {
			p.require(k)
		}
	}
	for _, clause := range schema.AllOf {
		if !isConditional(clause) {
			merge(clause)
			continue
		}
		if matchesIf(clause.If, value) {
			merge(clause.Then)
		} else {
			merge(clause.Else)
		}
	}
	return p
}

// declaredKeys is every key the schema can declare in any branch — a listener
// may target a field that only exists while some branch is live.
func declaredKeys(schema *Schema) map[string]bool {
	keys := map[string]bool{}
	for k := range schema.Properties {
		keys[k] = true
	}
	for _, clause := range schema.AllOf {
		for _, branch := range []*Schema{clause, clause.Then, clause.Else} {
			if branch == nil {
				continue
			}
			for k := range branch.Properties {
				keys[k] = true
			}
		}
	}
	return keys
}

// candidates returns, in first-declared order, every key the object can show
// and every schema that key can take across its branches.
func candidates(schema *Schema) ([]string, map[string][]*Schema) {
	var keys []string
	out := map[string][]*Schema{}
	add := func(branch *Schema) {
		if branch == nil {
			return
		}
		for _, k := range branch.PropertyOrder() {
			if _, ok := out[k]; !ok {
				keys = append(keys, k)
			}
			out[k] = append(out[k], branch.Properties[k])
		}
	}
	add(schema)
	for _, clause := range schema.AllOf {
		add(clause)
		add(clause.Then)
		add(clause.Else)
	}
	return keys, out
}
