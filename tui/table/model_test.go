package table_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/tui/table"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTable(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Reusable terminal table") }

type item struct {
	Name string
	SQL  string
}

func (item) Columns() []api.ColumnDef {
	return []api.ColumnDef{api.Column("name").Build(), api.Column("sql").Label("SQL").Build()}
}
func (r item) Row() map[string]any { return map[string]any{"name": r.Name, "sql": r.SQL} }
func (r item) RowDetail() api.Textable {
	return api.Text{}.Append("first\nsecond\nthird\nfourth\nfifth\nlast")
}

var _ = Describe("table state", func() {
	It("retains stable selection and freezes the sample while inspecting a scrollable detail", func() {
		m := table.Model{}
		Expect(m.Set(api.NewTableFrom([]item{{Name: "one"}, {Name: "two"}}), []string{"1", "2"})).To(Succeed())
		m.Key("down")
		Expect(m.Set(api.NewTableFrom([]item{{Name: "two"}, {Name: "one"}}), []string{"2", "1"})).To(Succeed())
		Expect(m.Selected).To(Equal("2"))
		m.Key("enter")
		Expect(m.Set(api.NewTableFrom([]item{{Name: "new"}}), []string{"3"})).To(Succeed())
		Expect(m.Selected).To(Equal("2"))
		for range 5 {
			m.Key("down")
		}
		Expect(m.View(80, 4)).To(ContainSubstring("last"))
	})
	It("fits Unicode rows and column headers to both requested terminal sizes", func() {
		m := table.Model{}
		Expect(m.Set(api.NewTableFrom([]item{{Name: "長い名前", SQL: strings.Repeat("select ", 40)}}), []string{"1"})).To(Succeed())
		for _, width := range []int{80, 120} {
			view := m.View(width, 24)
			Expect(view).To(ContainSubstring("SQL"))
			for _, line := range strings.Split(view, "\n") {
				Expect(ansi.StringWidth(line)).To(BeNumerically("<=", width))
			}
		}
	})
})
