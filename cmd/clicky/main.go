package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/tools/go/analysis/singlechecker"
	"gopkg.in/yaml.v3"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/lint"
	"github.com/flanksource/commons/logger"
)

// Build information (set by goreleaser)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var schemaFile string
	var options formatters.FormatOptions

	rootCmd := &cobra.Command{
		Use:   "clicky",
		Short: "A CLI tool for formatting structured data using YAML schema definitions",
		Long: `Clicky is a flexible CLI tool that formats structured data (JSON, YAML, etc.)
using YAML schema definitions. It supports multiple output formats including
pretty-printed tables, HTML, PDF, Markdown, and more.

For backward compatibility, you can use the root command directly, or use the
'pretty' subcommand explicitly.`,
		Example: `  clicky --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  clicky version`,
		Args: func(cmd *cobra.Command, args []string) error {
			// If no subcommand and no args, show help
			if len(args) == 0 && schemaFile == "" {
				return fmt.Errorf("requires either a subcommand or data files with --schema flag")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args but subcommands exist, user probably wants help
			if len(args) == 0 {
				return cmd.Help()
			}

			// Backward compatibility: behave like the old clicky
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required when using data files")
			}

			options.ResolveNoColor()
			options.Format = options.ResolveFormat()

			// Load schema directly into options
			parser := api.NewStructParser()
			schema, err := parser.LoadSchemaFromYAML(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}
			options.Schema = schema

			// Set verbose to true for CLI usage
			options.Verbose = true

			// Create format manager and format all files
			manager := formatters.NewFormatManager()
			for _, dataFile := range args {
				if err := formatDataFile(manager, dataFile, options); err != nil {
					return fmt.Errorf("error processing %s: %w", dataFile, err)
				}
			}

			return nil
		},
	}

	// Add flags to root command for backward compatibility
	rootCmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema")
	formatters.BindPFlags(rootCmd.Flags(), &options)

	// Add subcommands
	rootCmd.AddCommand(newPrettyCommand())
	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newSchemaCommand())
	rootCmd.AddCommand(newLintCommand())
	rootCmd.SetHelpCommand(newClickyHelpCommand())

	// Add OpenAPI and MCP commands using extensions
	extensions.CobraExtensions(rootCmd).
		OpenAPICommand().
		MCPCommand()

	return rootCmd
}

func newPrettyCommand() *cobra.Command {
	var schemaFile string
	var options formatters.FormatOptions

	cmd := &cobra.Command{
		Use:   "pretty [flags] <file> [file...]",
		Short: "Format data files using a YAML schema",
		Long: `Format structured data files (JSON, YAML, etc.) using a YAML schema definition.

The pretty command is the main functionality of clicky, allowing you to transform
raw data into beautifully formatted output using customizable schemas.

For the full pretty-printing API guide, run:
  clicky help pretty`,
		Example: `  clicky pretty --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  clicky pretty --schema product-schema.yaml --format csv products.json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required")
			}

			options.ResolveNoColor()
			options.Format = options.ResolveFormat()

			// Load schema directly into options
			parser := api.NewStructParser()
			schema, err := parser.LoadSchemaFromYAML(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}
			options.Schema = schema

			// Set verbose to true for CLI usage
			options.Verbose = true

			// Create format manager and format all files
			manager := formatters.NewFormatManager()
			for _, dataFile := range args {
				if err := formatDataFile(manager, dataFile, options); err != nil {
					return fmt.Errorf("error processing %s: %w", dataFile, err)
				}
			}

			return nil
		},
	}

	// Add schema flag
	cmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema (required)")
	if err := cmd.MarkFlagRequired("schema"); err != nil {
		panic(fmt.Sprintf("Failed to mark schema flag as required: %v", err))
	}

	// Add formatting flags using the new BindPFlags function
	formatters.BindPFlags(cmd.Flags(), &options)

	return cmd
}

func newClickyHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Use "clicky help pretty" for the full pretty-printing API guide.`,
		Args: cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 1 && args[0] == "pretty" {
				renderPrettyAPIHelp(cmd)
				return
			}

			target, _, err := cmd.Root().Find(args)
			if target == nil || err != nil {
				cmd.Printf("Unknown help topic %#q\n", args)
				cobra.CheckErr(cmd.Root().Usage())
				return
			}

			target.InitDefaultHelpFlag()
			target.InitDefaultVersionFlag()
			cobra.CheckErr(target.Help())
		},
	}
}

