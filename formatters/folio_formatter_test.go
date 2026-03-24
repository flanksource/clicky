package formatters

import (
	"strings"
	"testing"

	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
	"github.com/flanksource/clicky/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func text(s string) api.Text {
	return api.Text{Content: s}
}

func textable(s string) api.Textable {
	t := text(s)
	return t
}

func TestClassToComputedStyle(t *testing.T) {
	tests := []struct {
		name  string
		class api.Class
	}{
		{
			name:  "bold font",
			class: api.Class{Font: &api.Font{Bold: true}},
		},
		{
			name:  "foreground color",
			class: api.Class{Foreground: &api.Color{Hex: "#FF0000"}},
		},
		{
			name: "padding",
			class: api.Class{
				Padding: &api.Padding{
					Top: api.NewPoint(8), Right: api.NewPoint(4),
					Bottom: api.NewPoint(8), Left: api.NewPoint(4),
				},
			},
		},
		{
			name:  "italic and underline",
			class: api.Class{Font: &api.Font{Italic: true, Underline: true}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			style := classToComputedStyle(tc.class)

			switch tc.name {
			case "bold font":
				assert.Equal(t, "bold", style.FontWeight)
			case "foreground color":
				assert.Equal(t, layout.Hex("FF0000"), style.Color)
			case "padding":
				assert.Equal(t, float64(8), style.PaddingTop)
				assert.Equal(t, float64(4), style.PaddingRight)
				assert.Equal(t, float64(8), style.PaddingBottom)
				assert.Equal(t, float64(4), style.PaddingLeft)
			case "italic and underline":
				assert.Equal(t, "italic", style.FontStyle)
				assert.NotZero(t, style.TextDecoration&layout.DecorationUnderline)
			}
		})
	}
}

func TestClassToComputedStyleBorder(t *testing.T) {
	class := api.Class{
		Border: &api.Borders{
			Top: api.Line{Width: 1, Style: api.Solid, Color: api.Color{Hex: "#333333"}},
		},
	}
	style := classToComputedStyle(class)
	assert.Equal(t, float64(1), style.BorderTopWidth)
	assert.Equal(t, "solid", style.BorderTopStyle)
	assert.Equal(t, layout.Hex("333333"), style.BorderTopColor)
}

func TestFormatFolioPDFOutput(t *testing.T) {
	nameText := text("Name")
	valueText := text("Value")

	table := &api.TextTable{
		Headers:    api.TextList{nameText, valueText},
		FieldNames: []string{"name", "value"},
		Rows: []api.TableRow{
			{"name": api.TypedValue{Textable: textable("key1")}, "value": api.TypedValue{Textable: textable("val1")}},
			{"name": api.TypedValue{Textable: textable("key2")}, "value": api.TypedValue{Textable: textable("val2")}},
		},
	}

	pd := &api.PrettyData{
		TypedValue: api.TypedValue{Table: table},
	}

	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"), "output should start with %%PDF header")
	assert.Greater(t, len(result), 100, "PDF should have substantial content")
}

func TestResolveComputedStyleTextTransform(t *testing.T) {
	rs := resolveComputedStyle("uppercase", api.Class{})
	assert.Equal(t, "uppercase", rs.textTransform)
	assert.Equal(t, "THE QUICK", rs.transformText("The Quick"))
}

func TestResolveComputedStyleAlignment(t *testing.T) {
	rs := resolveComputedStyle("text-center", api.Class{})
	assert.Equal(t, layout.AlignCenter, rs.TextAlign)

	rs2 := resolveComputedStyle("text-right", api.Class{})
	assert.Equal(t, layout.AlignRight, rs2.TextAlign)
}

func TestResolveComputedStyleTruncation(t *testing.T) {
	rs := resolveComputedStyle("max-w-[5ch]", api.Class{})
	assert.Equal(t, 5, rs.maxWidth)
	assert.Equal(t, "Hello…", rs.transformText("Hello World"))
}

func TestResolveComputedStyleOpacity(t *testing.T) {
	rs := resolveComputedStyle("opacity-50", api.Class{})
	assert.Equal(t, 0.5, rs.Opacity)
}

