package entity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// formatForTest renders o with the given options using a fresh format manager.
// The entity package cannot import the root clicky package (which owns the
// clicky.Format helper), so tests render through formatters directly.
func formatForTest(o any, opts formatters.FormatOptions) (string, error) {
	return formatters.NewFormatManager().FormatWithOptions(opts, o)
}

type sampleTableEntity struct {
	ID     string
	Name   string
	Status string
}

func (s sampleTableEntity) GetID() string   { return s.ID }
func (s sampleTableEntity) GetName() string { return s.Name }
func (s sampleTableEntity) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "Name"},
		{Name: "Status"},
	}
}

func (s sampleTableEntity) Row() map[string]any {
	return map[string]any{
		"Name":   s.Name,
		"Status": s.Status,
	}
}

type samplePrettyRowEntity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s samplePrettyRowEntity) GetID() string   { return s.ID }
func (s samplePrettyRowEntity) GetName() string { return s.Name }
func (s samplePrettyRowEntity) PrettyRow(_ interface{}) map[string]api.Text {
	return map[string]api.Text{
		"Name":   {Content: s.Name, Style: "order-1"},
		"Status": {Content: s.Status, Style: "order-2"},
	}
}

type sampleDualEntity struct {
	sampleTableEntity
}

func (s sampleDualEntity) PrettyRow(_ interface{}) map[string]api.Text {
	return map[string]api.Text{
		"Name":   {Content: "pretty-row-value", Style: "order-1"},
		"Status": {Content: "pretty-row-status", Style: "order-2"},
	}
}

type samplePlainEntity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s samplePlainEntity) GetID() string   { return s.ID }
func (s samplePlainEntity) GetName() string { return s.Name }

func TestWrappedEntityPrettyAndHTMLFormatting(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		contains []string
		excludes []string
	}{
		{
			name: "table provider",
			input: []entityWithID[sampleTableEntity]{
				{ID: "1", Inner: sampleTableEntity{ID: "1", Name: "Disbursement", Status: "Ready"}},
			},
			contains: []string{"Disbursement", "Ready"},
		},
		{
			name: "pretty row",
			input: []entityWithID[samplePrettyRowEntity]{
				{ID: "2", Inner: samplePrettyRowEntity{ID: "2", Name: "Claim", Status: "Queued"}},
			},
			contains: []string{"Claim", "Queued"},
		},
		{
			name: "table provider preferred over pretty row",
			input: []entityWithID[sampleDualEntity]{
				{ID: "3", Inner: sampleDualEntity{sampleTableEntity: sampleTableEntity{ID: "3", Name: "table-provider-value", Status: "table-provider-status"}}},
			},
			contains: []string{"table-provider-value", "table-provider-status"},
			excludes: []string{"pretty-row-value", "pretty-row-status"},
		},
		{
			name: "plain struct",
			input: []entityWithID[samplePlainEntity]{
				{ID: "4", Inner: samplePlainEntity{ID: "4", Name: "Inquiry", Status: "Active"}},
			},
			contains: []string{"Inquiry", "Active"},
		},
	}

	formats := []string{"pretty", "html"}

	for _, tt := range testCases {
		for _, format := range formats {
			output, err := formatForTest(tt.input, formatters.FormatOptions{
				Format:  format,
				NoColor: true,
			})
			if err != nil {
				t.Fatalf("%s %s format failed: %v", tt.name, format, err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Fatalf("%s %s format missing %q in output:\n%s", tt.name, format, expected, output)
				}
			}

			for _, unexpected := range tt.excludes {
				if strings.Contains(output, unexpected) {
					t.Fatalf("%s %s format unexpectedly contained %q in output:\n%s", tt.name, format, unexpected, output)
				}
			}
		}
	}
}

func TestWrappedEntityClickyRowsIncludeHiddenID(t *testing.T) {
	testCases := []struct {
		name   string
		input  any
		wantID string
	}{
		{
			name: "table provider",
			input: []entityWithID[sampleTableEntity]{
				{ID: "table-id", Inner: sampleTableEntity{ID: "table-id", Name: "Disbursement", Status: "Ready"}},
			},
			wantID: "table-id",
		},
		{
			name: "pretty row",
			input: []entityWithID[samplePrettyRowEntity]{
				{ID: "pretty-id", Inner: samplePrettyRowEntity{ID: "pretty-id", Name: "Claim", Status: "Queued"}},
			},
			wantID: "pretty-id",
		},
		{
			name: "plain struct",
			input: []entityWithID[samplePlainEntity]{
				{ID: "plain-id", Inner: samplePlainEntity{ID: "plain-id", Name: "Inquiry", Status: "Active"}},
			},
			wantID: "plain-id",
		},
	}

	for _, tt := range testCases {
		output, err := formatForTest(tt.input, formatters.FormatOptions{Format: "html-react"})
		if err != nil {
			t.Fatalf("%s html-react format failed: %v", tt.name, err)
		}

		var doc struct {
			Node struct {
				Columns []struct {
					Name string `json:"name"`
				} `json:"columns"`
				Rows []struct {
					Cells map[string]struct {
						Plain string `json:"plain"`
						Text  string `json:"text"`
					} `json:"cells"`
				} `json:"rows"`
			} `json:"node"`
		}
		payload := output
		const marker = `<script id="clicky-data" type="application/json">`
		if start := strings.Index(output, marker); start >= 0 {
			payload = output[start+len(marker):]
			if end := strings.Index(payload, "</script>"); end >= 0 {
				payload = payload[:end]
			}
		}
		if err := json.Unmarshal([]byte(payload), &doc); err != nil {
			t.Fatalf("%s html-react output is invalid JSON: %v\n%s", tt.name, err, output)
		}
		if len(doc.Node.Rows) != 1 {
			t.Fatalf("%s expected one row, got %d", tt.name, len(doc.Node.Rows))
		}
		// Plain is omitted when it would only repeat Text, so the id is read the
		// way every consumer reads it — plain first, then text.
		cell := doc.Node.Rows[0].Cells["_id"]
		got := cell.Plain
		if got == "" {
			got = cell.Text
		}
		if got != tt.wantID {
			t.Fatalf("%s _id text = %q, want %q", tt.name, got, tt.wantID)
		}
		for _, column := range doc.Node.Columns {
			if column.Name == "_id" {
				t.Fatalf("%s _id should be available as row metadata, not a visible column", tt.name)
			}
		}
	}
}
