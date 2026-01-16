package formatters

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/flanksource/clicky/api"
)

const (
	slackMaxTextLen  = 3000
	slackMaxFields   = 10
	slackDividerType = "divider"
	slackSectionType = "section"
	slackHeaderType  = "header"
	slackActionsType = "actions"
	slackButtonType  = "button"
	slackTextMrkdwn  = "mrkdwn"
	slackTextPlain   = "plain_text"
)

var slackHTMLTagRE = regexp.MustCompile("<[^>]+>")

type SlackFormatter struct {
	MaxTextLen int
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackBlock struct {
	Type     string        `json:"type"`
	Text     *slackText    `json:"text,omitempty"`
	Fields   []slackText   `json:"fields,omitempty"`
	Elements []slackButton `json:"elements,omitempty"`
}

type slackButton struct {
	Type     string    `json:"type"`
	Text     slackText `json:"text"`
	URL      string    `json:"url,omitempty"`
	ActionID string    `json:"action_id,omitempty"`
	Value    string    `json:"value,omitempty"`
	Style    string    `json:"style,omitempty"`
}

type slackMessage struct {
	Blocks []slackBlock `json:"blocks"`
}

func NewSlackFormatter() *SlackFormatter {
	return &SlackFormatter{MaxTextLen: slackMaxTextLen}
}

// Format formats data into Slack Block Kit JSON.
func (f *SlackFormatter) Format(in interface{}, options FormatOptions) (string, error) {
	if slice, ok := in.([]interface{}); ok && len(slice) == 1 {
		in = slice[0]
	}

	switch v := in.(type) {
	case api.TextList:
		return f.encodeBlocks(f.blocksForTextList(v))
	case *api.TextList:
		return f.encodeBlocks(f.blocksForTextList(*v))
	case api.List:
		return f.encodeBlocks(f.blocksForText(v.Markdown()))
	case *api.List:
		return f.encodeBlocks(f.blocksForText(v.Markdown()))
	case api.ButtonGroup:
		return f.encodeBlocks(f.blocksForActions(v))
	case *api.ButtonGroup:
		return f.encodeBlocks(f.blocksForActions(*v))
	case api.Button:
		return f.encodeBlocks(f.blocksForActions(api.ButtonGroup{Buttons: []api.Button{v}}))
	case *api.Button:
		return f.encodeBlocks(f.blocksForActions(api.ButtonGroup{Buttons: []api.Button{*v}}))
	case api.Text:
		return f.encodeBlocks(f.blocksForTextItem(v))
	case *api.Text:
		return f.encodeBlocks(f.blocksForTextItem(*v))
	case api.HtmlElement:
		if f.isDividerElement(v) {
			return f.encodeBlocks([]slackBlock{{Type: slackDividerType}})
		}
		return f.encodeBlocks(f.blocksForText(v.Markdown()))
	case *api.HtmlElement:
		if f.isDividerElement(*v) {
			return f.encodeBlocks([]slackBlock{{Type: slackDividerType}})
		}
		return f.encodeBlocks(f.blocksForText(v.Markdown()))
	case *api.PrettyData:
		return f.FormatPrettyData(v, options)
	case api.PrettyData:
		return f.FormatPrettyData(&v, options)
	case api.Pretty:
		return f.formatText(v.Pretty().Markdown(), options)
	case api.Textable:
		return f.formatText(v.Markdown(), options)
	}

	prettyData, err := ToPrettyDataWithOptions(in, options)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}
	return f.FormatPrettyData(prettyData, options)
}

// FormatPrettyData formats PrettyData into Slack Block Kit JSON.
func (f *SlackFormatter) FormatPrettyData(data *api.PrettyData, options FormatOptions) (string, error) {
	if data == nil || data.Schema == nil {
		return f.encodeBlocks(nil)
	}

	if blocks, ok := f.blocksForSchemaData(data); ok {
		return f.encodeBlocks(blocks)
	}

	value := data.Value()
	switch v := value.(type) {
	case *api.TextTable:
		return f.encodeBlocks(f.blocksForTable(v))
	case api.TextTable:
		return f.encodeBlocks(f.blocksForTable(&v))
	case *api.TextTree:
		return f.encodeBlocks(f.blocksForTree(v))
	case api.TextTree:
		return f.encodeBlocks(f.blocksForTree(&v))
	case api.Textable:
		return f.formatText(v.Markdown(), options)
	default:
		return f.formatText(fmt.Sprintf("%v", value), options)
	}
}

