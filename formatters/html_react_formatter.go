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

// ClickyDocument is the {version:1, node:{…}} envelope consumed by
// @flanksource/clicky-ui's <Clicky data={…} /> component. It is built from a
// PrettyData tree (clicky-json / html-react formatters) or directly from a
// Textable via NewClickyDocument. Every field below is concrete (no interfaces),
// so the document round-trips through encoding/json (Marshal and Unmarshal) with
// the stock codec — callers can persist it and reload it without loss.
type ClickyDocument struct {
	Version  int            `json:"version"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Node     ClickyNode     `json:"node"`
}

// ClickyDocumentProvider lets rich producers provide a fully structured Clicky
// document without first flattening through PrettyData.
type ClickyDocumentProvider interface {
	ClickyDocument() ClickyDocument
}

type ClickyStyle struct {
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

type ClickyField struct {
	Name  string     `json:"name"`
	Label string     `json:"label,omitempty"`
	Value ClickyNode `json:"value"`
}

type ClickyColumn struct {
	Name   string      `json:"name"`
	Label  string      `json:"label,omitempty"`
	Header *ClickyNode `json:"header,omitempty"`
	Align  string      `json:"align,omitempty"`
}

type ClickyRow struct {
	Cells  map[string]ClickyNode `json:"cells"`
	Detail *ClickyNode           `json:"detail,omitempty"`
}

type ClickyTreeItem struct {
	ID       string           `json:"id"`
	Label    ClickyNode       `json:"label"`
	Children []ClickyTreeItem `json:"children,omitempty"`
}

type ClickyStackFrame struct {
	FunctionName      string   `json:"functionName,omitempty"`
	DisplayName       string   `json:"displayName,omitempty"`
	File              string   `json:"file,omitempty"`
	Line              int      `json:"line,omitempty"`
	Location          string   `json:"location,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Runtime           bool     `json:"runtime,omitempty"`
	NativeMethod      bool     `json:"nativeMethod,omitempty"`
	AnnotationText    string   `json:"annotationText,omitempty"`
	Class             string   `json:"class,omitempty"`
	Method            string   `json:"method,omitempty"`
	SourceLines       []string `json:"sourceLines,omitempty"`
	SourceLineNumbers []int    `json:"sourceLineNumbers,omitempty"`
	SourceStartLine   int      `json:"sourceStartLine,omitempty"`
	SourceLanguage    string   `json:"sourceLanguage,omitempty"`
}

