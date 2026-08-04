package api

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ANSIText", func() {
	It("preserves SGR while removing active terminal controls", func() {
		value := ANSIText{Content: "\x1b[31mred\x1b[0m\x1b[2A\x1b[2J\x1b]8;;https://example.com\x07link\x1b]8;;\x07"}

		rendered := value.ANSI()

		Expect(rendered).To(ContainSubstring("\x1b[31mred\x1b[0m"))
		Expect(rendered).To(ContainSubstring("link"))
		Expect(rendered).NotTo(ContainSubstring("\x1b[2A"))
		Expect(rendered).NotTo(ContainSubstring("\x1b[2J"))
		Expect(rendered).NotTo(ContainSubstring("\x1b]8;"))
	})

	It("converts SGR to escaped HTML spans", func() {
		value := ANSIText{Content: "\x1b[1;38;2;12;34;56m<&>\x1b[0m"}

		rendered := value.HTML()

		Expect(rendered).To(ContainSubstring("font-weight:bold"))
		Expect(rendered).To(ContainSubstring("color:#0c2238"))
		Expect(rendered).To(ContainSubstring("&lt;&amp;&gt;"))
		Expect(rendered).NotTo(ContainSubstring("\x1b["))
		Expect(rendered).NotTo(ContainSubstring("<&>"))
	})

	It("returns independently renderable visible lines with carried styles", func() {
		value := ANSIText{Content: "\x1b[32mfirst\nsecond\x1b[0m\n\x1b[2J\nthird"}

		lines := value.VisibleLines()

		Expect(lines).To(HaveLen(3))
		Expect(lines[0].String()).To(Equal("first"))
		Expect(lines[1].String()).To(Equal("second"))
		Expect(lines[2].String()).To(Equal("third"))
		Expect(lines[0].ANSI()).To(ContainSubstring("\x1b[32m"))
		Expect(lines[1].ANSI()).To(ContainSubstring("\x1b[32m"))
		Expect(strings.Join([]string{lines[0].HTML(), lines[1].HTML()}, "")).To(ContainSubstring("color:#0dbc79"))
	})

	It("bounds multiline TextTree labels to terminal width", func() {
		long := strings.Repeat("x", 2_000)
		tree := TextTree{Node: Text{}.Append("fixture").NewLine().Add(ANSIText{Content: "\x1b[31m" + long + "\x1b[0m"})}

		output := tree.ANSI()

		Expect(output).To(ContainSubstring("fixture"))
		for _, line := range strings.Split(output, "\n") {
			Expect(lipgloss.Width(line)).To(BeNumerically("<", GetTerminalWidth()-1))
			Expect(line).NotTo(HaveSuffix(" "))
		}
	})
})
