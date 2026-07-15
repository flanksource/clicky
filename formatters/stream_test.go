package formatters

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v3"
)

type sliceRowIterator struct {
	columns []api.ColumnDef
	rows    []map[string]any
	index   int
}

type generatedStructuredRowIterator struct {
	count int
	index int
}

func (i *generatedStructuredRowIterator) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "id", Type: "number"},
		{Name: "labels", Type: api.ColumnTypeKeyValue},
		{Name: "payload", Type: api.ColumnTypeJSON},
	}
}
func (i *generatedStructuredRowIterator) Next() bool {
	if i.index >= i.count {
		return false
	}
	i.index++
	return true
}
func (i *generatedStructuredRowIterator) Row() map[string]any {
	return map[string]any{
		"id":      i.index,
		"labels":  map[string]any{"env": "prod", "shard": i.index % 32},
		"payload": map[string]any{"active": true, "index": i.index},
	}
}
func (i *generatedStructuredRowIterator) Err() error   { return nil }
func (i *generatedStructuredRowIterator) Close() error { return nil }

func (i *sliceRowIterator) Columns() []api.ColumnDef { return i.columns }
func (i *sliceRowIterator) Next() bool {
	if i.index >= len(i.rows) {
		return false
	}
	i.index++
	return true
}
func (i *sliceRowIterator) Row() map[string]any { return i.rows[i.index-1] }
func (i *sliceRowIterator) Err() error          { return nil }
func (i *sliceRowIterator) Close() error        { return nil }

func TestWriteTableStreamStructuredFormatsPreserveSchemaLessRows(t *testing.T) {
	rows := []map[string]any{{"a": 1}, {"a": 2, "later": true}}

	var jsonOutput bytes.Buffer
	count, err := WriteTableStream(context.Background(), &jsonOutput, &sliceRowIterator{rows: rows}, StreamOptions{Format: "json"})
	if err != nil || count != 2 {
		t.Fatalf("json count=%d err=%v", count, err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v: %s", err, jsonOutput.String())
	}
	if got := decoded[1]["later"]; got != true {
		t.Fatalf("later row field was lost: %#v", decoded[1])
	}

	var ndjson bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &ndjson, &sliceRowIterator{rows: rows}, StreamOptions{Format: "ndjson"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimSpace(ndjson.String()), "\n"); got != 1 {
		t.Fatalf("expected two ndjson records, got %q", ndjson.String())
	}

	var yamlOutput bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &yamlOutput, &sliceRowIterator{rows: rows}, StreamOptions{Format: "yaml"}); err != nil {
		t.Fatal(err)
	}
	var yamlRows []map[string]any
	if err := yaml.Unmarshal(yamlOutput.Bytes(), &yamlRows); err != nil {
		t.Fatalf("invalid yaml: %v: %s", err, yamlOutput.String())
	}
	if got := yamlRows[1]["later"]; got != true {
		t.Fatalf("later row field was lost: %#v", yamlRows[1])
	}
}

func TestWriteTableStreamTabularFormatsUseDeclaredColumns(t *testing.T) {
	columns := []api.ColumnDef{
		{Name: "name", Label: "Display name"},
		{Name: "secret", Hidden: true},
		{Name: "count"},
	}
	rows := []map[string]any{{"count": 3, "name": "a|b", "secret": "hidden"}}

	for _, format := range []string{"csv", "markdown", "html"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			count, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: format})
			if err != nil || count != 1 {
				t.Fatalf("count=%d err=%v", count, err)
			}
			if strings.Contains(output.String(), "secret") || strings.Contains(output.String(), "hidden") {
				t.Fatalf("hidden column leaked: %s", output.String())
			}
			if !strings.Contains(output.String(), "Display name") || !strings.Contains(output.String(), "Count") {
				t.Fatalf("declared headers missing: %s", output.String())
			}
		})
	}
}

func TestWriteTableStreamStructuredColumns(t *testing.T) {
	columns := []api.ColumnDef{
		{Name: "labels", Type: api.ColumnTypeKeyValue},
		{Name: "metadata", Type: api.ColumnTypeJSON},
	}
	rows := []map[string]any{{
		"labels":   map[string]any{"team": "core", "env": "prod"},
		"metadata": map[string]any{"retries": 3, "enabled": true},
	}}

	var csvOutput bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &csvOutput, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv"}); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(csvOutput.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if records[1][0] != "env=prod, team=core" || records[1][1] != `{"enabled":true,"retries":3}` {
		t.Fatalf("unexpected structured CSV: %#v", records)
	}

	var jsonOutput bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &jsonOutput, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "json"}); err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded[0]["labels"].(map[string]any); !ok {
		t.Fatalf("raw labels were flattened: %#v", decoded[0]["labels"])
	}
	if _, ok := decoded[0]["metadata"].(map[string]any); !ok {
		t.Fatalf("raw JSON was flattened: %#v", decoded[0]["metadata"])
	}
}

func TestWriteTableStreamExcel(t *testing.T) {
	var output bytes.Buffer
	rows := &sliceRowIterator{
		columns: []api.ColumnDef{{Name: "name"}, {Name: "count"}},
		rows:    []map[string]any{{"name": "alpha", "count": 3}},
	}
	if _, err := WriteTableStream(context.Background(), &output, rows, StreamOptions{Format: "xlsx"}); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if cell, _ := book.GetCellValue("Sheet1", "A2"); cell != "alpha" {
		t.Fatalf("unexpected cell A2: %q", cell)
	}
}

func TestWriteTableStreamPDFRequiresAndEnforcesLimit(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}}
	var output bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{rows: rows}, StreamOptions{Format: "pdf"}); err == nil {
		t.Fatal("expected missing PDF limit to fail")
	}
	if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{rows: rows}, StreamOptions{Format: "pdf", MaxRows: 1}); err == nil || !strings.Contains(err.Error(), "maximum 1") {
		t.Fatalf("expected PDF row limit error, got %v", err)
	}
}

func TestWriteTableStreamEmptyYAMLIsSequence(t *testing.T) {
	var output bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{}, StreamOptions{Format: "yaml"}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[]\n" {
		t.Fatalf("unexpected empty yaml: %q", output.String())
	}
}

func TestWriteTableStreamMillionStructuredRows(t *testing.T) {
	if os.Getenv("CLICKY_MILLION_ROW_SMOKE") == "" {
		t.Skip("set CLICKY_MILLION_ROW_SMOKE=1 to run the million-row export smoke test")
	}
	rows := &generatedStructuredRowIterator{count: 1_000_000}
	count, err := WriteTableStream(context.Background(), io.Discard, rows, StreamOptions{Format: "ndjson"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1_000_000 || rows.index != 1_000_000 {
		t.Fatalf("count=%d generated=%d", count, rows.index)
	}
}
