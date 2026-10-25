package schemaform

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/huh"
)

// field is one leaf control of the form, bound to a path in the State.
type field struct {
	st      *State
	path    []string
	key     string
	kind    Kind
	base    *Schema
	section string

	input  huh.Field
	note   *huh.Note
	resync func()
	// huh's accessible runner never evaluates TitleFunc / DescriptionFunc /
	// OptionsFunc, so before each accessible prompt retitle sets the live
	// title and description statically and refresh does the same for a
	// select's options, returning how many there are.
	retitle func()
	refresh func() int
	// check validates a candidate typed value against the current schema.
	check func(any) error
	// inline fields are edited in the inline form, which draws the label and
	// description itself: their huh editor is built without a title.
	inline bool
	// discriminator marks the object's x-discriminator key.
	discriminator bool
	// shown are the options the editor was last refreshed with.
	shown []Option
	// slot is the x-clicky-schema slot that built the field, if any.
	slot *schemaSlot
}

// titled binds the live title and description to a huh editor, unless the
// field is inline: huh draws a (blank) title line for any TitleFunc, even
// one returning "".
func titled[F interface {
	TitleFunc(func() string, any) F
	DescriptionFunc(func() string, any) F
}](f *field, editor F) F {
	if f.inline {
		return editor
	}
	return editor.TitleFunc(f.title, &f.st.rev).DescriptionFunc(f.description, &f.st.rev)
}

// label is the field's own label, without its sections.
func (f *field) label() string {
	s, _ := f.current()
	return s.Label(f.key)
}

// summary is the value as a blurred inline row shows it: an option's label,
// Yes/No, lists joined, maps as k=v.
func (f *field) summary() string {
	v := f.value()
	if isEmpty(v) {
		return "—"
	}
	switch t := v.(type) {
	case bool:
		if t {
			return "Yes"
		}
		return "No"
	case []any:
		parts := make([]string, len(t))
		for i, item := range t {
			parts[i] = f.optionLabel(item)
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		keys := sortedKeys(t)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + displayValue(t[k])
		}
		return strings.Join(parts, ", ")
	}
	return f.optionLabel(v)
}

// optionDescription is the description of the option holding value.
func (f *field) optionDescription(value string) string {
	for _, o := range f.shown {
		if o.Value == value {
			return o.Description
		}
	}
	return ""
}

// chosenDescription describes the single option the field holds, if any.
func (f *field) chosenDescription() string {
	switch v := f.value().(type) {
	case nil, []any, map[string]any:
		return ""
	default:
		return f.optionDescription(formatScalar(v))
	}
}

// hoveredDescription describes the option under an open select's cursor.
func (f *field) hoveredDescription() string {
	var hovered string
	var ok bool
	switch input := f.input.(type) {
	case *huh.Select[string]:
		hovered, ok = input.Hovered()
	case *huh.MultiSelect[string]:
		hovered, ok = input.Hovered()
	}
	if !ok {
		return ""
	}
	return f.optionDescription(hovered)
}

func (f *field) optionLabel(v any) string {
	for _, o := range f.shown {
		if o.Value == formatScalar(v) {
			return o.Label
		}
	}
	return displayValue(v)
}

// current is the field's live schema (patched by listeners), falling back to
// the schema it was built from while it is not shown.
func (f *field) current() (*Schema, bool) {
	s, required, visible := f.st.field(f.path)
	if !visible || s == nil {
		return f.base, required
	}
	return s, required
}

// visible reports whether the editable control is shown: the path is in the
// current shape, not hidden, resolves to this field's kind, and is neither
// readOnly nor disabled (those render the note instead).
func (f *field) visible(readonlyView bool) bool {
	s, _, visible := f.st.field(f.path)
	if !visible || s == nil || ResolveControl(s).Kind != f.kind {
		return false
	}
	return (s.ReadOnly || s.Disabled) == readonlyView
}

// title is the field label, prefixed with its enclosing object sections
// ("Owner › Email") since every field shares one page.
func (f *field) title() string {
	s, _ := f.current()
	return joinSection(f.section, s.Label(f.key))
}

func (f *field) description() string {
	s, _ := f.current()
	return s.Helper()
}

