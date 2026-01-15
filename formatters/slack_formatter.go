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
	}

	if prettyData, ok := in.(*api.PrettyData); ok {
		return f.FormatPrettyData(prettyData, options)
	}

	if pretty, ok := in.(api.Pretty); ok {
		return f.formatText(pretty.Pretty().Markdown(), options)
	}

	if textable, ok := in.(api.Textable); ok {
		return f.formatText(textable.Markdown(), options)
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
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: part},
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
				Type: "header",
				Text: &slackText{Type: "plain_text", Text: header},
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

			value := ""
			if cell, ok := row[fieldName]; ok {
				value = cell.Markdown()
			}
			value = f.sanitizeSlackText(value)
			if value == "" {
				value = " "
			}

			label := header
			if label == "" {
				label = fieldName
			}

			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s*\n%s", label, value),
			})
		}

		for start := 0; start < len(fields); start += slackMaxFields {
			end := start + slackMaxFields
			if end > len(fields) {
				end = len(fields)
			}
			blocks = append(blocks, slackBlock{
				Type:   "section",
				Fields: fields[start:end],
			})
		}

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
			Type:     "button",
			Text:     slackText{Type: "plain_text", Text: label},
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
		Type:     "actions",
		Elements: elements,
	}}
}

func (f *SlackFormatter) blocksForSchemaData(data *api.PrettyData) ([]slackBlock, bool) {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil, false
	}

	var blocks []slackBlock
	fields := make([]slackText, 0, len(data.Schema.Fields))
	hasHeader := false

	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}
		if f.isTitleField(field) {
			hasHeader = true
			break
		}
	}

	if hasHeader {
		return f.blocksForSchemaWithHeaders(data), true
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
			if len(fields) > 0 {
				for start := 0; start < len(fields); start += slackMaxFields {
					end := start + slackMaxFields
					if end > len(fields) {
						end = len(fields)
					}
					blocks = append(blocks, slackBlock{
						Type:   "section",
						Fields: fields[start:end],
					})
				}
				fields = nil
			}
			blocks = append(blocks, f.blocksForActions(actions)...)
			continue
		}

		value := f.sanitizeSlackText(typedValue.Markdown())
		if value == "" {
			value = " "
		}

		if f.isTitleField(field) {
			hasHeader = true
			headerText := strings.TrimSpace(value)
			if headerText == "" {
				headerText = field.Label
			}
			headerText = f.truncateSlackHeader(f.sanitizeSlackText(headerText))
			if headerText != "" {
				blocks = append(blocks, slackBlock{
					Type: "header",
					Text: &slackText{Type: "plain_text", Text: headerText},
				})
			}
			continue
		}

		label := field.Label
		if label == "" {
			label = api.PrettifyFieldName(field.Name)
		}

		fields = append(fields, slackText{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*%s*\n%s", label, value),
		})
	}

	if len(fields) == 0 {
		return nil, false
	}

	for start := 0; start < len(fields); start += slackMaxFields {
		end := start + slackMaxFields
		if end > len(fields) {
			end = len(fields)
		}
		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields[start:end],
		})
	}

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

		value := f.sanitizeSlackText(typedValue.Markdown())
		if value == "" {
			value = " "
		}

		label := field.Label
		if label == "" {
			label = api.PrettifyFieldName(field.Name)
		}

		if f.isTitleField(field) {
			headerText := label
			labelSet := field.FormatOptions != nil && field.FormatOptions["label_set"] == "true"
			titleSet := field.FormatOptions != nil && field.FormatOptions["title"] == "true"
			if field.TableOptions.Title != "" {
				headerText = field.TableOptions.Title
			} else if titleSet && !labelSet {
				headerText = value
			}
			headerText = f.truncateSlackHeader(f.sanitizeSlackText(headerText))
			if headerText != "" {
				blocks = append(blocks, slackBlock{
					Type: "header",
					Text: &slackText{Type: "plain_text", Text: headerText},
				})
			}
			if titleSet && strings.TrimSpace(value) != "" && (labelSet || field.TableOptions.Title != "") {
				blocks = append(blocks, slackBlock{
					Type: "section",
					Text: &slackText{Type: "mrkdwn", Text: value},
				})
			}
			continue
		}

		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("%s: %s", label, value)},
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
