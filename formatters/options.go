package formatters

import (
	"flag"

	"github.com/spf13/pflag"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
)

type PrettyMixin interface {
	Pretty() api.Text
}

// FormatOptions contains options for formatting operations
type FormatOptions struct {
	Format     string
	NoColor    bool
	Output     string
	Verbose    bool
	DumpSchema bool
	Schema     *api.PrettyObject // Schema for schema-aware formatting

	// Format-specific boolean flags (mutually exclusive)
	JSON     bool
	YAML     bool
	CSV      bool
	Markdown bool
	Pretty   bool
	HTML     bool
	PDF      bool
}

func MergeOptions(opts ...FormatOptions) FormatOptions {
	merged := FormatOptions{}
	for _, opt := range opts {
		if opt.Format != "" {
			merged.Format = opt.Format
		}
		if opt.NoColor {
			merged.NoColor = true
		}
		if opt.Output != "" {
			merged.Output = opt.Output
		}
		if opt.Verbose {
			merged.Verbose = true
		}
		if opt.DumpSchema {
			merged.DumpSchema = true
		}
		if opt.Schema != nil {
			merged.Schema = opt.Schema
		}
		if opt.JSON {
			merged.JSON = true
			continue // Only one format can be set
		}
		if opt.YAML {
			merged.YAML = true
			continue // Only one format can be set
		}
		if opt.CSV {
			merged.CSV = true
			continue // Only one format can be set
		}
		if opt.Markdown {
			merged.Markdown = true
			continue // Only one format can be set
		}
		if opt.Pretty {
			merged.Pretty = true
			continue // Only one format can be set
		}
		if opt.HTML {
			merged.HTML = true
			continue // Only one format can be set
		}
		if opt.PDF {
			merged.PDF = true
			continue // Only one format can be set
		}
	}
	return merged
}

// BindFlags adds formatting flags to the provided flag set
func BindFlags(flags *flag.FlagSet, options *FormatOptions) {
	flags.StringVar(&options.Format, "format", "", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.Verbose, "verbose", false, "Enable verbose output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&options.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&options.PDF, "pdf", false, "Output in PDF format")
}

// BindPFlags adds formatting flags to the provided pflag set (for cobra)
func BindPFlags(flags *pflag.FlagSet, options *FormatOptions) {
	flags.StringVar(&options.Format, "format", "", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.Verbose, "verbose", false, "Enable verbose output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&options.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&options.PDF, "pdf", false, "Output in PDF format")
}

// ResolveFormat resolves the output format from format-specific flags
func (options *FormatOptions) ResolveFormat() error {
	logger.Debugf("%+v", *options)
	// Count how many format flags are set
	selectedFormat := []string{}

	if options.Format != "" {
		selectedFormat = append(selectedFormat, options.Format)
	} else if options.JSON {
		selectedFormat = append(selectedFormat, "json")
	} else if options.YAML {
		selectedFormat = append(selectedFormat, "yaml")
	} else if options.CSV {
		selectedFormat = append(selectedFormat, "csv")
	} else if options.Markdown {
		selectedFormat = append(selectedFormat, "markdown")
	} else if options.HTML {
		selectedFormat = append(selectedFormat, "html")
	} else if options.PDF {
		selectedFormat = append(selectedFormat, "pdf")
	} else if options.Pretty {
		selectedFormat = append(selectedFormat, "pretty")
	}

	// If a format-specific flag was set, override the --format flag
	if len(selectedFormat) == 1 {
		options.Format = selectedFormat[0]
	} else {
		options.Format = "pretty" // Default format
	}

	logger.Tracef("Using format: %s", options.Format)

	return nil
}
