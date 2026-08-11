package formatters

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v3"
)

// csvBOM makes Excel on Windows decode a downloaded export as UTF-8 instead of
// the machine's ANSI code page, which otherwise mangles every non-ASCII cell.
const csvBOM = "\xEF\xBB\xBF"

// RowIterator is the bounded-memory table source consumed by WriteTableStream.
// Columns may be empty; in that case the first row establishes a stable,
// alphabetically sorted schema for table-oriented formats.
type RowIterator interface {
	Columns() []api.ColumnDef
	Next() bool
	Row() map[string]any
	Err() error
	Close() error
}

// StreamOptions controls incremental table formatting. MaxRows is enforced
// before PDF rendering and is otherwise zero (unlimited) unless a caller opts
// into a cap.
type StreamOptions struct {
	Format  string
	MaxRows int64
	// FlushEvery pushes buffered output downstream every N rows so a long
	// running HTTP export appears as it is produced instead of arriving in one
	// burst. Zero keeps the single flush at the end of the stream.
	FlushEvery int
	// CSVBOM writes a UTF-8 BOM ahead of CSV output. It exists for files that
	// are downloaded and opened in a spreadsheet; a caller streaming CSV into a
	// pipe or a parser must not receive bytes it did not ask for, so the same
	// endpoint serving a plain API read leaves this false.
	CSVBOM bool
}