func (f *field) value() any { return f.st.get(f.path) }

func (f *field) commit(v any) { f.st.commit(f.path, v) }

// validate checks the value currently held in state.
func (f *field) validate() error { return f.check(f.value()) }

func (f *field) requiredCheck(v any) error {
	if _, required := f.current(); required && isEmpty(v) {
		return fmt.Errorf("%s is required", f.title())
	}
	return nil
}

func (f *field) buildNote() {
	f.note = huh.NewNote().
		TitleFunc(f.title, &f.st.rev).
		DescriptionFunc(f.noteValue, &f.st.rev)
}

func (f *field) noteValue() string { return displayValue(f.value()) }

func displayValue(v any) string {
	if v == nil {
		return "—"
	}
	switch t := v.(type) {
	case []any, map[string]any:
		raw, _ := json.Marshal(t)
		return string(raw)
	}
	return formatScalar(v)
}

// textCodec turns the text a user types into the typed value stored in state
// (parse) and back (format); invalid text is kept verbatim so the user can
// keep editing it, and check rejects it.
type textCodec struct {
	parse  func(string) (any, error)
	format func(any) string
}

func (f *field) textAccessor(c textCodec) *accessor[string] {
	return &accessor[string]{
		get: func() string {
			if s, ok := f.value().(string); ok {
				return s
			}
			return c.format(f.value())
		},
		set: func(text string) {
			if strings.TrimSpace(text) == "" {
				f.commit(nil)
				return
			}
			v, err := c.parse(text)
			if err != nil {
				f.commit(text)
				return
			}
			f.commit(v)
		},
	}
}

// textValidator validates typed text through the codec and check.
func (f *field) textValidator(c textCodec) func(string) error {
	return func(text string) error {
		if strings.TrimSpace(text) == "" {
			return f.check(nil)
		}
		v, err := c.parse(text)
		if err != nil {
			return err
		}
		return f.check(v)
	}
}

func (f *field) buildInput(c textCodec, placeholder string) *huh.Input {
	s, _ := f.current()
	in := titled(f, huh.NewInput()).
		Placeholder(placeholder).
		Validate(f.textValidator(c))
	if s.Format == "password" {
		in.EchoMode(huh.EchoModePassword)
	}
	acc := f.textAccessor(c)
	in.Accessor(acc)
	f.resync = func() { in.Accessor(acc) }
	f.retitle = func() { in.Title(f.title()).Description(f.description()) }
	f.input = in
	return in
}

func (f *field) buildText(c textCodec, placeholder string) *huh.Text {
	t := titled(f, huh.NewText()).
		Placeholder(placeholder).
		Validate(f.textValidator(c))
	acc := f.textAccessor(c)
	t.Accessor(acc)
	f.resync = func() { t.Accessor(acc) }
	f.retitle = func() { t.Title(f.title()).Description(f.description()) }
	f.input = t
	return t
}

// buildControl builds the huh field for f.kind.
func (f *field) buildControl(lookup *lookupBinding) error {
	switch f.kind {
	case KindString:
		f.check = f.checkString
		f.buildInput(stringCodec, f.placeholder(""))
	case KindTextarea, KindMarkdown:
		f.check = f.checkString
		f.buildText(stringCodec, f.placeholder(""))
	case KindDate:
		f.check = f.checkDate
		f.buildInput(stringCodec, f.placeholder(dateLayoutHint(f.base.Format)))
	case KindNumber:
		f.check = f.checkNumber
		f.buildInput(numberCodec(f.base), f.placeholder(""))
	case KindJSON:
		f.check = f.requiredCheck
		f.buildText(jsonCodec, f.placeholder("JSON"))
	case KindUnion:
		f.check = f.checkUnion
		f.buildInput(unionCodec(f.base), f.placeholder(""))
	case KindTags:
		f.check = f.checkTags
		f.buildText(tagsCodec(f.base.Items), f.placeholder("one per line, or comma separated"))
	case KindStringMap:
		f.check = f.checkMap
		f.buildText(mapCodec(f.base.AdditionalProperties), f.placeholder("key=value, one per line or comma separated"))
	case KindBoolean:
		f.check = func(any) error { return nil }
		f.buildConfirm()
	case KindEnum:
		f.check = f.checkEnum
		f.buildSelect(func() []Option { return ResolveControl(f.liveSchema()).Options })
	case KindMultiEnum:
		f.check = f.checkMultiEnum
		f.buildMultiSelect(func() []Option { return ResolveControl(f.liveSchema()).Options })
	case KindLookup:
		return f.buildLookup(lookup)
	default:
		return fmt.Errorf("%s: unsupported control %q", joinPath(f.path), f.kind)
	}
	return nil
}

