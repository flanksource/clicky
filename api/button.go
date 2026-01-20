package api

import (
	"fmt"
	"strings"
)

// Button represents a platform-agnostic clickable action across output formats.
type Button struct {
	Label   string
	Href    string
	ID      string
	Payload string
	Variant string
}

func (b Button) String() string {
	if b.Href != "" {
		return fmt.Sprintf("%s (%s)", b.Label, b.Href)
	}
	return b.Label
}

func (b Button) ANSI() string {
	return b.String()
}

func (b Button) Markdown() string {
	if b.Href != "" {
		return fmt.Sprintf("[%s](%s)", b.Label, b.Href)
	}
	return b.Label
}

func (b Button) HTML() string {
	label := htmlEscapeString(b.Label)
	if b.Href != "" {
		classes := strings.TrimSpace("inline-flex items-center px-3 py-1 rounded-md bg-gray-100 text-gray-900 text-sm")
		if b.Variant != "" {
			classes = b.Variant
		}
		return fmt.Sprintf(`<a href="%s" class="%s">%s</a>`, htmlEscapeString(b.Href), classes, label)
	}
	return label
}

// ButtonGroup represents a set of buttons rendered together.
type ButtonGroup struct {
	Buttons []Button
}

func (a ButtonGroup) String() string {
	if len(a.Buttons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(a.Buttons))
	for _, b := range a.Buttons {
		parts = append(parts, b.String())
	}
	return strings.Join(parts, " ")
}

func (a ButtonGroup) ANSI() string {
	return a.String()
}

func (a ButtonGroup) Markdown() string {
	if len(a.Buttons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(a.Buttons))
	for _, b := range a.Buttons {
		parts = append(parts, b.Markdown())
	}
	return strings.Join(parts, " ")
}

func (a ButtonGroup) HTML() string {
	if len(a.Buttons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(a.Buttons))
	for _, b := range a.Buttons {
		parts = append(parts, b.HTML())
	}
	return strings.Join(parts, " ")
}
