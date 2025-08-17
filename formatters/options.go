package formatters

import (
	"flag"
	"fmt"

	"github.com/flanksource/clicky/api"
	"github.com/spf13/pflag"
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
	flags.StringVar(&options.Format, "format", "pretty", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
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
	flags.StringVar(&options.Format, "format", "pretty", "Output format: pretty, json, yaml, csv, html, pdf, markdown")
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
	// Count how many format flags are set
	formatCount := 0
	selectedFormat := ""

	if options.JSON {
		formatCount++
		selectedFormat = "json"
	}
	if options.YAML {
		formatCount++
		selectedFormat = "yaml"
	}
	if options.CSV {
		formatCount++
		selectedFormat = "csv"
	}
	if options.Markdown {
		formatCount++
		selectedFormat = "markdown"
	}
	if options.Pretty {
		formatCount++
		selectedFormat = "pretty"
	}
	if options.HTML {
		formatCount++
		selectedFormat = "html"
	}
	if options.PDF {
		formatCount++
		selectedFormat = "pdf"
	}

	// Check for mutual exclusivity
	if formatCount > 1 {
		return fmt.Errorf("multiple format flags specified; please use only one format flag")
	}

	// If a format-specific flag was set, override the --format flag
	if formatCount == 1 {
		options.Format = selectedFormat
	}

	return nil
}
