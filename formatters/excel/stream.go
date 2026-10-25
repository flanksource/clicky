package excel

import (
	"context"
	"io"

	"github.com/xuri/excelize/v2"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

func writeStream(ctx context.Context, w io.Writer, table *formatters.TableStream) (int64, error) {
	file := excelize.NewFile()
	defer file.Close()
	stream, err := file.NewStreamWriter("Sheet1")
	if err != nil {
		return 0, err
	}
	headers := make([]any, len(table.Columns))
	for i, column := range table.Columns {
		headers[i] = formatters.EscapeSpreadsheetFormula(column.DisplayLabel())
	}
	if err := stream.SetRow("A1", headers); err != nil {
		return 0, err
	}
	rowNumber := 2
	// excelize writes strings as inline strings, which Excel does not evaluate,
	// but the quote also survives a re-export of the sheet to CSV where it would.
	count, err := table.Each(ctx, func(row map[string]any) error {
		values := make([]any, len(table.Columns))
		for i, column := range table.Columns {
			value := row[column.Name]
			if api.IsStructuredColumnType(column.Type) {
				values[i] = formatters.EscapeSpreadsheetFormula(api.ColumnString(column, value))
			} else if text, isText := value.(string); isText {
				values[i] = formatters.EscapeSpreadsheetFormula(text)
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
