package formatters

import (
	"encoding/json"
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type nullableRow struct{ Value any }

func (nullableRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{api.Column("value").FilterKey("filter.value").Build()}
}
func (r nullableRow) Row() map[string]any {
	return map[string]any{"value": api.TableCell{Value: api.Text{}.Append("—"), FilterValue: r.Value}}
}

var _ = Describe("nullable table cells", func() {
	It("serializes a presented null as an explicit native null", func() {
		table := api.NewTableFrom([]nullableRow{{}})
		node := convertTable(&table)
		data, err := json.Marshal(node.Rows[0].Cells["value"])
		Expect(err).NotTo(HaveOccurred())
		var cell map[string]any
		Expect(json.Unmarshal(data, &cell)).To(Succeed())
		Expect(cell).To(HaveKeyWithValue("filterValue", BeNil()))
	})
})
