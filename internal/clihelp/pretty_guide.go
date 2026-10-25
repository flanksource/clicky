// Package clihelp holds the long-form help content of the clicky CLI: the
// pretty-printing API guide and the schema documentation/example.
package clihelp

import (
	"fmt"
	"io"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"github.com/spf13/cobra"
)

// PrettyGuide renders the pretty-printing API guide; Command supplies the live
// usage line and flags of the `pretty` command when it is present.
type PrettyGuide struct {
	Command *cobra.Command
}

// RenderPrettyGuide writes the guide for root's `pretty` subcommand to w.
func RenderPrettyGuide(w io.Writer, root *cobra.Command) error {
	opts := formatters.FormatOptions{}
	opts.ResolveNoColor()

	prettyCmd, _, err := root.Find([]string{"pretty"})
	if err != nil {
		prettyCmd = nil
	}
	if prettyCmd != nil {
		prettyCmd.InitDefaultHelpFlag()
		prettyCmd.InitDefaultVersionFlag()
	}

	out, err := clicky.Format(PrettyGuide{Command: prettyCmd}, formatters.FormatOptions{
		Format:  "pretty",
		NoColor: opts.NoColor,
	})
	if err != nil {
		return err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_, err = fmt.Fprint(w, out)
	return err
}

func (g PrettyGuide) Pretty() api.Text {
	doc := clicky.Text("")
	doc = doc.Add(helpTitle("Clicky Pretty Printing")).NewLine()
	doc = addIntro(doc, g.Command)
	doc = addFlagReference(doc, g.Command)
	doc = addRenderingGuide(doc)
	return addComponentGuide(doc)
}

func addIntro(doc api.Text, command *cobra.Command) api.Text {
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
	return doc.NewLine()
}

func addFlagReference(doc api.Text, command *cobra.Command) api.Text {
	doc = doc.Add(helpSection("Flag Reference")).NewLine()
	if command != nil {
		doc = doc.Add(prettyFlagsTable(command.Flags())).NewLine()
		doc = doc.NewLine()
	}
	doc = doc.Add(formatSpecTable()).NewLine()
	doc = doc.Add(helpText("A bare format renders to stdout. ")).Add(helpCodeInline("format=file")).Append(" writes a file sink. Comma-separated specs may mix one stdout format with many file sinks.").NewLine()
	return doc.NewLine()
}

func addRenderingGuide(doc api.Text) api.Text {
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
	return doc.NewLine()
}

func addComponentGuide(doc api.Text) api.Text {
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