func renderPrettyAPIHelp(cmd *cobra.Command) {
	opts := formatters.FormatOptions{}
	opts.ResolveNoColor()

	prettyCmd, _, err := cmd.Root().Find([]string{"pretty"})
	if err != nil || prettyCmd == nil {
		prettyCmd = nil
	}
	if prettyCmd != nil {
		prettyCmd.InitDefaultHelpFlag()
		prettyCmd.InitDefaultVersionFlag()
	}

	out, err := clicky.Format(prettyHelpGuide{command: prettyCmd}, formatters.FormatOptions{
		Format:  "pretty",
		NoColor: opts.NoColor,
	})
	if err != nil {
		cobra.CheckErr(err)
		return
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
}

type prettyHelpGuide struct {
	command *cobra.Command
}

func (g prettyHelpGuide) Pretty() api.Text {
	return prettyAPIHelp(g.command)
}

func prettyAPIHelp(command *cobra.Command) api.Text {
	doc := clicky.Text("")
	doc = doc.Add(helpTitle("Clicky Pretty Printing")).NewLine()

	doc = doc.Add(helpSection("Intro")).NewLine()
	doc = doc.Add(helpText("Build terminal, Markdown, HTML, Slack, PDF, Excel, and web UI output from one Clicky render tree. Values stay structured until the formatter boundary, so tables, links, code blocks, badges, diagnostics, and trees can render appropriately for each output.")).NewLine()
	doc = doc.NewLine()
	if command != nil {
		doc = doc.Add(helpCode(command.UseLine())).NewLine()
	} else {
		doc = doc.Add(helpCode("clicky pretty [flags] <file> [file...]")).NewLine()
	}
	doc = doc.Add(helpBullet("Input files may be JSON or YAML. Multiple files are rendered in order.")).NewLine()
	doc = doc.Add(helpBullet("The schema controls labels, field order, styles, nested fields, table columns, tree options, and value formatting.")).NewLine()
	doc = doc.Add(helpBullet("The root command keeps the old form: ")).Add(helpCodeInline("clicky --schema schema.yaml data.json")).Append(".").NewLine()
	doc = doc.Add(helpBullet("Pretty-print schema-backed data:")).NewLine()
	doc = doc.Add(helpCode("clicky pretty --schema examples/order-schema.yaml examples/example-data.json")).NewLine()
	doc = doc.Add(helpBullet("Render stdout and write additional sinks in one pass:")).NewLine()
	doc = doc.Add(helpCode("clicky pretty --schema schema.yaml --format 'pretty,json=out.json,markdown=summary.md' data.yaml")).NewLine()
	doc = doc.Add(helpBullet("Filter table or tree rows with CEL:")).NewLine()
	doc = doc.Add(helpCode(`clicky pretty --schema schema.yaml --filter "status == 'active' && total > 100" data.json`)).NewLine()
	doc = doc.Add(helpBullet("Force a structure when automatic detection is not enough:")).NewLine()
	doc = doc.Add(helpCode("clicky pretty --schema schema.yaml --table data.json")).NewLine()
	doc = doc.Add(helpCode("clicky pretty --schema schema.yaml --tree data.json")).NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Flag Reference")).NewLine()
	if command != nil {
		doc = doc.Add(prettyFlagsTable(command.Flags())).NewLine()
		doc = doc.NewLine()
	}
	doc = doc.Add(formatSpecTable()).NewLine()
	doc = doc.Add(helpText("A bare format renders to stdout. ")).Add(helpCodeInline("format=file")).Append(" writes a file sink. Comma-separated specs may mix one stdout format with many file sinks.").NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Quickstart: Add Pretty() Method")).NewLine()
	doc = doc.Add(helpText("Implement ")).Add(helpCodeInline("Pretty() api.Text")).Append(" when a type owns its normal rich display. Compose Clicky values and return them; do not render to a string inside the method.").NewLine()
	doc = doc.Add(helpCodeBlock("go", `
type Service struct {
  Name   string
  Status string
  URL    string
}

func (s Service) Pretty() api.Text {
  return clicky.Text(s.Name, "font-bold").
    Space().
    Add(clicky.Badge(s.Status, "bg-green-100 text-green-800")).
    Space().
    Add(clicky.Link(s.URL).Append("open", "text-blue-600 underline"))
}

out, err := clicky.Format(Service{
  Name: "api", Status: "healthy", URL: "https://status.example.com",
}, clicky.FormatOptions{Format: "pretty"})
`)).NewLine()
	doc = doc.Add(helpText("Render with ")).Add(helpCodeInline("clicky.Format")).Append(", ").Add(helpCodeInline("clicky.MustPrint")).Append(", ").Add(helpCodeInline("clicky.FormatToFile")).Append(", or ").Add(helpCodeInline("clicky.PrintAndWriteSinks")).Append(".").NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Add Table Support")).NewLine()
	doc = doc.Add(helpText("For slices, implement ")).Add(helpCodeInline("PrettyRow(opts any) map[string]api.Text")).Append(" when each item should control its table cells. Use ").Add(helpCodeInline("api.TableMixin")).Append(" only when the type already owns headers and cells directly.").NewLine()
	doc = doc.Add(helpCodeBlock("go", `
type User struct {
  Name   string
  Role   string
  Active bool
}

func (u User) PrettyRow(opts any) map[string]api.Text {
  status := "disabled"
  style := "text-muted"
  if u.Active {
    status = "active"
    style = "text-green-600 font-bold"
  }
  return map[string]api.Text{
    "Name":   clicky.Text(u.Name, "font-bold"),
    "Role":   clicky.Text(u.Role),
    "Status": clicky.Text(status, style),
  }
}

clicky.MustPrint([]User{users[0], users[1]}, clicky.FormatOptions{Format: "pretty"})
clicky.MustPrint([]User{users[0], users[1]}, clicky.FormatOptions{Format: "markdown", Table: true})
`)).NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Tree Rendering")).NewLine()
	doc = doc.Add(helpText("Use ")).Add(helpCodeInline("api.TreeNode")).Append(" or ").Add(helpCodeInline("api.TreeMixin")).Append(" for hierarchical data. The same tree can render in the terminal, Markdown, static HTML, or the React Clicky document.").NewLine()
	doc = doc.Add(helpCodeBlock("go", `
type Folder struct {
  Name     string
  Children []Folder
}

func (f Folder) Pretty() api.Text {
  return clicky.Text(f.Name, "font-bold")
}

func (f Folder) GetChildren() []api.TreeNode {
  children := make([]api.TreeNode, 0, len(f.Children))
  for i := range f.Children {
    children = append(children, f.Children[i])
  }
  return children
}

clicky.MustPrint([]Folder{root}, clicky.FormatOptions{Format: "tree"})
`)).NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Clicky Components: Reference And Examples")).NewLine()
	doc = doc.Add(componentExamplesTable()).NewLine()
	doc = doc.Add(helpText("Web payloads: ")).Add(helpCodeInline("html-react")).Append(" and ").Add(helpCodeInline("clicky-json")).Append(" preserve the structured Clicky document consumed by ").Add(helpCodeInline("@flanksource/clicky-ui")).Append(".").NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Anti-patterns")).NewLine()
	doc = doc.Add(antiPatternTable()).NewLine()
	doc = doc.Add(clicky.Admonition(
		api.SeverityWarning,
		helpText("Render only at the boundary"),
		helpText("Call .ANSI(), .HTML(), .Markdown(), or .String() only when writing to an output sink. Calling them inside Pretty(), PrettyRow(), TreeNode.Pretty(), or component builders discards structure that other formatters need."),
	)).NewLine()
	doc = doc.NewLine()

	doc = doc.Add(helpSection("Struct Tags")).NewLine()
	doc = doc.Add(helpText("Struct tags mirror schema settings for reflection-based formatting. Use them when the Go type is the source of truth; use YAML schema files when the CLI is formatting external JSON or YAML.")).NewLine()
	doc = doc.Add(structTagReferenceTable()).NewLine()
	doc = doc.Add(helpText("Common field formats include ")).Add(helpCodeInline("currency")).Append(", ").Add(helpCodeInline("date")).Append(", ").Add(helpCodeInline("float")).Append(", ").Add(helpCodeInline("table")).Append(", ").Add(helpCodeInline("tree")).Append(", ").Add(helpCodeInline("struct")).Append(", ").Add(helpCodeInline("hide")).Append(", ").Add(helpCodeInline("compact")).Append(", and ").Add(helpCodeInline("short")).Append(".").NewLine()
	doc = doc.Add(helpText("Color options support exact matches and numeric comparisons such as ")).Add(helpCodeInline("green=>=36")).Append(", ").Add(helpCodeInline("yellow=>=24")).Append(", ").Add(helpCodeInline("red=<24")).Append(".").NewLine()
	doc = doc.Add(helpCodeBlock("go", `
type Order struct {
  ID      string  `+"`"+`pretty:"label=Order,style=text-blue-600 font-bold"`+"`"+`
  Status  string  `+"`"+`pretty:"label=Status,green=completed,yellow=processing,red=failed"`+"`"+`
  Total   float64 `+"`"+`pretty:"label=Total,format=currency,digits=2"`+"`"+`
  Lines   []Line  `+"`"+`pretty:"table,title=Line Items,header_style=bg-blue-50 font-bold"`+"`"+`
  Private string  `+"`"+`pretty:"hide"`+"`"+`
}
`)).NewLine()

	return doc
}

func componentExamplesTable() api.TextTable {
	table := helpTable("Component", "Example", "Use")
	for _, row := range []struct {
		name    string
		example string
		use     string
	}{
		{"clicky.Text", `clicky.Text("healthy", "text-green-600 font-bold")`, "Styled text with child nodes."},
		{"clicky.List", `clicky.List(clicky.Text("one"), clicky.Text("two"))`, "Inline or vertical lists."},
		{"clicky.TextList", `clicky.TextList(clicky.Text("one"), clicky.Text("two"))`, "Join existing Textable values."},
		{"clicky.KeyValue", `clicky.KeyValue("status", "active")`, "Compact key/value pairs."},
		{"clicky.Map", `clicky.Map(map[string]string{"env": "prod"})`, "Sorted description list from a map."},
		{"clicky.Table", `clicky.Table("Name", "Status")`, "Explicit rich tables."},
		{"clicky.Tree", `clicky.Tree(clicky.Text("root"))`, "Explicit tree values."},
		{"clicky.CodeBlock", `clicky.CodeBlock("go", source)`, "Syntax-highlighted code and Markdown fences."},
		{"clicky.Link", `clicky.Link("/orders/1").Append("ORD-1")`, "Structured links."},
		{"clicky.LinkCommand", `clicky.LinkCommand("kubectl get pods").Append("run")`, "Command links/actions for UI surfaces."},
		{"clicky.Button", `clicky.Button("Open", "/orders")`, "Platform-neutral action button."},
		{"clicky.ButtonGroup", `clicky.ButtonGroup(open, cancel)`, "Grouped actions."},
		{"clicky.Badge", `clicky.Badge("active", "bg-green-100 text-green-800")`, "Single-value pill."},
		{"clicky.LabelBadge", `clicky.LabelBadge("Status", "active")`, "Label/value pill."},
		{"clicky.Collapsed", `clicky.Collapsed("Details", detail)`, "Expandable details in HTML; direct content in terminal."},
		{"clicky.Admonition", `clicky.Admonition(api.SeverityWarning, nil, msg)`, "Note, info, tip, warning, and danger callouts."},
		{"clicky.Diff", `clicky.Diff(before, after, "old", "new")`, "Unified diffs with terminal and HTML styling."},
		{"clicky.StackTrace", `clicky.StackTrace(trace, clicky.WithMaxStackFrames(20))`, "Stack traces with source context."},
		{"clicky.Comment", `clicky.Comment("generated")`, "HTML/Markdown comments."},
		{"clicky.HTMLElement", `clicky.HTMLElement("span", "raw")`, "Custom HTML with text fallback."},
		{"clicky.ClickyText", `clicky.ClickyText(detail)`, "JSON field wrapper for Clicky document payloads."},
	} {
		table.Rows = append(table.Rows, helpRow(
			helpCodeInline(row.name),
			helpCodeInline(row.example),
			helpText(row.use),
		))
	}
	return table
}

func antiPatternTable() api.TextTable {
	table := helpTable("Avoid", "Use Instead")
	for _, row := range []struct {
		avoid string
		use   string
	}{
		{"Calling .ANSI(), .HTML(), .Markdown(), or .String() inside Pretty().", "Return clicky.Text(...) or another api.Textable and let clicky.Format render it."},
		{"Using fmt.Sprintf to align columns or draw tables.", "Return PrettyRow, TableMixin, or clicky.Table(...)."},
		{"Writing to stdout or stderr from Pretty(), PrettyRow(), or TreeNode.Pretty().", "Return structured values; write only in command/output code."},
		{"Embedding raw terminal escape sequences.", "Use Tailwind-like styles such as text-green-600, font-bold, bg-blue-50."},
		{"Flattening links, badges, diffs, or code blocks into plain strings.", "Use clicky.Link, clicky.Badge, clicky.Diff, clicky.CodeBlock, and other components."},
		{"Duplicating CLI flag docs by hand.", "Read Cobra flags live when building command help."},
	} {
		table.Rows = append(table.Rows, helpRow(helpText(row.avoid), helpText(row.use)))
	}
	return table
}

func structTagReferenceTable() api.TextTable {
	table := helpTable("Tag", "Effect")
	for _, row := range []struct {
		tag    string
		effect string
	}{
		{"label=Order", "Display label."},
		{"style=text-blue-600 font-bold", "Value style."},
		{"label_style=text-muted", "Label style."},
		{"format=currency,digits=2", "Formatter hint and formatter option."},
		{"table,title=Line Items", "Render a slice as a table and set the table title."},
		{"sort=total,dir=desc", "Sort table rows by a field."},
		{"header_style=bg-blue-50 font-bold", "Table header style."},
		{"row_style=bg-gray-50", "Table row style."},
		{"tree,max_depth=3,no_icons", "Render as tree with tree options."},
		{"short", "Use PrettyShort() for compact cell rendering."},
		{"compact", "Compact nested items."},
		{"hide", "Hide the field."},
		{"green=completed,red=failed", "Value-based color options."},
	} {
		table.Rows = append(table.Rows, helpRow(helpCodeInline(row.tag), helpText(row.effect)))
	}
	return table
}

func prettyFlagsTable(flags *pflag.FlagSet) api.TextTable {
	table := helpTable("Flag", "Purpose")
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		purpose := flag.Usage
		if _, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; ok {
			purpose = "Required. " + purpose
		}
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "0" {
			purpose += fmt.Sprintf(" Default: %s.", flag.DefValue)
		}

		table.Rows = append(table.Rows, helpRow(
			helpCodeInline(flagUsage(flag)),
			helpText(purpose),
		))
	})
	return table
}

