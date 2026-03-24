package formatters

import (
	"strings"

	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
	"github.com/flanksource/clicky/formatters/fonts"
)

var colorBody = layout.Hex("333333") // Chrome pdf_formatter.go body { color: #333 }

func classToComputedStyle(class api.Class) foliohtml.ComputedStyle {
	style := foliohtml.DefaultStyle()
	style.FontSize = ptBody    // override folio's 12pt default to match pdf.css 12px body
	style.Color = colorBody    // match Chrome's body color: #333
	style.LineHeight = 1.4     // match Chrome's line-height: 1.4

	if class.Foreground != nil && class.Foreground.Hex != "" {
		if c, ok := foliohtml.ParseColor(class.Foreground.Hex); ok {
			style.Color = c
		} else {
			style.Color = layout.Hex(class.Foreground.Hex)
		}
	}

	if class.Background != nil && class.Background.Hex != "" {
		if c, ok := foliohtml.ParseColor(class.Background.Hex); ok {
			style.BackgroundColor = &c
		} else {
			c := layout.Hex(class.Background.Hex)
			style.BackgroundColor = &c
		}
	}

	if class.Font != nil {
		f := class.Font
		if f.Bold {
			style.FontWeight = "bold"
		}
		if f.Italic {
			style.FontStyle = "italic"
		}
		if f.Size > 0 {
			style.FontSize = tailwindFontSizePt(f.Size)
		}
		if f.Name != "" {
			style.FontFamily = strings.ToLower(f.Name)
		}
		if f.Underline {
			style.TextDecoration |= layout.DecorationUnderline
		}
		if f.Strikethrough {
			style.TextDecoration |= layout.DecorationStrikethrough
		}
		if f.Faint {
			style.Opacity = 0.5
		}
	}

	if class.Padding != nil {
		style.PaddingTop = class.Padding.Top.Float64()
		style.PaddingRight = class.Padding.Right.Float64()
		style.PaddingBottom = class.Padding.Bottom.Float64()
		style.PaddingLeft = class.Padding.Left.Float64()
	}

	if class.Border != nil {
		applyBorderSide := func(line api.Line) (float64, string, layout.Color) {
			if line.Width <= 0 || line.Style == api.None {
				return 0, "", layout.Color{}
			}
			borderStyle := "solid"
			switch line.Style {
			case api.Dashed:
				borderStyle = "dashed"
			case api.Dotted:
				borderStyle = "dotted"
			case api.Double:
				borderStyle = "double"
			}
			color := layout.ColorBlack
			if line.Color.Hex != "" {
				color = layout.Hex(line.Color.Hex)
			}
			return line.Width, borderStyle, color
		}

		style.BorderTopWidth, style.BorderTopStyle, style.BorderTopColor = applyBorderSide(class.Border.Top)
		style.BorderRightWidth, style.BorderRightStyle, style.BorderRightColor = applyBorderSide(class.Border.Right)
		style.BorderBottomWidth, style.BorderBottomStyle, style.BorderBottomColor = applyBorderSide(class.Border.Bottom)
		style.BorderLeftWidth, style.BorderLeftStyle, style.BorderLeftColor = applyBorderSide(class.Border.Left)
	}

	return style
}

// resolvedStyle bundles a ComputedStyle with text-level transforms
// that must be applied to the content string before rendering.
type resolvedStyle struct {
	foliohtml.ComputedStyle
	textTransform string // "uppercase", "lowercase", "capitalize", ""
	maxWidth      int    // character truncation width (0 = no limit)
	truncateMode  string // "suffix", "prefix", ""
}