type ClickyNode struct {
	Kind            string             `json:"kind"`
	Plain           string             `json:"plain,omitempty"`
	Style           *ClickyStyle       `json:"style,omitempty"`
	Text            string             `json:"text,omitempty"`
	Children        []ClickyNode       `json:"children,omitempty"`
	Tooltip         *ClickyNode        `json:"tooltip,omitempty"`
	HTML            string             `json:"html,omitempty"`
	Inline          bool               `json:"inline,omitempty"`
	Ordered         bool               `json:"ordered,omitempty"`
	Checked         *bool              `json:"checked,omitempty"`
	Unstyled        bool               `json:"unstyled,omitempty"`
	Bullet          *ClickyNode        `json:"bullet,omitempty"`
	Items           []ClickyNode       `json:"items,omitempty"`
	Fields          []ClickyField      `json:"fields,omitempty"`
	Columns         []ClickyColumn     `json:"columns,omitempty"`
	Rows            []ClickyRow        `json:"rows,omitempty"`
	Roots           []ClickyTreeItem   `json:"roots,omitempty"`
	Label           *ClickyNode        `json:"label,omitempty"`
	Content         *ClickyNode        `json:"content,omitempty"`
	Level           int                `json:"level,omitempty"`
	Severity        string             `json:"severity,omitempty"`
	Attributes      map[string]string  `json:"attributes,omitempty"`
	Href            string             `json:"href,omitempty"`
	Target          string             `json:"target,omitempty"`
	Command         string             `json:"command,omitempty"`
	Args            []string           `json:"args,omitempty"`
	Flags           map[string]string  `json:"flags,omitempty"`
	AutoRun         bool               `json:"autoRun,omitempty"`
	ID              string             `json:"id,omitempty"`
	Payload         string             `json:"payload,omitempty"`
	Variant         string             `json:"variant,omitempty"`
	Iconify         string             `json:"iconify,omitempty"`
	Unicode         string             `json:"unicode,omitempty"`
	Language        string             `json:"language,omitempty"`
	Source          string             `json:"source,omitempty"`
	HighlightedHTML string             `json:"highlightedHtml,omitempty"`
	ExceptionClass  string             `json:"exceptionClass,omitempty"`
	Message         string             `json:"message,omitempty"`
	CausedBy        []string           `json:"causedBy,omitempty"`
	Frames          []ClickyStackFrame `json:"frames,omitempty"`
	// Badge fields — set for Kind == "badge" (LabelBadge). Rendered by
	// clicky-ui as <Badge variant="label" label={Value1} value={Value2} />.
	BadgeLabel string `json:"badgeLabel,omitempty"`
	BadgeValue string `json:"badgeValue,omitempty"`
	BadgeColor string `json:"badgeColor,omitempty"`
	BadgeText  string `json:"badgeText,omitempty"`
	BadgeShape string `json:"badgeShape,omitempty"`
	BadgeIcon  string `json:"badgeIcon,omitempty"`
	// Object/execution tree payloads — set for Kind "object-graph" / "execution-tree".
	// Passed through verbatim to clicky-ui's <ObjectGraph>/<ExecutionTree> roots,
	// so producers can supply their own node shape without mirroring it here.
	Objects        any `json:"objects,omitempty"`
	ExecutionRoots any `json:"executionRoots,omitempty"`
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

	if provider, ok := data.(ClickyDocumentProvider); ok {
		payload := provider.ClickyDocument()
		if indent {
			return json.MarshalIndent(payload, "", "  ")
		}
		return json.Marshal(payload)
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

func convertPrettyData(pd *api.PrettyData) ClickyDocument {
	return ClickyDocument{
		Version: 1,
		Node:    convertTypedValue(&pd.TypedValue, pd.Schema),
	}
}

func convertTypedValue(tv *api.TypedValue, schema *api.PrettyObject) ClickyNode {
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

func convertTextable(t api.Textable) ClickyNode {
	if provider, ok := t.(ClickyDocumentProvider); ok {
		return provider.ClickyDocument().Node
	}

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
	case api.StackTrace:
		return convertStackTrace(v)
	case *api.StackTrace:
		return convertStackTrace(*v)
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
	case api.Heading:
		return convertHeading(v)
	case *api.Heading:
		return convertHeading(*v)
	case api.Blockquote:
		return convertBlockquote(v)
	case *api.Blockquote:
		return convertBlockquote(*v)
	case api.Admonition:
		return convertAdmonition(v)
	case *api.Admonition:
		return convertAdmonition(*v)
	case api.FootnoteRef:
		return convertFootnoteRef(v)
	case *api.FootnoteRef:
		return convertFootnoteRef(*v)
	case api.Footnote:
		return convertFootnote(v)
	case *api.Footnote:
		return convertFootnote(*v)
	case api.Footnotes:
		return convertFootnotes(v)
	case *api.Footnotes:
		return convertFootnotes(*v)
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
		return ClickyNode{Kind: "html", Plain: v.String(), HTML: v.HTML()}
	case *api.HtmlElement:
		return ClickyNode{Kind: "html", Plain: v.String(), HTML: v.HTML()}
	case api.Comment:
		return ClickyNode{Kind: "comment", Plain: string(v), Text: string(v)}
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
	case api.LabelBadge:
		return convertLabelBadge(v)
	case *api.LabelBadge:
		return convertLabelBadge(*v)
	default:
		return ClickyNode{Kind: "html", Plain: t.String(), HTML: t.HTML()}
	}
}

func convertText(text api.Text) ClickyNode {
	return convertInlineTextNode("text", text)
}

func convertInlineTextNode(kind string, text api.Text) ClickyNode {
	node := ClickyNode{
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

func convertLink(link api.Link) ClickyNode {
	node := convertInlineTextNode("link", link.Content)
	node.Href = link.Href
	node.Target = string(link.Target)
	return node
}

func convertLinkCommand(link api.LinkCommand) ClickyNode {
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

func convertList(list api.List) ClickyNode {
	node := ClickyNode{
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

func convertTextList(list api.TextList) ClickyNode {
	node := ClickyNode{Kind: "list", Plain: list.String()}
	for _, item := range list {
		node.Items = append(node.Items, convertTextable(item))
	}
	return node
}

func convertTextMap(tm api.TextMap, schema *api.PrettyObject) ClickyNode {
	fields := make([]ClickyField, 0, len(tm))
	for _, key := range orderedFieldNames(schema, mapKeysTextMap(tm)) {
		value, ok := tm[key]
		if !ok {
			continue
		}
		fields = append(fields, ClickyField{
			Name:  key,
			Label: clickySchemaFieldLabel(schema, key),
			Value: convertTextable(value),
		})
	}
	return ClickyNode{Kind: "map", Plain: tm.String(), Fields: fields}
}

func convertTypedMap(tm *api.TypedMap, schema *api.PrettyObject) ClickyNode {
	if tm == nil {
		return ClickyNode{Kind: "map"}
	}

	fields := make([]ClickyField, 0, len(*tm))
	for _, key := range orderedFieldNames(schema, mapKeysTypedMap(*tm)) {
		value, ok := (*tm)[key]
		if !ok {
			continue
		}
		fields = append(fields, ClickyField{
			Name:  key,
			Label: clickySchemaFieldLabel(schema, key),
			Value: convertTypedValue(&value, nil),
		})
	}
	return ClickyNode{Kind: "map", Plain: tm.String(), Fields: fields}
}

func convertTypedList(tl *api.TypedList) ClickyNode {
	if tl == nil {
		return ClickyNode{Kind: "list"}
	}

	node := ClickyNode{Kind: "list", Plain: tl.String()}
	for _, item := range *tl {
		node.Items = append(node.Items, convertTypedValue(&item, nil))
	}
	return node
}

func convertTable(table *api.TextTable) ClickyNode {
	if table == nil {
		return ClickyNode{Kind: "table"}
	}

	node := ClickyNode{Kind: "table", Plain: table.String()}

	if len(table.Headers) > 0 {
		for i, header := range table.Headers {
			column := ClickyColumn{}
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
			column := ClickyColumn{
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
		rowNode := ClickyRow{Cells: map[string]ClickyNode{}}
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
		for cellName, cell := range row {
			if _, exists := rowNode.Cells[cellName]; exists {
				continue
			}
			rowNode.Cells[cellName] = convertTypedValue(&cell, nil)
		}

		if rowIndex < len(table.RowDetail) && table.RowDetail[rowIndex] != nil {
			detail := convertTextable(table.RowDetail[rowIndex])
			rowNode.Detail = &detail
		}

		node.Rows = append(node.Rows, rowNode)
	}

	return node
}

func convertTree(tree *api.TextTree) ClickyNode {
	if tree == nil {
		return ClickyNode{Kind: "tree"}
	}

	node := ClickyNode{Kind: "tree", Plain: tree.String()}
	if tree.Node == nil {
		for index, child := range tree.Children {
			node.Roots = append(node.Roots, convertTreeItem(&child, fmt.Sprintf("root-%d", index)))
		}
		return node
	}

	node.Roots = append(node.Roots, convertTreeItem(tree, "root"))
	return node
}

func convertTreeItem(tree *api.TextTree, id string) ClickyTreeItem {
	item := ClickyTreeItem{
		ID:    id,
		Label: convertTextable(tree.Node),
	}

	for index, child := range tree.Children {
		item.Children = append(item.Children, convertTreeItem(&child, fmt.Sprintf("%s-%d", id, index)))
	}

	return item
}

func convertCode(code api.Code) ClickyNode {
	return ClickyNode{
		Kind:            "code",
		Plain:           code.String(),
		Style:           convertTextStyle(code.Style, api.Class{}, true),
		Language:        code.Language,
		Source:          code.Content,
		HighlightedHTML: code.HTML(),
	}
}

func convertHeading(heading api.Heading) ClickyNode {
	node := ClickyNode{
		Kind:  "heading",
		Plain: heading.String(),
		Level: clampClickyHeadingLevel(heading.Level),
	}
	if heading.Content != nil {
		content := convertTextable(heading.Content)
		node.Content = &content
	}
	return node
}

func clampClickyHeadingLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

func convertBlockquote(blockquote api.Blockquote) ClickyNode {
	node := ClickyNode{Kind: "blockquote", Plain: blockquote.String()}
	if blockquote.Content != nil {
		content := convertTextable(blockquote.Content)
		node.Content = &content
	}
	return node
}

func convertAdmonition(admonition api.Admonition) ClickyNode {
	node := ClickyNode{
		Kind:     "admonition",
		Plain:    admonition.String(),
		Severity: admonition.Severity.String(),
	}
	if admonition.Title != nil {
		label := convertTextable(admonition.Title)
		node.Label = &label
	}
	if admonition.Body != nil {
		content := convertTextable(admonition.Body)
		node.Content = &content
	}
	return node
}

func convertFootnoteRef(ref api.FootnoteRef) ClickyNode {
	return ClickyNode{
		Kind:  "footnote-ref",
		Plain: ref.String(),
		ID:    strings.TrimSpace(ref.ID),
	}
}

func convertFootnote(footnote api.Footnote) ClickyNode {
	node := ClickyNode{
		Kind:  "footnote",
		Plain: footnote.String(),
		ID:    strings.TrimSpace(footnote.ID),
	}
	if footnote.Content != nil {
		content := convertTextable(footnote.Content)
		node.Content = &content
	}
	return node
}

func convertFootnotes(footnotes api.Footnotes) ClickyNode {
	node := ClickyNode{Kind: "footnotes", Plain: footnotes.String()}
	for _, footnote := range footnotes.Items {
		if strings.TrimSpace(footnote.ID) == "" {
			continue
		}
		node.Items = append(node.Items, convertFootnote(footnote))
	}
	return node
}

func convertStackTrace(trace api.StackTrace) ClickyNode {
	node := ClickyNode{
		Kind:           "stacktrace",
		Plain:          trace.String(),
		ExceptionClass: trace.ExceptionClass,
		Message:        trace.Message,
		CausedBy:       append([]string(nil), trace.CausedBy...),
		Language:       trace.Language,
	}
	if node.Language == "" {
		node.Language = "java"
	}
	for _, frame := range trace.Frames {
		functionName := frame.Class
		if frame.Method != "" {
			if functionName != "" {
				functionName += "."
			}
			functionName += frame.Method
		}
		location := frame.File
		if frame.Line > 0 {
			if location != "" {
				location += fmt.Sprintf(":%d", frame.Line)
			} else {
				location = fmt.Sprintf(":%d", frame.Line)
			}
		}
		kind := "user"
		if frame.Runtime {
			kind = "runtime"
		}
		node.Frames = append(node.Frames, ClickyStackFrame{
			FunctionName:      functionName,
			DisplayName:       functionName,
			File:              frame.File,
			Line:              frame.Line,
			Location:          location,
			Kind:              kind,
			Runtime:           frame.Runtime,
			NativeMethod:      frame.Native,
			AnnotationText:    frame.Annotation,
			Class:             frame.Class,
			Method:            frame.Method,
			SourceLines:       append([]string(nil), frame.SourceLines...),
			SourceLineNumbers: append([]int(nil), frame.SourceLineNumbers...),
			SourceStartLine:   frame.SourceStartLine,
			SourceLanguage:    frame.SourceLanguage,
		})
	}
	return node
}

func convertCollapsed(collapsed api.Collapsed) ClickyNode {
	label := ClickyNode{
		Kind:  "text",
		Plain: collapsed.Label,
		Text:  collapsed.Label,
		Style: convertTextStyle(collapsed.Style, api.Class{}, false),
	}
	if collapsed.Icon != nil {
		label.Children = append([]ClickyNode{convertIcon(*collapsed.Icon)}, label.Children...)
	}

	node := ClickyNode{
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

func convertButton(button api.Button) ClickyNode {
	label := clickyTextNode(button.Label)
	return ClickyNode{
		Kind:    "button",
		Plain:   button.String(),
		Label:   &label,
		Href:    button.Href,
		ID:      button.ID,
		Payload: button.Payload,
		Variant: button.Variant,
	}
}

func convertButtonGroup(group api.ButtonGroup) ClickyNode {
	node := ClickyNode{Kind: "button-group", Plain: group.String()}
	for _, button := range group.Buttons {
		node.Items = append(node.Items, convertButton(button))
	}
	return node
}

func convertIcon(icon apiicons.Icon) ClickyNode {
	return ClickyNode{
		Kind:    "icon",
		Plain:   icon.String(),
		Style:   convertTextStyle(icon.Style, api.Class{}, false),
		Unicode: icon.Unicode,
		Iconify: icon.Iconify,
	}
}

func convertKeyValuePair(pair api.KeyValuePair) ClickyNode {
	if pair.IsEmpty() {
		return ClickyNode{Kind: "map"}
	}
	return ClickyNode{
		Kind:  "map",
		Plain: pair.String(),
		Fields: []ClickyField{
			{
				Name:  pair.Key,
				Label: pair.Key,
				Value: convertAnyToNode(pair.Value),
			},
		},
	}
}

func convertLabelBadge(b api.LabelBadge) ClickyNode {
	return ClickyNode{
		Kind:       "badge",
		Plain:      b.String(),
		BadgeLabel: b.Label,
		BadgeValue: b.Value,
		BadgeColor: b.Color,
		BadgeText:  b.TextColor,
		BadgeShape: b.Shape,
		BadgeIcon:  b.Icon,
	}
}

func convertDescriptionList(list api.DescriptionList) ClickyNode {
	node := ClickyNode{Kind: "map", Plain: list.String()}
	for _, item := range list.Items {
		if item.IsEmpty() {
			continue
		}
		node.Fields = append(node.Fields, ClickyField{
			Name:  item.Key,
			Label: item.Key,
			Value: convertAnyToNode(item.Value),
		})
	}
	return node
}

func convertAnyToNode(value any) ClickyNode {
	if typed := api.TryTypedValue(value); typed != nil {
		return convertTypedValue(typed, nil)
	}
	return clickyTextNode(fmt.Sprintf("%v", value))
}

func clickyTextNode(text string) ClickyNode {
	return ClickyNode{Kind: "text", Plain: text, Text: text}
}

func convertTextStyle(styleStr string, class api.Class, monospace bool) *ClickyStyle {
	style := ClickyStyle{
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

	if style == (ClickyStyle{}) {
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
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@flanksource/clicky-ui@^0.2.1/src/styles/tokens.css" />
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
          "@flanksource/clicky-ui": "https://esm.sh/@flanksource/clicky-ui@^0.2.1?alias=react:preact/compat,react-dom:preact/compat&deps=preact@10.24.3&external=preact"
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
      import { Clicky, StackTrace } from "@flanksource/clicky-ui";

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

      function stackTraceInput(node) {
        return {
          exceptionClass: node.exceptionClass,
          message: node.message,
          causedBy: node.causedBy ?? [],
          frames: node.frames ?? [],
          language: node.language || "java",
        };
      }
      function isStackTraceMap(node) {
        if (!node || node.kind !== "map" || !Array.isArray(node.fields) || node.fields.length === 0) return false;
        return node.fields.every(f => f && f.value && f.value.kind === "stacktrace");
      }
      function renderStackTraceSection(field, idx) {
        const label = field?.label ?? field?.name ?? "";
        return h("section", { key: idx, className: "stack-trace-section space-y-1" },
          label ? h("h3", { className: "text-sm font-semibold text-slate-700" }, label) : null,
          h(StackTrace, { input: stackTraceInput(field.value), className: "space-y-1" })
        );
      }
      function buildRoot(data) {
        const node = data?.node;
        if (node?.kind === "stacktrace") {
          return h(StackTrace, { input: stackTraceInput(node), className: "space-y-1" });
        }
        if (isStackTraceMap(node)) {
          return h("div", { className: "space-y-3" }, node.fields.map(renderStackTraceSection));
        }
        return h(Clicky, { data });
      }

      const output = document.getElementById("root");
      const data = JSON.parse(document.getElementById("clicky-data").textContent);
      render(buildRoot(data), output);
      highlightCodeBlocks(output);
      new MutationObserver(() => highlightCodeBlocks(output)).observe(output, { childList: true, subtree: true });
    </script>
  </body>
</html>`)
	return b.String()
}