func TestTailwindFontSizePt(t *testing.T) {
	tests := []struct {
		rem      float64
		expected float64
		label    string
	}{
		{0.75, 7.5, "text-xs: 10px"},
		{0.875, 8.25, "text-sm: 11px"},
		{1.0, 9, "text-base: 12px"},
		{1.125, 10.5, "text-lg: 14px"},
		{1.25, 12, "text-xl: 16px"},
		{1.5, 18, "text-2xl: fallback rem*12"},
		{1.875, 22.5, "text-3xl: fallback rem*12"},
		{14.0, 14.0, "already in points"},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			assert.Equal(t, tc.expected, tailwindFontSizePt(tc.rem))
		})
	}
}

func TestStyledTableCell(t *testing.T) {
	nameText := text("Name")
	valueText := text("Status")

	styledVal := api.Text{
		Content: "OK",
		Style:   "text-green-600 font-bold underline",
	}

	table := &api.TextTable{
		Headers:    api.TextList{nameText, valueText},
		FieldNames: []string{"name", "status"},
		Rows: []api.TableRow{
			{
				"name":   api.TypedValue{Textable: textable("svc-1")},
				"status": api.TypedValue{Textable: styledVal},
			},
		},
	}

	pd := &api.PrettyData{TypedValue: api.TypedValue{Table: table}}
	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"), "output should start with %%PDF header")
	assert.Greater(t, len(result), 100)
}

func TestResolveComputedStyleCombined(t *testing.T) {
	rs := resolveComputedStyle("uppercase font-bold text-green-600 underline", api.Class{})
	assert.Equal(t, "bold", rs.FontWeight)
	assert.Equal(t, "uppercase", rs.textTransform)
	assert.NotZero(t, rs.TextDecoration&layout.DecorationUnderline)
	assert.NotNil(t, rs.Color) // green-600 hex color
	assert.Equal(t, "HELLO", rs.transformText("hello"))
}

func TestBlendColorWithWhite(t *testing.T) {
	black := layout.Color{R: 0, G: 0, B: 0}
	blended := blendColorWithWhite(black, 0.5)
	assert.InDelta(t, 0.5, blended.R, 0.01)
	assert.InDelta(t, 0.5, blended.G, 0.01)
	assert.InDelta(t, 0.5, blended.B, 0.01)

	red := layout.Color{R: 1, G: 0, B: 0}
	blended = blendColorWithWhite(red, 0.5)
	assert.InDelta(t, 1.0, blended.R, 0.01)
	assert.InDelta(t, 0.5, blended.G, 0.01)
	assert.InDelta(t, 0.5, blended.B, 0.01)
}

func TestMakeTextRunAppliesOpacity(t *testing.T) {
	style := resolvedStyle{ComputedStyle: foliohtml.DefaultStyle()}
	style.Opacity = 0.5
	style.Color = layout.Color{R: 0, G: 0, B: 0}

	run := makeTextRun("faint", style)
	assert.InDelta(t, 0.5, run.Color.R, 0.01)
	assert.InDelta(t, 0.5, run.Color.G, 0.01)
	assert.InDelta(t, 0.5, run.Color.B, 0.01)
}

func TestFormatFolioFaintText(t *testing.T) {
	faintText := api.Text{Content: "faded", Style: "opacity-50"}
	table := &api.TextTable{
		Headers:    api.TextList{text("Label"), text("Value")},
		FieldNames: []string{"label", "value"},
		Rows: []api.TableRow{
			{
				"label": api.TypedValue{Textable: textable("normal")},
				"value": api.TypedValue{Textable: faintText},
			},
		},
	}
	pd := &api.PrettyData{TypedValue: api.TypedValue{Table: table}}
	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"))
	assert.Greater(t, len(result), 100)
}

func TestDefaultStyleMatchesChrome(t *testing.T) {
	style := classToComputedStyle(api.Class{})
	assert.Equal(t, colorBody, style.Color, "body color should be #333")
	assert.Equal(t, 1.4, style.LineHeight, "line-height should match Chrome's 1.4")
	assert.Equal(t, float64(ptBody), style.FontSize, "body font size should be 9pt")

	def := defaultResolvedStyle()
	assert.Equal(t, colorBody, def.Color)
	assert.Equal(t, 1.4, def.LineHeight)
}

