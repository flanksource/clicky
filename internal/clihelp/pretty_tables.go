package clihelp

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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
