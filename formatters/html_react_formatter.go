package formatters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	apiicons "github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/api/tailwind"
)

func init() {
	htmlReact := &HTMLReactFormatter{}
	RegisterFormatter("html-react", htmlReact.Format)

	clickyJSON := &ClickyJSONFormatter{}
	RegisterFormatter("clicky-json", clickyJSON.Format)
}

type HTMLReactFormatter struct{}

// ClickyJSONFormatter emits the Clicky document JSON (application/json+clicky).
// The HTTP layer also accepts the legacy application/clicky+json alias. The
// payload is the same shape consumed by @flanksource/clicky-ui's <Clicky data={...} />.
type ClickyJSONFormatter struct{}

type clickyDocument struct {
	Version int        `json:"version"`
	Node    clickyNode `json:"node"`
}

type clickyStyle struct {
	ClassName       string `json:"className,omitempty"`
	Color           string `json:"color,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	Bold            bool   `json:"bold,omitempty"`
	Faint           bool   `json:"faint,omitempty"`
	Italic          bool   `json:"italic,omitempty"`
	Underline       bool   `json:"underline,omitempty"`
	Strikethrough   bool   `json:"strikethrough,omitempty"`
	TextTransform   string `json:"textTransform,omitempty"`
	MaxWidth        int    `json:"maxWidth,omitempty"`
	MaxLines        int    `json:"maxLines,omitempty"`
	TruncateMode    string `json:"truncateMode,omitempty"`
	Monospace       bool   `json:"monospace,omitempty"`
}

type clickyField struct {
	Name  string     `json:"name"`
	Label string     `json:"label,omitempty"`
	Value clickyNode `json:"value"`
}

type clickyColumn struct {
	Name   string      `json:"name"`
	Label  string      `json:"label,omitempty"`
	Header *clickyNode `json:"header,omitempty"`
	Align  string      `json:"align,omitempty"`
}

type clickyRow struct {
	Cells  map[string]clickyNode `json:"cells"`
	Detail *clickyNode           `json:"detail,omitempty"`
}

type clickyTreeItem struct {
	ID       string           `json:"id"`
	Label    clickyNode       `json:"label"`
	Children []clickyTreeItem `json:"children,omitempty"`
}

type clickyNode struct {
	Kind            string            `json:"kind"`
	Plain           string            `json:"plain,omitempty"`
	Style           *clickyStyle      `json:"style,omitempty"`
	Text            string            `json:"text,omitempty"`
	Children        []clickyNode      `json:"children,omitempty"`
	Tooltip         *clickyNode       `json:"tooltip,omitempty"`
	HTML            string            `json:"html,omitempty"`
	Inline          bool              `json:"inline,omitempty"`
	Ordered         bool              `json:"ordered,omitempty"`
	Unstyled        bool              `json:"unstyled,omitempty"`
	Bullet          *clickyNode       `json:"bullet,omitempty"`
	Items           []clickyNode      `json:"items,omitempty"`
	Fields          []clickyField     `json:"fields,omitempty"`
	Columns         []clickyColumn    `json:"columns,omitempty"`
	Rows            []clickyRow       `json:"rows,omitempty"`
	Roots           []clickyTreeItem  `json:"roots,omitempty"`
	Label           *clickyNode       `json:"label,omitempty"`
	Content         *clickyNode       `json:"content,omitempty"`
	Href            string            `json:"href,omitempty"`
	Target          string            `json:"target,omitempty"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Flags           map[string]string `json:"flags,omitempty"`
	AutoRun         bool              `json:"autoRun,omitempty"`
	ID              string            `json:"id,omitempty"`
	Payload         string            `json:"payload,omitempty"`
	Variant         string            `json:"variant,omitempty"`
	Iconify         string            `json:"iconify,omitempty"`
	Unicode         string            `json:"unicode,omitempty"`
	Language        string            `json:"language,omitempty"`
	Source          string            `json:"source,omitempty"`
	HighlightedHTML string            `json:"highlightedHtml,omitempty"`
}

