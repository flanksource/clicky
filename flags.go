package clicky

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type AllFlags struct {
	TaskManagerOptions
	FormatOptions
	logger.Flags
}

var Flags *AllFlags = &AllFlags{
	FormatOptions:      FormatOptions{},
	TaskManagerOptions: *DefaultTaskManagerOptions(),
	Flags: logger.Flags{
		Level:        "info",
		LevelCount:   0,
		JsonLogs:     false,
		ReportCaller: false,
		LogToStderr:  true,
	},
}

// FlagCategoryAnnotation is the pflag annotation key used to tag flags with
// a display category, so the usage template can render them in grouped sections.
const FlagCategoryAnnotation = "clicky_category"

// Flag category labels rendered as section headers in --help output.
const (
	CategoryLogging = "Logging"
	CategoryTasks   = "Tasks"
	CategoryFormat  = "Format"
)

// categoryOrder defines the rendering order for known flag categories. Any
// category not in this list is appended alphabetically after these.
var categoryOrder = []string{CategoryLogging, CategoryTasks, CategoryFormat}

// setCategory tags a flag with its display category for grouped usage rendering.
func setCategory(fs *pflag.FlagSet, name, category string) {
	_ = fs.SetAnnotation(name, FlagCategoryAnnotation, []string{category})
}

// BindAllFlags adds clicky's logging, task and format flags to the given pflag set.
// Each flag is annotated with a category so SetGroupedUsage can render them in
// titled sections.
func BindAllFlags(flags *pflag.FlagSet, filters ...string) *AllFlags {
	flags.CountVarP(&Flags.LevelCount, "loglevel", "v", "Increase logging level")
	flags.StringVar(&Flags.Level, "log-level", "info", "Set the default log level")
	flags.BoolVar(&Flags.JsonLogs, "json-logs", false, "Print logs in json format to stderr")
	flags.BoolVar(&Flags.ReportCaller, "report-caller", false, "Report log caller info")
	flags.BoolVar(&Flags.LogToStderr, "log-to-stderr", true, "Log to stderr instead of stdout")
	for _, n := range []string{"loglevel", "log-level", "json-logs", "report-caller", "log-to-stderr"} {
		setCategory(flags, n, CategoryLogging)
	}

	if collections.MatchItems("tasks", filters...) {
		flags.BoolVar(&Flags.NoProgress, "no-progress", Flags.NoProgress,
			"Disable progress display")
		flags.IntVar(&Flags.MaxConcurrent, "max-concurrent", 4,
			"Maximum concurrent tasks (0 = unlimited)")
		flags.DurationVar(&Flags.GracefulTimeout, "graceful-timeout", Flags.GracefulTimeout,
			"Timeout for graceful shutdown on interrupt")
		flags.IntVar(&Flags.MaxRetries, "max-retries", Flags.MaxRetries,
			"Maximum retry attempts for failed tasks")
		flags.DurationVar(&Flags.RetryDelay, "retry-delay", Flags.RetryDelay,
			"Base delay between retry attempts")
		for _, n := range []string{"no-progress", "max-concurrent", "graceful-timeout", "max-retries", "retry-delay"} {
			setCategory(flags, n, CategoryTasks)
		}
	}

	if collections.MatchItems("format", filters...) {
		flags.StringVar(&Flags.Format, "format", "",
			"Output format. Either a single format (pretty|json|ndjson|toon|yaml|csv|html|markdown|pdf|slack) "+
				"rendered to stdout, or a comma-separated list of format=file sinks "+
				"(e.g. 'json=out.json,markdown=summary.md') written to files. "+
				"A bare format and format=file pairs may be mixed in one spec.")
		flags.StringVar(&Flags.Filter, "filter", "", "CEL expression to filter output data")
		flags.BoolVar(&Flags.NoColor, "no-color", false, "Disable colored output")
		flags.BoolVar(&Flags.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")

		// Format-specific flags (mutually exclusive)
		flags.BoolVar(&Flags.JSON, "json", false, "Output in JSON format")
		flags.BoolVar(&Flags.YAML, "yaml", false, "Output in YAML format")
		flags.BoolVar(&Flags.CSV, "csv", false, "Output in CSV format")
		flags.BoolVar(&Flags.Markdown, "markdown", false, "Output in Markdown format")
		flags.BoolVar(&Flags.Pretty, "pretty", false, "Output in pretty format (default)")
		flags.BoolVar(&Flags.HTML, "html", false, "Output in HTML format")
		flags.BoolVar(&Flags.PDF, "pdf", false, "Output in PDF format")

		// Display structure flags (additive with format)
		flags.BoolVar(&Flags.Tree, "tree", false, "Display in tree structure (additive with format)")
		flags.BoolVar(&Flags.Table, "table", false, "Display in table structure (additive with format)")
		for _, n := range []string{
			"format", "filter", "no-color", "dump-schema",
			"json", "yaml", "csv", "markdown", "pretty", "html", "pdf",
			"tree", "table",
		} {
			setCategory(flags, n, CategoryFormat)
		}
	}

	return Flags
}

// BindAllFlagsToCommand binds clicky's flags to cmd's persistent flag set and
// installs a grouped usage template so --help renders flags in titled sections
// (Logging, Tasks, Format, then any uncategorized flags under "Flags").
func BindAllFlagsToCommand(cmd *cobra.Command, filters ...string) *AllFlags {
	f := BindAllFlags(cmd.PersistentFlags(), filters...)
	SetGroupedUsage(cmd)
	return f
}

// SetGroupedUsage installs a usage function on cmd that renders flags in
// sections grouped by their FlagCategoryAnnotation. Flags without a category
// fall under a generic "Flags" heading. The function is also propagated as the
// usage template inheritance chain expects, so subcommands display the same
// grouping for inherited flags.
func SetGroupedUsage(cmd *cobra.Command) {
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return renderGroupedUsage(c, c.OutOrStderr())
	})
}

