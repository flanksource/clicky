package api

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ANSIText is captured terminal text that retains SGR presentation without
// replaying cursor movement, screen erasure, OSC, or other active controls.
// HTML output converts the retained presentation to escaped span markup.
type ANSIText struct {
	Content string
}

func (t ANSIText) String() string {
	return ansi.Strip(sanitizeANSIText(t.Content))
}

func (t ANSIText) ANSI() string {
	return closeANSIStyle(sanitizeANSIText(t.Content))
}

func (t ANSIText) HTML() string {
	return ansiTextToHTML(sanitizeANSIText(t.Content))
}

func (t ANSIText) Markdown() string {
	return t.String()
}

// VisibleLines returns independently renderable, non-blank lines. Active SGR
// state is replayed at the start of a continuation line and reset at its end.
func (t ANSIText) VisibleLines() []ANSIText {
	input := sanitizeANSIText(t.Content)
	var lines []ANSIText
	var line strings.Builder
	var active []string

	appendLine := func() {
		content := line.String()
		if len(active) > 0 {
			content += "\x1b[0m"
		}
		if strings.TrimSpace(ansi.Strip(content)) != "" {
			lines = append(lines, ANSIText{Content: content})
		}
		line.Reset()
		for _, sequence := range active {
			line.WriteString(sequence)
		}
	}

	for i := 0; i < len(input); {
		if input[i] == '\n' {
			appendLine()
			i++
			continue
		}
		if input[i] == '\x1b' && i+1 < len(input) && input[i+1] == '[' {
			sequence, consumed, keep := consumeCSI(input[i:])
			if consumed > 0 && keep {
				line.WriteString(sequence)
				active = updateActiveSGR(active, sequence)
				i += consumed
				continue
			}
		}
		line.WriteByte(input[i])
		i++
	}
	appendLine()
	return lines
}

func sanitizeANSIText(input string) string {
	var output strings.Builder
	output.Grow(len(input))

	for i := 0; i < len(input); {
		switch input[i] {
		case '\x1b':
			if i+1 >= len(input) {
				i++
				continue
			}
			switch input[i+1] {
			case '[':
				sequence, consumed, keep := consumeCSI(input[i:])
				if keep {
					output.WriteString(sequence)
				}
				i += consumed
			case ']', 'P', 'X', '^', '_':
				i += consumeANSIString(input[i:])
			default:
				i += min(2, len(input)-i)
			}
		case '\r':
			if i+1 < len(input) && input[i+1] == '\n' {
				i++
			} else {
				output.WriteByte('\n')
				i++
			}
		case '\n', '\t':
			output.WriteByte(input[i])
			i++
		default:
			if input[i] < 0x20 || input[i] == 0x7f {
				i++
				continue
			}
			output.WriteByte(input[i])
			i++
		}
	}
	return output.String()
}

func consumeCSI(input string) (string, int, bool) {
	for i := 2; i < len(input); i++ {
		if input[i] < 0x40 || input[i] > 0x7e {
			continue
		}
		sequence := input[:i+1]
		if input[i] != 'm' || !validSGRParams(input[2:i]) {
			return sequence, i + 1, false
		}
		return sequence, i + 1, true
	}
	return "", len(input), false
}

func validSGRParams(params string) bool {
	for i := range len(params) {
		if (params[i] < '0' || params[i] > '9') && params[i] != ';' && params[i] != ':' {
			return false
		}
	}
	return true
}

func consumeANSIString(input string) int {
	for i := 2; i < len(input); i++ {
		if input[i] == '\a' {
			return i + 1
		}
		if input[i] == '\x1b' && i+1 < len(input) && input[i+1] == '\\' {
			return i + 2
		}
	}
	return len(input)
}

func closeANSIStyle(input string) string {
	if !strings.Contains(input, "\x1b[") {
		return input
	}
	return input + "\x1b[0m"
}

func updateActiveSGR(active []string, sequence string) []string {
	params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
	if params == "" {
		return nil
	}
	codes := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	hasReset := false
	hasStyle := false
	for _, code := range codes {
		if code == "" || code == "0" {
			hasReset = true
		} else {
			hasStyle = true
		}
	}
	if hasReset {
		active = nil
	}
	if hasStyle {
		return append(active, sequence)
	}
	if !hasReset {
		return append(active, sequence)
	}
	return active
}

type ansiHTMLStyle struct {
	foreground string
	background string
	bold       bool
	faint      bool
	italic     bool
	underline  bool
	strike     bool
}

