package api

import (
	"fmt"
	"html"
	"strings"
)

// LabelBadge is a two-part pill: a muted label followed by an emphasised
// value, rendered by clicky-ui's <Badge variant="label" />. It is distinct
// from the single-label Badge() helper in html.go, which produces a plain
// pill with no label/value split.
//
// The Color/TextColor fields accept tailwind class names (e.g. "bg-blue-100"
// and "text-slate-700"); Shape controls the corner radius and Icon is an
// optional iconify name rendered before the label.
type LabelBadge struct {
	Label     string
	Value     string
	Color     string // tailwind bg class, e.g. "bg-blue-100"
	TextColor string // tailwind text class, e.g. "text-slate-700"
	Shape     string // "pill" | "rounded" | "square"; default "rounded"
	Icon      string // iconify icon name, optional
}

func (b LabelBadge) String() string {
	if b.Label == "" {
		return b.Value
	}
	if b.Value == "" {
		return b.Label
	}
	return b.Label + ": " + b.Value
}

func (b LabelBadge) ANSI() string {
	return b.String()
}

func (b LabelBadge) HTML() string {
	shape := "rounded-md"
	switch b.Shape {
	case "pill":
		shape = "rounded-full"
	case "square":
		shape = "rounded-none"
	}
	classes := []string{"inline-flex", "items-center", shape, "text-xs"}
	if b.Color != "" {
		classes = append(classes, b.Color)
	} else {
		classes = append(classes, "bg-gray-100")
	}
	if b.TextColor != "" {
		classes = append(classes, b.TextColor)
	}

	var inner strings.Builder
	if b.Icon != "" {
		fmt.Fprintf(&inner, `<span class="px-1 iconify" data-icon="%s" aria-hidden="true"></span>`, html.EscapeString(b.Icon))
	}
	if b.Label != "" {
		fmt.Fprintf(&inner, `<span class="px-1 font-medium opacity-70">%s</span>`, html.EscapeString(b.Label))
	}
	if b.Value != "" {
		fmt.Fprintf(&inner, `<span class="px-1">%s</span>`, html.EscapeString(b.Value))
	}

	return fmt.Sprintf(`<span class="%s">%s</span>`, strings.Join(classes, " "), inner.String())
}

func (b LabelBadge) Markdown() string {
	if b.Label == "" {
		return b.Value
	}
	if b.Value == "" {
		return fmt.Sprintf("**%s**", b.Label)
	}
	return fmt.Sprintf("**%s**: %s", b.Label, b.Value)
}
