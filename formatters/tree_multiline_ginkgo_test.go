package formatters

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type multilineTreeNode struct {
	label    api.Text
	children []api.TreeNode
}

func (n multilineTreeNode) Pretty() api.Text            { return n.label }
func (n multilineTreeNode) GetChildren() []api.TreeNode { return n.children }

var _ = Describe("TreeFormatter multiline labels", func() {
	It("renders Textable children and bounds every physical line", func() {
		long := strings.Repeat("x", 2_000)
		root := multilineTreeNode{
			label: api.Text{}.Append("fixture").NewLine().Add(api.ANSIText{Content: "\x1b[31m" + long + "\x1b[0m"}),
		}
		formatter := NewTreeFormatter(api.DefaultTheme(), false, nil)

		output := formatter.FormatTreeFromRoot(root)

		Expect(output).To(ContainSubstring("fixture"))
		Expect(output).To(ContainSubstring("\x1b[31m"))
		for _, line := range strings.Split(output, "\n") {
			Expect(lipgloss.Width(line)).To(BeNumerically("<", api.GetTerminalWidth()-1))
			Expect(line).NotTo(HaveSuffix(" "))
		}
	})
})