func flagUsage(flag *pflag.Flag) string {
	name := "--" + flag.Name
	if flag.Shorthand != "" {
		name = "-" + flag.Shorthand + ", " + name
	}
	if flag.Value != nil && flag.Value.Type() != "bool" {
		name += " " + flag.Value.Type()
	}
	return name
}

func formatSpecTable() api.TextTable {
	table := helpTable("Format", "Use")
	for _, row := range []struct {
		format string
		use    string
	}{
		{"pretty", "ANSI terminal output from Clicky Textable/PrettyData values."},
		{"json, yaml/yml", "Machine-readable data output."},
		{"csv", "Flat table output for spreadsheets and shell pipelines."},
		{"markdown/md", "Markdown tables, code fences, links, lists, and callouts."},
		{"html", "Interactive HTML output."},
		{"html-static", "Static HTML without JavaScript dependencies."},
		{"html-react", "React/Clicky document payload for web UI rendering."},
		{"clicky-json", "Structured Clicky document JSON."},
		{"pdf", "PDF output from the static HTML renderer."},
		{"slack", "Slack Block Kit JSON."},
		{"excel/xlsx", "Excel workbook output; use a file sink."},
		{"tree", "Tree renderer output."},
	} {
		table.Rows = append(table.Rows, helpRow(helpCodeInline(row.format), helpText(row.use)))
	}
	return table
}

