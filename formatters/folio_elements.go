package formatters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/flanksource/clicky/api"
)

func needsEmbeddedFont(text string) bool {
	for _, r := range text {
		if r > 127 {
			return true
		}
	}
	return false
}

func stripEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0xFFFF {
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == len(s) {
		return s
	}
	return strings.TrimSpace(b.String())
}

func addTextable(doc *document.Document, t api.Textable) {
	switch v := t.(type) {
	case *api.Text:
		addText(doc, v)
	case api.Text:
		addText(doc, &v)
	case *api.TextTable:
		addTable(doc, v)
	case *api.TextTree:
		addTree(doc, v, 0)
	default:
		addParagraph(doc, t.String(), defaultResolvedStyle())
	}
}

func addText(doc *document.Document, t *api.Text) {
	style := resolveComputedStyle(t.Style, t.Class)
	if len(t.Children) == 0 {
		addParagraph(doc, t.Content, style)
		return
	}

	var runs []layout.TextRun
	if t.Content != "" {
		runs = append(runs, makeTextRun(t.Content, style))
	}
	for _, child := range t.Children {
		switch c := child.(type) {
		case *api.Text:
			childStyle := resolveComputedStyle(c.Style, c.Class)
			runs = append(runs, makeTextRun(c.String(), childStyle))
		case api.Text:
			childStyle := resolveComputedStyle(c.Style, c.Class)
			runs = append(runs, makeTextRun(c.String(), childStyle))
		default:
			runs = append(runs, makeTextRun(child.String(), style))
		}
	}

	if len(runs) == 0 {
		return
	}
	p := layout.NewStyledParagraph(runs...)
	p.SetAlign(style.TextAlign)
	if style.LineHeight > 0 {
		p.SetLeading(style.LineHeight)
	}
	doc.Add(p)
}

func addTable(doc *document.Document, tt *api.TextTable) {
	cleaned := tt.WithoutEmptyColumns()
	if len(cleaned.Headers) == 0 {
		return
	}

	tbl := layout.NewTable()
	tbl.SetBorderCollapse(true)

	headerBg := layout.Hex("F3F4F6")
	cellPadding := layout.Padding{Top: 3, Right: 6, Bottom: 3, Left: 6}
	grayBorder := layout.SolidBorder(0.5, layout.Hex("D1D5DB"))

	headerRow := tbl.AddHeaderRow()
	for _, h := range cleaned.Headers {
		headerStyle := defaultResolvedStyle()
		headerStyle.FontWeight = "bold"
		headerStyle.FontSize = ptTableHeader
		run := makeTextRun(h.String(), headerStyle)
		run.FontSize = ptTableHeader
		cell := headerRow.AddCellElement(layout.NewStyledParagraph(run))
		cell.SetBackground(headerBg)
		cell.SetPaddingSides(cellPadding)
		cell.SetBorders(layout.AllBorders(grayBorder))
	}

	for _, row := range cleaned.Rows {
		dataRow := tbl.AddRow()
		for i, fieldName := range cleaned.FieldNames {
			val, ok := row[fieldName]
			cellText := ""
			cellStyle := defaultResolvedStyle()
			cellStyle.FontSize = ptTableCell

			if ok && val.Textable != nil {
				cellText = val.Textable.String()
				switch t := val.Textable.(type) {
				case *api.Text:
					cellStyle = resolveComputedStyle(t.Style, t.Class)
				case api.Text:
					cellStyle = resolveComputedStyle(t.Style, t.Class)
				}
				if cellStyle.FontSize == ptBody {
					cellStyle.FontSize = ptTableCell
				}
			} else if ok {
				cellText = fmt.Sprintf("%v", val)
			}

			if i < len(cleaned.Columns) && cleaned.Columns[i].Style != "" {
				cellStyle = resolveComputedStyle(cleaned.Columns[i].Style, api.Class{})
				if cellStyle.FontSize == ptBody {
					cellStyle.FontSize = ptTableCell
				}
			}

			run := makeTextRun(cellText, cellStyle)
			cell := dataRow.AddCellElement(layout.NewStyledParagraph(run))
			cell.SetPaddingSides(cellPadding)
			cell.SetBorders(layout.AllBorders(grayBorder))
			if cellStyle.BackgroundColor != nil {
				cell.SetBackground(*cellStyle.BackgroundColor)
			}
		}
	}

	doc.Add(tbl)
}

func treeStyle() resolvedStyle {
	s := defaultResolvedStyle()
	s.FontFamily = "mono"
	s.FontSize = ptTableCell // pdf.css .tree-view { font-size: 11px } → 8.25pt
	return s
}

func addTree(doc *document.Document, tree *api.TextTree, depth int) {
	style := treeStyle()
	if tree.Node != nil {
		prefix := ""
		if depth > 0 {
			prefix = strings.Repeat("    ", depth-1) + "├── "
		}
		nodeText := prefix + tree.Node.String()
		addParagraph(doc, nodeText, style)
	}

	for i, child := range tree.Children {
		childCopy := child
		if i == len(tree.Children)-1 && depth > 0 {
			prefix := strings.Repeat("    ", depth-1) + "└── "
			nodeText := prefix + child.Node.String()
			addParagraph(doc, nodeText, style)
			for _, grandchild := range child.Children {
				gc := grandchild
				addTree(doc, &gc, depth+1)
			}
			continue
		}
		addTree(doc, &childCopy, depth+1)
	}
}

