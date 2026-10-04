package formatters

import (
	"context"
	"io"

	"github.com/flanksource/clicky/api"
)

func writeLLMStream(ctx context.Context, w io.Writer, rows RowIterator, columns []api.ColumnDef, first map[string]any, ok bool, opts StreamOptions) (int64, error) {
	table := api.NewEmptyTable(columns)
	count, err := streamRows(ctx, rows, first, ok, api.LLMPageSize, func(row map[string]any) error {
		cells := api.TableRow{}
		for _, column := range columns {
			cells[column.Name] = api.NewTypedValue(api.ColumnTextable(column, row[column.Name]))
		}
		table.Rows = append(table.Rows, cells)
		return nil
	})
	if err != nil {
		return count, err
	}
	out, err := formatLLM(table, FormatOptions{LLMPage: opts.LLMPage})
	if err != nil {
		return count, err
	}
	_, err = io.WriteString(w, out)
	return count, err
}