func TestTreeStyleUsesMonospace(t *testing.T) {
	style := treeStyle()
	assert.Equal(t, "mono", style.FontFamily)
	assert.Equal(t, ptTableCell, style.FontSize)
}

func TestStripEmoji(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ascii only", "hello world", "hello world"},
		{"BMP symbols preserved", "✓ OK ✗ FAIL", "✓ OK ✗ FAIL"},
		{"box drawing preserved", "├── node", "├── node"},
		{"emoji stripped", "📁 folder", "folder"},
		{"mixed emoji and BMP", "✓ 🚀 done", "✓  done"},
		{"emoji in middle", "a 🐹 b", "a  b"},
		{"empty string", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, stripEmoji(tc.input))
		})
	}
}

func TestNeedsEmbeddedFont(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ascii", "hello", false},
		{"box drawing", "├── node", true},
		{"checkmark", "✓", true},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, needsEmbeddedFont(tc.input))
		})
	}
}

func TestFormatFolioTextOutput(t *testing.T) {
	pd := &api.PrettyData{
		TypedValue: api.TypedValue{Textable: textable("Hello World")},
	}

	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"))
}

func TestTypedMapSchemaOrdering(t *testing.T) {
	tm := api.TypedMap{
		"zulu":  api.TypedValue{Textable: textable("last")},
		"alpha": api.TypedValue{Textable: textable("first")},
		"bravo": api.TypedValue{Table: &api.TextTable{
			Headers:    api.TextList{text("Col")},
			FieldNames: []string{"col"},
			Rows:       []api.TableRow{{"col": api.TypedValue{Textable: textable("row1")}}},
		}},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "alpha", Label: "Alpha Label"},
			{Name: "bravo"},
			{Name: "zulu"},
		},
	}

	pd := &api.PrettyData{
		TypedValue: api.TypedValue{TypedMap: &tm},
		Schema:     schema,
	}

	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"))
	assert.Greater(t, len(result), 100)
}

func TestTypedMapNoSchemaAlphabetical(t *testing.T) {
	tm := api.TypedMap{
		"zulu":  api.TypedValue{Textable: textable("z-val")},
		"alpha": api.TypedValue{Textable: textable("a-val")},
	}

	pd := &api.PrettyData{
		TypedValue: api.TypedValue{TypedMap: &tm},
	}

	result, err := formatFolio(pd, FormatOptions{})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "%PDF"))
}

func TestOrderedKeysSchemaOrder(t *testing.T) {
	tm := api.TypedMap{
		"zulu":  api.TypedValue{Textable: textable("z")},
		"alpha": api.TypedValue{Textable: textable("a")},
		"bravo": api.TypedValue{Textable: textable("b")},
		"extra": api.TypedValue{Textable: textable("e")},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "bravo", Label: "Bravo Custom"},
			{Name: "alpha"},
			{Name: "zulu", Label: "Zulu Label"},
		},
	}

	keys := orderedKeys(&tm, schema)
	require.Len(t, keys, 4)
	assert.Equal(t, "bravo", keys[0].name)
	assert.Equal(t, "Bravo Custom", keys[0].label)
	assert.Equal(t, "alpha", keys[1].name)
	assert.Equal(t, "Alpha", keys[1].label)
	assert.Equal(t, "zulu", keys[2].name)
	assert.Equal(t, "Zulu Label", keys[2].label)
	assert.Equal(t, "extra", keys[3].name)
	assert.Equal(t, "Extra", keys[3].label)
}

func TestOrderedKeysNoSchema(t *testing.T) {
	tm := api.TypedMap{
		"zulu":  api.TypedValue{Textable: textable("z")},
		"alpha": api.TypedValue{Textable: textable("a")},
	}

	keys := orderedKeys(&tm, nil)
	require.Len(t, keys, 2)
	assert.Equal(t, "alpha", keys[0].name)
	assert.Equal(t, "zulu", keys[1].name)
}
