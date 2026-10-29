// Package excel renders and reads xlsx workbooks for clicky. It is the only
// clicky package importing excelize, so binaries that never touch spreadsheets
// don't compile it; opt in with Use().
package excel

import (
	"errors"
	"fmt"
	"io"
	"strings"

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
	flags.SetSpreadsheetReader(readWorksheet)
}

// WriteFile renders data as an xlsx workbook at path.
func WriteFile(data any, path string) error {
	return NewFormatter().FormatToFile(data, path)
}

func readWorksheet(r io.Reader, options flags.SpreadsheetReadOptions) (sheet string, rows [][]string, err error) {
	if options.Sheet != "" && options.Index != 0 {
		return "", nil, fmt.Errorf("select a worksheet by name or index, not both")
	}
	if options.Index < 0 {
		return "", nil, fmt.Errorf("worksheet index must be a positive integer")
	}
	file, err := excelize.OpenReader(r)
	if err != nil {
		return "", nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return "", nil, fmt.Errorf("workbook has no sheets")
	}
	sheet = sheets[0]
	if options.Index > len(sheets) {
		return "", nil, fmt.Errorf("worksheet index %d out of range (workbook has %d sheets)", options.Index, len(sheets))
	}
	if options.Index > 0 {
		sheet = sheets[options.Index-1]
	}
	if options.Sheet != "" {
		sheet = ""
		for _, name := range sheets {
			if strings.EqualFold(name, options.Sheet) {
				sheet = name
				break
			}
		}
		if sheet == "" {
			return "", nil, fmt.Errorf("worksheet %q not found (available: %s)", options.Sheet, strings.Join(sheets, ", "))
		}
	}
	rows, err = file.GetRows(sheet)
	if err != nil {
		return "", nil, fmt.Errorf("reading sheet %q: %w", sheet, err)
	}
	return sheet, rows, nil
}
