package clicky

import (
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/spf13/pflag"
)

type AllFlags struct {
	TaskManagerOptions
	FormatOptions
	DependencyScannerOptions
	logger.Flags
}

type DependencyScannerOptions struct {
	CacheTTL time.Duration // Cache TTL for dependency scans
	NoCache  bool          // Disable caching (equivalent to --cache-ttl=0)
}

// GetEffectiveTTL returns the effective cache TTL considering the no-cache flag
func (d DependencyScannerOptions) GetEffectiveTTL() time.Duration {
	if d.NoCache {
		return 0 // --no-cache sets TTL to 0
	}
	return d.CacheTTL
}

var Flags AllFlags = AllFlags{
	FormatOptions:      FormatOptions{},
	TaskManagerOptions: *DefaultTaskManagerOptions(),
	DependencyScannerOptions: DependencyScannerOptions{
		CacheTTL: 24 * time.Hour, // Default 24 hour cache
		NoCache:  false,
	},
	Flags: logger.Flags{
		Level:        "info",
		LevelCount:   0,
		JsonLogs:     false,
		ReportCaller: false,
		LogToStderr:  true,
	},
}

// BindTaskManagerPFlags adds TaskManager flags to pflag set (for Cobra)
func BindAllFlags(flags *pflag.FlagSet) AllFlags {
	flags.CountVarP(&Flags.Flags.LevelCount, "loglevel", "v", "Increase logging level")
	flags.StringVar(&Flags.Flags.Level, "log-level", "info", "Set the default log level")
	flags.BoolVar(&Flags.Flags.JsonLogs, "json-logs", false, "Print logs in json format to stderr")

	flags.BoolVar(&Flags.Flags.ReportCaller, "report-caller", false, "Report log caller info")
	flags.BoolVar(&Flags.Flags.LogToStderr, "log-to-stderr", true, "Log to stderr instead of stdout")

	flags.BoolVar(&Flags.NoProgress, "no-progress", Flags.NoProgress,
		"Disable progress display")
	flags.IntVar(&Flags.MaxConcurrent, "max-concurrent", Flags.MaxConcurrent,
		"Maximum concurrent tasks (0 = unlimited)")
	flags.DurationVar(&Flags.GracefulTimeout, "graceful-timeout", Flags.GracefulTimeout,
		"Timeout for graceful shutdown on interrupt")
	flags.IntVar(&Flags.MaxRetries, "max-retries", Flags.MaxRetries,
		"Maximum retry attempts for failed tasks")
	flags.DurationVar(&Flags.RetryDelay, "retry-delay", Flags.RetryDelay,
		"Base delay between retry attempts")

	// Format Options

	flags.StringVar(&Flags.FormatOptions.Format, "format", "pretty", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
	flags.BoolVar(&Flags.FormatOptions.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&Flags.FormatOptions.Verbose, "verbose", false, "Enable verbose output")
	flags.BoolVar(&Flags.FormatOptions.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&Flags.FormatOptions.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&Flags.FormatOptions.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&Flags.FormatOptions.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&Flags.FormatOptions.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&Flags.FormatOptions.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&Flags.FormatOptions.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&Flags.FormatOptions.PDF, "pdf", false, "Output in PDF format")
	
	// Dependency Scanner flags
	flags.DurationVar(&Flags.DependencyScannerOptions.CacheTTL, "cache-ttl", 24*time.Hour, "Cache TTL for dependency scans")
	flags.BoolVar(&Flags.DependencyScannerOptions.NoCache, "no-cache", false, "Disable caching (equivalent to --cache-ttl=0)")
	
	return Flags
}

func (a AllFlags) String() string {
	s, _ := Format(a, FormatOptions{YAML: true})
	return s
}

func (a AllFlags) UseFlags() {
	logger.Configure(a.Flags)
	logger.Debugf("Using logger flags: %s", a)
	UseGlobalTaskManager(a.TaskManagerOptions)
	UseFormatter(a.FormatOptions)
}