func (f *SlackFormatter) formatText(text string, options FormatOptions) (string, error) {
	blocks := f.blocksForText(text)
	return f.encodeBlocks(blocks)
}

func (f *SlackFormatter) blocksForText(text string) []slackBlock {
	clean := f.sanitizeSlackText(text)
	if clean == "" {
		clean = " "
	}

	parts := splitSlackText(clean, f.maxTextLen())
	blocks := make([]slackBlock, 0, len(parts))
	for _, part := range parts {
		blocks = append(blocks, slackBlock{
			Type: slackSectionType,
			Text: &slackText{Type: slackTextMrkdwn, Text: part},
		})
	}
	return blocks
}

func (f *SlackFormatter) blocksForTextItem(text api.Text) []slackBlock {
	if f.isHeaderText(text) {
		header := f.truncateSlackHeader(f.sanitizeSlackText(text.String()))
		if header == "" {
			return nil
		}
		return []slackBlock{
			{
				Type: slackHeaderType,
				Text: &slackText{Type: slackTextPlain, Text: header},
			},
		}
	}
	return f.blocksForText(text.Markdown())
}

func (f *SlackFormatter) blocksForTree(tree *api.TextTree) []slackBlock {
	if tree == nil {
		return nil
	}
	text := strings.TrimSpace(tree.Markdown())
	if text == "" {
		return nil
	}
	return f.blocksForText("```\n" + text + "\n```")
}

func (f *SlackFormatter) blocksForTable(table *api.TextTable) []slackBlock {
	if table == nil || len(table.Headers) == 0 {
		return nil
	}

	headers := make([]string, len(table.Headers))
	for i, header := range table.Headers {
		label := f.sanitizeSlackText(header.Markdown())
		if label == "" {
			label = header.String()
		}
		headers[i] = label
	}

	var blocks []slackBlock
	for rowIdx, row := range table.Rows {
		fields := make([]slackText, 0, len(headers))
		for i, header := range headers {
			fieldName := header
			if i < len(table.FieldNames) && table.FieldNames[i] != "" {
				fieldName = table.FieldNames[i]
			}

			value := f.sanitizeOrPlaceholder(markdownForCell(row, fieldName))

			label := header
			if label == "" {
				label = fieldName
			}

			fields = append(fields, slackText{
				Type: slackTextMrkdwn,
				Text: fmt.Sprintf("*%s*\n%s", label, value),
			})
		}

		blocks = appendFieldSections(blocks, fields)

		if rowIdx < len(table.Rows)-1 {
			blocks = append(blocks, slackBlock{Type: slackDividerType})
		}
	}

	return blocks
}

func (f *SlackFormatter) blocksForTextList(list api.TextList) []slackBlock {
	if len(list) == 0 {
		return nil
	}

	var blocks []slackBlock
	var section strings.Builder

	appendText := func(text string) {
		clean := strings.TrimSpace(text)
		if clean == "" {
			return
		}
		if section.Len() > 0 {
			section.WriteString("\n")
		}
		section.WriteString(clean)
	}

	flush := func() {
		if section.Len() == 0 {
			return
		}
		blocks = append(blocks, f.blocksForText(section.String())...)
		section.Reset()
	}

	for _, item := range list {
		switch v := item.(type) {
		case api.HtmlElement:
			if f.isDividerElement(v) {
				flush()
				blocks = append(blocks, slackBlock{Type: slackDividerType})
				continue
			}
		case api.ButtonGroup:
			flush()
			blocks = append(blocks, f.blocksForActions(v)...)
			continue
		case api.Button:
			flush()
			blocks = append(blocks, f.blocksForActions(api.ButtonGroup{Buttons: []api.Button{v}})...)
			continue
		case api.List:
			flush()
			blocks = append(blocks, f.blocksForText(v.Markdown())...)
			continue
		case api.Text:
			if f.isHeaderText(v) {
				flush()
				blocks = append(blocks, f.blocksForTextItem(v)...)
				continue
			}
		}

		appendText(item.Markdown())
	}

	flush()
	return blocks
}

