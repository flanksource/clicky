package formatters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

// flushRecordingWriter records everything received so far each time it is
// flushed, so a test can tell what was actually observable mid-stream.
type flushRecordingWriter struct {
	mu       sync.Mutex
	received bytes.Buffer
	flushes  []string
}

func (w *flushRecordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.received.Write(p)
}

func (w *flushRecordingWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushes = append(w.flushes, w.received.String())
	return nil
}

// gatedRowIterator stalls before the second row until the consumer signals that
// it has already seen the first one.
type gatedRowIterator struct {
	columns  []api.ColumnDef
	rows     []map[string]any
	index    int
	gate     chan struct{}
	timedOut bool
}

func (i *gatedRowIterator) Columns() []api.ColumnDef { return i.columns }
func (i *gatedRowIterator) Next() bool {
	if i.index == 1 {
		select {
		case <-i.gate:
		case <-time.After(5 * time.Second):
			i.timedOut = true
		}
	}
	if i.index >= len(i.rows) {
		return false
	}
	i.index++
	return true
}
func (i *gatedRowIterator) Row() map[string]any { return i.rows[i.index-1] }
func (i *gatedRowIterator) Err() error          { return nil }
func (i *gatedRowIterator) Close() error        { return nil }

func parseStreamCSV(t *testing.T, output string) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(output, csvBOM))).ReadAll()
	if err != nil {
		t.Fatalf("invalid csv %q: %v", output, err)
	}
	return records
}

func TestWriteTableStreamFlushEveryDrainsTheCSVBufferFirst(t *testing.T) {
	columns := []api.ColumnDef{{Name: "name"}}
	rows := []map[string]any{{"name": "alpha"}, {"name": "beta"}, {"name": "gamma"}}

	flushed := &flushRecordingWriter{}
	if _, err := WriteTableStream(context.Background(), flushed, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv", FlushEvery: 1}); err != nil {
		t.Fatal(err)
	}
	if len(flushed.flushes) != len(rows) {
		t.Fatalf("expected one flush per row, got %d", len(flushed.flushes))
	}
	if !strings.Contains(flushed.flushes[0], "alpha") {
		t.Fatalf("csv buffer was not drained before the writer flush: %q", flushed.flushes[0])
	}
	if strings.Contains(flushed.flushes[0], "beta") {
		t.Fatalf("first flush leaked later rows: %q", flushed.flushes[0])
	}

	buffered := &flushRecordingWriter{}
	if _, err := WriteTableStream(context.Background(), buffered, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv"}); err != nil {
		t.Fatal(err)
	}
	if len(buffered.flushes) != 0 {
		t.Fatalf("FlushEvery=0 must not flush, got %d flushes", len(buffered.flushes))
	}
}

func TestWriteTableStreamFlushEveryDeliversRowsBeforeTheResponseEnds(t *testing.T) {
	iterator := &gatedRowIterator{
		columns: []api.ColumnDef{{Name: "name"}},
		rows:    []map[string]any{{"name": "alpha"}, {"name": "beta"}},
		gate:    make(chan struct{}),
	}
	done := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := WriteTableStream(r.Context(), w, iterator, StreamOptions{Format: "csv", FlushEvery: 1})
		done <- err
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body := bufio.NewReader(resp.Body)
	if _, err := body.ReadString('\n'); err != nil {
		t.Fatalf("header not delivered: %v", err)
	}
	firstRow, err := body.ReadString('\n')
	if err != nil {
		t.Fatalf("first row not delivered: %v", err)
	}
	close(iterator.gate)

	rest, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if iterator.timedOut {
		t.Fatalf("first row only reached the client once the response ended: %q", firstRow)
	}
	if !strings.Contains(firstRow, "alpha") || !strings.Contains(string(rest), "beta") {
		t.Fatalf("unexpected stream: first=%q rest=%q", firstRow, rest)
	}
}