func (f *field) liveSchema() *Schema {
	s, _ := f.current()
	return s
}

func (f *field) placeholder(fallback string) string {
	if f.base.Placeholder != "" {
		return f.base.Placeholder
	}
	return fallback
}

func (f *field) buildConfirm() {
	acc := &accessor[bool]{
		get: func() bool { b, _ := f.value().(bool); return b },
		set: func(b bool) { f.commit(b) },
	}
	c := titled(f, huh.NewConfirm()).
		Accessor(acc)
	f.resync = func() { c.Accessor(acc) }
	f.retitle = func() { c.Title(f.title()).Description(f.description()) }
	f.input = c
}

// huhOptions converts options, prepending a "(none)" choice to an optional
// single select so it can be left empty.
func (f *field) huhOptions(options []Option, single bool) []huh.Option[string] {
	var out []huh.Option[string]
	if _, required := f.current(); single && !required {
		out = append(out, huh.NewOption("(none)", ""))
	}
	for _, o := range options {
		out = append(out, huh.NewOption(o.Label, o.Value))
	}
	return out
}

func rawFor(options []Option, value string) any {
	for _, o := range options {
		if o.Value == value {
			return o.Raw
		}
	}
	return value
}

func (f *field) buildSelect(options func() []Option) {
	acc := &accessor[string]{
		get: func() string { return formatScalar(f.value()) },
		set: func(v string) {
			if v == "" {
				f.commit(nil)
				return
			}
			f.commit(rawFor(options(), v))
		},
	}
	sel := titled(f, huh.NewSelect[string]()).
		Validate(func(v string) error {
			if v == "" {
				return f.check(nil)
			}
			return f.check(rawFor(options(), v))
		})
	sel.Accessor(acc)
	f.resync = func() { sel.Accessor(acc) }
	f.retitle = func() { sel.Title(f.title()).Description(f.description()) }
	f.refresh = func() int {
		f.shown = options()
		opts := f.huhOptions(f.shown, true)
		f.st.withoutCommits(func() { sel.Options(opts...).Height(f.optionsHeight(len(opts))) })
		return len(opts)
	}
	f.refresh()
	f.input = sel
}

func (f *field) buildMultiSelect(options func() []Option) {
	acc := &accessor[[]string]{
		get: func() []string {
			items, _ := f.value().([]any)
			out := make([]string, len(items))
			for i, item := range items {
				out[i] = formatScalar(item)
			}
			return out
		},
		set: func(values []string) { f.commit(multiValue(options(), values)) },
	}
	ms := titled(f, huh.NewMultiSelect[string]()).
		Filterable(true).
		Validate(func(values []string) error { return f.check(multiValue(options(), values)) })
	if max := f.base.MaxItems; max != nil {
		ms.Limit(*max)
	}
	ms.Accessor(acc)
	f.resync = func() { ms.Accessor(acc) }
	f.retitle = func() { ms.Title(f.title()).Description(f.description()) }
	f.refresh = func() int {
		f.shown = options()
		opts := f.huhOptions(f.shown, false)
		f.st.withoutCommits(func() { ms.Options(opts...).Height(f.optionsHeight(len(opts))) })
		return len(opts)
	}
	f.refresh()
	f.input = ms
}

// maxVisibleOptions caps a select's list so a long one (every prompt, every
// model) scrolls instead of pushing the rest of the form off screen.
const maxVisibleOptions = 10

// optionsHeight is the huh field height (title and description included) that
// shows at most maxVisibleOptions rows; 0 lets huh fit every option.
func (f *field) optionsHeight(count int) int {
	switch {
	case count <= maxVisibleOptions:
		return 0
	case f.inline:
		return maxVisibleOptions
	}
	height := maxVisibleOptions + strings.Count(f.title(), "\n") + 1
	if description := f.description(); description != "" {
		height += strings.Count(description, "\n") + 1
	}
	return height
}

