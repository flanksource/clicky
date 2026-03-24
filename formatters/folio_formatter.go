package formatters

import (
	"bytes"
	"fmt"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
	"github.com/flanksource/clicky/api"
)

func init() {
	RegisterFormatter("pdf-folio", formatFolio)
}

func formatFolio(data interface{}, opts FormatOptions) (string, error) {
	if items, ok := data.([]any); ok && len(items) == 1 {
		data = items[0]
	}

	if pd, ok := data.(*api.PrettyData); ok {
		return formatPrettyDataFolio(pd)
	}

	pd, err := ToPrettyDataWithOptions(data, opts)
	if err != nil {
		return "", fmt.Errorf("folio: failed to convert to PrettyData: %w", err)
	}
	return formatPrettyDataFolio(pd)
}

func formatPrettyDataFolio(pd *api.PrettyData) (string, error) {
	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 14.4, Right: 14.4, Bottom: 14.4, Left: 14.4})

	if pd.Textable != nil {
		addTextable(doc, pd.Textable)
	}
	if pd.Table != nil {
		addTable(doc, pd.Table)
	}
	if pd.Tree != nil {
		addTree(doc, pd.Tree, 0)
	}
	if pd.Slice != nil {
		addTextList(doc, *pd.Slice)
	}
	if pd.TypedMap != nil {
		addTypedMap(doc, pd.TypedMap, pd.Schema)
	}
	if pd.TypedList != nil {
		addTypedList(doc, pd.TypedList)
	}

	if pd.Textable == nil && pd.Table == nil && pd.Tree == nil &&
		pd.Slice == nil && pd.TypedMap == nil && pd.TypedList == nil {
		addParagraph(doc, fmt.Sprintf("%v", pd.Original), defaultResolvedStyle())
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return "", fmt.Errorf("folio: failed to write PDF: %w", err)
	}
	return buf.String(), nil
}