func ansiTextToHTML(input string) string {
	style := ansiHTMLStyle{}
	var output strings.Builder
	textStart := 0

	writeText := func(end int) {
		if end <= textStart {
			return
		}
		content := html.EscapeString(input[textStart:end])
		if css := style.css(); css != "" {
			fmt.Fprintf(&output, `<span style="%s">%s</span>`, css, content)
		} else {
			output.WriteString(content)
		}
	}

	for i := 0; i < len(input); {
		if input[i] != '\x1b' || i+1 >= len(input) || input[i+1] != '[' {
			i++
			continue
		}
		sequence, consumed, keep := consumeCSI(input[i:])
		if consumed == 0 || !keep {
			i++
			continue
		}
		writeText(i)
		style.apply(sequence)
		i += consumed
		textStart = i
	}
	writeText(len(input))
	return output.String()
}

func (s *ansiHTMLStyle) apply(sequence string) {
	params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
	params = strings.ReplaceAll(params, ":", ";")
	if params == "" {
		*s = ansiHTMLStyle{}
		return
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch {
		case code == 0:
			*s = ansiHTMLStyle{}
		case code == 1:
			s.bold = true
		case code == 2:
			s.faint = true
		case code == 3:
			s.italic = true
		case code == 4:
			s.underline = true
		case code == 9:
			s.strike = true
		case code == 22:
			s.bold, s.faint = false, false
		case code == 23:
			s.italic = false
		case code == 24:
			s.underline = false
		case code == 29:
			s.strike = false
		case code == 39:
			s.foreground = ""
		case code == 49:
			s.background = ""
		case code >= 30 && code <= 37:
			s.foreground = ansiColor(code - 30)
		case code >= 90 && code <= 97:
			s.foreground = ansiColor(code - 90 + 8)
		case code >= 40 && code <= 47:
			s.background = ansiColor(code - 40)
		case code >= 100 && code <= 107:
			s.background = ansiColor(code - 100 + 8)
		case code == 38 || code == 48:
			color, consumed := extendedANSIColor(parts[i+1:])
			if color != "" {
				if code == 38 {
					s.foreground = color
				} else {
					s.background = color
				}
				i += consumed
			}
		}
	}
}

func (s ansiHTMLStyle) css() string {
	var styles []string
	if s.foreground != "" {
		styles = append(styles, "color:"+s.foreground)
	}
	if s.background != "" {
		styles = append(styles, "background-color:"+s.background)
	}
	if s.bold {
		styles = append(styles, "font-weight:bold")
	}
	if s.faint {
		styles = append(styles, "opacity:0.7")
	}
	if s.italic {
		styles = append(styles, "font-style:italic")
	}
	var decorations []string
	if s.underline {
		decorations = append(decorations, "underline")
	}
	if s.strike {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		styles = append(styles, "text-decoration:"+strings.Join(decorations, " "))
	}
	return strings.Join(styles, ";")
}

func extendedANSIColor(parts []string) (string, int) {
	if len(parts) >= 2 && parts[0] == "5" {
		value, err := strconv.Atoi(parts[1])
		if err == nil && value >= 0 && value <= 255 {
			return ansiColor(value), 2
		}
	}
	if len(parts) >= 4 && parts[0] == "2" {
		r, errR := strconv.Atoi(parts[1])
		g, errG := strconv.Atoi(parts[2])
		b, errB := strconv.Atoi(parts[3])
		if errR == nil && errG == nil && errB == nil && validRGB(r, g, b) {
			return fmt.Sprintf("#%02x%02x%02x", r, g, b), 4
		}
	}
	return "", 0
}

func validRGB(values ...int) bool {
	for _, value := range values {
		if value < 0 || value > 255 {
			return false
		}
	}
	return true
}

func ansiColor(index int) string {
	standard := [...]string{
		"#1e1e1e", "#cd3131", "#0dbc79", "#e5e510",
		"#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
		"#666666", "#f14c4c", "#23d18b", "#f5f543",
		"#3b8eea", "#d670d6", "#29b8db", "#ffffff",
	}
	if index >= 0 && index < len(standard) {
		return standard[index]
	}
	if index >= 16 && index <= 231 {
		levels := [...]int{0, 95, 135, 175, 215, 255}
		value := index - 16
		return fmt.Sprintf("#%02x%02x%02x", levels[value/36], levels[(value/6)%6], levels[value%6])
	}
	if index >= 232 && index <= 255 {
		level := 8 + (index-232)*10
		return fmt.Sprintf("#%02x%02x%02x", level, level, level)
	}
	return ""
}
