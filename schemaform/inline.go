package schemaform

import (
	"context"
	"reflect"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// inlineForm is the terminal form: every field is one line showing its value,
// and only the focused field opens its huh editor. Fields that do not fit on
// screen (and are not required) fold under an Advanced row.
type inlineForm struct {
	ctx                context.Context
	title, description string
	theme              *huh.Theme
	editors            *huh.Theme
	keymap             *huh.KeyMap
	// steps are the fields and x-clicky-schema slots; fields expands them,
	// a slot contributing the fields of the schema it has loaded.
	steps  []any
	slots  []*schemaSlot
	fields []*field
	keys   inlineKeys
	help   help.Model
	// failure is a slot's schema load that failed: it ends the form.
	failure error

	// order is the split's display order: shown rows first (priority order),
	// then folded ones (document order). folded marks the latter.
	order    []*field
	folded   map[*field]bool
	expanded bool

	cursor row
	// editing is set while a multi-line editor (select, multi-select, text)
	// is open: those open on enter, single-line ones on focus.
	editing bool
	// quitting is set by an esc with nothing to close or revert; a second esc
	// in a row cancels the form.
	quitting bool
	// entry is the focused field's value when it got focus, for esc.
	entry any
	errs  map[*field]error

	width, height, offset int
	done, aborted         bool
}

type rowKind int

const (
	rowField rowKind = iota
	rowAdvanced
	rowSubmit
)

type row struct {
	kind rowKind
	f    *field
}

type inlineKeys struct {
	Next, Prev, Up, Down, Toggle, Edit, Enter, Esc, Submit, Quit key.Binding
}

func newInlineKeys() inlineKeys {
	return inlineKeys{
		Next:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Prev:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back")),
		Up:     key.NewBinding(key.WithKeys("up")),
		Down:   key.NewBinding(key.WithKeys("down")),
		Toggle: key.NewBinding(key.WithKeys("enter", " ", "right", "left"), key.WithHelp("enter", "expand")),
		Edit:   key.NewBinding(key.WithKeys("enter", " ", "right"), key.WithHelp("enter", "edit")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
		Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc esc", "quit")),
		Submit: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit")),
		Quit:   key.NewBinding(key.WithKeys("ctrl+c")),
	}
}

// defaultWidth and defaultHeight size the form until the terminal reports
// its size.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

type inlineFormOptions struct {
	title, description string
	theme              *huh.Theme
}

// newInlineForm lays out steps (fields and x-clicky-schema slots, already
// synced once).
func newInlineForm(ctx context.Context, steps []any, opts inlineFormOptions) *inlineForm {
	theme := opts.theme
	if theme == nil {
		theme = huh.ThemeCharm()
	}
	h := help.New()
	h.Styles = theme.Help
	fields, slots := expand(steps)
	m := &inlineForm{
		ctx: ctx, title: opts.title, description: opts.description, theme: theme, steps: steps, slots: slots,
		editors: inlineTheme(theme), keymap: inlineKeyMap(),
		keys: newInlineKeys(), help: h, errs: map[*field]error{},
	}
	for _, f := range fields {
		m.prepare(f)
	}
	m.fields = fields
	m.resize(defaultWidth, defaultHeight)
	m.cursor = m.firstFocusable(m.rows())
	return m
}

// prepare dresses a field's editor for the form.
func (m *inlineForm) prepare(f *field) {
	f.input = f.input.WithTheme(m.editors).WithKeyMap(m.keymap).WithWidth(m.editorWidth(f))
}

// syncSlots reloads the slots whose scope changed and lays out the fields they
// rebuilt; a load that fails ends the form with its error.
func (m *inlineForm) syncSlots() tea.Cmd {
	if len(m.slots) == 0 {
		return nil
	}
	changed, err := syncSlots(m.ctx, m.slots)
	if err != nil {
		m.failure = err
		return tea.Quit
	}
	if !changed {
		return nil
	}
	known := map[*field]bool{}
	for _, f := range m.fields {
		known[f] = true
	}
	fields, _ := expand(m.steps)
	var cmds []tea.Cmd
	for _, f := range fields {
		if !known[f] {
			m.prepare(f)
			cmds = append(cmds, f.input.Init())
		}
	}
	m.fields = fields
	return tea.Batch(append(cmds, m.keepCursor())...)
}

// inlineKeyMap is huh's keymap without the editors' own submit: enter moves
// on (and submits from the last field) through the form, which also owns
// ctrl+s.
func inlineKeyMap() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	for _, submit := range []*key.Binding{
		&keymap.Input.Submit, &keymap.Text.Submit, &keymap.Select.Submit,
		&keymap.MultiSelect.Submit, &keymap.Confirm.Submit,
	} {
		submit.SetEnabled(false)
	}
	return keymap
}

// inlineTheme is theme without the field frame (huh's left bar and padding):
// an editor sits inside a row the form draws.
func inlineTheme(theme *huh.Theme) *huh.Theme {
	t := *theme
	t.Focused.Base = lipgloss.NewStyle()
	t.Blurred.Base = lipgloss.NewStyle()
	return &t
}

func (m *inlineForm) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.fields)+1)
	for _, f := range m.fields {
		cmds = append(cmds, f.input.Init())
	}
	return tea.Batch(append(cmds, m.enter(m.cursor))...)
}

