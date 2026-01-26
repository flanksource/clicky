package formatters

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
)

const (
	teamsSchema  = "http://adaptivecards.io/schemas/adaptive-card.json"
	teamsType    = "AdaptiveCard"
	teamsVersion = "1.4"
	teamsTextMax = 5000
)

// TeamsFormatter formats data into Microsoft Teams Adaptive Card JSON.
type TeamsFormatter struct{}

type teamsCard struct {
	Schema  string         `json:"$schema,omitempty"`
	Type    string         `json:"type"`
	Version string         `json:"version"`
	Body    []teamsElement `json:"body,omitempty"`
	Actions []teamsAction  `json:"actions,omitempty"`
}

type teamsElement struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Weight    string         `json:"weight,omitempty"`
	Size      string         `json:"size,omitempty"`
	Wrap      bool           `json:"wrap,omitempty"`
	Separator bool           `json:"separator,omitempty"`
	Columns   []teamsColumn  `json:"columns,omitempty"`
	Facts     []teamsFact    `json:"facts,omitempty"`
	Items     []teamsElement `json:"items,omitempty"`
}

type teamsColumn struct {
	Type  string         `json:"type"`
	Width string         `json:"width,omitempty"`
	Items []teamsElement `json:"items,omitempty"`
}

type teamsFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type teamsAction struct {
	Type  string                 `json:"type"`
	Title string                 `json:"title,omitempty"`
	URL   string                 `json:"url,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

func NewTeamsFormatter() *TeamsFormatter {
	return &TeamsFormatter{}
}

func (f *TeamsFormatter) Format(in interface{}, options FormatOptions) (string, error) {
	if slice, ok := in.([]interface{}); ok && len(slice) == 1 {
		in = slice[0]
	}

	switch v := in.(type) {
	case api.TextList:
		return f.encodeCard(f.elementsForTextList(v))
	case *api.TextList:
		return f.encodeCard(f.elementsForTextList(*v))
	case api.List:
		return f.encodeCard(f.elementsForText(v.Markdown()))
	case *api.List:
		return f.encodeCard(f.elementsForText(v.Markdown()))
	case api.ButtonGroup:
		return f.encodeCard(nil, f.actionsForButtons(v))
	case *api.ButtonGroup:
		return f.encodeCard(nil, f.actionsForButtons(*v))
	case api.Button:
		return f.encodeCard(nil, f.actionsForButtons(api.ButtonGroup{Buttons: []api.Button{v}}))
	case *api.Button:
		return f.encodeCard(nil, f.actionsForButtons(api.ButtonGroup{Buttons: []api.Button{*v}}))
	case api.Text:
		return f.encodeCard(f.elementsForTextItem(v))
	case *api.Text:
		return f.encodeCard(f.elementsForTextItem(*v))
	case api.HtmlElement:
		if isDividerElement(v) {
			return f.encodeCard([]teamsElement{dividerElement()})
		}
		return f.encodeCard(f.elementsForText(v.Markdown()))
	case *api.HtmlElement:
		if isDividerElement(*v) {
			return f.encodeCard([]teamsElement{dividerElement()})
		}
		return f.encodeCard(f.elementsForText(v.Markdown()))
	case *api.PrettyData:
		return f.FormatPrettyData(v, options)
	case api.PrettyData:
		return f.FormatPrettyData(&v, options)
	case api.Pretty:
		return f.formatText(v.Pretty().Markdown())
	case api.Textable:
		return f.formatText(v.Markdown())
	}

	prettyData, err := ToPrettyDataWithOptions(in, options)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}
	return f.FormatPrettyData(prettyData, options)
}

func (f *TeamsFormatter) FormatPrettyData(data *api.PrettyData, options FormatOptions) (string, error) {
	if data == nil || data.Schema == nil {
		return f.encodeCard(nil)
	}

	if elements, actions, ok := f.elementsForSchemaData(data); ok {
		return f.encodeCard(elements, actions)
	}

	value := data.Value()
	switch v := value.(type) {
	case *api.TextTable:
		return f.encodeCard(f.elementsForTable(v))
	case api.TextTable:
		return f.encodeCard(f.elementsForTable(&v))
	case *api.TextTree:
		return f.encodeCard(f.elementsForTree(v))
	case api.TextTree:
		return f.encodeCard(f.elementsForTree(&v))
	case api.Textable:
		return f.formatText(v.Markdown())
	default:
		return f.formatText(fmt.Sprintf("%v", value))
	}
}

func (f *TeamsFormatter) formatText(text string) (string, error) {
	return f.encodeCard(f.elementsForText(text))
}

func (f *TeamsFormatter) elementsForText(text string) []teamsElement {
	clean := f.sanitizeTeamsText(text)
	if clean == "" {
		clean = " "
	}
	parts := splitSlackText(clean, teamsTextMax)
	elements := make([]teamsElement, 0, len(parts))
	for _, part := range parts {
		elements = append(elements, textElement(part))
	}
	return elements
}

func (f *TeamsFormatter) elementsForTextItem(text api.Text) []teamsElement {
	if isHeaderText(text) {
		header := f.sanitizeTeamsText(text.String())
		if header == "" {
			return nil
		}
		return []teamsElement{headerElement(header, true)}
	}
	return f.elementsForText(text.Markdown())
}

func (f *TeamsFormatter) elementsForTree(tree *api.TextTree) []teamsElement {
	if tree == nil {
		return nil
	}
	text := strings.TrimSpace(tree.Markdown())
	if text == "" {
		return nil
	}
	return f.elementsForText("```\n" + text + "\n```")
}

func (f *TeamsFormatter) elementsForTable(table *api.TextTable) []teamsElement {
	if table == nil || len(table.Headers) == 0 {
		return nil
	}

	var elements []teamsElement
	for rowIdx, row := range table.Rows {
		var section strings.Builder
		for i, header := range table.Headers {
			label := f.sanitizeTeamsText(header.Markdown())
			if label == "" {
				label = header.String()
			}
			fieldName := label
			if i < len(table.FieldNames) && table.FieldNames[i] != "" {
				fieldName = table.FieldNames[i]
			}
			value := f.sanitizeOrPlaceholder(markdownForCell(row, fieldName))
			if section.Len() > 0 {
				section.WriteString("\n")
			}
			section.WriteString(fmt.Sprintf("%s: %s", label, value))
		}
		if section.Len() > 0 {
			elements = append(elements, textElement(section.String()))
		}
		if rowIdx < len(table.Rows)-1 {
			elements = append(elements, dividerElement())
		}
	}
	return elements
}

func (f *TeamsFormatter) elementsForTextList(list api.TextList) []teamsElement {
	if len(list) == 0 {
		return nil
	}

	var elements []teamsElement
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
		elements = append(elements, f.elementsForText(section.String())...)
		section.Reset()
	}

	for _, item := range list {
		switch v := item.(type) {
		case api.HtmlElement:
			if isDividerElement(v) {
				flush()
				elements = append(elements, dividerElement())
				continue
			}
		case api.TextTable:
			flush()
			elements = append(elements, f.elementsForTable(&v)...)
			continue
		case *api.TextTable:
			flush()
			elements = append(elements, f.elementsForTable(v)...)
			continue
		case api.ButtonGroup:
			flush()
			actions := f.actionsForButtons(v)
			if len(actions) > 0 {
				for _, action := range actions {
					element := actionAsText(action)
					if element.Type != "" {
						elements = append(elements, element)
					}
				}
			}
			continue
		case api.Button:
			flush()
			actions := f.actionsForButtons(api.ButtonGroup{Buttons: []api.Button{v}})
			if len(actions) > 0 {
				for _, action := range actions {
					element := actionAsText(action)
					if element.Type != "" {
						elements = append(elements, element)
					}
				}
			}
			continue
		case api.List:
			flush()
			elements = append(elements, f.elementsForText(v.Markdown())...)
			continue
		case api.Text:
			if isHeaderText(v) {
				flush()
				elements = append(elements, f.elementsForTextItem(v)...)
				continue
			}
		}

		appendText(item.Markdown())
	}

	flush()
	return elements
}

func (f *TeamsFormatter) actionsForButtons(actions api.ButtonGroup) []teamsAction {
	if len(actions.Buttons) == 0 {
		return nil
	}

	buttons := make([]teamsAction, 0, len(actions.Buttons))
	for _, button := range actions.Buttons {
		label := f.sanitizeTeamsText(button.Label)
		if label == "" {
			continue
		}
		if button.Href != "" {
			buttons = append(buttons, teamsAction{Type: "Action.OpenUrl", Title: label, URL: button.Href})
			continue
		}
		data := map[string]interface{}{}
		if button.ID != "" {
			data["id"] = button.ID
		}
		if button.Payload != "" {
			data["payload"] = button.Payload
		}
		buttons = append(buttons, teamsAction{Type: "Action.Submit", Title: label, Data: data})
	}

	if len(buttons) == 0 {
		return nil
	}
	return buttons
}

func (f *TeamsFormatter) elementsForSchemaData(data *api.PrettyData) ([]teamsElement, []teamsAction, bool) {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil, nil, false
	}

	if hasTitleField(data.Schema.Fields, f.isTitleField) {
		elements, actions := f.elementsForSchemaWithHeaders(data)
		return elements, actions, true
	}

	var elements []teamsElement
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}

		typedValue, ok := data.GetValue(field.Name)
		if !ok {
			continue
		}

		if actions, ok := typedValue.Textable.(api.ButtonGroup); ok {
			return elements, f.actionsForButtons(actions), true
		}

		value := f.sanitizeOrPlaceholder(typedValue.Markdown())
		label := fieldLabel(field)
		if label == "" {
			elements = append(elements, textElement(value))
			continue
		}
		elements = append(elements, textElement(fmt.Sprintf("%s: %s", label, value)))
	}

	if len(elements) == 0 {
		return nil, nil, false
	}

	return elements, nil, true
}

func (f *TeamsFormatter) elementsForSchemaWithHeaders(data *api.PrettyData) ([]teamsElement, []teamsAction) {
	if data == nil || data.Schema == nil || data.TypedMap == nil {
		return nil, nil
	}

	var elements []teamsElement
	var actions []teamsAction

	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatHide {
			continue
		}

		typedValue, ok := data.GetValue(field.Name)
		if !ok {
			continue
		}

		if textable, ok := typedValue.Textable.(api.HtmlElement); ok && isDividerElement(textable) {
			elements = append(elements, dividerElement())
			continue
		}

		if buttonGroup, ok := typedValue.Textable.(api.ButtonGroup); ok {
			actions = append(actions, f.actionsForButtons(buttonGroup)...)
			continue
		}

		value := f.sanitizeOrPlaceholder(typedValue.Markdown())
		label := fieldLabel(field)

		if f.isTitleField(field) {
			headerText, includeValue := f.headerTextForField(field, value, label)
			if headerText != "" {
				elements = append(elements, headerElement(headerText, f.isPrimaryTitle(headerText, value, includeValue)))
			}
			if includeValue {
				elements = append(elements, f.elementsForText(value)...)
			}
			continue
		}

		if label == "" {
			elements = append(elements, textElement(value))
			continue
		}
		elements = append(elements, textElement(fmt.Sprintf("%s: %s", label, value)))
	}

	return elements, actions
}

func (f *TeamsFormatter) encodeCard(body []teamsElement, actions ...[]teamsAction) (string, error) {
	var actionsList []teamsAction
	if len(actions) > 0 {
		actionsList = actions[0]
	}
	if body == nil {
		body = []teamsElement{}
	}

	card := teamsCard{
		Schema:  teamsSchema,
		Type:    teamsType,
		Version: teamsVersion,
		Body:    body,
		Actions: actionsList,
	}
	out, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal teams card: %w", err)
	}
	return string(out), nil
}

func (f *TeamsFormatter) sanitizeTeamsText(text string) string {
	if text == "" {
		return ""
	}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	return slackHTMLTagRE.ReplaceAllString(clean, "")
}

func (f *TeamsFormatter) sanitizeOrPlaceholder(text string) string {
	clean := f.sanitizeTeamsText(text)
	if clean == "" {
		return " "
	}
	return clean
}

func (f *TeamsFormatter) isTitleField(field api.PrettyField) bool {
	if field.TableOptions.Title != "" {
		return true
	}
	if field.FormatOptions == nil {
		return false
	}
	_, ok := field.FormatOptions["title"]
	return ok
}

func (f *TeamsFormatter) headerTextForField(field api.PrettyField, value, label string) (string, bool) {
	labelSet := field.FormatOptions != nil && field.FormatOptions["label_set"] == "true"
	titleSet := field.FormatOptions != nil && field.FormatOptions["title"] == "true"

	headerText := label
	if field.TableOptions.Title != "" {
		headerText = field.TableOptions.Title
	} else if titleSet && !labelSet {
		headerText = value
	}
	headerText = f.sanitizeTeamsText(headerText)

	includeValue := titleSet && strings.TrimSpace(value) != "" && (labelSet || field.TableOptions.Title != "")
	return headerText, includeValue
}

func (f *TeamsFormatter) isPrimaryTitle(headerText, value string, includeValue bool) bool {
	if includeValue {
		return false
	}
	return headerText == value
}

func dividerElement() teamsElement {
	return teamsElement{Type: "TextBlock", Text: " ", Separator: true}
}

func textElement(text string) teamsElement {
	return teamsElement{Type: "TextBlock", Text: text, Wrap: true}
}

func headerElement(text string, primary bool) teamsElement {
	element := teamsElement{Type: "TextBlock", Text: text, Wrap: true, Weight: "Bolder"}
	if primary {
		element.Size = "Large"
	}
	return element
}

func actionAsText(action teamsAction) teamsElement {
	label := action.Title
	if label == "" {
		return teamsElement{}
	}
	return textElement(label)
}

func isDividerElement(el api.HtmlElement) bool {
	return strings.EqualFold(el.Tag, "hr")
}

func isHeaderText(text api.Text) bool {
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