func TestWriteTableStreamCSVEscapesSpreadsheetFormulas(t *testing.T) {
	dangerous := map[string]string{
		"equals": "=1+1",
		"plus":   "+1+1",
		"minus":  "-1+1",
		"at":     "@SUM(A1)",
		"tab":    "\t=cmd|'/c calc'!A0",
		"cr":     "\r=cmd|'/c calc'!A0",
	}
	columns := []api.ColumnDef{
		{Name: "equals"}, {Name: "plus"}, {Name: "minus"}, {Name: "at"}, {Name: "tab"}, {Name: "cr"},
		{Name: "safe"}, {Name: "negative", Type: "number"},
	}
	row := map[string]any{"safe": "hello", "negative": -5}
	for name, value := range dangerous {
		row[name] = value
	}

	var output bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{columns: columns, rows: []map[string]any{row}}, StreamOptions{Format: "csv"}); err != nil {
		t.Fatal(err)
	}
	records := parseStreamCSV(t, output.String())
	cells := map[string]string{}
	for i, column := range columns {
		cells[column.Name] = records[1][i]
	}
	for name, value := range dangerous {
		if cells[name] != "'"+value {
			t.Fatalf("%s cell was not neutralised: %q", name, cells[name])
		}
	}
	if cells["safe"] != "hello" {
		t.Fatalf("harmless value was rewritten: %q", cells["safe"])
	}
	if cells["negative"] != "-5" {
		t.Fatalf("numeric cell must stay a number, got %q", cells["negative"])
	}
}

// Rows come from arbitrary backends, so a number reaches the exporter as a
// pointer or a named type just as often as an int. Matching concrete types made
// those cells text: -5 was written as '-5 and stopped sorting as a number.
func TestEscapeSpreadsheetCellExemptsEveryNumericKind(t *testing.T) {
	type megabytes float64
	type revision uint16
	negative := int64(-5)

	numeric := map[string]any{
		"int":              -5,
		"float64":          -5.0,
		"uint16":           uint16(5),
		"json.Number":      json.Number("-5"),
		"bool":             false,
		"named float":      megabytes(-5),
		"named uint":       revision(5),
		"pointer to int64": &negative,
		"nil":              nil,
	}
	for name, value := range numeric {
		if got := escapeSpreadsheetCell(value, "-5"); got != "-5" {
			t.Errorf("%s cell must stay a number, got %q", name, got)
		}
	}

	// []byte is a slice, not a number: it carries text a sheet would evaluate.
	textual := map[string]any{
		"string": "-5+1",
		"[]byte": []byte("-5+1"),
	}
	for name, value := range textual {
		if got := escapeSpreadsheetCell(value, "-5+1"); got != "'-5+1" {
			t.Errorf("%s cell was not neutralised, got %q", name, got)
		}
	}
}

func TestWriteTableStreamStructuredFormatsKeepFormulaText(t *testing.T) {
	columns := []api.ColumnDef{{Name: "note"}}
	rows := []map[string]any{{"note": "=1+1"}}

	for _, format := range []string{"json", "ndjson", "yaml"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: format}); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "'=1+1") {
				t.Fatalf("%s must not spreadsheet-escape values: %s", format, output.String())
			}
		})
	}
}

func TestWriteTableStreamExcelEscapesFormulasButKeepsNumbers(t *testing.T) {
	columns := []api.ColumnDef{{Name: "note"}, {Name: "delta", Type: "number"}}
	rows := []map[string]any{{"note": "=1+1", "delta": -5}}

	var output bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &output, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "excel"}); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if note, _ := book.GetCellValue("Sheet1", "A2"); note != "'=1+1" {
		t.Fatalf("formula cell was not neutralised: %q", note)
	}
	if delta, _ := book.GetCellValue("Sheet1", "B2"); delta != "-5" {
		t.Fatalf("numeric cell must stay a number, got %q", delta)
	}
	deltaType, err := book.GetCellType("Sheet1", "B2")
	if err != nil {
		t.Fatal(err)
	}
	if deltaType == excelize.CellTypeInlineString || deltaType == excelize.CellTypeSharedString {
		t.Fatalf("negative number was written as text (%v)", deltaType)
	}
}

func TestWriteTableStreamCSVBOM(t *testing.T) {
	columns := []api.ColumnDef{{Name: "name"}}
	rows := []map[string]any{{"name": "Ünicode"}}

	// A plain API read is piped into cut/awk or another parser, so the first
	// header name must not pick up three extra bytes.
	var apiRead bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &apiRead, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv"}); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(apiRead.String(), csvBOM) {
		t.Fatalf("csv gained a BOM the caller did not ask for: %q", apiRead.String())
	}

	var download bytes.Buffer
	if _, err := WriteTableStream(context.Background(), &download, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv", CSVBOM: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(download.String(), csvBOM) {
		t.Fatalf("CSVBOM did not emit the UTF-8 BOM: %q", download.String())
	}
	if strings.TrimPrefix(download.String(), csvBOM) != apiRead.String() {
		t.Fatalf("BOM changed more than the prefix: %q vs %q", download.String(), apiRead.String())
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