func (f *SlackFormatter) blocksForActions(actions api.ButtonGroup) []slackBlock {
	if len(actions.Buttons) == 0 {
		return nil
	}

	elements := make([]slackButton, 0, len(actions.Buttons))
	for i, button := range actions.Buttons {
		label := f.truncateSlackButton(f.sanitizeSlackText(button.Label))
		if label == "" {
			continue
		}
		actionID := button.ID
		if actionID == "" {
			actionID = fmt.Sprintf("action_%d", i+1)
		}
		style := strings.ToLower(button.Variant)
		if style != "primary" && style != "danger" {
			style = ""
		}
		elements = append(elements, slackButton{
			Type:     slackButtonType,
			Text:     slackText{Type: slackTextPlain, Text: label},
			URL:      button.Href,
			ActionID: actionID,
			Value:    button.Payload,
			Style:    style,
		})
	}

	if len(elements) == 0 {
		return nil
	}

	return []slackBlock{{
		Type:     slackActionsType,
		Elements: elements,
	}}
}

func (f *SlackFormatter) blocksForSchemaData(data *api.PrettyData) ([]slackBlock, bool) {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil, false
	}

	var blocks []slackBlock
	fields := make([]slackText, 0, len(data.Schema.Fields))

	if hasTitleField(data.Schema.Fields, f.isTitleField) {
		return f.blocksForSchemaWithHeaders(data), true
	}

	flushFields := func() {
		if len(fields) == 0 {
			return
		}
		blocks = appendFieldSections(blocks, fields)
		fields = nil
	}

	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}

		typedValue, ok := data.GetValue(field.Name)
		if !ok {
			continue
		}

		if actions, ok := typedValue.Textable.(api.ButtonGroup); ok {
			flushFields()
			blocks = append(blocks, f.blocksForActions(actions)...)
			continue
		}

		value := f.sanitizeOrPlaceholder(typedValue.Markdown())

		label := fieldLabel(field)

		fields = append(fields, slackText{
			Type: slackTextMrkdwn,
			Text: fmt.Sprintf("*%s*\n%s", label, value),
		})
	}

	if len(fields) == 0 {
		return nil, false
	}

	blocks = appendFieldSections(blocks, fields)

	return blocks, true
}

func (f *SlackFormatter) blocksForSchemaWithHeaders(data *api.PrettyData) []slackBlock {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil
	}

	var blocks []slackBlock

	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}

		typedValue, ok := data.GetValue(field.Name)
		if !ok {
			continue
		}

		if textable, ok := typedValue.Textable.(api.HtmlElement); ok && f.isDividerElement(textable) {
			blocks = append(blocks, slackBlock{Type: slackDividerType})
			continue
		}

		if actions, ok := typedValue.Textable.(api.ButtonGroup); ok {
			blocks = append(blocks, f.blocksForActions(actions)...)
			continue
		}

		value := f.sanitizeOrPlaceholder(typedValue.Markdown())
		label := fieldLabel(field)

		if f.isTitleField(field) {
			headerText, includeValue := f.headerTextForField(field, value, label)
			if headerText != "" {
				blocks = append(blocks, slackBlock{
					Type: slackHeaderType,
					Text: &slackText{Type: slackTextPlain, Text: headerText},
				})
			}
			if includeValue {
				blocks = append(blocks, slackBlock{
					Type: slackSectionType,
					Text: &slackText{Type: slackTextMrkdwn, Text: value},
				})
			}
			continue
		}

		blocks = append(blocks, slackBlock{
			Type: slackSectionType,
			Text: &slackText{Type: slackTextMrkdwn, Text: fmt.Sprintf("%s: %s", label, value)},
		})
	}

	return blocks
}

func (f *SlackFormatter) encodeBlocks(blocks []slackBlock) (string, error) {
	if blocks == nil {
		blocks = []slackBlock{}
	}
	payload := slackMessage{Blocks: blocks}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal slack blocks: %w", err)
	}
	return string(out), nil
}