func helpTable(headers ...string) api.TextTable {
	table := clicky.Table()
	for i, header := range headers {
		table.Headers = append(table.Headers, clicky.Text(header, "font-bold text-cyan-600"))
		table.FieldNames = append(table.FieldNames, fmt.Sprintf("col%d", i))
	}
	return table
}

func helpRow(cells ...api.Textable) api.TableRow {
	row := api.TableRow{}
	for i, cell := range cells {
		row[fmt.Sprintf("col%d", i)] = api.NewTypedValue(cell)
	}
	return row
}

func helpTitle(text string) api.Text {
	return clicky.Text(text, "font-bold text-blue-600")
}

func helpSection(text string) api.Text {
	return clicky.Text(text, "font-bold text-cyan-600")
}

func helpText(text string) api.Text {
	return clicky.Text(text)
}

func helpBullet(text string) api.Text {
	return clicky.Text("- ", "text-muted").Append(text)
}

func helpCode(text string) api.Text {
	return clicky.Text("  "+text, "font-mono text-yellow-600")
}

func helpCodeInline(text string) api.Text {
	return clicky.Text(text, "font-mono text-yellow-600")
}

func helpCodeBlock(language, text string) api.Code {
	return clicky.CodeBlock(language, strings.TrimSpace(text))
}