func (m *inlineForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if cmd, handled := m.key(msg); handled {
			return m, cmd
		}
	}
	switch msg {
	case huh.NextField():
		// Synced before deciding: the answer may load fields that follow it.
		if synced := m.syncSlots(); m.failure != nil {
			return m, synced
		}
		if next := m.step(1); next.kind == rowSubmit {
			return m, m.submit()
		}
		return m, m.move(1)
	case huh.PrevField():
		return m, m.move(-1)
	}
	if m.cursor.kind != rowField {
		return m, nil
	}
	model, cmd := m.cursor.f.input.Update(msg)
	m.cursor.f.input = model.(huh.Field)
	return m, tea.Batch(cmd, m.keepCursor())
}

// key handles the form's own keys; the rest go to the focused editor.
func (m *inlineForm) key(msg tea.KeyMsg) (tea.Cmd, bool) {
	if key.Matches(msg, m.keys.Esc) {
		return m.escape(), true
	}
	m.quitting = false
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.aborted = true
		return tea.Quit, true
	case key.Matches(msg, m.keys.Submit):
		return m.submit(), true
	case key.Matches(msg, m.keys.Next):
		return m.move(1), true
	case key.Matches(msg, m.keys.Prev):
		return m.move(-1), true
	}
	vertical := key.Matches(msg, m.keys.Up, m.keys.Down)
	delta := 1
	if key.Matches(msg, m.keys.Up) {
		delta = -1
	}
	switch m.cursor.kind {
	case rowAdvanced:
		if key.Matches(msg, m.keys.Toggle) {
			m.expanded = !m.expanded
			return nil, true
		}
		if vertical {
			return m.move(delta), true
		}
		return nil, true
	case rowSubmit:
		if key.Matches(msg, m.keys.Enter) {
			return m.submit(), true
		}
		if vertical {
			return m.move(delta), true
		}
		return nil, true
	}
	f := m.cursor.f
	switch {
	case singleLine(f) || m.editing:
		// An open editor keeps ↑/↓ unless it is a single line.
		if vertical && singleLine(f) {
			return m.move(delta), true
		}
		return nil, false
	case key.Matches(msg, m.keys.Edit):
		m.editing = true
		return f.input.Focus(), true
	case vertical:
		return m.move(delta), true
	}
	// A closed multi-line row takes no typing.
	return nil, true
}