// WriteTableStream writes rows incrementally in a Clicky-supported tabular
// format. Text formats keep only the current row in memory. Excel uses
// excelize's streaming worksheet writer; PDF deliberately buffers no more than
// MaxRows so the existing Chromium formatter can render a bounded document.
func WriteTableStream(ctx context.Context, w io.Writer, rows RowIterator, opts StreamOptions) (count int64, err error) {
	defer func() {
		if closeErr := rows.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	format := canonicalFormat(strings.ToLower(strings.TrimSpace(opts.Format)))
	if format == "" {
		format = "json"
	}

	first, ok, err := nextStreamRow(ctx, rows)
	if err != nil {
		return 0, err
	}
	columns := visibleStreamColumns(rows.Columns())
	structuredColumns := columns
	if len(columns) == 0 && ok {
		columns = derivedStreamColumns(first)
	}

	switch format {
	case "json":
		return writeJSONStream(ctx, w, rows, structuredColumns, first, ok, false, opts)
	case "ndjson":
		return writeJSONStream(ctx, w, rows, structuredColumns, first, ok, true, opts)
	case "yaml":
		return writeYAMLStream(ctx, w, rows, structuredColumns, first, ok, opts)
	case "csv":
		return writeCSVStream(ctx, w, rows, columns, first, ok, opts)
	case "markdown":
		return writeMarkdownStream(ctx, w, rows, columns, first, ok, opts)
	case "html":
		return writeHTMLStream(ctx, w, rows, columns, first, ok, opts)
	case "excel":
		return writeExcelStream(ctx, w, rows, columns, first, ok, opts)
	case "pdf":
		return writePDFStream(ctx, w, rows, columns, first, ok, opts)
	default:
		return 0, fmt.Errorf("unsupported streaming format: %s", format)
	}
}

func nextStreamRow(ctx context.Context, rows RowIterator) (map[string]any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	return rows.Row(), true, nil
}

func visibleStreamColumns(columns []api.ColumnDef) []api.ColumnDef {
	out := make([]api.ColumnDef, 0, len(columns))
	for _, column := range columns {
		if !column.Hidden {
			out = append(out, column)
		}
	}
	return out
}

func derivedStreamColumns(row map[string]any) []api.ColumnDef {
	names := make([]string, 0, len(row))
	for name := range row {
		names = append(names, name)
	}
	sort.Strings(names)
	columns := make([]api.ColumnDef, len(names))
	for i, name := range names {
		columns[i] = api.ColumnDef{Name: name}
	}
	return columns
}

func projectedStreamRow(row map[string]any, columns []api.ColumnDef) map[string]any {
	if len(columns) == 0 {
		return row
	}
	out := make(map[string]any, len(columns))
	for _, column := range columns {
		out[column.Name] = row[column.Name]
	}
	return out
}

func streamRows(ctx context.Context, rows RowIterator, first map[string]any, hasFirst bool, maxRows int64, emit func(map[string]any) error) (int64, error) {
	var count int64
	emitRow := func(row map[string]any) error {
		if maxRows > 0 && count >= maxRows {
			return fmt.Errorf("row limit exceeded: maximum %d", maxRows)
		}
		if err := emit(row); err != nil {
			return err
		}
		count++
		return nil
	}
	if hasFirst {
		if err := emitRow(first); err != nil {
			return count, err
		}
	}
	for {
		row, ok, err := nextStreamRow(ctx, rows)
		if err != nil {
			return count, err
		}
		if !ok {
			return count, nil
		}
		if err := emitRow(row); err != nil {
			return count, err
		}
	}
}

// flushingEmit wraps emit so buffered bytes reach the client every n rows.
// n <= 0 returns emit untouched, keeping the historical behaviour where an
// export is only visible once the whole response has been produced.
func flushingEmit(n int, flush func() error, emit func(map[string]any) error) func(map[string]any) error {
	if n <= 0 {
		return emit
	}
	written := 0
	return func(row map[string]any) error {
		if err := emit(row); err != nil {
			return err
		}
		written++
		if written%n != 0 {
			return nil
		}
		return flush()
	}
}

// flushStream pushes whatever w has buffered towards the client. HTTP handlers
// need a ResponseController because net/http and any middleware wrap the
// original writer; other writers qualify only if they expose Flush themselves,
// and the rest (files, buffers) are unbuffered so there is nothing to push.
func flushStream(w io.Writer) error {
	switch target := w.(type) {
	case http.ResponseWriter:
		// A transport that cannot flush is not an export failure.
		if err := http.NewResponseController(target).Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	case interface{ Flush() error }:
		return target.Flush()
	case interface{ Flush() }:
		target.Flush()
	}
	return nil
}

// escapeSpreadsheetFormula neutralises a cell that Excel, LibreOffice or Sheets
// would evaluate as a formula. Row values come from backends the operator does
// not control, and these exports are opened as downloads; a leading single
// quote forces the cell to be read as literal text.
func escapeSpreadsheetFormula(text string) string {
	if text == "" {
		return text
	}
	switch text[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + text
	}
	return text
}

// escapeSpreadsheetCell escapes only string-shaped values. A number is emitted
// as a number and cannot carry a formula, so quoting a value like -5 would just
// turn it into text and break sorting and arithmetic in the sheet.
func escapeSpreadsheetCell(value any, rendered string) string {
	if isSpreadsheetNumber(value) {
		return rendered
	}
	return escapeSpreadsheetFormula(rendered)
}

// isSpreadsheetNumber reports whether the sheet holds the value as a number (or
// a boolean), which no spreadsheet evaluates as a formula.
//
// The kind is resolved through reflection rather than matched against a list of
// concrete types, because rows come from arbitrary backends: a *int64, a named
// type like `type Bytes uint64`, or a pointer to one are all still numbers, and
// listing types meant a negative number reached the sheet as the text '-5.
// json.Number is a string kind but a number in the sheet; []byte is a slice
// carrying text, so it stays escaped.
func isSpreadsheetNumber(value any) bool {
	switch value.(type) {
	case nil, json.Number:
		return true
	case []byte:
		return false
	}
	valueType := reflect.TypeOf(value)
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func writeJSONStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok, ndjson bool, opts StreamOptions) (int64, error) {
	enc := json.NewEncoder(w)
	firstJSON := true
	if !ndjson {
		if _, err := io.WriteString(w, "["); err != nil {
			return 0, err
		}
	}
	flush := func() error { return flushStream(w) }
	count, err := streamRows(ctx, rows, first, ok, opts.MaxRows, flushingEmit(opts.FlushEvery, flush, func(row map[string]any) error {
		row = projectedStreamRow(row, columns)
		if ndjson {
			return enc.Encode(row)
		}
		if !firstJSON {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		firstJSON = false
		data, err := json.Marshal(row)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}))
	if err != nil {
		return count, err
	}
	if !ndjson {
		_, err = io.WriteString(w, "]\n")
	}
	return count, err
}

func writeYAMLStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	if !ok {
		_, err := io.WriteString(w, "[]\n")
		return 0, err
	}
	flush := func() error { return flushStream(w) }
	return streamRows(ctx, rows, first, ok, opts.MaxRows, flushingEmit(opts.FlushEvery, flush, func(row map[string]any) error {
		data, err := yaml.Marshal(projectedStreamRow(row, columns))
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		for i, line := range lines {
			prefix := "  "
			if i == 0 {
				prefix = "- "
			}
			if _, err := fmt.Fprintln(w, prefix+line); err != nil {
				return err
			}
		}
		return nil
	}))
}

func writeCSVStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	if opts.CSVBOM {
		if _, err := io.WriteString(w, csvBOM); err != nil {
			return 0, err
		}
	}
	writer := csv.NewWriter(w)
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = escapeSpreadsheetFormula(column.DisplayLabel())
	}
	if err := writer.Write(headers); err != nil {
		return 0, err
	}
	// csv.Writer holds its own 4KiB buffer, so it has to be drained before the
	// response writer is flushed or nothing reaches the client.
	flush := func() error {
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		return flushStream(w)
	}
	count, err := streamRows(ctx, rows, first, ok, opts.MaxRows, flushingEmit(opts.FlushEvery, flush, func(row map[string]any) error {
		values := make([]string, len(columns))
		for i, column := range columns {
			values[i] = escapeSpreadsheetCell(row[column.Name], streamCell(row[column.Name], column, "plain"))
		}
		return writer.Write(values)
	}))
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	return count, err
}

func writeMarkdownStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	headers := make([]string, len(columns))
	separators := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = escapeMarkdownStream(column.DisplayLabel())
		separators[i] = "---"
	}
	if _, err := fmt.Fprintf(w, "| %s |\n| %s |\n", strings.Join(headers, " | "), strings.Join(separators, " | ")); err != nil {
		return 0, err
	}
	flush := func() error { return flushStream(w) }
	return streamRows(ctx, rows, first, ok, opts.MaxRows, flushingEmit(opts.FlushEvery, flush, func(row map[string]any) error {
		values := make([]string, len(columns))
		for i, column := range columns {
			values[i] = escapeMarkdownStream(streamCell(row[column.Name], column, "markdown"))
		}
		_, err := fmt.Fprintf(w, "| %s |\n", strings.Join(values, " | "))
		return err
	}))
}

func writeHTMLStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	if _, err := io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>Clicky export</title><style>body{font-family:system-ui,sans-serif;margin:24px}table{border-collapse:collapse;width:100%}th,td{border:1px solid #d1d5db;padding:6px 8px;text-align:left;vertical-align:top}th{background:#f3f4f6;position:sticky;top:0}</style></head><body><table><thead><tr>"); err != nil {
		return 0, err
	}
	for _, column := range columns {
		if _, err := fmt.Fprintf(w, "<th>%s</th>", html.EscapeString(column.DisplayLabel())); err != nil {
			return 0, err
		}
	}
	if _, err := io.WriteString(w, "</tr></thead><tbody>"); err != nil {
		return 0, err
	}
	flush := func() error { return flushStream(w) }
	count, err := streamRows(ctx, rows, first, ok, opts.MaxRows, flushingEmit(opts.FlushEvery, flush, func(row map[string]any) error {
		if _, err := io.WriteString(w, "<tr>"); err != nil {
			return err
		}
		for _, column := range columns {
			if _, err := fmt.Fprintf(w, "<td>%s</td>", streamCell(row[column.Name], column, "html")); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</tr>")
		return err
	}))
	if err != nil {
		return count, err
	}
	_, err = io.WriteString(w, "</tbody></table></body></html>\n")
	return count, err
}

func writeExcelStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	file := excelize.NewFile()
	defer file.Close()
	stream, err := file.NewStreamWriter("Sheet1")
	if err != nil {
		return 0, err
	}
	headers := make([]any, len(columns))
	for i, column := range columns {
		headers[i] = escapeSpreadsheetFormula(column.DisplayLabel())
	}
	if err := stream.SetRow("A1", headers); err != nil {
		return 0, err
	}
	rowNumber := 2
	// excelize writes strings as inline strings, which Excel does not evaluate,
	// but the quote also survives a re-export of the sheet to CSV where it would.
	count, err := streamRows(ctx, rows, first, ok, opts.MaxRows, func(row map[string]any) error {
		values := make([]any, len(columns))
		for i, column := range columns {
			value := row[column.Name]
			if api.IsStructuredColumnType(column.Type) {
				values[i] = escapeSpreadsheetFormula(api.ColumnString(column, value))
			} else if text, isText := value.(string); isText {
				values[i] = escapeSpreadsheetFormula(text)
			} else {
				values[i] = value
			}
		}
		cell, err := excelize.CoordinatesToCellName(1, rowNumber)
		if err != nil {
			return err
		}
		rowNumber++
		return stream.SetRow(cell, values)
	})
	if err != nil {
		return count, err
	}
	if err := stream.Flush(); err != nil {
		return count, err
	}
	return count, file.Write(w)
}

func writePDFStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	if opts.MaxRows <= 0 {
		return 0, fmt.Errorf("pdf streaming requires a positive row limit")
	}
	table := api.TextTable{}
	for _, column := range columns {
		table.Headers = append(table.Headers, api.Text{Content: column.DisplayLabel()})
		table.FieldNames = append(table.FieldNames, column.Name)
		table.Columns = append(table.Columns, api.PrettyField{Name: column.Name, Label: column.DisplayLabel(), Type: column.Type, Format: column.Format, Unit: column.Unit})
	}
	count, err := streamRows(ctx, rows, first, ok, opts.MaxRows, func(row map[string]any) error {
		out := api.TableRow{}
		for _, column := range columns {
			out[column.Name] = api.NewTypedValue(api.ColumnTextable(column, row[column.Name]))
		}
		table.Rows = append(table.Rows, out)
		return nil
	})
	if err != nil {
		return count, err
	}
	manager := NewFormatManager()
	output, err := manager.FormatWithOptions(FormatOptions{Format: "pdf"}, table)
	if err != nil {
		return count, err
	}
	_, err = io.WriteString(w, output)
	return count, err
}

func streamCell(value any, column api.ColumnDef, format string) string {
	text := api.ColumnTextable(column, value)
	switch format {
	case "html":
		return text.HTML()
	default:
		return api.ColumnString(column, value)
	}
}

func escapeMarkdownStream(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", "<br>")
}
