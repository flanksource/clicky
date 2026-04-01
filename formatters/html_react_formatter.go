package formatters

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
	reactAssets "github.com/flanksource/clicky/formatters/html/react"
)

func init() {
	formatter := &HTMLReactFormatter{}
	RegisterFormatter("html-react", formatter.Format)
}

type HTMLReactFormatter struct{}

type reactData struct {
	Type     string               `json:"type"`
	Schema   []reactSchemaField   `json:"schema,omitempty"`
	Table    *reactTable          `json:"table,omitempty"`
	Tree     *reactTree           `json:"tree,omitempty"`
	Fields   map[string]reactData `json:"fields,omitempty"`
	List     []reactData          `json:"list,omitempty"`
	Text     string               `json:"text,omitempty"`
	HTML     string               `json:"html,omitempty"`
	Original any                  `json:"original,omitempty"`
}

type reactSchemaField struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	Type  string `json:"type,omitempty"`
}

type reactCell struct {
	Text string `json:"text"`
	HTML string `json:"html,omitempty"`
}

type reactTable struct {
	Columns []reactColumn          `json:"columns"`
	Rows    []map[string]reactCell `json:"rows"`
}

type reactColumn struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
}

type reactTree struct {
	Label    string      `json:"label"`
	HTML     string      `json:"html,omitempty"`
	Children []reactTree `json:"children,omitempty"`
}

func (f *HTMLReactFormatter) Format(data any, opts FormatOptions) (string, error) {
	if slice, ok := data.([]interface{}); ok && len(slice) == 1 {
		data = slice[0]
	}

	prettyData, err := ToPrettyDataWithOptions(data, opts)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if prettyData == nil || prettyData.IsEmpty() {
		return "", nil
	}

	payload := convertPrettyData(prettyData)

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal react data: %w", err)
	}

	return buildReactHTML(string(jsonBytes), opts.ReactComponent), nil
}

func convertPrettyData(pd *api.PrettyData) reactData {
	rd := convertTypedValue(&pd.TypedValue, pd.Schema)
	rd.Original = pd.Original

	if pd.Schema != nil && rd.Schema == nil {
		rd.Schema = convertSchema(pd.Schema)
	}

	return rd
}

func convertTypedValue(tv *api.TypedValue, schema *api.PrettyObject) reactData {
	if tv.Table != nil {
		return convertTable(tv.Table, schema)
	}
	if tv.Tree != nil {
		return reactData{Type: "tree", Tree: convertTree(tv.Tree)}
	}
	if tv.TypedMap != nil {
		return convertTypedMap(tv.TypedMap, schema)
	}
	if tv.TypedList != nil {
		return convertTypedList(tv.TypedList)
	}
	if tv.Map != nil {
		fields := make(map[string]reactData)
		for key, val := range *tv.Map {
			fields[key] = convertTextable(val)
		}
		return reactData{Type: "map", Fields: fields}
	}
	if tv.Slice != nil {
		return convertTextList(*tv.Slice)
	}
	if tv.Textable != nil {
		return convertTextable(tv.Textable)
	}
	return reactData{Type: "text", Text: tv.String()}
}

func convertTable(t *api.TextTable, _ *api.PrettyObject) reactData {
	rt := &reactTable{}

	if len(t.Columns) > 0 {
		for _, col := range t.Columns {
			rt.Columns = append(rt.Columns, reactColumn{
				Name:  col.Name,
				Label: col.Label,
			})
		}
	} else {
		for i, h := range t.Headers {
			name := h.String()
			if i < len(t.FieldNames) && t.FieldNames[i] != "" {
				name = t.FieldNames[i]
			}
			rt.Columns = append(rt.Columns, reactColumn{
				Name:  name,
				Label: h.String(),
			})
		}
	}

	for _, row := range t.Rows {
		rowMap := make(map[string]reactCell)
		for key, val := range row {
			cell := reactCell{Text: val.String()}
			htmlStr := val.HTML()
			if htmlStr != cell.Text {
				cell.HTML = htmlStr
			}
			rowMap[key] = cell
		}
		rt.Rows = append(rt.Rows, rowMap)
	}

	return reactData{Type: "table", Table: rt}
}

func convertTree(t *api.TextTree) *reactTree {
	if t == nil {
		return nil
	}
	rt := &reactTree{}
	if t.Node != nil {
		rt.Label = t.Node.String()
		rt.HTML = t.Node.HTML()
	}
	for _, child := range t.Children {
		if ct := convertTree(&child); ct != nil {
			rt.Children = append(rt.Children, *ct)
		}
	}
	return rt
}

func convertTypedMap(tm *api.TypedMap, schema *api.PrettyObject) reactData {
	fields := make(map[string]reactData)
	for key, val := range *tm {
		fields[key] = convertTypedValue(&val, nil)
	}
	rd := reactData{Type: "map", Fields: fields}
	if schema != nil {
		rd.Schema = convertSchema(schema)
	}
	return rd
}

func convertTypedList(tl *api.TypedList) reactData {
	var items []reactData
	for _, item := range *tl {
		items = append(items, convertTypedValue(&item, nil))
	}
	return reactData{Type: "list", List: items}
}

func convertTextList(tl api.TextList) reactData {
	var items []reactData
	for _, item := range tl {
		items = append(items, convertTextable(item))
	}
	return reactData{Type: "list", List: items}
}

func convertTextable(t api.Textable) reactData {
	switch v := t.(type) {
	case api.TextTable:
		return convertTable(&v, nil)
	case *api.TextTable:
		return convertTable(v, nil)
	case api.TextTree:
		return reactData{Type: "tree", Tree: convertTree(&v)}
	case *api.TextTree:
		return reactData{Type: "tree", Tree: convertTree(v)}
	default:
		return reactData{Type: "text", Text: t.String(), HTML: t.HTML()}
	}
}

func convertSchema(schema *api.PrettyObject) []reactSchemaField {
	var fields []reactSchemaField
	for _, f := range schema.Fields {
		fields = append(fields, reactSchemaField{
			Name:  f.Name,
			Label: f.Label,
			Type:  f.Format,
		})
	}
	return fields
}

func buildReactHTML(jsonData, customJSX string) string {
	var b strings.Builder

	component := reactAssets.DefaultComponent
	if customJSX != "" {
		component = customJSX
	}

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="https://cdn.jsdelivr.net/npm/react@18.3.1/umd/react.production.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/react-dom@18.3.1/umd/react-dom.production.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@babel/standalone@7.26.10/babel.min.js"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://code.iconify.design/iconify-icon/2.0.0/iconify-icon.min.js"></script>
    <style>`)
	b.WriteString(api.GetChromaCSS())
	b.WriteString(`</style>
</head>
<body>
    <div id="root"></div>
    <script id="clicky-data" type="application/json">`)
	b.WriteString(jsonData)
	b.WriteString(`</script>
    <script type="text/babel">`)
	b.WriteString(component)
	b.WriteString(`
const __data = JSON.parse(document.getElementById('clicky-data').textContent);
ReactDOM.createRoot(document.getElementById('root')).render(<App data={__data} />);
</script>
</body>
</html>`)

	return b.String()
}
