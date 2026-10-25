package schemaform

import (
	"strings"
	"sync"
)

// State is the form's live value. Every huh field reads and writes it through
// an accessor bound to a path, so conditionals, listeners and hide funcs all
// see the same value the user is typing.
type State struct {
	mu       sync.Mutex
	schema   *Schema
	values   map[string]any
	evaluate Evaluator
	watchers map[string]func()
	// quiet suppresses commits while a field reloads its own options (huh's
	// Select.Options writes the highlighted option back through the accessor).
	quiet bool
	// rev increments on every commit; fields bind title/description funcs to
	// it so huh recomputes them when anything changes.
	rev int
	err error
}

// withoutCommits runs fn with commits suppressed.
func (s *State) withoutCommits(fn func()) {
	s.quiet = true
	defer func() { s.quiet = false }()
	fn()
}

func newState(schema *Schema, values map[string]any, evaluate Evaluator) *State {
	return &State{schema: schema, values: values, evaluate: evaluate, watchers: map[string]func(){}}
}

// Values returns the current value tree.
func (s *State) Values() map[string]any { return s.values }

// Err is the first schema/listener error raised while the form ran.
func (s *State) Err() error { return s.err }

func (s *State) fail(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

// snapshot is a deep copy of the values, safe to hand to a lookup that huh may
// run off the UI goroutine.
func (s *State) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, _ := cloneValue(s.values).(map[string]any)
	return out
}

func (s *State) get(path []string) any {
	v, _ := getPath(s.values, path)
	return v
}

// object returns the schema and effective properties (listener state
// applied) of the object at path, and whether the path is currently shown.
func (s *State) object(path []string) (*Schema, Properties, bool, error) {
	schema := s.schema
	opts := listenerOptions{root: s.values, evaluate: s.evaluate}
	props, err := applyListenerState(schema, s.values, opts)
	if err != nil {
		return nil, props, false, err
	}
	for i, seg := range path {
		child := props.Map[seg]
		if child == nil || child.Hidden {
			return nil, props, false, nil
		}
		value, _ := getPath(s.values, path[:i+1])
		object, _ := asObject(value)
		schema = child
		if props, err = applyListenerState(child, object, opts); err != nil {
			return nil, props, false, err
		}
	}
	return schema, props, true, nil
}

// field returns the effective schema of the field at path, whether it is
// required, and whether it is currently shown.
func (s *State) field(path []string) (*Schema, bool, bool) {
	_, props, ok, err := s.object(path[:len(path)-1])
	if err != nil {
		s.fail(err)
		return nil, false, false
	}
	key := path[len(path)-1]
	prop := props.Map[key]
	if !ok || prop == nil || prop.Hidden {
		return prop, false, false
	}
	return prop, props.IsRequired(key), true
}

// commit writes v at path (nil deletes) and runs the x-on-change value
// actions of the edited key, notifying the fields those actions changed.
func (s *State) commit(path []string, v any) {
	if s.quiet {
		return
	}
	s.mu.Lock()
	if jsonEqual(s.get(path), v) {
		_, had := getPath(s.values, path)
		if had == (v != nil) {
			s.mu.Unlock()
			return
		}
	}
	parentPath, key := path[:len(path)-1], path[len(path)-1]
	parent, _, _, err := s.object(parentPath)
	if err != nil || parent == nil {
		s.fail(err)
		setPath(s.values, path, v)
		s.rev++
		s.mu.Unlock()
		return
	}
	current, _ := asObject(s.get(parentPath))
	if current == nil {
		current = map[string]any{}
	}
	opts := listenerOptions{root: s.values, evaluate: s.evaluate, topLevel: len(parentPath) == 0}
	next, changed, err := applyChangeEffects(parent, current, valueChange{target: key, value: v}, opts)
	if err != nil {
		s.fail(err)
		next = shallowCopy(current)
		writeKey(next, key, v)
	}
	s.replaceObject(parentPath, next)
	s.rev++
	watchers := s.watchersUnder(parentPath, changed)
	s.mu.Unlock()
	for _, w := range watchers {
		w()
	}
}

func (s *State) replaceObject(path []string, next map[string]any) {
	if len(path) == 0 {
		for k := range s.values {
			delete(s.values, k)
		}
		for k, v := range next {
			s.values[k] = v
		}
		return
	}
	setPath(s.values, path, next)
}

// replace writes v at path (nil deletes) without running listeners: a slot
// installing the value for the schema it just loaded.
func (s *State) replace(path []string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	setPath(s.values, path, v)
	s.rev++
}

// watch registers the resync callback of the field at path.
func (s *State) watch(path []string, resync func()) { s.watchers[joinPath(path)] = resync }

// unwatch drops the callbacks of the fields at and under path.
func (s *State) unwatch(path []string) {
	prefix := joinPath(path)
	for p := range s.watchers {
		if p == prefix || strings.HasPrefix(p, prefix+".") {
			delete(s.watchers, p)
		}
	}
}

func (s *State) watchersUnder(parent []string, changed []string) []func() {
	var out []func()
	for _, key := range changed {
		prefix := joinPath(appendPath(parent, key))
		for p, w := range s.watchers {
			if p == prefix || strings.HasPrefix(p, prefix+".") {
				out = append(out, w)
			}
		}
	}
	return out
}

// prune drops values of keys some branch declares but the current shape does
// not (an inactive if/then branch), recursively through fixed objects.
func (s *State) prune() map[string]any {
	return pruneObject(s.schema, s.values, listenerOptions{root: s.values, evaluate: s.evaluate})
}

func pruneObject(schema *Schema, value map[string]any, opts listenerOptions) map[string]any {
	props, err := applyListenerState(schema, value, opts)
	if err != nil {
		return value
	}
	declared := declaredKeys(schema)
	out := map[string]any{}
	for k, v := range value {
		prop, active := props.Map[k]
		if declared[k] && !active {
			continue
		}
		if object, ok := asObject(v); ok && active && rendersAsObject(prop) && !isOpenStringMap(prop) {
			v = pruneObject(prop, object, opts)
		}
		out[k] = v
	}
	return out
}

// accessor adapts a typed huh value to the state at a path.
type accessor[T any] struct {
	get func() T
	set func(T)
}

func (a *accessor[T]) Get() T  { return a.get() }
func (a *accessor[T]) Set(v T) { a.set(v) }