func newLintCommand() *cobra.Command {
	helpOpts := defaultLintCLIOptions()
	cmd := &cobra.Command{
		Use:   "lint [flags] [packages...]",
		Short: "Run the clicky API linter on Go packages (defaults to ./...)",
		Long: `Run the clicky API linter (clickylint) on the given Go packages.

Detects bad usage patterns of the clicky text API, such as incorrect
composite literal usage of api.Text, direct stdout writes, and calling
.ANSI()/.HTML()/.Markdown()/.String() inside Pretty/render-builder code.

By default, clicky lint renders a colored tree summary grouped by rule and
affected file. Use --format json for structured output, or --raw / -- for the
underlying go/analysis driver flags (-json, -fix, -c=N, etc.).

If no packages are given, defaults to ./... in the current directory.`,
		Example: `  clicky lint ./...
  clicky lint ./pkg/foo ./pkg/bar
  clicky lint --summary-limit 2 ./...
  clicky lint --format json ./...
  clicky lint -- -json ./...`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rawArgs, raw := rawLintArgs(args); raw {
				return runRawLint(rawArgs)
			}
			if lintHelpRequested(args) {
				return cmd.Help()
			}

			opts, packages, err := parseLintArgs(args)
			if err != nil {
				return err
			}
			result, err := lint.Run(lint.RunOptions{
				Packages:     packages,
				IncludeTests: true,
			})
			if err != nil {
				return err
			}
			if err := renderLintResult(cmd, result, opts); err != nil {
				return err
			}
			if result.HasIssues() {
				return lintExitError{result: result}
			}
			return nil
		},
	}
	bindLintFlags(cmd.Flags(), &helpOpts)
	return cmd
}