func multiValue(options []Option, values []string) any {
	if len(values) == 0 {
		return nil
	}
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = rawFor(options, v)
	}
	return out
}

var stringCodec = textCodec{
	parse:  func(s string) (any, error) { return s, nil },
	format: formatScalar,
}

func numberCodec(s *Schema) textCodec {
	typ := TypeList{"number"}
	if s.Type.Has("integer") {
		typ = TypeList{"integer"}
	}
	return textCodec{
		parse:  func(text string) (any, error) { return parseScalar(text, typ) },
		format: formatScalar,
	}
}

var jsonCodec = textCodec{
	parse: func(text string) (any, error) {
		var v any
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return v, nil
	},
	format: func(v any) string {
		raw, _ := json.MarshalIndent(v, "", "  ")
		return string(raw)
	},
}

// unionCodec reads a JSON literal (number, boolean, null, object, array) when
// it fits one of the union's branches, and the text as a string otherwise — so
// "123" stays a string in a string|object union.
func unionCodec(s *Schema) textCodec {
	return textCodec{
		parse: func(text string) (any, error) {
			var v any
			if err := json.Unmarshal([]byte(text), &v); err == nil && matchesAnyBranch(unionBranches(s), v) {
				return v, nil
			}
			return text, nil
		},
		format: unionFormat,
	}
}

func unionFormat(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

func tagsCodec(items *Schema) textCodec {
	typ := TypeList{"string"}
	if items != nil && len(items.Type) > 0 {
		typ = items.Type
	}
	return textCodec{
		parse: func(text string) (any, error) {
			parts := splitList(text)
			out := make([]any, len(parts))
			for i, part := range parts {
				v, err := parseScalar(part, typ)
				if err != nil {
					return nil, fmt.Errorf("item %d: %w", i+1, err)
				}
				out[i] = v
			}
			return out, nil
		},
		format: func(v any) string {
			items, _ := v.([]any)
			out := make([]string, len(items))
			for i, item := range items {
				out[i] = formatScalar(item)
			}
			return strings.Join(out, "\n")
		},
	}
}

func mapCodec(values *Schema) textCodec {
	typ := TypeList{"string"}
	if values != nil && len(values.Type) > 0 && scalarItemsType(values) != "" {
		typ = values.Type
	}
	return textCodec{
		parse: func(text string) (any, error) {
			out := map[string]any{}
			for _, line := range splitList(text) {
				k, raw, ok := strings.Cut(line, "=")
				if !ok || strings.TrimSpace(k) == "" {
					return nil, fmt.Errorf("%q is not key=value", line)
				}
				v, err := parseScalar(strings.TrimSpace(raw), typ)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", k, err)
				}
				out[strings.TrimSpace(k)] = v
			}
			return out, nil
		},
		format: func(v any) string {
			object, _ := asObject(v)
			lines := make([]string, 0, len(object))
			for _, k := range sortedKeys(object) {
				lines = append(lines, k+"="+formatScalar(object[k]))
			}
			return strings.Join(lines, "\n")
		},
	}
}

func (f *field) checkString(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("expected text, got %T", v)
	}
	schema := f.liveSchema()
	n := utf8.RuneCountInString(s)
	if schema.MinLength != nil && n < *schema.MinLength {
		return fmt.Errorf("must be at least %d characters", *schema.MinLength)
	}
	if schema.MaxLength != nil && n > *schema.MaxLength {
		return fmt.Errorf("must be at most %d characters", *schema.MaxLength)
	}
	if schema.Pattern != "" {
		re, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return fmt.Errorf("schema pattern %q: %w", schema.Pattern, err)
		}
		if !re.MatchString(s) {
			return fmt.Errorf("must match %s", schema.Pattern)
		}
	}
	return nil
}

var dateLayouts = map[string][]string{
	"date":      {time.DateOnly},
	"date-time": {time.RFC3339, "2006-01-02T15:04", time.DateTime},
}

func dateLayoutHint(format string) string {
	if format == "date" {
		return "YYYY-MM-DD"
	}
	return "YYYY-MM-DDTHH:MM:SSZ"
}