// escape closes an open editor, else reverts a changed field; with nothing
// to close or revert, it asks for a second esc, which cancels the form.
func (m *inlineForm) escape() tea.Cmd {
	if m.quitting {
		m.aborted = true
		return tea.Quit
	}
	if m.cursor.kind == rowField {
		f := m.cursor.f
		changed := !reflect.DeepEqual(f.value(), m.entry)
		if changed {
			f.commit(m.entry)
			f.resync()
		}
		if m.editing {
			m.editing = false
			return f.input.Blur()
		}
		if changed {
			return nil
		}
	}
	m.quitting = true
	return nil
}

// singleLine editors open on focus and have no use for ↑/↓, so those move
// between rows; multi-line ones open on enter.
func singleLine(f *field) bool {
	switch f.input.(type) {
	case *huh.Input, *huh.Confirm:
		return true
	}
	return false
}

// resize re-splits the fields for a terminal of width × height.
func (m *inlineForm) resize(width, height int) {
	m.width, m.height = width, height
	for _, f := range m.fields {
		f.input = f.input.WithWidth(m.editorWidth(f))
	}
	m.split()
}

// split folds what does not fit under Advanced. Only shown fields take room;
// a field hidden now (a branch, a listener) folds, and shows up front once it
// is required (promoted in rows).
func (m *inlineForm) split() {
	var shown []*field
	m.folded = map[*field]bool{}
	for _, f := range m.fields {
		if f.slot != nil {
			continue // placed after its scope field by rows
		}
		if m.shown(f) {
			shown = append(shown, f)
		} else {
			m.folded[f] = true
		}
	}
	rows := make([]splitRow, len(shown))
	for i, f := range shown {
		rows[i] = splitRowFor(f)
	}
	primary, advanced := splitAdvanced(rows, m.budget())
	m.order = m.order[:0]
	for _, i := range primary {
		m.order = append(m.order, shown[i])
	}
	for _, i := range advanced {
		m.folded[shown[i]] = true
	}
	for _, f := range m.fields {
		if m.folded[f] {
			m.order = append(m.order, f)
		}
	}
}

// placeSlots puts each slot's fields right after its scope field (the end of
// the shown rows when that field is folded): a prompt's variables follow the
// prompt.
func (m *inlineForm) placeSlots(primary []*field) []*field {
	for _, s := range m.slots {
		at := len(primary)
		for i, p := range primary {
			for _, name := range s.spec.Scope {
				if joinPath(p.path) == joinPath(s.scopePath(name)) {
					at = i + 1
				}
			}
		}
		var shown []*field
		for _, f := range s.fields {
			if m.shown(f) {
				shown = append(shown, f)
			}
		}
		primary = append(primary[:at], append(shown, primary[at:]...)...)
	}
	return primary
}

// budget is how many field rows fit: the height less the header, the footer
// (blank, Advanced, blank, Submit, blank, help) and room for an open select.
func (m *inlineForm) budget() int {
	return m.height - len(m.headerLines()) - 6 - (maxVisibleOptions + 2)
}

func (m *inlineForm) shown(f *field) bool { return f.visible(false) || f.visible(true) }

func required(f *field) bool {
	_, req := f.current()
	return req
}

// rows lays the form out: shown fields, the Advanced toggle, folded fields
// when expanded, Submit. A folded field that is required now (a listener's
// require, a chosen branch) is promoted after the shown field before it in
// document order.
func (m *inlineForm) rows() []row {
	var primary, folded []*field
	for _, f := range m.order {
		switch {
		case !m.shown(f):
		case m.folded[f] && !required(f):
			folded = append(folded, f)
		case m.folded[f]:
			primary = m.promote(primary, f)
		default:
			primary = append(primary, f)
		}
	}
	primary = m.placeSlots(primary)
	rows := make([]row, 0, len(primary)+len(folded)+2)
	for _, f := range primary {
		rows = append(rows, row{kind: rowField, f: f})
	}
	if len(folded) > 0 {
		rows = append(rows, row{kind: rowAdvanced})
		if m.expanded {
			for _, f := range folded {
				rows = append(rows, row{kind: rowField, f: f})
			}
		}
	}
	return append(rows, row{kind: rowSubmit})
}

