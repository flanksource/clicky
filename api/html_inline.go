package api

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
)

// htmlOpenTag renders `<tag class="…" style="…">` for structured HTML
// renderers. The classes keep Tailwind-hosted pages unchanged; the inline style
// carries the same look into clients that strip <style> blocks and class
// attributes (Outlook, Gmail, Word, Confluence paste).
func htmlOpenTag(tag, classes string) string {
	if css := tailwindInlineCSS(classes); css != "" {
		return fmt.Sprintf(`<%s class="%s" style="%s">`, tag, classes, css)
	}
	return fmt.Sprintf(`<%s class="%s">`, tag, classes)
}

// tailwindInlineCSS converts Tailwind utility classes into inline CSS
// declarations. Colours resolve through tailwind.Color / ParseRawTailwindColor,
// the mapping api.Text uses, so a class that names no colour emits nothing.
// Classes with no visual effect are skipped.
func tailwindInlineCSS(classes string) string {
	var decls []string
	var borders inlineBorders
	for _, class := range strings.Fields(classes) {
		if decl, ok := inlineStaticCSS[class]; ok {
			decls = append(decls, decl)
		} else if !borders.parse(class) {
			decls = appendNonEmpty(decls, inlineDynamicCSS(class))
		}
	}
	return strings.Join(append(decls, borders.css()...), "; ")
}

var inlineStaticCSS = map[string]string{
	"font-mono":       "font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
	"font-normal":     "font-weight: 400",
	"font-medium":     "font-weight: 500",
	"font-semibold":   "font-weight: 600",
	"font-bold":       "font-weight: 700",
	"italic":          "font-style: italic",
	"leading-tight":   "line-height: 1.25",
	"rounded":         "border-radius: 0.25rem",
	"whitespace-pre":  "white-space: pre",
	"text-left":       "text-align: left",
	"text-center":     "text-align: center",
	"text-right":      "text-align: right",
	"flex":            "display: flex",
	"flex-wrap":       "flex-wrap: wrap",
	"flex-none":       "flex: none",
	"flex-1":          "flex: 1 1 0%",
	"items-baseline":  "align-items: baseline",
	"overflow-hidden": "overflow: hidden",
	"overflow-x-auto": "overflow-x: auto",
	"select-none":     "user-select: none",
}

var inlineSpacingRegexp = regexp.MustCompile(`^(p|px|py|pt|pr|pb|pl|m|mx|my|mt|mr|mb|ml|w|gap-x)-(.+)$`)

var inlineSpacingProps = map[string][]string{
	"p": {"padding"}, "px": {"padding-left", "padding-right"}, "py": {"padding-top", "padding-bottom"},
	"pt": {"padding-top"}, "pr": {"padding-right"}, "pb": {"padding-bottom"}, "pl": {"padding-left"},
	"m": {"margin"}, "mx": {"margin-left", "margin-right"}, "my": {"margin-top", "margin-bottom"},
	"mt": {"margin-top"}, "mr": {"margin-right"}, "mb": {"margin-bottom"}, "ml": {"margin-left"},
	"w": {"width"}, "gap-x": {"column-gap"},
}

func inlineDynamicCSS(class string) string {
	switch {
	case strings.HasPrefix(class, "text-"):
		if size := tailwind.ParseFontSize(class); size > 0 {
			return fmt.Sprintf("font-size: %grem", size)
		}
		if tailwind.IsTextUtilityClass(class) {
			return ""
		}
		return cssDecl("color", tailwind.Color(class))
	case strings.HasPrefix(class, "bg-"):
		return cssDecl("background-color", tailwind.Color(class))
	case strings.HasPrefix(class, "opacity-"):
		if n, err := strconv.Atoi(strings.TrimPrefix(class, "opacity-")); err == nil {
			return fmt.Sprintf("opacity: %g", float64(n)/100)
		}
		return ""
	}
	m := inlineSpacingRegexp.FindStringSubmatch(class)
	if m == nil {
		return ""
	}
	value, ok := tailwind.TailwindSpacing[m[2]]
	if !ok {
		return ""
	}
	var decls []string
	for _, prop := range inlineSpacingProps[m[1]] {
		decls = append(decls, fmt.Sprintf("%s: %grem", prop, value))
	}
	return strings.Join(decls, "; ")
}

// inlineBorders accumulates border-* classes so side widths and the shared
// border colour combine into one declaration per side, as Tailwind does.
type inlineBorders struct {
	width [4]float64 // top, right, bottom, left
	color string
}

const inlineBorderSides = "trbl"

// tailwindDefaultBorderColor is Tailwind's preflight border colour (gray-200).
const tailwindDefaultBorderColor = "#e5e7eb"

func (b *inlineBorders) parse(class string) bool {
	if class != "border" && !strings.HasPrefix(class, "border-") {
		return false
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(class, "border"), "-")
	sides := []int{0, 1, 2, 3}
	if side, after, _ := strings.Cut(rest, "-"); len(side) == 1 && strings.Contains(inlineBorderSides, side) {
		sides, rest = []int{strings.Index(inlineBorderSides, side)}, after
	}
	width := 1.0
	if rest != "" {
		n, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			if color, err := tailwind.ParseRawTailwindColor(class); err == nil && color != "" {
				b.color = color
			}
			return true
		}
		width = n
	}
	for _, side := range sides {
		b.width[side] = width
	}
	return true
}

func (b inlineBorders) css() []string {
	color := b.color
	if color == "" {
		color = tailwindDefaultBorderColor
	}
	var decls []string
	for i, prop := range []string{"border-top", "border-right", "border-bottom", "border-left"} {
		if b.width[i] > 0 {
			decls = append(decls, fmt.Sprintf("%s: %gpx solid %s", prop, b.width[i], color))
		}
	}
	return decls
}

func cssDecl(prop, value string) string {
	if value == "" {
		return ""
	}
	return prop + ": " + value
}

func appendNonEmpty(list []string, value string) []string {
	if value == "" {
		return list
	}
	return append(list, value)
}