func (f *field) checkDate(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	s, _ := v.(string)
	format := f.liveSchema().Format
	if format == "" {
		format = f.base.Format
	}
	for _, layout := range dateLayouts[format] {
		if _, err := time.Parse(layout, s); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid %s (%s)", s, format, dateLayoutHint(format))
}

func (f *field) checkNumber(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	n, ok := v.(float64)
	if !ok {
		return fmt.Errorf("%q is not a number", formatScalar(v))
	}
	schema := f.liveSchema()
	if schema.Type.Has("integer") && n != math.Trunc(n) {
		return fmt.Errorf("%v is not an integer", n)
	}
	if schema.Minimum != nil && n < *schema.Minimum {
		return fmt.Errorf("must be ≥ %v", *schema.Minimum)
	}
	if schema.Maximum != nil && n > *schema.Maximum {
		return fmt.Errorf("must be ≤ %v", *schema.Maximum)
	}
	if m := schema.MultipleOf; m != nil && *m != 0 && math.Abs(math.Remainder(n, *m)) > 1e-9 {
		return fmt.Errorf("must be a multiple of %v", *m)
	}
	return nil
}

func (f *field) checkUnion(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	if matchesAnyBranch(unionBranches(f.liveSchema()), v) {
		return nil
	}
	return fmt.Errorf("%s matches none of the allowed shapes", displayValue(v))
}

func matchesAnyBranch(branches []*Schema, v any) bool {
	for _, branch := range branches {
		if matchesUnionBranch(branch, v) {
			return true
		}
	}
	return false
}

func (f *field) checkEnum(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	for _, o := range ResolveControl(f.liveSchema()).Options {
		if jsonEqual(o.Raw, v) {
			return nil
		}
	}
	return fmt.Errorf("%s is not one of the allowed options", displayValue(v))
}

func (f *field) checkItems(v any, check func(any) error) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("expected a list, got %T", v)
	}
	schema := f.liveSchema()
	if schema.MinItems != nil && len(items) < *schema.MinItems {
		return fmt.Errorf("needs at least %d items", *schema.MinItems)
	}
	if schema.MaxItems != nil && len(items) > *schema.MaxItems {
		return fmt.Errorf("allows at most %d items", *schema.MaxItems)
	}
	for i, item := range items {
		if schema.UniqueItems {
			for _, earlier := range items[:i] {
				if jsonEqual(earlier, item) {
					return fmt.Errorf("%s is listed twice", displayValue(item))
				}
			}
		}
		if err := check(item); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	return nil
}

func (f *field) checkTags(v any) error {
	return f.checkItems(v, func(item any) error {
		if _, ok := item.(string); ok && scalarItemsType(f.liveSchema().Items) != "string" {
			return fmt.Errorf("%q is not a %s", item, scalarItemsType(f.liveSchema().Items))
		}
		return nil
	})
}

func (f *field) checkMultiEnum(v any) error {
	options := ResolveControl(f.liveSchema()).Options
	return f.checkItems(v, func(item any) error {
		for _, o := range options {
			if jsonEqual(o.Raw, item) {
				return nil
			}
		}
		return fmt.Errorf("%s is not one of the allowed options", displayValue(item))
	})
}

func (f *field) checkMap(v any) error {
	if err := f.requiredCheck(v); err != nil || v == nil {
		return err
	}
	object, ok := asObject(v)
	if !ok {
		return fmt.Errorf("expected key=value lines, got %T", v)
	}
	schema := f.liveSchema()
	for _, k := range sortedKeys(object) {
		if names := schema.PropertyNames; names != nil && len(names.Enum) > 0 && !containsJSON(names.Enum, k) {
			return fmt.Errorf("key %q is not one of the allowed keys", k)
		}
		if len(schema.PatternProperties) > 0 && schema.AdditionalProperties == nil && !matchesAnyPattern(schema.PatternProperties, k) {
			return fmt.Errorf("key %q matches none of the allowed key patterns", k)
		}
	}
	return nil
}

func matchesAnyPattern(patterns map[string]*Schema, key string) bool {
	for pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil && re.MatchString(key) {
			return true
		}
	}
	return false
}
