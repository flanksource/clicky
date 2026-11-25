package clicky

import (
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
	flagmeta "github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/formatters"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// HelpTheme supplies Clicky style classes for the parts of command help.
type HelpTheme struct {
	Heading string
	Usage   string
	Command string
	Flag    string
	Hint    string
	Value   string
}

// DefaultHelpTheme returns the style classes used by Clicky's CLI help.
func DefaultHelpTheme() HelpTheme {
	return HelpTheme{
		Heading: "text-cyan-600 font-bold",
		Usage:   "text-blue-500 font-bold",
		Command: "text-cyan-600",
		Flag:    "text-teal-600",
		Hint:    "text-muted",
		Value:   "text-amber-600 italic",
	}
}

// CommandHelp is the help for a Cobra command as a Clicky pretty value.
// Use DefaultHelpTheme for the CLI palette.
type CommandHelp struct {
	Command *cobra.Command
	Theme   HelpTheme
}

func (h CommandHelp) Pretty() api.Textable {
	doc := api.Text{}
	description := h.Command.Long
	if description == "" {
		description = h.Command.Short
	}
	if description = strings.TrimRightFunc(description, unicode.IsSpace); description != "" {
		doc = doc.Add(h.description(description)).NewLine().NewLine()
	}
	if h.Command.Runnable() || h.Command.HasSubCommands() {
		doc = doc.Add(h.usage())
	}
	return doc
}

// FormatView supplies the help document to clicky.Format.
func (h CommandHelp) FormatView(FormatOptions) (any, error) {
	return h.Pretty(), nil
}

// SetColoredHelp installs grouped usage and themed help inherited by subcommands.
func SetColoredHelp(cmd *cobra.Command, theme HelpTheme) {
	SetGroupedUsage(cmd)
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		output := c.OutOrStdout()
		doc := CommandHelp{Command: c, Theme: theme}.Pretty()
		var rendered string
		if useHelpColor(c, output) {
			// A Windows console without virtual terminal processing prints raw escapes.
			restore, err := termenv.EnableVirtualTerminalProcessing(termenv.NewOutput(output, termenv.WithProfile(termenv.TrueColor)))
			if err == nil {
				rendered = doc.ANSI()
				defer func() {
					if err := restore(); err != nil {
						c.PrintErrln(err)
					}
				}()
			}
		}
		if rendered == "" {
			rendered = ansi.Strip(doc.String())
		}
		if _, err := io.WriteString(output, rendered); err != nil {
			c.PrintErrln(err)
		}
	})
}

func useHelpColor(cmd *cobra.Command, output io.Writer) bool {
	flag := cmd.Flag("no-color")
	if formatters.IsNoColor() || (flag != nil && flag.Value.String() == "true") {
		return false
	}
	if os.Getenv("CLICKY_FORCE_INTERACTIVE") != "" {
		return true
	}
	terminal, ok := output.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(terminal.Fd()))
}

func (h CommandHelp) usage() api.Textable {
	c := h.Command
	doc := api.Text{}
	if c.Runnable() || c.HasAvailableSubCommands() {
		doc = doc.Append("Usage:", h.Theme.Heading).NewLine()
		if c.Runnable() {
			doc = doc.Append("  ").Append(c.UseLine(), h.Theme.Usage).NewLine()
		}
		if c.HasAvailableSubCommands() {
			doc = doc.Append("  ").Append(c.CommandPath()+" [command]", h.Theme.Usage).NewLine()
		}
	}
	if len(c.Aliases) > 0 {
		doc = doc.NewLine().Append("Aliases:", h.Theme.Heading).NewLine().Append("  ").Append(c.NameAndAliases(), h.Theme.Command).NewLine()
	}
	if c.HasExample() {
		example := CodeBlock("bash", c.Example)
		example.LiteralStyle = h.Theme.Value
		doc = doc.NewLine().Append("Examples:", h.Theme.Heading).NewLine().Add(example).NewLine()
	}
	if c.HasAvailableSubCommands() {
		doc = doc.NewLine().Append("Available Commands:", h.Theme.Heading).NewLine()
		for _, sub := range c.Commands() {
			if sub.IsAvailableCommand() || sub.Name() == "help" {
				doc = doc.Add(h.commandLine(helpCommandLine{Name: sub.Name(), Width: sub.NamePadding(), Description: sub.Short}))
			}
		}
	}
	for _, group := range CommandFlagGroups(c) {
		doc = doc.NewLine().Append(group.Title+":", h.Theme.Heading).NewLine().Add(h.flagLines(group))
	}
	if c.HasHelpSubCommands() {
		doc = doc.NewLine().Append("Additional help topics:", h.Theme.Heading).NewLine()
		for _, sub := range c.Commands() {
			if sub.IsAdditionalHelpTopicCommand() {
				doc = doc.Add(h.commandLine(helpCommandLine{Name: sub.CommandPath(), Width: sub.CommandPathPadding(), Description: sub.Short}))
			}
		}
	}
	if c.HasAvailableSubCommands() {
		hint := "Use \"" + c.CommandPath() + " [command] --help\" for more information about a command."
		doc = doc.NewLine().Append(hint, h.Theme.Hint).NewLine()
	}
	return doc
}

