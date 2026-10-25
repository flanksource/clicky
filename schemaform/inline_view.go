package schemaform

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const (
	marker        = "› "
	maxLabelWidth = 24
	// maxDescriptionLines caps the highlighted option's description.
	maxDescriptionLines = 3
	// foldIndent indents the fields under an expanded Advanced row.
	foldIndent = "  "
)

func (m *inlineForm) View() string {
	if m.aborted {
		return ""
	}
	header := m.headerLines()
	if m.done {
		body, _, _ := m.bodyLines(false)
		return strings.Join(append(header, body...), "\n") + "\n"
	}
	body, from, to := m.bodyLines(true)
	footer := []string{"", m.helpView()}
	room := m.height - len(header) - len(footer)
	if room > 0 && len(body) > room {
		m.offset = min(max(m.offset, to-room), from)
		m.offset = max(0, min(m.offset, len(body)-room))
		body = body[m.offset : m.offset+room]
	}
	lines := append(append(header, body...), footer...)
	return strings.Join(lines, "\n")
}

func (m *inlineForm) headerLines() []string {
	var lines []string
	if m.title != "" {
		lines = append(lines, m.theme.Focused.Title.Render(m.title))
	}
	if m.description != "" {
		lines = append(lines, m.theme.Focused.Description.Render(m.description))
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return lines
}

// bodyLines renders the rows, and the line range the cursor's row takes so
// the scroll window can keep it visible. Without live, every row is blurred
// and the Submit row is left out (the record left behind once the form is
// done).
func (m *inlineForm) bodyLines(live bool) (lines []string, from, to int) {
	width := m.labelWidth()
	section := ""
	for _, r := range m.rows() {
		focused := live && r == m.cursor
		start := len(lines)
		switch r.kind {
		case rowField:
			indent := ""
			if m.folded[r.f] && !required(r.f) {
				indent = foldIndent
			}
			if r.f.section != section && r.f.section != "" {
				lines = append(lines, indent+m.theme.Blurred.Title.Render(r.f.section))
			}
			section = r.f.section
			lines = append(lines, m.fieldLines(r.f, focused, indent, width)...)
		case rowAdvanced:
			lines = append(lines, m.advancedLine(focused))
		case rowSubmit:
			if !live {
				continue
			}
			lines = append(lines, "", m.submitLine(focused))
		}
		if focused {
			from, to = start, len(lines)
		}
	}
	return lines, from, to
}

func (m *inlineForm) labelWidth() int {
	width := 0
	for _, f := range m.fields {
		width = max(width, lipgloss.Width(f.label()))
	}
	return min(width, maxLabelWidth)
}

func (m *inlineForm) editorWidth(f *field) int {
	if singleLine(f) {
		return max(20, m.width-len(marker)-m.labelWidth()-len(foldIndent)-2)
	}
	return max(20, m.width-len(marker)-len(foldIndent)-2)
}

func (m *inlineForm) fieldLines(f *field, focused bool, indent string, width int) []string {
	label := lipgloss.NewStyle().Width(width).MaxWidth(width).Render(f.label())
	if !focused {
		lines := []string{indent + "  " + m.theme.Blurred.Title.Render(label) + "  " + m.valueView(f)}
		if err := m.errs[f]; err != nil {
			lines = append(lines, indent+"  "+strings.Repeat(" ", width)+"  "+m.theme.Focused.ErrorMessage.Render(err.Error()))
		}
		return lines
	}
	// A single-line editor sits in the value column with the description under
	// it; a multi-line one opens below the label, which the description
	// follows.
	head := indent + m.theme.Focused.Title.Render(marker+label) + "  "
	pad := indent + strings.Repeat(" ", len(marker)+width+2)
	description := m.theme.Focused.Description.Render(f.description())
	var lines []string
	if !singleLine(f) && !m.editing {
		// A closed multi-line row shows its value until enter opens it.
		lines = append(lines, head+m.valueView(f))
		if f.description() != "" {
			lines = append(lines, pad+description)
		}
		if err := m.errs[f]; err != nil {
			lines = append(lines, pad+m.theme.Focused.ErrorMessage.Render(err.Error()))
		}
		return lines
	}
	editor := strings.Split(f.input.View(), "\n")
	if singleLine(f) {
		lines = append(lines, head+editor[0])
		lines = append(lines, prefixLines(pad, editor[1:])...)
		if f.description() != "" {
			lines = append(lines, pad+description)
		}
	} else {
		lines = append(lines, head+description)
		lines = append(lines, prefixLines(indent+"    ", editor)...)
		pad = indent + "    "
		// The highlighted option's description, wrapped to at most three lines.
		if hovered := f.hoveredDescription(); hovered != "" {
			wrapped := strings.Split(m.theme.Focused.Description.Width(max(20, m.width-len(pad)-2)).Render(hovered), "\n")
			lines = append(lines, prefixLines(pad, wrapped[:min(len(wrapped), maxDescriptionLines)])...)
		}
	}
	if err := m.errs[f]; err != nil {
		lines = append(lines, pad+m.theme.Focused.ErrorMessage.Render(err.Error()))
	}
	return lines
}

func (m *inlineForm) valueView(f *field) string {
	value := f.summary()
	style := lipgloss.NewStyle()
	if isEmpty(f.value()) {
		style = m.theme.Blurred.TextInput.Placeholder
	}
	room := max(10, m.width-m.labelWidth()-8)
	value = truncate(value, room)
	out := style.Render(value)
	// The chosen option's description follows its label while it fits.
	if described := f.chosenDescription(); described != "" && room-lipgloss.Width(value) > 12 {
		out += "  " + m.theme.Blurred.Description.Render(truncate(described, room-lipgloss.Width(value)-2))
	}
	if f.visible(true) {
		out += " " + m.theme.Blurred.Description.Render("read-only")
	}
	return out
}

func (m *inlineForm) advancedLine(focused bool) string {
	var count, set int
	for _, f := range m.fields {
		if m.folded[f] && !required(f) && m.shown(f) {
			count++
			if !isEmpty(f.value()) {
				set++
			}
		}
	}
	arrow := "▸"
	if m.expanded {
		arrow = "▾"
	}
	text := fmt.Sprintf("%s Advanced", arrow)
	detail := fmt.Sprintf("%d options", count)
	if set > 0 {
		detail += fmt.Sprintf(" · %d set", set)
	}
	if focused {
		return m.theme.Focused.Title.Render(marker+text) + "  " + m.theme.Focused.Description.Render(detail)
	}
	return "  " + m.theme.Blurred.Title.Render(text) + "  " + m.theme.Blurred.Description.Render(detail)
}

func (m *inlineForm) submitLine(focused bool) string {
	if focused {
		return m.theme.Focused.Title.Render(marker) + m.theme.Focused.FocusedButton.Render("Submit")
	}
	return "  " + m.theme.Blurred.BlurredButton.Render("Submit")
}

// helpView is the focused row's keys followed by the form's own.
func (m *inlineForm) helpView() string {
	if m.quitting {
		return m.theme.Focused.ErrorMessage.Render("press esc again to quit")
	}
	var bindings []key.Binding
	switch {
	case m.cursor.kind == rowField && !singleLine(m.cursor.f) && !m.editing:
		bindings = append(bindings, m.keys.Edit)
	case m.cursor.kind == rowField:
		for _, b := range m.cursor.f.input.KeyBinds() {
			if !covered(b, m.keys.Next, m.keys.Prev) {
				bindings = append(bindings, b)
			}
		}
	case m.cursor.kind == rowAdvanced:
		toggle := m.keys.Toggle
		if m.expanded {
			toggle.SetHelp("enter", "collapse")
		}
		bindings = append(bindings, toggle)
	case m.cursor.kind == rowSubmit:
		bindings = append(bindings, m.keys.Enter)
	}
	esc := m.keys.Esc
	if m.editing {
		esc.SetHelp("esc", "close")
	}
	bindings = append(bindings, m.keys.Next, m.keys.Prev, esc, m.keys.Submit)
	return m.help.ShortHelpView(bindings)
}

// covered reports whether every key of b is one of others' keys: the
// editor's own shift+tab help gives way to the form's, its enter+tab "next"
// stays.
func covered(b key.Binding, others ...key.Binding) bool {
	ours := map[string]bool{}
	for _, o := range others {
		for _, k := range o.Keys() {
			ours[k] = true
		}
	}
	for _, k := range b.Keys() {
		if !ours[k] {
			return false
		}
	}
	return len(b.Keys()) > 0
}

func prefixLines(prefix string, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = prefix + l
	}
	return out
}

func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