func renderGroupedUsage(c *cobra.Command, w io.Writer) error {
	if c.Runnable() || c.HasAvailableSubCommands() {
		fmt.Fprint(w, "Usage:")
		if c.Runnable() {
			fmt.Fprintf(w, "\n  %s", c.UseLine())
		}
		if c.HasAvailableSubCommands() {
			fmt.Fprintf(w, "\n  %s [command]", c.CommandPath())
		}
		fmt.Fprintln(w)
	}

	if len(c.Aliases) > 0 {
		fmt.Fprintf(w, "\nAliases:\n  %s\n", c.NameAndAliases())
	}

	if c.HasExample() {
		fmt.Fprintf(w, "\nExamples:\n%s\n", c.Example)
	}

	if c.HasAvailableSubCommands() {
		fmt.Fprintln(w, "\nAvailable Commands:")
		for _, sub := range c.Commands() {
			if sub.IsAvailableCommand() || sub.Name() == "help" {
				fmt.Fprintf(w, "  %s %s\n", rpad(sub.Name(), sub.NamePadding()), sub.Short)
			}
		}
	}

	if c.HasAvailableLocalFlags() {
		writeGroupedFlags(w, "Flags", c.LocalFlags())
	}

	if c.HasAvailableInheritedFlags() {
		writeGroupedFlags(w, "Global Flags", c.InheritedFlags())
	}

	if c.HasHelpSubCommands() {
		fmt.Fprintln(w, "\nAdditional help topics:")
		for _, sub := range c.Commands() {
			if sub.IsAdditionalHelpTopicCommand() {
				fmt.Fprintf(w, "  %s %s\n", rpad(sub.CommandPath(), sub.CommandPathPadding()), sub.Short)
			}
		}
	}

	if c.HasAvailableSubCommands() {
		fmt.Fprintf(w, "\nUse \"%s [command] --help\" for more information about a command.\n", c.CommandPath())
	}

	return nil
}

// writeGroupedFlags renders a flag set under a heading, partitioned by the
// FlagCategoryAnnotation. Uncategorized flags appear under the base heading;
// each known category gets its own subsection.
func writeGroupedFlags(w io.Writer, heading string, fs *pflag.FlagSet) {
	groups := map[string]*pflag.FlagSet{}
	uncategorized := pflag.NewFlagSet("uncategorized", pflag.ContinueOnError)

	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		cats := f.Annotations[FlagCategoryAnnotation]
		if len(cats) == 0 {
			uncategorized.AddFlag(f)
			return
		}
		cat := cats[0]
		g, ok := groups[cat]
		if !ok {
			g = pflag.NewFlagSet(cat, pflag.ContinueOnError)
			groups[cat] = g
		}
		g.AddFlag(f)
	})

	if uncategorized.HasAvailableFlags() {
		fmt.Fprintf(w, "\n%s:\n%s", heading, strings.TrimRight(uncategorized.FlagUsages(), " \n"))
		fmt.Fprintln(w)
	}

	for _, cat := range orderedCategories(groups) {
		g := groups[cat]
		fmt.Fprintf(w, "\n%s %s:\n%s", cat, strings.ToLower(heading), strings.TrimRight(g.FlagUsages(), " \n"))
		fmt.Fprintln(w)
	}
}

// orderedCategories returns category keys in the canonical order, with unknown
// categories sorted alphabetically at the end.
func orderedCategories(groups map[string]*pflag.FlagSet) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(groups))
	for _, c := range categoryOrder {
		if _, ok := groups[c]; ok {
			out = append(out, c)
			seen[c] = true
		}
	}
	extras := make([]string, 0)
	for c := range groups {
		if !seen[c] {
			extras = append(extras, c)
		}
	}
	sort.Strings(extras)
	return append(out, extras...)
}

func rpad(s string, padding int) string {
	if padding <= len(s) {
		return s
	}
	return s + strings.Repeat(" ", padding-len(s))
}

func (a *AllFlags) String() string {
	s, _ := Format(a, FormatOptions{YAML: true})
	return s
}

func (a *AllFlags) UseFlags() {
	a.ResolveNoColor()
	if a.NoColor {
		a.Color = false
	} else {
		a.Color = true
	}
	logger.Configure(a.Flags)
	UseFormatter(a.FormatOptions)
	a.Apply()
}
