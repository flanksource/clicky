package schemaform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// ErrNoInput is returned in accessible mode when the input ends before a
// required or invalid field has a valid answer.
var ErrNoInput = errors.New("input ended before the form was complete")

// Options configures Run.
type Options struct {
	Title       string
	Description string
	// Default pre-fills the form; schema defaults fill what it leaves out.
	Default map[string]any
	// Lookup resolves x-clicky-lookup fields; required when the schema has any.
	Lookup Lookup
	// LoadSchema loads the schema of x-clicky-schema properties; required when the
	// schema has any.
	LoadSchema SchemaLoader
	// Evaluate evaluates listener `when.expr`; nil uses FilterEvaluator.
	Evaluate Evaluator
	Input    io.Reader
	Output   io.Writer
	// Accessible runs a line-based prompt per field (screen readers, piped
	// input, tests) instead of the full-screen form.
	Accessible bool
	// RequiredFirst floats required fields up within each object.
	RequiredFirst bool
	Theme         *huh.Theme
}

// Run parses a JSON Schema and collects a value for it interactively. The
// result is JSON-shaped (numbers are float64), excludes empty values and the
// values of inactive branches, and satisfies every field's constraints.
func Run(ctx context.Context, raw json.RawMessage, opts Options) (map[string]any, error) {
	schema, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	return RunSchema(ctx, schema, opts)
}

// RunSchema is Run for an already parsed schema.
func RunSchema(ctx context.Context, schema *Schema, opts Options) (map[string]any, error) {
	if opts.Evaluate == nil {
		opts.Evaluate = FilterEvaluator()
	}
	if opts.Input == nil {
		opts.Input = os.Stdin
	}
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	preset, _ := normalize(opts.Default).(map[string]any)
	if preset == nil {
		preset = map[string]any{}
	}
	st := newState(schema, ApplyDefaults(schema, preset), opts.Evaluate)
	if _, _, _, err := st.object(nil); err != nil {
		return nil, err
	}
	b := &builder{st: st, lookup: &lookupBinding{ctx: ctx, fn: opts.Lookup, cache: map[string]LookupResult{}}, requiredFirst: opts.RequiredFirst, inline: !opts.Accessible, loadSchema: opts.LoadSchema}
	steps, err := b.build(schema, nil, "")
	if err != nil {
		return nil, err
	}
	if _, slots := expand(steps); len(slots) > 0 {
		if _, err := syncSlots(ctx, slots); err != nil {
			return nil, err
		}
	}
	r := &runner{ctx: ctx, st: st, opts: opts}
	if opts.Accessible {
		r.lines = &lineReader{r: opts.Input}
	}
	if err := r.run(steps); err != nil {
		return nil, err
	}
	if err := st.Err(); err != nil {
		return nil, err
	}
	fields, _ := expand(steps)
	for _, f := range fields {
		if f.visible(false) {
			if err := f.validate(); err != nil {
				return nil, fmt.Errorf("%s: %w", joinPath(f.path), err)
			}
		}
	}
	return st.prune(), nil
}

type runner struct {
	ctx   context.Context
	st    *State
	opts  Options
	lines *lineReader
}

func (r *runner) run(steps []any) error {
	header := r.opts.Title != "" || r.opts.Description != ""
	for _, segment := range segments(steps) {
		if list, ok := segment[0].(*objectList); ok {
			if err := r.objectList(list); err != nil {
				return err
			}
			continue
		}
		if err := r.fields(segment, header); err != nil {
			return err
		}
		header = false
	}
	return nil
}

