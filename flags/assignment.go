package flags

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/flanksource/commons/logger"
	"github.com/xuri/excelize/v2"
)

const ARGS = "__args__"

// AssignFieldValue assigns a flag value to a struct field using the field path
func AssignFieldValue(structValue reflect.Value, fv *FlagValue, args []string, isStdinAvailable bool) error {
	fieldValue := GetFieldByPath(structValue, fv.FieldPath)

	if !fieldValue.IsValid() || !fieldValue.CanSet() {
		return fmt.Errorf("cannot set field at path %v", fv.FieldPath)
	}

	// Process based on type
	switch fv.FieldType.Kind() {
	case reflect.String:
		val := *fv.StringPtr

		// Priority: args first, then stdin, then flag value
		if val == "" && fv.IsArgs && len(args) > 0 {
			val = args[0]
		} else if val == "" && fv.IsStdin && len(args) > 0 {
			// stdin:"true" can also use args as fallback
			val = args[0]
		} else if val == "" && fv.IsStdin && isStdinAvailable {
			if lines, err := loadFromStdin(); err != nil {
				return err
			} else if len(lines) > 0 {
				val = lines[0]
			}
		}

		if loaded, err := loadFromFileOrURL(val); err != nil {
			return fmt.Errorf("loading %s: %w", fv.FieldName, err)
		} else {
			fieldValue.SetString(loaded)
		}

	case reflect.Int:
		fieldValue.SetInt(int64(*fv.IntPtr))

	case reflect.Bool:
		fieldValue.SetBool(*fv.BoolPtr)

	case reflect.Slice:
		switch fv.FieldType.Elem().Kind() {
		case reflect.String:
			val := *fv.StringSlicePtr

			// Priority: args first, then stdin, then flag value
			if len(val) == 0 && fv.IsArgs && len(args) > 0 {
				val = args
			} else if len(val) == 0 && fv.IsStdin && len(args) > 0 {
				// stdin:"true" can also use args as fallback
				val = args
			} else if len(val) == 0 && fv.IsStdin && isStdinAvailable {
				if val, err := loadFromStdin(); err != nil {
					return err
				} else {
					return assignValue(fieldValue, val)
				}
			}

			if len(val) == 1 {
				if lines, err := loadLinesFromFileOrURL(val[0]); err != nil {
					return err
				} else {
					if err := assignValue(fieldValue, lines); err != nil {
						return err
					}
				}
			} else {
				if err := assignValue(fieldValue, val); err != nil {
					return err
				}
			}

		case reflect.Int:
			if err := assignValue(fieldValue, *fv.IntSlicePtr); err != nil {
				return err
			}
		}

	default:
		// Handle special types
		typeName := fv.FieldType.String()
		switch typeName {
		case "duration.Duration":
			fieldValue.Set(reflect.ValueOf(*fv.DurationPtr))

		case "time.Time":
			fieldValue.Set(reflect.ValueOf(*fv.TimePtr))
		}
	}

	return nil
}

func assignValue(fieldValue reflect.Value, value any) error {
	val := reflect.ValueOf(value)
	if val.Type().AssignableTo(fieldValue.Type()) {
		fieldValue.Set(val)
		return nil
	}
	if val.Type().ConvertibleTo(fieldValue.Type()) {
		fieldValue.Set(val.Convert(fieldValue.Type()))
		return nil
	}
	return fmt.Errorf("cannot assign %s to %s", val.Type(), fieldValue.Type())
}

// loadFromStdin reads lines from stdin
func loadFromStdin() ([]string, error) {
	if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil, nil
	} else if stat != nil {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines, nil
	}
	return nil, nil
}

