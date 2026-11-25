package clicky_test

import (
	"bytes"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("colored CLI help", func() {
	var root *cobra.Command
	var child *cobra.Command
	var output bytes.Buffer
	var theme clicky.HelpTheme

	BeforeEach(func() {
		output.Reset()
		root = &cobra.Command{Use: "demo", Short: "Demo short description", Long: "Demo long description", Example: `  demo render "manifest.yaml"`}
		child = &cobra.Command{Use: "render <file>", Short: "Render a file", Run: func(*cobra.Command, []string) {}}
		root.AddCommand(child)
		root.PersistentFlags().Bool("no-color", false, "Disable colored output")
		root.Flags().StringP("source", "s", "input.yaml", "Read from `file`\nAccepts YAML")
		root.SetOut(&output)
		theme = clicky.DefaultHelpTheme()
		clicky.SetColoredHelp(root, theme)
	})

	It("renders semantic styles while retaining pflag usage and command metadata", func() {
		help := clicky.CommandHelp{Command: root, Theme: theme}
		document := help.Pretty()
		Expect(clicky.Format(help, clicky.FormatOptions{Format: "pretty"})).To(Equal(document.ANSI()))
		Expect(document.String()).To(Equal("Demo long description\n\n" + root.UsageString()))
		Expect(document.String()).To(And(
			ContainSubstring("demo [command]"),
			ContainSubstring("-s, --source file"),
			ContainSubstring(`Accepts YAML (default "input.yaml")`),
			ContainSubstring("Render a file"),
		))
		Expect(document.HTML()).To(And(
			ContainSubstring(clicky.Text("Usage:", theme.Heading).HTML()),
			ContainSubstring(clicky.Text("demo [command]", theme.Usage).HTML()),
			ContainSubstring(clicky.Text("render", theme.Command).HTML()),
			ContainSubstring(clicky.Text("  -s, --source", theme.Flag).HTML()),
			ContainSubstring("color:"),
		))
		Expect(document.Markdown()).To(ContainSubstring("```bash\n" + root.Example + "\n```"))
		rendered := document.ANSI()
		Expect(rendered).To(MatchRegexp("\x1b\\[3m\x1b\\[38;2;[0-9;]+m\"manifest\\.yaml\""))
		Expect(rendered).To(ContainSubstring(clicky.Text(`"input.yaml"`, "text-amber-600 italic").ANSI()))
		Expect(document.HTML()).To(MatchRegexp(`font-style:\s*italic[^>]*>[^<]*manifest\.yaml`))
		Expect(ansi.Strip(document.ANSI())).To(Equal(document.String()))
	})

	It("wraps long flag descriptions at the terminal width with a hanging indent", func() {
		const width = 50
		const flagPrefix = "      --target   "
		const description = "Write the report to a temporary directory before copying it to the final destination."
		api.SetTerminalWidth(width)
		DeferCleanup(api.InvalidateTerminalSize)
		child.Flags().Bool("target", false, description)
		help := clicky.CommandHelp{Command: child, Theme: theme}.Pretty().String()
		lines := strings.Split(groupedHelpSection(help, "Command flags"), "\n")
		Expect(len(lines)).To(BeNumerically(">", 1))
		Expect(lines[0]).To(HavePrefix(flagPrefix))
		for _, line := range lines[1:] {
			Expect(len(line) - len(strings.TrimLeft(line, " "))).To(Equal(len(flagPrefix)))
		}
		for _, line := range lines {
			Expect(len(line)).To(BeNumerically("<=", width))
		}
		Expect(strings.Join(strings.Fields(strings.Join(lines, "\n")[len(flagPrefix):]), " ")).To(Equal(description))
	})

	DescribeTable("places the styled shortcuts below the full format description",
		func(width, indent int) {
			api.SetTerminalWidth(width)
			DeferCleanup(api.InvalidateTerminalSize)
			formatters.BindPFlags(child.Flags(), &formatters.FormatOptions{})
			document := clicky.CommandHelp{Command: child, Theme: theme}.Pretty()
			const shortcuts = "--csv --html --json --llm --markdown --pdf --pretty --slack --yaml"
			formatting := groupedHelpSection(document.String(), "Formatting")
			shortcutLine := strings.Repeat(" ", indent) + "Shortcuts: " + shortcuts
			Expect(formatting).To(MatchRegexp(`(?m)^ +--format string +Output format\.[^\n]*(?:\n {24}[^\n]+)+\n` + shortcutLine + `\n +--no-color`))
			Expect(formatting).To(And(
				ContainSubstring("--format string"),
				ContainSubstring("--table"),
				ContainSubstring("--tree"),
				Not(MatchRegexp(`(?m)^ +--(?:csv|html|json|llm|markdown|pdf|pretty|slack|yaml)\b`)),
			))
			Expect(document.ANSI()).To(And(
				ContainSubstring(clicky.Text(shortcuts, theme.Flag).ANSI()),
				ContainSubstring("("+clicky.Text("pretty", theme.Value).ANSI()+" |"),
				ContainSubstring(clicky.Text("tree", theme.Value).ANSI()+")"),
			))
		},
		Entry("at 120 columns, aligned with the description", 120, 24),
		Entry("at 80 columns, aligned to fit the shortcuts on one line", 80, 2),
	)

	It("wraps the shortcuts between names when they exceed a narrow terminal", func() {
		const width = 50
		const hangingIndent = 2 + len("Shortcuts: ")
		api.SetTerminalWidth(width)
		DeferCleanup(api.InvalidateTerminalSize)
		formatters.BindPFlags(child.Flags(), &formatters.FormatOptions{})
		formatting := groupedHelpSection(clicky.CommandHelp{Command: child, Theme: theme}.Pretty().String(), "Formatting")
		var names, shortcutLines []string
		for _, line := range strings.Split(formatting, "\n") {
			if rest, found := strings.CutPrefix(line, "  Shortcuts: "); found {
				shortcutLines = append(shortcutLines, line)
				names = strings.Fields(rest)
			} else if len(shortcutLines) > 0 && strings.HasPrefix(line, strings.Repeat(" ", hangingIndent)+"--") {
				shortcutLines = append(shortcutLines, line)
				names = append(names, strings.Fields(line)...)
			}
		}
		Expect(names).To(Equal(strings.Fields("--csv --html --json --llm --markdown --pdf --pretty --slack --yaml")), formatting)
		Expect(len(shortcutLines)).To(BeNumerically(">", 1), formatting)
		for _, line := range shortcutLines {
			Expect(len(line)).To(BeNumerically("<=", width), line)
		}
	})

	It("colors --help for an interactive destination while keeping UsageString plain", func() {
		theme.Heading = "text-purple-600 font-bold"
		clicky.SetColoredHelp(root, theme)
		GinkgoT().Setenv("CLICKY_FORCE_INTERACTIVE", "1")
		GinkgoT().Setenv("NO_COLOR", "")
		GinkgoT().Setenv("COLOR", "")
		GinkgoT().Setenv("TERM", "xterm")
		root.SetArgs([]string{"render", "--help"})
		Expect(root.Execute()).To(Succeed())
		help := output.String()
		Expect(help).To(ContainSubstring(clicky.Text("Usage:", theme.Heading).ANSI()))
		Expect(help).To(ContainSubstring("\x1b["))
		Expect(child.UsageString()).NotTo(ContainSubstring("\x1b["))
		Expect(ansi.Strip(help)).To(Equal("Render a file\n\n" + child.UsageString()))
	})

	DescribeTable("prints plain help when",
		func(force, noColor string, args []string) {
			GinkgoT().Setenv("CLICKY_FORCE_INTERACTIVE", force)
			GinkgoT().Setenv("NO_COLOR", noColor)
			GinkgoT().Setenv("TERM", "xterm")
			root.SetArgs(args)
			Expect(root.Execute()).To(Succeed())
			Expect(output.String()).To(And(ContainSubstring("Usage:"), Not(ContainSubstring("\x1b["))))
		},
		Entry("output is captured", "", "", []string{"--help"}),
		Entry("NO_COLOR is set", "1", "1", []string{"--help"}),
		Entry("the inherited --no-color flag is set", "1", "", []string{"render", "--no-color", "--help"}),
	)
})
