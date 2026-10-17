package formatters

import (
	"fmt"

	"github.com/flanksource/clicky/api"
	"github.com/xuri/excelize/v2"
)

type ExcelSheet struct {
	Name       string
	Provenance string
	Table      api.TextTable
}

type ExcelWorkbook struct {
	Sheets []ExcelSheet
}

func formatExcelWorkbook(workbook ExcelWorkbook) (string, error) {
	if len(workbook.Sheets) == 0 {
		return "", fmt.Errorf("Excel workbook has no result sets")
	}
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	for index, sheet := range workbook.Sheets {
		if sheet.Name == "" {
			return "", fmt.Errorf("Excel sheet %d has no name", index+1)
		}
		if index == 0 {
			if err := file.SetSheetName("Sheet1", sheet.Name); err != nil {
				return "", fmt.Errorf("rename first Excel sheet: %w", err)
			}
		} else if _, err := file.NewSheet(sheet.Name); err != nil {
			return "", fmt.Errorf("create Excel sheet %s: %w", sheet.Name, err)
		}
		writer, err := file.NewStreamWriter(sheet.Name)
		if err != nil {
			return "", fmt.Errorf("stream Excel sheet %s: %w", sheet.Name, err)
		}
		if err := writer.SetRow("A1", []any{sheet.Provenance}); err != nil {
			return "", err
		}
		headers := make([]any, len(sheet.Table.Headers))
		for i, header := range sheet.Table.Headers {
			headers[i] = escapeSpreadsheetFormula(header.String())
		}
		if err := writer.SetRow("A2", headers); err != nil {
			return "", err
		}
		for rowIndex, row := range sheet.Table.Rows {
			values := make([]any, len(sheet.Table.Headers))
			for columnIndex, header := range sheet.Table.Headers {
				key := header.String()
				if columnIndex < len(sheet.Table.FieldNames) {
					key = sheet.Table.FieldNames[columnIndex]
				}
				cell := row[key]
				switch value := cell.FilterValue.(type) {
				case string:
					values[columnIndex] = escapeSpreadsheetFormula(value)
				case nil:
					values[columnIndex] = cell.String()
				default:
					values[columnIndex] = value
				}
			}
			address, err := excelize.CoordinatesToCellName(1, rowIndex+3)
			if err != nil {
				return "", err
			}
			if err := writer.SetRow(address, values); err != nil {
				return "", err
			}
		}
		if err := writer.Flush(); err != nil {
			return "", fmt.Errorf("flush Excel sheet %s: %w", sheet.Name, err)
		}
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", fmt.Errorf("write Excel workbook: %w", err)
	}
	return buffer.String(), nil
}