// loadFromFileOrURL loads content from a file or URL
func loadFromFileOrURL(path string) (string, error) {
	if !strings.HasPrefix(path, "@") {
		return path, nil
	}
	path = strings.TrimPrefix(path, "@")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path)
		if err != nil {
			return "", err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				logger.Errorf("failed to close response body: %v", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadLinesFromFileOrURL loads lines from a file or URL for slice flags.
//
// Supported syntaxes (all prefixed with @):
//
//	@file.txt                 — one value per line, lines starting with # are skipped
//	@https://example.com/list — same, fetched over HTTP
//	@data.csv:ColumnName      — values from the named column of a CSV (first row is the header)
//	@data.xlsx:ColumnName     — values from the named column of the first worksheet
//	@data.xls:ColumnName      — same
//
// For the :ColumnName forms, empty cells are skipped. The column split only
// kicks in for .csv/.xls/.xlsx files so Windows paths like @C:/data.txt
// still work as plain files.
func loadLinesFromFileOrURL(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, "@") {
		// Not a file reference; caller will treat it as a single literal value.
		return []string{raw}, nil
	}

	path, column := splitColumnSelector(strings.TrimPrefix(raw, "@"))
	if column != "" {
		return loadColumnFromFile(path, column)
	}

	content, err := loadFromFileOrURL("@" + path)
	if err != nil {
		return nil, err
	}
	return parseLines(content), nil
}

// splitColumnSelector splits path:Column into (path, column) when the file
// extension is one of the tabular formats (.csv, .xls, .xlsx). For any other
// extension (or no colon) it returns (raw, "").
//
// This is deliberately extension-gated so that a Windows-style drive letter
// prefix like "C:/data.txt" or a URL like "https://..." is not misread as a
// column selector.
func splitColumnSelector(raw string) (string, string) {
	idx := strings.LastIndex(raw, ":")
	if idx < 0 {
		return raw, ""
	}
	candidate := raw[:idx]
	column := raw[idx+1:]
	if column == "" {
		return raw, ""
	}
	switch strings.ToLower(filepath.Ext(candidate)) {
	case ".csv", ".xls", ".xlsx":
		return candidate, column
	}
	return raw, ""
}

// loadColumnFromFile reads the named column from a CSV or Excel file and
// returns the non-empty cell values in row order.
func loadColumnFromFile(path, column string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return loadColumnFromCSV(path, column)
	case ".xls", ".xlsx":
		return loadColumnFromExcel(path, column)
	}
	return nil, fmt.Errorf("unsupported file extension %q for column selector", ext)
}

func loadColumnFromCSV(path, column string) ([]string, error) {
	content, err := loadFromFileOrURL("@" + path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader([]byte(content)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	idx := indexOfHeader(records[0], column)
	if idx < 0 {
		return nil, fmt.Errorf("column %q not found in %s (available: %s)", column, path, strings.Join(records[0], ", "))
	}

	var out []string
	for _, row := range records[1:] {
		if idx >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[idx])
		if val != "" {
			out = append(out, val)
		}
	}
	return out, nil
}

func loadColumnFromExcel(path, column string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			logger.Errorf("failed to close excel file %s: %v", path, cerr)
		}
	}()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel file %s has no sheets", path)
	}
	sheet := sheets[0]

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("reading sheet %q from %s: %w", sheet, path, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	idx := indexOfHeader(rows[0], column)
	if idx < 0 {
		return nil, fmt.Errorf("column %q not found in %s[%s] (available: %s)", column, path, sheet, strings.Join(rows[0], ", "))
	}

	var out []string
	for _, row := range rows[1:] {
		if idx >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[idx])
		if val != "" {
			out = append(out, val)
		}
	}
	return out, nil
}

// indexOfHeader returns the index of column in headers using case-insensitive
// comparison with whitespace trimmed. Returns -1 when not found.
func indexOfHeader(headers []string, column string) int {
	target := strings.ToLower(strings.TrimSpace(column))
	for i, h := range headers {
		if strings.ToLower(strings.TrimSpace(h)) == target {
			return i
		}
	}
	return -1
}

// parseLines parses lines from content, skipping empty lines and comments.
func parseLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}