type lintCLIOptions struct {
	Format       string
	NoColor      bool
	Raw          bool
	SummaryLimit int
}

type lintExitError struct {
	result *lint.Result
}

func (e lintExitError) Error() string {
	if e.result == nil {
		return "clickylint found issues"
	}
	parts := []string{}
	if count := len(e.result.Violations); count > 0 {
		parts = append(parts, fmt.Sprintf("%d violations", count))
	}
	if count := len(e.result.Errors); count > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", count))
	}
	return "clickylint found " + strings.Join(parts, ", ")
}

func defaultLintCLIOptions() lintCLIOptions {
	return lintCLIOptions{
		Format:       "pretty",
		SummaryLimit: 5,
	}
}

func bindLintFlags(flags *pflag.FlagSet, opts *lintCLIOptions) {
	flags.StringVar(&opts.Format, "format", opts.Format, "Output format: pretty or json")
	flags.BoolVar(&opts.NoColor, "no-color", opts.NoColor, "Disable ANSI color output")
	flags.BoolVar(&opts.Raw, "raw", opts.Raw, "Use the raw go/analysis singlechecker driver")
	flags.IntVar(&opts.SummaryLimit, "summary-limit", opts.SummaryLimit, "Maximum file locations to show per rule")
}

func parseLintArgs(args []string) (lintCLIOptions, []string, error) {
	opts := defaultLintCLIOptions()
	flags := pflag.NewFlagSet("lint", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bindLintFlags(flags, &opts)
	if err := flags.Parse(args); err != nil {
		return opts, nil, err
	}
	if opts.Raw {
		return opts, nil, fmt.Errorf("--raw must be handled before lint flag parsing")
	}
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "pretty"
	}
	switch opts.Format {
	case "pretty", "json":
	default:
		return opts, nil, fmt.Errorf("unsupported lint format %q (expected pretty or json)", opts.Format)
	}
	packages := flags.Args()
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	return opts, packages, nil
}

func renderLintResult(cmd *cobra.Command, result *lint.Result, opts lintCLIOptions) error {
	switch opts.Format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		formatOpts := formatters.FormatOptions{
			Format:  "tree",
			NoColor: opts.NoColor,
		}
		formatOpts.ResolveNoColor()
		out, err := clicky.Format(lint.NewSummaryView(result, opts.SummaryLimit), formatOpts)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), out)
		return err
	}
}

func lintHelpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func rawLintArgs(args []string) ([]string, bool) {
	if len(args) > 0 && args[0] == "--" {
		return defaultRawLintPackages(args[1:]), true
	}
	for i, arg := range args {
		if arg == "--" {
			return defaultRawLintPackages(args[i+1:]), true
		}
		if arg == "--raw" {
			rawArgs := append([]string{}, args[:i]...)
			rawArgs = append(rawArgs, args[i+1:]...)
			if len(rawArgs) > 0 && rawArgs[0] == "--" {
				rawArgs = rawArgs[1:]
			}
			return defaultRawLintPackages(rawArgs), true
		}
		if strings.HasPrefix(arg, "--raw=") {
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--raw="))
			if value == "true" || value == "1" {
				rawArgs := append([]string{}, args[:i]...)
				rawArgs = append(rawArgs, args[i+1:]...)
				if len(rawArgs) > 0 && rawArgs[0] == "--" {
					rawArgs = rawArgs[1:]
				}
				return defaultRawLintPackages(rawArgs), true
			}
		}
		if isAnalyzerDriverFlag(arg) {
			return defaultRawLintPackages(args), true
		}
	}
	return nil, false
}

func defaultRawLintPackages(args []string) []string {
	if len(args) == 0 {
		return []string{"./..."}
	}
	return args
}

func isAnalyzerDriverFlag(arg string) bool {
	if arg == "" || arg == "-h" || arg == "--help" {
		return false
	}
	if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
		return true
	}
	if !strings.HasPrefix(arg, "--") {
		return false
	}
	name := strings.TrimPrefix(arg, "--")
	if idx := strings.Index(name, "="); idx >= 0 {
		name = name[:idx]
	}
	switch name {
	case "c", "context", "cpuprofile", "debug", "diff", "fix", "flags", "json", "memprofile", "test", "trace", "V":
		return true
	default:
		return false
	}
}