type helpCommandLine struct {
	Name        string
	Width       int
	Description string
}

func (h CommandHelp) commandLine(line helpCommandLine) api.Textable {
	return Text("  ").Append(line.Name, h.Theme.Command).
		Append(strings.Repeat(" ", max(1, line.Width-len(line.Name)+1))).Append(line.Description).NewLine()
}

// pflag indents flag labels by two or six spaces and continuation lines further.
var helpFlagLabel = regexp.MustCompile(`(?m)^(?:  -., |      )--[^[:space:]\[]+`)

var helpValue = regexp.MustCompile("`[^`]+`" +
	`|"(?:[^"\\]|\\.)*"|(?:^|[[:space:](])'[^']+'` +
	`|\([a-z][a-z0-9-]*(?:\s*\|\s*[a-z][a-z0-9-]*)+\)`)
var helpChoiceName = regexp.MustCompile(`[a-z][a-z0-9-]*`)

func (h CommandHelp) description(text string) api.Textable {
	doc := api.Text{}
	if h.Theme.Value == "" {
		return doc.Append(text)
	}
	end := 0
	for _, match := range helpValue.FindAllStringIndex(text, -1) {
		start := match[0]
		if (text[start] == '(' || unicode.IsSpace(rune(text[start]))) && text[start+1] == '\'' {
			start++
		}
		if text[start] == '(' {
			for _, choice := range helpChoiceName.FindAllStringIndex(text[start:match[1]], -1) {
				left, right := start+choice[0], start+choice[1]
				doc = doc.Append(text[end:left]).Append(text[left:right], h.Theme.Value)
				end = right
			}
			continue
		}
		doc = doc.Append(text[end:start]).Append(text[start:match[1]], h.Theme.Value)
		end = match[1]
	}
	return doc.Append(text[end:])
}

func (h CommandHelp) flagLines(group FlagGroup) api.Textable {
	var shortcuts []string
	flags := pflag.NewFlagSet(group.Title, pflag.ContinueOnError)
	group.Flags.VisitAll(func(flag *pflag.Flag) {
		if flagmeta.IsOutputFormatShortcut(flag) {
			shortcuts = append(shortcuts, "--"+flag.Name)
			return
		}
		flags.AddFlag(flag)
	})
	usage := strings.TrimRight(flags.FlagUsagesWrapped(api.GetTerminalWidth()), " \n")
	labels := helpFlagLabel.FindAllStringIndex(usage, -1)
	doc := api.Text{}
	prefix := len(usage)
	if len(labels) > 0 {
		prefix = labels[0][0]
	}
	if prefix > 0 {
		doc = doc.Append(strings.TrimSuffix(usage[:prefix], "\n")).NewLine()
	}
	for i, span := range labels {
		end := len(usage)
		if i+1 < len(labels) {
			end = labels[i+1][0] - 1
		}
		label, description := usage[span[0]:span[1]], usage[span[1]:end]
		doc = doc.Append(label, h.Theme.Flag).Add(h.description(description)).NewLine()
		if len(shortcuts) > 0 && strings.HasSuffix(label, " --format") {
			doc = doc.Add(h.shortcuts(shortcuts, helpDescriptionIndent(label, description)))
			shortcuts = nil
		}
	}
	if len(shortcuts) > 0 {
		doc = doc.Add(h.shortcuts(shortcuts, 2))
	}
	return doc
}

func helpDescriptionIndent(label, description string) int {
	line, continuation, _ := strings.Cut(description, "\n")
	_, words, found := strings.Cut(line, "   ")
	if found && strings.TrimSpace(words) != "" {
		return len(label) + len(line) - len(strings.TrimLeft(words, " "))
	}
	if line, _, _ := strings.Cut(continuation, "\n"); line != "" {
		return len(line) - len(strings.TrimLeft(line, " "))
	}
	return 2
}

func (h CommandHelp) shortcuts(names []string, indent int) api.Textable {
	const title = "Shortcuts: "
	width := api.GetTerminalWidth()
	if indent+len(title)+ansi.StringWidth(strings.Join(names, " ")) > width {
		indent = 2
	}
	hanging := indent + len(title)
	lines := [][]string{nil}
	lineWidth := hanging
	for _, name := range names {
		last := len(lines) - 1
		if len(lines[last]) > 0 && lineWidth+1+len(name) > width {
			lines, lineWidth = append(lines, nil), hanging
			last++
		}
		if len(lines[last]) > 0 {
			lineWidth++
		}
		lines[last] = append(lines[last], name)
		lineWidth += len(name)
	}
	doc := Text(strings.Repeat(" ", indent) + title)
	for i, line := range lines {
		if i > 0 {
			doc = doc.Append(strings.Repeat(" ", hanging))
		}
		doc = doc.Append(strings.Join(line, " "), h.Theme.Flag).NewLine()
	}
	return doc
}