func (m *inlineForm) promote(primary []*field, f *field) []*field {
	at := 0
	for i, p := range primary {
		if m.docIndex(p) < m.docIndex(f) {
			at = i + 1
		}
	}
	return append(primary[:at], append([]*field{f}, primary[at:]...)...)
}

func (m *inlineForm) docIndex(f *field) int {
	for i, d := range m.fields {
		if d == f {
			return i
		}
	}
	return -1
}

func (r row) focusable() bool { return r.kind != rowField || r.f.visible(false) }

func (m *inlineForm) firstFocusable(rows []row) row {
	for _, r := range rows {
		if r.focusable() {
			return r
		}
	}
	return rows[len(rows)-1]
}

func cursorIndex(rows []row, cursor row) int {
	for i, r := range rows {
		if r == cursor {
			return i
		}
	}
	return -1
}

// step is the focusable row delta rows away from the cursor, or the cursor
// itself at either end.
func (m *inlineForm) step(delta int) row {
	rows := m.rows()
	i := cursorIndex(rows, m.cursor)
	if i < 0 {
		return m.firstFocusable(rows)
	}
	for j := i + delta; j >= 0 && j < len(rows); j += delta {
		if rows[j].focusable() {
			return rows[j]
		}
	}
	return m.cursor
}

// move syncs the slots first, so a field the answer just loaded (a prompt's
// variables after the prompt) is the row it moves to.
func (m *inlineForm) move(delta int) tea.Cmd {
	synced := m.syncSlots()
	if m.failure != nil {
		return synced
	}
	next := m.step(delta)
	if next == m.cursor {
		return synced
	}
	return tea.Batch(synced, m.focus(next))
}

// focus leaves the current row (blurring and validating its field, without
// blocking on an error) and enters next.
func (m *inlineForm) focus(next row) tea.Cmd {
	leave := m.leave()
	m.cursor = next
	return tea.Batch(leave, m.enter(next))
}

func (m *inlineForm) leave() tea.Cmd {
	if m.cursor.kind != rowField {
		return nil
	}
	f := m.cursor.f
	cmd := f.input.Blur()
	m.editing = false
	m.errs[f] = f.validate()
	return cmd
}

func (m *inlineForm) enter(r row) tea.Cmd {
	if r.kind != rowField {
		return nil
	}
	if r.f.refresh != nil {
		r.f.refresh()
	}
	m.entry = r.f.value()
	if !singleLine(r.f) {
		return nil
	}
	return r.f.input.Focus()
}

// keepCursor moves off a field an answer just hid (a listener, a branch).
func (m *inlineForm) keepCursor() tea.Cmd {
	rows := m.rows()
	if i := cursorIndex(rows, m.cursor); i >= 0 && rows[i].focusable() {
		return nil
	}
	m.cursor = m.firstFocusable(rows)
	return m.enter(m.cursor)
}

// submit validates every shown field. The first invalid one gets focus (with
// Advanced expanded if it is folded); with none, the form is done.
func (m *inlineForm) submit() tea.Cmd {
	leave := m.leave()
	if synced := m.syncSlots(); m.failure != nil {
		return synced
	}
	var invalid *field
	for _, f := range m.fields {
		if !f.visible(false) {
			continue
		}
		m.errs[f] = f.validate()
		if m.errs[f] != nil && invalid == nil {
			invalid = f
		}
	}
	if invalid == nil {
		m.done = true
		return tea.Sequence(leave, tea.Quit)
	}
	if m.folded[invalid] && !required(invalid) {
		m.expanded = true
	}
	m.cursor = row{kind: rowField, f: invalid}
	return tea.Batch(leave, m.enter(m.cursor))
}