func runRawLint(args []string) error {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = append([]string{"clickylint"}, args...)
	singlechecker.Main(lint.Analyzer)
	return nil
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getVersionInfo())
		},
	}
}

func getVersionInfo() string {
	return fmt.Sprintf("clicky-schema %s (commit: %s, built: %s, go: %s)",
		version, commit, date, runtime.Version())
}

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema documentation and utilities",
		Long: `Work with clicky schema files - validate, generate examples, and view documentation.

The schema command provides tools for understanding and working with clicky's YAML schema format.`,
	}

	// Add subcommands
	cmd.AddCommand(newSchemaHelpCommand())
	cmd.AddCommand(newSchemaValidateCommand())
	cmd.AddCommand(newSchemaExampleCommand())

	return cmd
}

func newSchemaHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help",
		Short: "Show detailed schema documentation",
		Long:  `Display comprehensive documentation about the clicky schema format, including all available fields, types, and options.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getSchemaDocumentation())
		},
	}
}

func newSchemaValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a schema file",
		Long:  `Check if a schema file is valid and report any errors or warnings.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaFile := args[0]

			// Try to load the schema
			_, err := formatters.LoadSchemaFromYAML(schemaFile)
			if err != nil {
				return fmt.Errorf("schema validation failed: %w", err)
			}

			fmt.Printf("✓ Schema file '%s' is valid\n", schemaFile)
			return nil
		},
	}
}

func newSchemaExampleCommand() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "example",
		Short: "Generate an example schema file",
		Long:  `Generate a comprehensive example schema file demonstrating all available features and options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			example := getExampleSchema()

			if outputFile != "" {
				// Write to file
				err := os.WriteFile(outputFile, []byte(example), 0o644)
				if err != nil {
					return fmt.Errorf("failed to write example schema: %w", err)
				}
				fmt.Printf("Example schema written to %s\n", outputFile)
			} else {
				// Print to stdout
				fmt.Println(example)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for the example schema")

	return cmd
}

func getSchemaDocumentation() string {
	return `CLICKY SCHEMA DOCUMENTATION
===========================

The clicky schema is a YAML file that defines how to format and display structured data.

## Basic Structure

fields:
  - name: "field_name"        # Required: name of the field in your data
    type: "string"             # Optional: field type (string, int, float, boolean, struct, array)
    format: "format_type"      # Optional: special formatting (currency, date, table, etc.)
    style: "tailwind_classes"  # Optional: Tailwind CSS classes for styling
    label: "Display Label"     # Optional: custom label for display

## Field Types

- string: Text values
- int: Integer numbers
- float: Decimal numbers
- boolean: true/false values
- struct: Nested objects
- array: Lists of items
- date: Date/time values

## Format Types

- currency: Format as currency (e.g., $1,234.56)
- date: Format as date/time
- float: Format with specific decimal places
- table: Display array as a table
- tree: Display as a tree structure

## Styling

Use Tailwind CSS classes for styling:
- Colors: text-red-500, bg-blue-100
- Typography: font-bold, italic, uppercase
- Spacing: px-2, py-1
- Decorations: underline, line-through

## Color Options

Dynamic coloring based on values:

color_options:
  green: "completed"    # Use green when value is "completed"
  red: "failed"         # Use red when value is "failed"
  yellow: ">= 50"       # Use yellow for numeric comparisons

## Table Options

For array fields with format: "table":

table_options:
  title: "Table Title"
  header_style: "bg-blue-50 font-bold"
  fields:
    - name: "column1"
      type: "string"
      style: "text-gray-700"

## Format Options

Additional formatting parameters:

format_options:
  format: "epoch"       # For dates: parse from epoch timestamp
  digits: "2"           # For floats: decimal places
  sort: "field_name"    # For tables: sort by field
  dir: "desc"           # Sort direction: asc/desc

## Nested Fields

For struct types, define nested fields:

fields:
  - name: "address"
    type: "struct"
    fields:
      - name: "street"
        type: "string"
      - name: "city"
        type: "string"

## Example Usage

clicky --schema my-schema.yaml data.json
clicky pretty --schema my-schema.yaml --format html data.json
clicky schema validate my-schema.yaml
clicky schema example -o example-schema.yaml
`
}

func getExampleSchema() string {
	return `# Example Clicky Schema
# This demonstrates all available features

