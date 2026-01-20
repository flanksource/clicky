package formatters

import (
	"os"
	"testing"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters"
)

type notificationField struct {
	Label string
	Value string
}

type notificationMessagePayload struct {
	Title            string
	Description      string
	Fields           []notificationField
	LabelFields      []notificationField
	RecentEvents     []string
	GroupedResources []string
}

func (p notificationMessagePayload) toTextList() api.TextList {
	var out api.TextList

	if p.Title != "" {
		out = append(out, api.Text{Content: p.Title, Style: "header text-xl font-semibold"})
	}

	addDivider := func() {
		if len(out) > 0 {
			out = append(out, api.HR)
		}
	}

	contentItems := 0
	if p.Description != "" {
		contentItems++
	}
	if len(p.Fields) > 0 {
		contentItems++
	}
	if len(p.LabelFields) > 0 || len(p.RecentEvents) > 0 || len(p.GroupedResources) > 0 {
		contentItems++
	}
	if contentItems > 0 {
		addDivider()
	}

	if p.Description != "" {
		out = append(out, api.Text{Content: p.Description})
	}

	if len(p.Fields) > 0 {
		out = append(out, fieldsTable(p.Fields))
	}

	if len(p.LabelFields) > 0 {
		out = append(out, api.Text{Content: "Labels", Style: "font-semibold"})
		out = append(out, fieldsTable(p.LabelFields))
	}

	if len(p.RecentEvents) > 0 {
		out = append(out, labeledInlineList("Recent Events", p.RecentEvents))
	}

	if len(p.GroupedResources) > 0 {
		out = append(out, labeledList("Also Failing", p.GroupedResources))
	}

	return out
}

func fieldsTable(fields []notificationField) api.TextTable {
	headers := make(api.TextList, 0, len(fields))
	fieldNames := make([]string, 0, len(fields))
	row := api.TableRow{}

	for _, f := range fields {
		key := f.Label
		headers = append(headers, api.Text{Content: f.Label})
		fieldNames = append(fieldNames, key)
		row[key] = api.NewTypedValue(f.Value)
	}

	return api.TextTable{
		Headers:    headers,
		FieldNames: fieldNames,
		Rows:       []api.TableRow{row},
	}
}

func labeledList(label string, items []string) api.Text {
	title := api.Text{Content: label + ": ", Style: "font-semibold"}
	return title.Add(api.Text{Content: joinLines(items)})
}

func labeledInlineList(label string, items []string) api.Text {
	title := api.Text{Content: label + ": ", Style: "font-semibold"}
	return title.Add(api.Text{Content: joinInline(items)})
}

func joinLines(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "\n"
		}
		out += item
	}
	return out
}

func joinInline(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}

func TestNotificationPayloadHTML(t *testing.T) {
	payload := notificationMessagePayload{
		Title:       "\U0001F534 adguard-sync is unhealthy",
		Description: "HelmRelease status is failing",
		Fields: []notificationField{
			{Label: "Type", Value: "Kubernetes::HelmRelease"},
			{Label: "Status", Value: "Failed"},
		},
		LabelFields: []notificationField{
			{Label: "cluster", Value: "homelab"},
			{Label: "namespace", Value: "network"},
			{Label: "app", Value: "adguard-sync"},
		},
		RecentEvents: []string{"FailedCreate", "ProgressDeadlineExceeded", "Unhealthy"},
		GroupedResources: []string{
			"default/HelmRelease/adguard-sync",
			"default/HelmRelease/adguard-sync-exporter",
		},
	}

	formatter := NewEmailHTMLFormatter()
	output, err := formatter.Format(payload.toTextList(), FormatOptions{Format: "email"})
	if err != nil {
		t.Fatalf("failed to format html: %v", err)
	}

	if err := os.WriteFile("notification_payload.html", []byte(output), 0o644); err != nil {
		t.Fatalf("failed to write html output: %v", err)
	}
}