// fields runs a segment of fields and x-clicky-schema slots.
func (r *runner) fields(steps []any, header bool) error {
	if r.lines != nil {
		return r.accessibleFields(steps, header)
	}
	opts := inlineFormOptions{theme: r.opts.Theme}
	if header {
		opts.title, opts.description = r.opts.Title, r.opts.Description
	}
	form := newInlineForm(r.ctx, steps, opts)
	program := tea.NewProgram(form, tea.WithInput(r.opts.Input), tea.WithOutput(r.opts.Output), tea.WithContext(r.ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("form cancelled: %w", err)
	}
	switch {
	case form.failure != nil:
		return form.failure
	case form.aborted:
		return fmt.Errorf("form cancelled: %w", huh.ErrUserAborted)
	}
	return nil
}

func (r *runner) runForm(theme *huh.Theme, groups ...*huh.Group) error {
	form := huh.NewForm(groups...).WithInput(r.opts.Input).WithOutput(r.opts.Output).WithShowHelp(true)
	if theme != nil {
		form = form.WithTheme(theme)
	}
	if err := form.RunWithContext(r.ctx); err != nil {
		return fmt.Errorf("form cancelled: %w", err)
	}
	return nil
}

// accessibleFields asks each shown field in turn. huh's own accessible runner
// ignores hide funcs and drops field errors, so the runner drives the fields
// itself, re-asking a field until its answer validates.
func (r *runner) accessibleFields(steps []any, header bool) error {
	w := r.opts.Output
	if header {
		_, _ = fmt.Fprintf(w, "%s\n%s\n\n", r.opts.Title, r.opts.Description)
	}
	var fields []*field
	var slots []*schemaSlot
	for _, step := range steps {
		switch t := step.(type) {
		case *field:
			fields = append(fields, t)
		case *schemaSlot:
			slots = append(slots, t)
		}
	}
	primary, folded := accessibleSplit(fields)
	for _, f := range primary {
		if err := r.askField(f); err != nil {
			return err
		}
		if _, err := syncSlots(r.ctx, slots); err != nil {
			return err
		}
	}
	// A slot's fields (loaded for the answers so far) follow the fields asked
	// up front.
	for _, s := range slots {
		for _, f := range s.fields {
			if err := r.askField(f); err != nil {
				return err
			}
		}
	}
	// A folded field a branch or listener has made required is asked anyway,
	// until an answer requires no further one.
	asked := map[*field]bool{}
	for progress := true; progress; {
		progress = false
		for _, f := range folded {
			if asked[f] || !required(f) || !f.visible(false) {
				continue
			}
			asked[f], progress = true, true
			if err := r.askField(f); err != nil {
				return err
			}
		}
	}
	var optional []*field
	for _, f := range folded {
		if !asked[f] {
			optional = append(optional, f)
		}
	}
	shown := 0
	for _, f := range optional {
		if f.visible(false) || f.visible(true) {
			shown++
		}
	}
	if shown == 0 {
		return nil
	}
	noun := "options"
	if shown == 1 {
		noun = "option"
	}
	advanced, err := r.confirm(fmt.Sprintf("Configure %d advanced %s?", shown, noun), false)
	if err != nil || !advanced {
		return err
	}
	for _, f := range optional {
		if err := r.askField(f); err != nil {
			return err
		}
	}
	return nil
}

// accessibleSplit is the inline form's split without a screen to fill: the
// discriminator, required fields and fields with a value are asked first;
// the rest, and fields not shown yet, fold. Both keep document order, so a
// line-based session reads top to bottom.
func accessibleSplit(fields []*field) (primary, folded []*field) {
	for _, f := range fields {
		shown := f.visible(false) || f.visible(true)
		if row := splitRowFor(f); shown && (row.pinned() || row.hasValue) {
			primary = append(primary, f)
		} else {
			folded = append(folded, f)
		}
	}
	return primary, folded
}

// askField asks one field if it is shown: a readOnly/disabled one prints its
// note, an editable one is asked until its answer validates.
func (r *runner) askField(f *field) error {
	w := r.opts.Output
	if f.visible(true) {
		f.note.Title(f.title()).Description(f.noteValue())
		if err := f.note.RunAccessible(w, r.lines); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w)
		return nil
	}
	if !f.visible(false) {
		return nil
	}
	f.retitle()
	if f.refresh != nil && f.refresh() == 0 {
		return fmt.Errorf("%s: no options to choose from", joinPath(f.path))
	}
	if err := r.askAccessible(f.input, f.validate); err != nil {
		return fmt.Errorf("%s: %w", joinPath(f.path), err)
	}
	return nil
}

func (r *runner) askAccessible(input huh.Field, validate func() error) error {
	w := r.opts.Output
	for {
		input.Init()
		input.Focus()
		if err := input.RunAccessible(w, r.lines); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w)
		err := validate()
		if err == nil {
			return nil
		}
		if r.lines.eof {
			return fmt.Errorf("%w: %v", ErrNoInput, err)
		}
		_, _ = fmt.Fprintln(w, err)
	}
}

func (r *runner) confirm(title string, value bool) (bool, error) {
	c := huh.NewConfirm().Title(title).Value(&value)
	if r.lines != nil {
		return value, r.askAccessible(c, func() error { return nil })
	}
	return value, r.runForm(r.opts.Theme, huh.NewGroup(c))
}

// objectList collects an array of objects: keep (or drop) the existing items,
// then add items one sub-form at a time within minItems/maxItems.
func (r *runner) objectList(l *objectList) error {
	s, _, visible := r.st.field(l.path)
	if !visible || ResolveControl(s).Kind != KindObjectList {
		return nil
	}
	noun, plural := itemNoun(s)
	items, _ := r.st.get(l.path).([]any)
	label := joinSection(l.section, s.Label(l.key))
	if len(items) > 0 {
		keep, err := r.confirm(fmt.Sprintf("%s: keep %d existing %s?", label, len(items), plural), true)
		if err != nil {
			return err
		}
		if !keep {
			items = nil
		}
	}
	for s.MaxItems == nil || len(items) < *s.MaxItems {
		if s.MinItems == nil || len(items) >= *s.MinItems {
			add, err := r.confirm(fmt.Sprintf("%s: add a %s?", label, noun), false)
			if err != nil {
				return err
			}
			if !add {
				break
			}
		}
		sub := r.opts
		sub.Default, sub.Title, sub.Description = nil, fmt.Sprintf("%s › %s %d", label, noun, len(items)+1), ""
		if r.lines != nil {
			sub.Input = r.lines
		}
		item, err := RunSchema(r.ctx, s.Items, sub)
		if err != nil {
			return err
		}
		items = append(items, item)
		_, _ = fmt.Fprintf(r.opts.Output, "added %s\n", itemTitle(s.Item, item, len(items)-1))
	}
	if len(items) == 0 {
		r.st.commit(l.path, nil)
		return nil
	}
	r.st.commit(l.path, items)
	return nil
}

// lineReader hands out one line per Read. huh's accessible prompts each wrap
// the input in a fresh bufio.Scanner, which would otherwise swallow the lines
// meant for the prompts after it. eof records that the input ran out.
type lineReader struct {
	r   io.Reader
	eof bool
}

func (l *lineReader) Read(p []byte) (int, error) {
	if l.eof {
		return 0, io.EOF
	}
	n := 0
	var b [1]byte
	for n < len(p) {
		read, err := l.r.Read(b[:])
		if read == 1 {
			p[n] = b[0]
			n++
			if b[0] == '\n' {
				return n, nil
			}
		}
		if err != nil {
			l.eof = true
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
	}
	return n, nil
}