func resolveComputedStyle(tailwindStr string, class api.Class) resolvedStyle {
	merged := class
	var twStyle tailwind.Style

	if tailwindStr != "" {
		twStyle = tailwind.ParseStyle(tailwindStr)
		resolved := api.ResolveStyles(tailwindStr)

		// Merge colors
		if resolved.Foreground != nil && merged.Foreground == nil {
			merged.Foreground = resolved.Foreground
		}
		if resolved.Background != nil && merged.Background == nil {
			merged.Background = resolved.Background
		}

		// Merge font
		if resolved.Font != nil {
			if merged.Font == nil {
				merged.Font = resolved.Font
			} else {
				if resolved.Font.Bold {
					merged.Font.Bold = true
				}
				if resolved.Font.Italic {
					merged.Font.Italic = true
				}
				if resolved.Font.Underline {
					merged.Font.Underline = true
				}
				if resolved.Font.Strikethrough {
					merged.Font.Strikethrough = true
				}
				if resolved.Font.Faint {
					merged.Font.Faint = true
				}
				if resolved.Font.Size > 0 && merged.Font.Size == 0 {
					merged.Font.Size = resolved.Font.Size
				}
				if resolved.Font.Name != "" && merged.Font.Name == "" {
					merged.Font.Name = resolved.Font.Name
				}
			}
		}
		if resolved.Padding != nil && merged.Padding == nil {
			merged.Padding = resolved.Padding
		}
		if resolved.Border != nil && merged.Border == nil {
			merged.Border = resolved.Border
		}
	}

	cs := classToComputedStyle(merged)

	// Map text alignment from tailwind classes
	if tailwindStr != "" {
		align := tailwind.ParseAlignment(tailwindStr)
		switch align.Horizontal {
		case tailwind.Center:
			cs.TextAlign = layout.AlignCenter
		case tailwind.Right:
			cs.TextAlign = layout.AlignRight
		case tailwind.Justify:
			cs.TextAlign = layout.AlignJustify
		default:
			cs.TextAlign = layout.AlignLeft
		}

		// Map text transform to folio's ComputedStyle.TextTransform
		if twStyle.TextTransform != "" {
			cs.TextTransform = twStyle.TextTransform
		}
	}

	return resolvedStyle{
		ComputedStyle: cs,
		textTransform: twStyle.TextTransform,
		maxWidth:      twStyle.MaxWidth,
		truncateMode:  twStyle.TruncateMode,
	}
}

// transformText applies text-level transforms (case, truncation) to content.
func (rs resolvedStyle) transformText(text string) string {
	if rs.textTransform != "" {
		text = tailwind.TransformText(text, rs.textTransform)
	}
	mode := rs.truncateMode
	if mode == "" && rs.maxWidth > 0 {
		mode = "suffix"
	}
	if mode != "" && rs.maxWidth > 0 {
		text = tailwind.TruncateText(text, 0, rs.maxWidth, mode)
	}
	return text
}

func makeTextRun(text string, style resolvedStyle) layout.TextRun {
	text = stripEmoji(style.transformText(text))
	color := style.Color
	if style.Opacity > 0 && style.Opacity < 1 {
		color = blendColorWithWhite(color, style.Opacity)
	}
	run := layout.TextRun{
		Text:       text,
		FontSize:   style.FontSize,
		Color:      color,
		Decoration: style.TextDecoration,
	}
	if style.FontFamily == "mono" || needsEmbeddedFont(text) {
		run.Embedded = fonts.MonoFont
	} else {
		run.Font = foliohtml.ResolveFont(style.ComputedStyle)
	}
	return run
}

func blendColorWithWhite(c layout.Color, opacity float64) layout.Color {
	return layout.Color{
		R:     c.R*opacity + 1.0*(1-opacity),
		G:     c.G*opacity + 1.0*(1-opacity),
		B:     c.B*opacity + 1.0*(1-opacity),
		Space: c.Space,
	}
}

const (
	ptBody        = 9    // 12px * 0.75 — matches pdf.css body font-size
	ptTableCell   = 8.25 // 11px * 0.75 — matches pdf.css td font-size
	ptTableHeader = 7.5  // 10px * 0.75 — matches pdf.css th font-size
)

// tailwindRemToPt maps tailwind rem values to pt sizes matching pdf.css overrides.
// Sizes not in the map fall back to rem * 12.
var tailwindRemToPt = map[float64]float64{
	0.75:  7.5,  // text-xs  → 10px * 0.75
	0.875: 8.25, // text-sm  → 11px * 0.75
	1.0:   9,    // text-base→ 12px * 0.75
	1.125: 10.5, // text-lg  → 14px * 0.75
	1.25:  12,   // text-xl  → 16px * 0.75
}

// tailwindFontSizePt converts a tailwind rem value to PDF points,
// matching the pdf.css pixel overrides (px * 0.75).
// Values >= 10 are assumed to already be in points.
func tailwindFontSizePt(rem float64) float64 {
	if rem >= 10 {
		return rem
	}
	if pt, ok := tailwindRemToPt[rem]; ok {
		return pt
	}
	return rem * 12
}

func defaultResolvedStyle() resolvedStyle {
	s := foliohtml.DefaultStyle()
	s.FontSize = ptBody
	s.Color = colorBody
	s.LineHeight = 1.4
	return resolvedStyle{ComputedStyle: s}
}