func (f *SlackFormatter) sanitizeSlackText(text string) string {
	if text == "" {
		return ""
	}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	return slackHTMLTagRE.ReplaceAllString(clean, "")
}

func (f *SlackFormatter) sanitizeOrPlaceholder(text string) string {
	clean := f.sanitizeSlackText(text)
	if clean == "" {
		return " "
	}
	return clean
}

func (f *SlackFormatter) headerTextForField(field api.PrettyField, value, label string) (string, bool) {
	labelSet := field.FormatOptions != nil && field.FormatOptions["label_set"] == "true"
	titleSet := field.FormatOptions != nil && field.FormatOptions["title"] == "true"

	headerText := label
	if field.TableOptions.Title != "" {
		headerText = field.TableOptions.Title
	} else if titleSet && !labelSet {
		headerText = value
	}
	headerText = f.truncateSlackHeader(f.sanitizeSlackText(headerText))

	includeValue := titleSet && strings.TrimSpace(value) != "" && (labelSet || field.TableOptions.Title != "")
	return headerText, includeValue
}

func (f *SlackFormatter) isDividerElement(el api.HtmlElement) bool {
	return strings.EqualFold(el.Tag, "hr")
}

func (f *SlackFormatter) isTitleField(field api.PrettyField) bool {
	if field.TableOptions.Title != "" {
		return true
	}
	if field.FormatOptions == nil {
		return false
	}
	_, ok := field.FormatOptions["title"]
	return ok
}

func (f *SlackFormatter) isHeaderText(text api.Text) bool {
	if strings.Contains(text.Style, "header") {
		return true
	}
	content := strings.TrimSpace(text.String())
	if content == "" {
		return false
	}
	if strings.HasSuffix(content, ":") && (strings.Contains(text.Style, "font-bold") || strings.Contains(text.Style, "bold")) {
		return true
	}
	return false
}

func (f *SlackFormatter) truncateSlackHeader(text string) string {
	const slackHeaderMax = 150
	if len(text) <= slackHeaderMax {
		return text
	}
	return text[:slackHeaderMax]
}

func (f *SlackFormatter) truncateSlackButton(text string) string {
	const slackButtonMax = 75
	if len(text) <= slackButtonMax {
		return text
	}
	return text[:slackButtonMax]
}

func (f *SlackFormatter) maxTextLen() int {
	if f.MaxTextLen <= 0 {
		return slackMaxTextLen
	}
	return f.MaxTextLen
}

func fieldLabel(field api.PrettyField) string {
	if field.Label != "" {
		return field.Label
	}
	return api.PrettifyFieldName(field.Name)
}

func hasTitleField(fields []api.PrettyField, matcher func(api.PrettyField) bool) bool {
	for _, field := range fields {
		if field.Format == api.FormatHide {
			continue
		}
		if matcher(field) {
			return true
		}
	}
	return false
}

func appendFieldSections(blocks []slackBlock, fields []slackText) []slackBlock {
	for start := 0; start < len(fields); start += slackMaxFields {
		end := start + slackMaxFields
		if end > len(fields) {
			end = len(fields)
		}
		blocks = append(blocks, slackBlock{
			Type:   slackSectionType,
			Fields: fields[start:end],
		})
	}
	return blocks
}

func markdownForCell(row api.TableRow, fieldName string) string {
	if cell, ok := row[fieldName]; ok {
		return cell.Markdown()
	}
	return ""
}

func splitSlackText(text string, max int) []string {
	if max <= 0 || len(text) <= max {
		return []string{text}
	}

	lines := strings.Split(text, "\n")
	var parts []string
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		parts = append(parts, current.String())
		current.Reset()
	}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) > max {
			flush()
			for start := 0; start < len(line); start += max {
				end := start + max
				if end > len(line) {
					end = len(line)
				}
				parts = append(parts, line[start:end])
			}
			continue
		}

		if current.Len() == 0 {
			current.WriteString(line)
			continue
		}

		if current.Len()+1+len(line) > max {
			flush()
			current.WriteString(line)
			continue
		}

		current.WriteString("\n")
		current.WriteString(line)
	}

	flush()
	return parts
}
