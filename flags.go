package clicky

import (
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
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

// BindTaskManagerPFlags adds TaskManager flags to pflag set (for Cobra)
func BindAllFlags(flags *pflag.FlagSet, filters ...string) *AllFlags {
	flags.CountVarP(&Flags.LevelCount, "loglevel", "v", "Increase logging level")
	flags.StringVar(&Flags.Level, "log-level", "info", "Set the default log level")
	flags.BoolVar(&Flags.JsonLogs, "json-logs", false, "Print logs in json format to stderr")

	flags.BoolVar(&Flags.ReportCaller, "report-caller", false, "Report log caller info")
	flags.BoolVar(&Flags.LogToStderr, "log-to-stderr", true, "Log to stderr instead of stdout")

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
	}
	// Format Options

	if collections.MatchItems("format", filters...) {

		flags.StringVar(&Flags.Format, "format", "", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
		flags.BoolVar(&Flags.FormatOptions.NoColor, "no-color", false, "Disable colored output")
		flags.BoolVar(&Flags.Verbose, "verbose", false, "Enable verbose output")
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
	}

	return Flags
}

func (a *AllFlags) String() string {
	s, _ := Format(a, FormatOptions{YAML: true})
	return s
}

func (a *AllFlags) UseFlags() {
	logger.Configure(a.Flags)
	logger.V(6).Infof("Using logger flags: %s", a)
	a.Apply()
	UseFormatter(a.FormatOptions)
	properties.Set("log.level", "trace")
	properties.Set("log.color", "true")
}