fields:
  # Simple string field with styling
  - name: "id"
    type: "string"
    style: "text-blue-600 font-bold"
    label: "Order ID"

  # Nested struct field
  - name: "customer"
    type: "struct"
    style: "text-gray-700"
    fields:
      - name: "name"
        type: "string"
        style: "font-semibold"
      - name: "email"
        type: "string"
        style: "text-blue-500 underline"
      - name: "account_type"
        type: "string"
        style: "uppercase"
        color_options:
          green: "premium"
          yellow: "standard"
          gray: "basic"

  # Field with dynamic coloring
  - name: "status"
    type: "string"
    style: "font-bold uppercase"
    color_options:
      green: "completed"
      yellow: "processing"
      orange: "pending"
      red: "canceled"

  # Currency field
  - name: "total_amount"
    type: "float"
    format: "currency"
    style: "text-green-600 font-bold text-lg"

  # Date field with epoch format
  - name: "order_date"
    type: "string"
    format: "date"
    style: "text-indigo-600"
    format_options:
      format: "epoch"

  # Array displayed as table
  - name: "items"
    type: "array"
    format: "table"
    format_options:
      sort: "price"
      dir: "desc"
    table_options:
      title: "Order Items"
      header_style: "bg-blue-50 text-blue-900 font-bold uppercase"
      fields:
        - name: "product_name"
          type: "string"
          style: "font-medium"
        - name: "quantity"
          type: "int"
          style: "text-center"
        - name: "price"
          type: "float"
          format: "currency"
          style: "text-green-600"
        - name: "discount"
          type: "float"
          format: "float"
          format_options:
            digits: "1"
          style: "text-red-500"
        - name: "warranty_months"
          type: "int"
          color_options:
            green: ">=36"
            yellow: ">=24"
            red: "<24"

  # Field with multiple style classes
  - name: "alert_message"
    type: "string"
    style: "uppercase text-red-600 bg-red-100 font-bold underline px-2 py-1 rounded"

  # Boolean field
  - name: "is_expedited"
    type: "boolean"
    style: "font-semibold"
    color_options:
      green: "true"
      gray: "false"

  # Tree structure field
  - name: "category_tree"
    type: "struct"
    format: "tree"
    tree_options:
      label_field: "name"
      children_field: "subcategories"
      style: "text-gray-700"
`
}

// formatDataFile loads a data file and formats it using the provided options
func formatDataFile(manager *formatters.FormatManager, dataFile string, options formatters.FormatOptions) error {
	// Load data file based on extension
	data, err := loadDataFile(dataFile)
	if err != nil {
		return fmt.Errorf("failed to load data file: %w", err)
	}

	var output string
	// Check if schema-aware formatting is needed
	if options.Schema != nil {
		// Use schema-aware formatting
		parser := api.NewStructParser()
		prettyData, err := parser.ParseDataWithSchema(data, options.Schema)
		if err != nil {
			return fmt.Errorf("failed to parse data with schema: %w", err)
		}
		output, err = manager.FormatWithSchema(prettyData, options)
		if err != nil {
			return fmt.Errorf("failed to format schema data: %w", err)
		}
	} else {
		// Use regular formatting
		output, err = manager.FormatWithOptions(options, data)
		if err != nil {
			return fmt.Errorf("failed to format data: %w", err)
		}
	}

	switch options.Output {
	case "", "stdout", "-", "/dev/stdout":
		// Output to stdout
		fmt.Print(output)
	case "stderr", "/dev/stderr":
		// Output to stderr
		fmt.Fprint(os.Stderr, output)
	default:
		// Create output file path
		outputFile := options.Output
		if strings.Contains(options.Output, "*") || filepath.Ext(options.Output) == "" {
			// Pattern or directory - generate filename
			base := strings.TrimSuffix(filepath.Base(dataFile), filepath.Ext(dataFile))
			ext := getOutputExtension(options.Format)
			if strings.Contains(options.Output, "*") {
				outputFile = strings.ReplaceAll(options.Output, "*", base)
			} else {
				outputFile = filepath.Join(options.Output, base+ext)
			}
		}

		// Ensure output directory exists
		if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Write to file
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		logger.Tracef("Output written to %s\n", outputFile)
	}

	return nil
}

// loadDataFile loads a data file based on its extension
func loadDataFile(filename string) (interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		var jsonData interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return jsonData, nil
	case ".yaml", ".yml":
		var yamlData interface{}
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		return yamlData, nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// getOutputExtension returns the file extension for a given format
func getOutputExtension(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yaml"
	case "csv":
		return ".csv"
	case "html":
		return ".html"
	case "markdown", "md":
		return ".md"
	case "pdf":
		return ".pdf"
	default:
		return ".txt"
	}
}