func addTextList(doc *document.Document, list api.TextList) {
	for _, item := range list {
		addTextable(doc, item)
	}
}

type fieldKey struct {
	name, label, labelStyle string
}

func orderedKeys(tm *api.TypedMap, schema *api.PrettyObject) []fieldKey {
	if schema != nil && len(schema.Fields) > 0 {
		var keys []fieldKey
		seen := map[string]bool{}
		for _, f := range schema.Fields {
			if _, exists := (*tm)[f.Name]; exists {
				label := f.Label
				if label == "" {
					label = api.PrettifyFieldName(f.Name)
				}
				keys = append(keys, fieldKey{name: f.Name, label: label, labelStyle: f.LabelStyle})
				seen[f.Name] = true
			}
		}
		for k := range *tm {
			if !seen[k] {
				keys = append(keys, fieldKey{name: k, label: api.PrettifyFieldName(k)})
			}
		}
		return keys
	}

	var keys []fieldKey
	for k := range *tm {
		keys = append(keys, fieldKey{name: k, label: api.PrettifyFieldName(k)})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].name < keys[j].name })
	return keys
}

func isSimpleTextable(val api.TypedValue) bool {
	return val.Textable != nil && val.Table == nil && val.Tree == nil &&
		val.TypedMap == nil && val.TypedList == nil && val.Slice == nil
}

func summaryLabelStyle(labelStyleTW string) resolvedStyle {
	if labelStyleTW != "" {
		return resolveComputedStyle(labelStyleTW, api.Class{})
	}
	s := defaultResolvedStyle()
	s.FontSize = ptTableCell // text-sm equivalent
	s.FontWeight = "bold"
	s.Color = layout.Hex("6B7280") // gray-500
	return s
}

func addTypedMap(doc *document.Document, tm *api.TypedMap, schema *api.PrettyObject) {
	keys := orderedKeys(tm, schema)

	var summaryKeys []fieldKey
	for _, key := range keys {
		if isSimpleTextable((*tm)[key.name]) {
			summaryKeys = append(summaryKeys, key)
		}
	}

	if len(summaryKeys) > 0 {
		cols := layout.NewColumns(2).SetGap(16)
		for i, key := range summaryKeys {
			val := (*tm)[key.name]
			labelRun := makeTextRun(key.label, summaryLabelStyle(key.labelStyle))
			valueRun := makeTextRun(val.Textable.String(), defaultResolvedStyle())
			labelP := layout.NewStyledParagraph(labelRun)
			valueP := layout.NewStyledParagraph(valueRun)
			cols.Add(i%2, labelP)
			cols.Add(i%2, valueP)
		}
		doc.Add(cols)
	}

	for _, key := range keys {
		val := (*tm)[key.name]
		if val.Table != nil {
			addHeading(doc, key.label, 2)
			addTable(doc, val.Table)
		} else if val.Tree != nil {
			addHeading(doc, key.label, 2)
			addTree(doc, val.Tree, 0)
		} else if val.TypedMap != nil {
			addHeading(doc, key.label, 2)
			addTypedMap(doc, val.TypedMap, nil)
		} else if val.TypedList != nil {
			addHeading(doc, key.label, 2)
			addTypedList(doc, val.TypedList)
		} else if val.Slice != nil {
			addHeading(doc, key.label, 2)
			addTextList(doc, *val.Slice)
		}
	}
}

func addTypedList(doc *document.Document, tl *api.TypedList) {
	for _, val := range *tl {
		if val.Textable != nil {
			addTextable(doc, val.Textable)
		} else if val.Table != nil {
			addTable(doc, val.Table)
		} else if val.Tree != nil {
			addTree(doc, val.Tree, 0)
		} else if val.TypedMap != nil {
			addTypedMap(doc, val.TypedMap, nil)
		} else if val.TypedList != nil {
			addTypedList(doc, val.TypedList)
		} else if val.Slice != nil {
			addTextList(doc, *val.Slice)
		}
	}
}

var chromeHeadingSizePt = map[int]float64{
	1: 12,   // not in pdf.css, use text-xl = 16px * 0.75
	2: 10.5, // pdf.css h2 { font-size: 14px } → 14 * 0.75
	3: 9,    // body size fallback
}

func addHeading(doc *document.Document, text string, level int) {
	size := chromeHeadingSizePt[level]
	if size == 0 {
		size = ptBody
	}
	run := layout.Run(text, font.HelveticaBold, size)
	p := layout.NewStyledParagraph(run)
	p.SetSpaceBefore(size)
	doc.Add(p)
}

func addParagraph(doc *document.Document, text string, style resolvedStyle) {
	if text == "" {
		return
	}
	run := makeTextRun(text, style)
	p := layout.NewStyledParagraph(run)
	p.SetAlign(style.TextAlign)
	if style.LineHeight > 0 {
		p.SetLeading(style.LineHeight)
	}
	doc.Add(p)
}
