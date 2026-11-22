package table_test

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/tui/table"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"strings"
)

type threadRow struct{}

func (threadRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("id").Label("ID").Build(), api.Column("cpu").Label("CPU%/core").Build(), api.Column("state").Label("State").Build(),
		api.Column("name").Label("Thread").MaxWidth(50).Build(), api.Column("time").Label("CPU ms").Build(), api.Column("blocked").Label("Blocked").Build(), api.Column("waited").Label("Waited").Build(), api.Column("owner").Label("Lock owner").Build(),
	}
}
func (threadRow) Row() map[string]any {
	return map[string]any{"id": "24", "cpu": "1.2", "state": "TIMED_WAITING", "name": "scheduler-worker-request-handler", "time": "120", "blocked": "0", "waited": "4", "owner": "—"}
}

var _ = Describe("column sizing", func() {
	It("uses content widths for counters and gives thread names their declared space", func() {
		m := table.Model{}
		Expect(m.Set(api.NewTableFrom([]threadRow{{}}), []string{"24"})).To(Succeed())
		m.SetSize(120, 10)
		view := ansi.Strip(m.View())
		lines := strings.Split(view, "\n")
		Expect(lines[1]).To(ContainSubstring("scheduler-worker-request-handler"))
		Expect(lines[0]).To(ContainSubstring("Lock owner"))
		Expect(strings.Index(lines[0], "CPU%/core")).To(Equal(5))
		for _, width := range []int{1, 2, 20, 40, 80, 120, 200} {
			m.SetSize(width, 10)
			for _, line := range strings.Split(m.View(), "\n") {
				Expect(ansi.StringWidth(line)).To(BeNumerically("<=", width))
			}
		}
	})
	It("honors character limits without treating browser pixel widths as terminal cells", func() {
		data := api.NewEmptyTable([]api.ColumnDef{api.Column("id").Label("ID").Build(), api.Column("name").Label("Name").MaxWidth(4).MaxWidthPixels(400).Build()})
		data.Rows = []api.TableRow{{"id": api.NewTypedValue("1"), "name": api.NewTypedValue("abcdefghij")}}
		m := table.Model{}
		Expect(m.Set(data, []string{"1"})).To(Succeed())
		m.SetSize(120, 10)
		lines := strings.Split(ansi.Strip(m.View()), "\n")
		Expect(strings.TrimSpace(lines[1])).To(Equal("› 1  abc…"))
		Expect(ansi.StringWidth(lines[0])).To(Equal(9))
	})
	It("clips empty table messages in narrow terminals", func() {
		m := table.Model{}
		Expect(m.Set(api.NewEmptyTable([]api.ColumnDef{api.Column("id").Label("ID").Build()}), nil)).To(Succeed())
		m.SetSize(8, 3)
		for _, line := range strings.Split(m.View(), "\n") {
			Expect(ansi.StringWidth(line)).To(BeNumerically("<=", 8))
		}
	})
})
