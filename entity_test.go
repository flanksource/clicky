package clicky

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

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
			output, err := Format(tt.input, FormatOptions{
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
