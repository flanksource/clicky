package clicky

import (
	"io"
	"sort"

	flagmeta "github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type AllFlags struct {
	TaskManagerOptions
	FormatOptions
	TimingsOptions
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
const FlagCategoryAnnotation = flagmeta.CategoryAnnotation

// Flag category labels rendered as section headers in --help output.
const (
	CategoryLogging = flagmeta.CategoryLogging
	CategoryTasks   = flagmeta.CategoryTasks
	CategoryFormat  = flagmeta.CategoryFormatting
)

// categoryOrder defines the rendering order for known flag categories. Any
// category not in this list is appended alphabetically after these.
var categoryOrder = []string{
	flagmeta.CategoryModelSelection, flagmeta.CategoryModelPermissions,
	flagmeta.CategoryModelRuntime, CategoryFormat, CategoryLogging, CategoryTasks,
}

// setCategory tags a flag with its display category for grouped usage rendering.
func setCategory(fs *pflag.FlagSet, name, category string) {
	flagmeta.SetCategory(fs.Lookup(name), category)
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
	Flags.BindFlags(flags)
	for _, n := range []string{"loglevel", "log-level", "json-logs", "report-caller", "log-to-stderr", "timings"} {
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
			"Output format. Either a single format (pretty | json | ndjson | toon | yaml | csv | html | markdown | llm | pdf | slack) "+
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
		flags.BoolVar(&Flags.LLM, "llm", false, "Output up to 25 rows in Markdown with paging information")
		flags.BoolVar(&Flags.Pretty, "pretty", false, "Output in pretty format (default)")
		flags.BoolVar(&Flags.HTML, "html", false, "Output in HTML format")
		flags.BoolVar(&Flags.PDF, "pdf", false, "Output in PDF format")

		// Display structure flags (additive with format)
		flags.BoolVar(&Flags.Tree, "tree", false, "Display in tree structure (additive with format)")
		flags.BoolVar(&Flags.Table, "table", false, "Display in table structure (additive with format)")
		for _, n := range []string{
			"format", "filter", "no-color", "dump-schema",
			"tree", "table",
		} {
			setCategory(flags, n, CategoryFormat)
		}
		for _, n := range []string{"json", "yaml", "csv", "markdown", "llm", "pretty", "html", "pdf"} {
			flagmeta.MarkOutputFormatShortcut(flags.Lookup(n))
		}
	}

	return Flags
}

// BindAllFlagsToCommand binds clicky's flags to cmd's persistent flag set and
// installs a grouped usage template so --help renders flags in titled sections
// with command-specific flags first and reusable categories below them.
func BindAllFlagsToCommand(cmd *cobra.Command, filters ...string) *AllFlags {
	f := BindAllFlags(cmd.PersistentFlags(), filters...)
	SetGroupedUsage(cmd)
	return f
}

// SetGroupedUsage installs a usage function on cmd that renders flags in
// sections grouped by their FlagCategoryAnnotation. Flags without a category
// fall under "Command flags" or "Global flags". Cobra's usage-function
// inheritance gives subcommands the same grouping for inherited flags.
func SetGroupedUsage(cmd *cobra.Command) {
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		help := CommandHelp{Command: c}
		_, err := io.WriteString(c.OutOrStderr(), help.usage().String())
		return err
	})
}

// FlagGroup is one nonempty section of a command's visible flags.
type FlagGroup struct {
	Title string
	Flags *pflag.FlagSet
}

// CommandFlagGroups groups local and inherited flags for standard and custom help.
func CommandFlagGroups(cmd *cobra.Command) []FlagGroup {
	groups := map[string]*pflag.FlagSet{}
	command := pflag.NewFlagSet("command", pflag.ContinueOnError)
	global := pflag.NewFlagSet("global", pflag.ContinueOnError)
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		uncategorized := command
		if cmd.PersistentFlags().Lookup(flag.Name) == flag {
			uncategorized = global
		}
		collectGroupedFlag(flag, uncategorized, groups)
	})
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		collectGroupedFlag(flag, global, groups)
	})
	var sections []FlagGroup
	if command.HasAvailableFlags() {
		sections = append(sections, FlagGroup{Title: "Command flags", Flags: command})
	}
	for _, cat := range orderedCategories(groups) {
		sections = append(sections, FlagGroup{Title: cat, Flags: groups[cat]})
	}
	if global.HasAvailableFlags() {
		sections = append(sections, FlagGroup{Title: "Global flags", Flags: global})
	}
	return sections
}

func collectGroupedFlag(flag *pflag.Flag, uncategorized *pflag.FlagSet, groups map[string]*pflag.FlagSet) {
	if flag.Hidden {
		return
	}
	category := flagmeta.Category(flag)
	if category == "" {
		uncategorized.AddFlag(flag)
		return
	}
	if groups[category] == nil {
		groups[category] = pflag.NewFlagSet(category, pflag.ContinueOnError)
	}
	groups[category].AddFlag(flag)
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

func (a *AllFlags) String() string {
	s, _ := Format(a, FormatOptions{YAML: true})
	return s
}

func (a *AllFlags) UseFlags() {
	// The logger writes to stderr, so only --no-color and the env turn its colour
	// off — never a redirected stdout, which ResolveNoColor also honours.
	a.Color = !a.NoColor && !formatters.IsNoColor()
	a.ResolveNoColor()
	logger.Configure(a.Flags)
	UseFormatter(a.FormatOptions)
	a.Apply()
}