func (f *HTMLReactFormatter) Format(data any, opts FormatOptions) (string, error) {
	jsonBytes, err := clickyDocumentJSON(data, opts, false)
	if err != nil || jsonBytes == nil {
		return "", err
	}
	return buildReactHTML(string(jsonBytes)), nil
}

// Format returns the Clicky document JSON for data. Indented for readability
// since this format is typically consumed by humans or piped into tools.
func (f *ClickyJSONFormatter) Format(data any, opts FormatOptions) (string, error) {
	jsonBytes, err := clickyDocumentJSON(data, opts, true)
	if err != nil || jsonBytes == nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// clickyDocumentJSON is the shared payload build for html-react and clicky-json.
// Returns nil bytes (and nil error) when the input data produces an empty
// document, so callers can short-circuit to an empty response.
func clickyDocumentJSON(data any, opts FormatOptions, indent bool) ([]byte, error) {
	if slice, ok := data.([]interface{}); ok && len(slice) == 1 {
		data = slice[0]
	}

	prettyData, err := ToPrettyDataWithOptions(data, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to PrettyData: %w", err)
	}
	if prettyData == nil || prettyData.IsEmpty() {
		return nil, nil
	}

	payload := convertPrettyData(prettyData)
	if indent {
		return json.MarshalIndent(payload, "", "  ")
	}
	return json.Marshal(payload)
}

func convertPrettyData(pd *api.PrettyData) clickyDocument {
	return clickyDocument{
		Version: 1,
		Node:    convertTypedValue(&pd.TypedValue, pd.Schema),
	}
}

func convertTypedValue(tv *api.TypedValue, schema *api.PrettyObject) clickyNode {
	if tv == nil {
		return clickyTextNode("")
	}
	if tv.Table != nil {
		return convertTable(tv.Table)
	}
	if tv.Tree != nil {
		return convertTree(tv.Tree)
	}
	if tv.TypedMap != nil {
		return convertTypedMap(tv.TypedMap, schema)
	}
	if tv.TypedList != nil {
		return convertTypedList(tv.TypedList)
	}
	if tv.Map != nil {
		return convertTextMap(*tv.Map, schema)
	}
	if tv.Slice != nil {
		return convertTextList(*tv.Slice)
	}
	if tv.Textable != nil {
		return convertTextable(tv.Textable)
	}
	return clickyTextNode(tv.String())
}

func convertTextable(t api.Textable) clickyNode {
	switch v := t.(type) {
	case api.Text:
		return convertText(v)
	case *api.Text:
		return convertText(*v)
	case api.Link:
		return convertLink(v)
	case *api.Link:
		return convertLink(*v)
	case api.LinkCommand:
		return convertLinkCommand(v)
	case *api.LinkCommand:
		return convertLinkCommand(*v)
	case api.TextTable:
		return convertTable(&v)
	case *api.TextTable:
		return convertTable(v)
	case api.TextTree:
		return convertTree(&v)
	case *api.TextTree:
		return convertTree(v)
	case api.TextList:
		return convertTextList(v)
	case *api.TextList:
		return convertTextList(*v)
	case api.List:
		return convertList(v)
	case *api.List:
		return convertList(*v)
	case api.TextMap:
		return convertTextMap(v, nil)
	case *api.TextMap:
		return convertTextMap(*v, nil)
	case api.TypedValue:
		return convertTypedValue(&v, nil)
	case *api.TypedValue:
		return convertTypedValue(v, nil)
	case api.TypedMap:
		return convertTypedMap(&v, nil)
	case *api.TypedMap:
		return convertTypedMap(v, nil)
	case api.TypedList:
		return convertTypedList(&v)
	case *api.TypedList:
		return convertTypedList(v)
	case api.Code:
		return convertCode(v)
	case *api.Code:
		return convertCode(*v)
	case api.Collapsed:
		return convertCollapsed(v)
	case *api.Collapsed:
		return convertCollapsed(*v)
	case api.Button:
		return convertButton(v)
	case *api.Button:
		return convertButton(*v)
	case api.ButtonGroup:
		return convertButtonGroup(v)
	case *api.ButtonGroup:
		return convertButtonGroup(*v)
	case api.HtmlElement:
		return clickyNode{Kind: "html", Plain: v.String(), HTML: v.HTML()}
	case *api.HtmlElement:
		return clickyNode{Kind: "html", Plain: v.String(), HTML: v.HTML()}
	case api.Comment:
		return clickyNode{Kind: "comment", Plain: string(v), Text: string(v)}
	case api.KeyValuePair:
		return convertKeyValuePair(v)
	case *api.KeyValuePair:
		return convertKeyValuePair(*v)
	case api.DescriptionList:
		return convertDescriptionList(v)
	case *api.DescriptionList:
		return convertDescriptionList(*v)
	case apiicons.Icon:
		return convertIcon(v)
	case *apiicons.Icon:
		return convertIcon(*v)
	default:
		return clickyNode{Kind: "html", Plain: t.String(), HTML: t.HTML()}
	}
}

func convertText(text api.Text) clickyNode {
	return convertInlineTextNode("text", text)
}

func convertInlineTextNode(kind string, text api.Text) clickyNode {
	node := clickyNode{
		Kind:  kind,
		Plain: text.String(),
		Text:  text.Content,
		Style: convertTextStyle(text.Style, text.Class, false),
	}

	if text.Tooltip != nil && text.Tooltip.String() != "" {
		tooltip := convertTextable(text.Tooltip)
		node.Tooltip = &tooltip
	}

	for _, child := range text.Children {
		node.Children = append(node.Children, convertTextable(child))
	}

	return node
}

func convertLink(link api.Link) clickyNode {
	node := convertInlineTextNode("link", link.Content)
	node.Href = link.Href
	node.Target = string(link.Target)
	return node
}

func convertLinkCommand(link api.LinkCommand) clickyNode {
	node := convertInlineTextNode("link-command", link.Content)
	node.Command = link.Command
	node.Target = string(link.Target)
	node.AutoRun = link.AutoRun
	if len(link.Args) > 0 {
		node.Args = append([]string(nil), link.Args...)
	}
	if len(link.Flags) > 0 {
		node.Flags = make(map[string]string, len(link.Flags))
		for key, value := range link.Flags {
			node.Flags[key] = value
		}
	}
	return node
}

func convertList(list api.List) clickyNode {
	node := clickyNode{
		Kind:     "list",
		Plain:    list.String(),
		Ordered:  list.Numbered || list.Ordered,
		Unstyled: list.Unstyled,
		Inline:   list.MaxInline > 0 && len(list.Items) <= list.MaxInline,
	}

	if list.Bullet != nil {
		bullet := convertTextable(list.Bullet)
		node.Bullet = &bullet
	}

	for _, item := range list.Items {
		node.Items = append(node.Items, convertTextable(item))
	}

	return node
}

func convertTextList(list api.TextList) clickyNode {
	node := clickyNode{Kind: "list", Plain: list.String()}
	for _, item := range list {
		node.Items = append(node.Items, convertTextable(item))
	}
	return node
}

func convertTextMap(tm api.TextMap, schema *api.PrettyObject) clickyNode {
	fields := make([]clickyField, 0, len(tm))
	for _, key := range orderedFieldNames(schema, mapKeysTextMap(tm)) {
		value, ok := tm[key]
		if !ok {
			continue
		}
		fields = append(fields, clickyField{
			Name:  key,
			Label: clickySchemaFieldLabel(schema, key),
			Value: convertTextable(value),
		})
	}
	return clickyNode{Kind: "map", Plain: tm.String(), Fields: fields}
}

func convertTypedMap(tm *api.TypedMap, schema *api.PrettyObject) clickyNode {
	if tm == nil {
		return clickyNode{Kind: "map"}
	}

	fields := make([]clickyField, 0, len(*tm))
	for _, key := range orderedFieldNames(schema, mapKeysTypedMap(*tm)) {
		value, ok := (*tm)[key]
		if !ok {
			continue
		}
		fields = append(fields, clickyField{
			Name:  key,
			Label: clickySchemaFieldLabel(schema, key),
			Value: convertTypedValue(&value, nil),
		})
	}
	return clickyNode{Kind: "map", Plain: tm.String(), Fields: fields}
}

func convertTypedList(tl *api.TypedList) clickyNode {
	if tl == nil {
		return clickyNode{Kind: "list"}
	}

	node := clickyNode{Kind: "list", Plain: tl.String()}
	for _, item := range *tl {
		node.Items = append(node.Items, convertTypedValue(&item, nil))
	}
	return node
}

func convertTable(table *api.TextTable) clickyNode {
	if table == nil {
		return clickyNode{Kind: "table"}
	}

	node := clickyNode{Kind: "table", Plain: table.String()}

	if len(table.Headers) > 0 {
		for i, header := range table.Headers {
			column := clickyColumn{}
			if i < len(table.FieldNames) && table.FieldNames[i] != "" {
				column.Name = table.FieldNames[i]
			} else if i < len(table.Columns) && table.Columns[i].Name != "" {
				column.Name = table.Columns[i].Name
			} else {
				column.Name = header.String()
			}

			if i < len(table.Columns) {
				column.Label = table.Columns[i].Label
				column.Align = alignFromStyle(table.Columns[i].Style)
			}
			if column.Label == "" {
				column.Label = header.String()
			}

			headerNode := convertTextable(header)
			column.Header = &headerNode
			node.Columns = append(node.Columns, column)
		}
	} else {
		for _, columnDef := range table.Columns {
			column := clickyColumn{
				Name:  columnDef.Name,
				Label: columnDef.Label,
				Align: alignFromStyle(columnDef.Style),
			}
			if column.Label == "" {
				column.Label = columnDef.Name
			}
			node.Columns = append(node.Columns, column)
		}
	}

	for rowIndex, row := range table.Rows {
		rowNode := clickyRow{Cells: map[string]clickyNode{}}
		for _, column := range node.Columns {
			if cell, ok := row[column.Name]; ok {
				rowNode.Cells[column.Name] = convertTypedValue(&cell, nil)
				continue
			}
			if cell, ok := row[column.Label]; ok {
				rowNode.Cells[column.Name] = convertTypedValue(&cell, nil)
				continue
			}
			rowNode.Cells[column.Name] = clickyTextNode("")
		}

		if rowIndex < len(table.RowDetail) && table.RowDetail[rowIndex] != nil {
			detail := convertTextable(table.RowDetail[rowIndex])
			rowNode.Detail = &detail
		}

		node.Rows = append(node.Rows, rowNode)
	}

	return node
}

func convertTree(tree *api.TextTree) clickyNode {
	if tree == nil {
		return clickyNode{Kind: "tree"}
	}

	node := clickyNode{Kind: "tree", Plain: tree.String()}
	if tree.Node == nil {
		for index, child := range tree.Children {
			node.Roots = append(node.Roots, convertTreeItem(&child, fmt.Sprintf("root-%d", index)))
		}
		return node
	}

	node.Roots = append(node.Roots, convertTreeItem(tree, "root"))
	return node
}

func convertTreeItem(tree *api.TextTree, id string) clickyTreeItem {
	item := clickyTreeItem{
		ID:    id,
		Label: convertTextable(tree.Node),
	}

	for index, child := range tree.Children {
		item.Children = append(item.Children, convertTreeItem(&child, fmt.Sprintf("%s-%d", id, index)))
	}

	return item
}

func convertCode(code api.Code) clickyNode {
	return clickyNode{
		Kind:            "code",
		Plain:           code.String(),
		Style:           convertTextStyle(code.Style, api.Class{}, true),
		Language:        code.Language,
		Source:          code.Content,
		HighlightedHTML: code.HTML(),
	}
}

func convertCollapsed(collapsed api.Collapsed) clickyNode {
	label := clickyNode{
		Kind:  "text",
		Plain: collapsed.Label,
		Text:  collapsed.Label,
		Style: convertTextStyle(collapsed.Style, api.Class{}, false),
	}
	if collapsed.Icon != nil {
		label.Children = append([]clickyNode{convertIcon(*collapsed.Icon)}, label.Children...)
	}

	node := clickyNode{
		Kind:  "collapsed",
		Plain: collapsed.String(),
		Label: &label,
	}

	if collapsed.Content != nil {
		content := convertTextable(collapsed.Content)
		node.Content = &content
	}

	return node
}

func convertButton(button api.Button) clickyNode {
	label := clickyTextNode(button.Label)
	return clickyNode{
		Kind:    "button",
		Plain:   button.String(),
		Label:   &label,
		Href:    button.Href,
		ID:      button.ID,
		Payload: button.Payload,
		Variant: button.Variant,
	}
}

func convertButtonGroup(group api.ButtonGroup) clickyNode {
	node := clickyNode{Kind: "button-group", Plain: group.String()}
	for _, button := range group.Buttons {
		node.Items = append(node.Items, convertButton(button))
	}
	return node
}

func convertIcon(icon apiicons.Icon) clickyNode {
	return clickyNode{
		Kind:    "icon",
		Plain:   icon.String(),
		Style:   convertTextStyle(icon.Style, api.Class{}, false),
		Unicode: icon.Unicode,
		Iconify: icon.Iconify,
	}
}

func convertKeyValuePair(pair api.KeyValuePair) clickyNode {
	if pair.IsEmpty() {
		return clickyNode{Kind: "map"}
	}
	return clickyNode{
		Kind:  "map",
		Plain: pair.String(),
		Fields: []clickyField{
			{
				Name:  pair.Key,
				Label: pair.Key,
				Value: convertAnyToNode(pair.Value),
			},
		},
	}
}

func convertDescriptionList(list api.DescriptionList) clickyNode {
	node := clickyNode{Kind: "map", Plain: list.String()}
	for _, item := range list.Items {
		if item.IsEmpty() {
			continue
		}
		node.Fields = append(node.Fields, clickyField{
			Name:  item.Key,
			Label: item.Key,
			Value: convertAnyToNode(item.Value),
		})
	}
	return node
}

func convertAnyToNode(value any) clickyNode {
	if typed := api.TryTypedValue(value); typed != nil {
		return convertTypedValue(typed, nil)
	}
	return clickyTextNode(fmt.Sprintf("%v", value))
}

func clickyTextNode(text string) clickyNode {
	return clickyNode{Kind: "text", Plain: text, Text: text}
}

func convertTextStyle(styleStr string, class api.Class, monospace bool) *clickyStyle {
	style := clickyStyle{
		ClassName: strings.TrimSpace(styleStr),
		Monospace: monospace || strings.Contains(styleStr, "font-mono"),
	}

	if styleStr != "" {
		parsed := tailwind.ParseStyle(styleStr)
		style.Color = parsed.Foreground
		style.BackgroundColor = parsed.Background
		style.Bold = parsed.Bold
		style.Faint = parsed.Faint
		style.Italic = parsed.Italic
		style.Underline = parsed.Underline
		style.Strikethrough = parsed.Strikethrough
		style.TextTransform = parsed.TextTransform
		style.MaxWidth = parsed.MaxWidth
		style.MaxLines = parsed.MaxLines
		style.TruncateMode = parsed.TruncateMode
	}

	if class.Foreground != nil {
		style.Color = class.Foreground.Hex
	}
	if class.Background != nil {
		style.BackgroundColor = class.Background.Hex
	}
	if class.Font != nil {
		style.Bold = style.Bold || class.Font.Bold
		style.Faint = style.Faint || class.Font.Faint
		style.Italic = style.Italic || class.Font.Italic
		style.Underline = style.Underline || class.Font.Underline
		style.Strikethrough = style.Strikethrough || class.Font.Strikethrough
		if isMonospaceFont(class.Font.Name) {
			style.Monospace = true
		}
	}

	if style == (clickyStyle{}) {
		return nil
	}

	return &style
}

func isMonospaceFont(name string) bool {
	switch strings.ToLower(name) {
	case "courier", "courier new", "sfmono-regular", "menlo", "monaco", "consolas":
		return true
	default:
		return false
	}
}

func alignFromStyle(style string) string {
	switch {
	case strings.Contains(style, "text-right"):
		return "right"
	case strings.Contains(style, "text-center"):
		return "center"
	default:
		return ""
	}
}

func mapKeysTextMap(tm api.TextMap) []string {
	keys := make([]string, 0, len(tm))
	for key := range tm {
		keys = append(keys, key)
	}
	return keys
}

func mapKeysTypedMap(tm api.TypedMap) []string {
	keys := make([]string, 0, len(tm))
	for key := range tm {
		keys = append(keys, key)
	}
	return keys
}

func orderedFieldNames(schema *api.PrettyObject, keys []string) []string {
	if len(keys) == 0 {
		return nil
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	ordered := make([]string, 0, len(keys))
	if schema != nil {
		for _, field := range schema.Fields {
			if _, ok := keySet[field.Name]; ok {
				ordered = append(ordered, field.Name)
				delete(keySet, field.Name)
			}
		}
	}

	remaining := make([]string, 0, len(keySet))
	for key := range keySet {
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)

	return append(ordered, remaining...)
}

func clickySchemaFieldLabel(schema *api.PrettyObject, name string) string {
	if schema == nil {
		return ""
	}
	for _, field := range schema.Fields {
		if field.Name == name {
			return field.Label
		}
	}
	return ""
}

func buildReactHTML(jsonData string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en" data-theme="light" data-density="comfortable">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Clicky</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@flanksource/clicky-ui@latest/src/styles/tokens.css" />
    <script src="https://code.iconify.design/iconify-icon/2.1.0/iconify-icon.min.js"></script>
    <link id="prism-light" rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1.29.0/themes/prism.min.css" />
    <link id="prism-dark" rel="stylesheet" href="https://cdn.jsdelivr.net/npm/prismjs@1.29.0/themes/prism-tomorrow.min.css" disabled />
    <script src="https://cdn.jsdelivr.net/npm/prismjs@1.29.0/prism.min.js" data-manual></script>
    <script src="https://cdn.jsdelivr.net/npm/prismjs@1.29.0/plugins/autoloader/prism-autoloader.min.js"></script>
    <style>
      iconify-icon { display: inline-block; width: 1em; height: 1em; vertical-align: -0.125em; }
      pre[class*="language-"], code[class*="language-"] { background: transparent !important; padding: 0 !important; font-size: inherit; text-shadow: none; }
    </style>
    <script src="https://cdn.tailwindcss.com/3.4.17"></script>
    <script>
      tailwind.config = {
        darkMode: ["selector", '[data-theme="dark"]'],
        theme: {
          extend: {
            colors: {
              border: "hsl(var(--border))",
              input: "hsl(var(--input))",
              ring: "hsl(var(--ring))",
              background: "hsl(var(--background))",
              foreground: "hsl(var(--foreground))",
              primary: { DEFAULT: "hsl(var(--primary))", foreground: "hsl(var(--primary-foreground))" },
              secondary: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
              destructive: { DEFAULT: "hsl(var(--destructive))", foreground: "hsl(var(--destructive-foreground))" },
              muted: { DEFAULT: "hsl(var(--muted))", foreground: "hsl(var(--muted-foreground))" },
              accent: { DEFAULT: "hsl(var(--accent))", foreground: "hsl(var(--accent-foreground))" },
              popover: { DEFAULT: "hsl(var(--popover))", foreground: "hsl(var(--popover-foreground))" },
              card: { DEFAULT: "hsl(var(--card))", foreground: "hsl(var(--card-foreground))" },
            },
            borderRadius: { lg: "var(--radius)", md: "calc(var(--radius) - 2px)", sm: "calc(var(--radius) - 4px)" },
            spacing: {
              "control-h": "var(--control-height)",
              "control-px": "var(--control-padding-x)",
              "density-1": "var(--spacing-unit)",
              "density-2": "calc(var(--spacing-unit) * 2)",
              "density-3": "calc(var(--spacing-unit) * 3)",
              "density-4": "calc(var(--spacing-unit) * 4)",
            },
            fontSize: { "density-base": "var(--font-size-base)" },
          },
        },
      };
    </script>
    <script type="importmap">
      {
        "imports": {
          "preact": "https://esm.sh/preact@10.24.3",
          "preact/": "https://esm.sh/preact@10.24.3/",
          "preact/hooks": "https://esm.sh/preact@10.24.3/hooks",
          "preact/compat": "https://esm.sh/preact@10.24.3/compat",
          "preact/jsx-runtime": "https://esm.sh/preact@10.24.3/jsx-runtime",
          "react": "https://esm.sh/preact@10.24.3/compat",
          "react/": "https://esm.sh/preact@10.24.3/compat/",
          "react/jsx-runtime": "https://esm.sh/preact@10.24.3/jsx-runtime",
          "react-dom": "https://esm.sh/preact@10.24.3/compat",
          "react-dom/client": "https://esm.sh/preact@10.24.3/compat",
          "@flanksource/clicky-ui": "https://esm.sh/@flanksource/clicky-ui@latest?alias=react:preact/compat,react-dom:preact/compat&deps=preact@10.24.3&external=preact"
        }
      }
    </script>
  </head>
  <body class="bg-background text-foreground antialiased">
    <main class="mx-auto max-w-7xl px-4 py-4">
      <div id="root"></div>
    </main>
    <script id="clicky-data" type="application/json">`)
	b.WriteString(jsonData)
	b.WriteString(`</script>
    <script type="module">
      import { h, render } from "preact";
      import { Clicky } from "@flanksource/clicky-ui";

      const LANGUAGE_ALIASES = { sh: "bash", shell: "bash", yml: "yaml", ts: "typescript", js: "javascript", py: "python" };
      function highlightCodeBlocks(root) {
        if (typeof Prism === "undefined") return;
        for (const pre of root.querySelectorAll("pre:not([data-prism])")) {
          pre.dataset.prism = "true";
          const wrapper = pre.parentElement;
          const label = wrapper?.firstElementChild?.textContent?.trim().toLowerCase() ?? "";
          const lang = LANGUAGE_ALIASES[label] ?? label;
          if (wrapper && wrapper.firstElementChild !== pre && wrapper.lastElementChild === pre) {
            wrapper.replaceWith(pre);
          }
          if (!lang) continue;
          const existingCode = pre.querySelector(":scope > code");
          const code = existingCode ?? document.createElement("code");
          if (!existingCode) {
            code.textContent = pre.textContent ?? "";
            pre.textContent = "";
            pre.appendChild(code);
          }
          pre.className = (pre.className + " language-" + lang).trim();
          code.className = "language-" + lang;
          Prism.highlightElement(code);
        }
      }

      const output = document.getElementById("root");
      const data = JSON.parse(document.getElementById("clicky-data").textContent);
      render(h(Clicky, { data }), output);
      highlightCodeBlocks(output);
      new MutationObserver(() => highlightCodeBlocks(output)).observe(output, { childList: true, subtree: true });
    </script>
  </body>
</html>`)
	return b.String()
}
