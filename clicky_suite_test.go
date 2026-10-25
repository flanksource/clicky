package clicky_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestClicky(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clicky Suite")
}

func groupedHelpSection(output, heading string) string {
	_, section, found := strings.Cut(output, "\n"+heading+":\n")
	Expect(found).To(BeTrue(), "missing section %q in %s", heading, output)
	section, _, _ = strings.Cut(section, "\n\n")
	return section
}

var _ = Describe("grouped CLI help", func() {
	BeforeEach(func() {
		previous := *clicky.Flags
		DeferCleanup(func() { *clicky.Flags = previous })
	})

	It("merges inherited categories and preserves command flags and local overrides", func() {
		root := &cobra.Command{Use: "demo"}
		clicky.BindAllFlagsToCommand(root, "format", "tasks")
		root.PersistentFlags().String("context", "", "Database context")
		root.PersistentFlags().String("model", "", "Inherited model")
		Expect(root.PersistentFlags().SetAnnotation("model", clicky.FlagCategoryAnnotation, []string{"Model selection"})).To(Succeed())
		parent := &cobra.Command{Use: "prompt"}
		child := &cobra.Command{Use: "render [id]", Run: func(*cobra.Command, []string) {}}
		child.Flags().String("source", "", "Prompt source")
		child.Flags().String("model", "", "Local model")
		child.Flags().String("fallback", "", "Fallback model")
		child.Flags().Bool("secret", false, "Hidden flag")
		Expect(child.Flags().MarkHidden("secret")).To(Succeed())
		for _, name := range []string{"model", "fallback", "secret"} {
			Expect(child.Flags().SetAnnotation(name, clicky.FlagCategoryAnnotation, []string{"Model selection"})).To(Succeed())
		}
		child.Flags().Bool("pretty", false, "Local formatting override")
		Expect(child.Flags().SetAnnotation("pretty", clicky.FlagCategoryAnnotation, []string{"Formatting"})).To(Succeed())
		parent.AddCommand(child)
		root.AddCommand(parent)
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"prompt", "render", "--help"})
		Expect(root.Execute()).To(Succeed())
		help := output.String()
		Expect(help).To(ContainSubstring("demo prompt render [id]"))
		Expect(groupedHelpSection(help, "Command flags")).To(ContainSubstring("--source"))
		Expect(groupedHelpSection(help, "Model selection")).To(And(ContainSubstring("--fallback"), ContainSubstring("--model"), ContainSubstring("Local model")))
		Expect(groupedHelpSection(help, "Formatting")).To(And(ContainSubstring("--json"), ContainSubstring("Local formatting override")))
		Expect(groupedHelpSection(help, "Global flags")).To(ContainSubstring("--context"))
		Expect(help).NotTo(ContainSubstring("--secret"))
		Expect(help).NotTo(ContainSubstring("Inherited model"))
		Expect(strings.Count(help, "--model ")).To(Equal(1))
		Expect(strings.Count(help, "--pretty ")).To(Equal(1))
		var expected []string
		child.Flags().VisitAll(func(flag *pflag.Flag) {
			if !flag.Hidden {
				expected = append(expected, flag.Name)
			}
		})
		var actual []string
		for _, match := range regexp.MustCompile(`(?m)^\s+(?:-\w, )?--([a-z0-9-]+)(?:[ \t]|$)`).FindAllStringSubmatch(help, -1) {
			actual = append(actual, match[1])
		}
		Expect(actual).To(ConsistOf(expected))
		output.Reset()
		Expect(child.Help()).To(Succeed())
		Expect(output.String()).To(Equal(help))
		previous := -1
		for _, heading := range []string{"Command flags", "Model selection", "Formatting", "Logging", "Tasks", "Global flags"} {
			index := strings.Index(help, "\n"+heading+":\n")
			Expect(index).To(BeNumerically(">", previous))
			previous = index
		}
		Expect(help).NotTo(ContainSubstring("Model permissions:"))
	})

	It("sorts custom categories after known categories and hides empty categories", func() {
		command := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
		clicky.SetGroupedUsage(command)
		for _, category := range []string{"Zebra", "Alpha", "Model permissions", "Model runtime", "Model selection"} {
			command.Flags().Bool(strings.ReplaceAll(strings.ToLower(category), " ", "-"), false, category)
			name := strings.ReplaceAll(strings.ToLower(category), " ", "-")
			Expect(command.Flags().SetAnnotation(name, clicky.FlagCategoryAnnotation, []string{category})).To(Succeed())
		}
		command.Flags().Bool("hidden", false, "Hidden category")
		Expect(command.Flags().SetAnnotation("hidden", clicky.FlagCategoryAnnotation, []string{"Hidden"})).To(Succeed())
		Expect(command.Flags().MarkHidden("hidden")).To(Succeed())
		var output bytes.Buffer
		command.SetOut(&output)
		Expect(command.Help()).To(Succeed())
		help := output.String()
		previous := -1
		for _, category := range []string{"Model selection", "Model permissions", "Model runtime", "Alpha", "Zebra"} {
			index := strings.Index(help, "\n"+category+":\n")
			Expect(index).To(BeNumerically(">", previous))
			previous = index
		}
		Expect(help).NotTo(ContainSubstring("Hidden:"))
		Expect(help).NotTo(ContainSubstring("Global flags:"))
	})

	It("categorizes every flag registered by the standalone formatting binder", func() {
		command := &cobra.Command{Use: "format", Run: func(*cobra.Command, []string) {}}
		formatters.BindPFlags(command.Flags(), &formatters.FormatOptions{})
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			Expect(flag.Annotations[clicky.FlagCategoryAnnotation]).To(Equal([]string{"Formatting"}), flag.Name)
		})
	})

	It("groups typed command options without moving positional arguments", func() {
		root := &cobra.Command{Use: "demo"}
		clicky.SetGroupedUsage(root)
		child := clicky.AddNamedCommand("render", root, struct {
			File  string `args:"true"`
			Model string `flag:"model" category:"Model selection"`
		}{}, func(options struct {
			File  string `args:"true"`
			Model string `flag:"model" category:"Model selection"`
		}) (string, error) {
			return options.File, nil
		})
		Expect(child).NotTo(BeNil())
		child.Use = "render <file>"
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"render", "--help"})
		Expect(root.Execute()).To(Succeed())
		Expect(groupedHelpSection(output.String(), "Model selection")).To(ContainSubstring("--model"))
		Expect(output.String()).To(ContainSubstring("demo render <file>"))
	})
})
