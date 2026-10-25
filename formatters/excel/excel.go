// Package excel renders and reads xlsx workbooks for clicky. It is the only
// clicky package importing excelize, so binaries that never touch spreadsheets
// don't compile it; opt in with Use().
package excel

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"

	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/formatters"
)

// Use registers the "excel" and "xlsx" formats, the "excel" streaming writer
// and the reader behind @data.xlsx:Column flag values.
func Use() {
	formatter := NewFormatter()
	format := func(data any, _ formatters.FormatOptions) (string, error) {
		return formatter.Format(data)
	}
	formatters.RegisterFormatter("excel", format)
	formatters.RegisterFormatter("xlsx", format)
	formatters.RegisterStreamWriter("excel", writeStream)
	flags.SetSpreadsheetReader(readFirstSheet)
}

// WriteFile renders data as an xlsx workbook at path.
func WriteFile(data any, path string) error {
	return NewFormatter().FormatToFile(data, path)
}

func readFirstSheet(r io.Reader) (string, [][]string, error) {
	file, err := excelize.OpenReader(r)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = file.Close() }()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return "", nil, fmt.Errorf("workbook has no sheets")
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return "", nil, fmt.Errorf("reading sheet %q: %w", sheets[0], err)
	}
	return sheets[0], rows, nil
}
