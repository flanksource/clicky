package celfilter_test

import (
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/filter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type nullableReading struct{ Value any }

func (nullableReading) Columns() []api.ColumnDef {
	return []api.ColumnDef{api.Column("value").FilterKey("filter.value").Build()}
}
func (r nullableReading) Row() map[string]any {
	return map[string]any{"value": api.TableCell{Value: api.Text{}.Append("—"), FilterValue: r.Value}}
}

var _ = Describe("explicitly presented scalar filters", func() {
	It("distinguishes null from zero and false despite identical display text", func() {
		table := api.NewTableFrom([]nullableReading{{Value: nil}, {Value: int64(0)}, {Value: false}})
		rows := make([]api.PrettyDataRow, len(table.Rows))
		for i, row := range table.Rows {
			rows[i] = api.PrettyDataRow(row)
		}
		Expect(filter.TableRows(rows, "value == null")).To(Equal(rows[:1]))
		Expect(filter.TableRows(rows, "value == 0")).To(Equal(rows[1:2]))
		Expect(filter.TableRows(rows, "value == false")).To(Equal(rows[2:]))
	})
})
