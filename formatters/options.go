package formatters

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"github.com/flanksource/clicky/api"
)

// knownFormats is the set of format names accepted in --format specs
// (both as a bare stdout format and inside "format=file" sinks).
var knownFormats = map[string]bool{
	"pretty":      true,
	"json":        true,
	"yaml":        true,
	"yml":         true,
	"csv":         true,
	"html":        true,
	"html-static": true,
	"markdown":    true,
	"md":          true,
	"pdf":         true,
	"slack":       true,
	"excel":       true,
	"xlsx":        true,
	"tree":        true,
}

// canonicalFormat normalises common aliases (e.g. "md" -> "markdown").
func canonicalFormat(f string) string {
	switch f {
	case "md":
		return "markdown"
	case "yml":
		return "yaml"
	case "xlsx":
		return "excel"
	}
	return f
}

// FormatSink is one (format, file) pair parsed from the --format spec.
// When File == "" the sink renders to stdout; otherwise it is written
// to the given file via FormatManager.FormatToFile.
type FormatSink struct {
	Format string
	File   string
}

type PrettyMixin interface {
	Pretty() api.Text
}

// FormatOptions contains options for formatting operations
type FormatOptions struct {
	Format     string            `json:"format,omitempty"`
	NoColor    bool              `json:"no_color,omitempty"`
	Output     string            `json:"output,omitempty"`
	Verbose    bool              `json:"verbose,omitempty"`
	DumpSchema bool              `json:"dump_schema,omitempty"`
	Schema     *api.PrettyObject `json:"-"`                // Schema for schema-aware formatting
	Filter     string            `json:"filter,omitempty"` // CEL expression for filtering table rows and tree nodes

	// Format-specific boolean flags (mutually exclusive)
	JSON     bool `json:"json,omitempty"`
	YAML     bool `json:"yaml,omitempty"`
	CSV      bool `json:"csv,omitempty"`
	Markdown bool `json:"markdown,omitempty"`
	Pretty   bool `json:"pretty,omitempty"`
	HTML     bool `json:"html,omitempty"`
	PDF      bool `json:"pdf,omitempty"`
	Slack    bool `json:"slack,omitempty"`

	// Display structure flags (additive with format flags)
	Tree  bool `json:"tree,omitempty"`  // Display in tree structure
	Table bool `json:"table,omitempty"` // Display in table structure

	// React-specific options
	ReactComponent string `json:"react_component,omitempty"` // Custom JSX/TSX source for html-react format

	// Paging options
	Page  int `json:"page,omitempty"`  // Current page (1-indexed)
	Limit int `json:"limit,omitempty"` // Items per page

	// Sinks is derived state populated by ParseFormatSpec from the raw Format
	// string. It holds zero or one stdout sink (File == "") plus zero or more
	// file sinks produced by "format=file" pairs in --format.
	Sinks []FormatSink `json:"-"`

	// Internal fields (not exposed via flags)
	depth int // Hidden field for tracking nesting depth in recursive formatting
}

// ParseFormatSpec parses the raw Format string into Sinks.
//
// Accepts a single format name ("json"), a comma-separated list of
// "format=file" pairs ("json=out.json,markdown=summary.md"), or a mix
// ("pretty,json=out.json"). A bare format name becomes a stdout sink;
// no more than one stdout sink may appear in a single spec. "format=file"
// pairs are additive file sinks and have no upper bound.
//
// When Sinks has already been set (e.g. by a previous call or direct
// assignment in tests) the method is a no-op.
//
// If Format is empty and one of the legacy mutually-exclusive boolean
// toggles (JSON/HTML/...) is set, a matching single stdout sink is
// synthesized so existing call sites keep working unchanged.
func (o *FormatOptions) ParseFormatSpec() error {
	if len(o.Sinks) > 0 {
		return nil
	}
	spec := strings.TrimSpace(o.Format)
	if spec == "" {
		if legacy := legacyBoolFormat(o); legacy != "" {
			o.Sinks = []FormatSink{{Format: legacy}}
		}
		return nil
	}

	var stdoutCount int
	for _, rawPart := range strings.Split(spec, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}
		name, file, hasFile := strings.Cut(part, "=")
		name = canonicalFormat(strings.TrimSpace(name))
		file = strings.TrimSpace(file)
		if name == "" {
			return fmt.Errorf("invalid --format entry %q: empty format name", part)
		}
		if !knownFormats[name] {
			return fmt.Errorf("invalid --format entry %q: unknown format %q", part, name)
		}
		if hasFile && file == "" {
			return fmt.Errorf("invalid --format entry %q: empty file path after '='", part)
		}
		if !hasFile {
			stdoutCount++
			if stdoutCount > 1 {
				return fmt.Errorf("invalid --format %q: more than one stdout format specified", spec)
			}
		}
		o.Sinks = append(o.Sinks, FormatSink{Format: name, File: file})
	}
	return nil
}

// legacyBoolFormat returns the format name selected by the legacy mutually
// exclusive booleans on FormatOptions, or "" if none is set. Used as a
// fallback when Format is empty so pre-existing call sites keep working.
func legacyBoolFormat(o *FormatOptions) string {
	switch {
	case o.JSON:
		return "json"
	case o.YAML:
		return "yaml"
	case o.CSV:
		return "csv"
	case o.HTML:
		return "html"
	case o.Markdown:
		return "markdown"
	case o.PDF:
		return "pdf"
	case o.Slack:
		return "slack"
	case o.Pretty:
		return "pretty"
	}
	return ""
}

func (o FormatOptions) SkipTable() bool {
	return !o.Table
}

func (o FormatOptions) SkipTree() bool {
	return !o.Tree
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
		if opt.Filter != "" {
			merged.Filter = opt.Filter
		}
		if opt.Tree {
			merged.Tree = true
		}
		if opt.Table {
			merged.Table = true
		}
		if opt.Page > 0 {
			merged.Page = opt.Page
		}
		if opt.Limit > 0 {
			merged.Limit = opt.Limit
		}
		if opt.depth > 0 {
			merged.depth = opt.depth
		}
		if len(opt.Sinks) > 0 {
			merged.Sinks = append(merged.Sinks, opt.Sinks...)
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
		if opt.Slack {
			merged.Slack = true
			continue // Only one format can be set
		}
	}
	return merged
}

// BindFlags adds formatting flags to the provided flag set
func BindFlags(flags *flag.FlagSet, options *FormatOptions) {
	flags.StringVar(&options.Format, "format", "", "Output format: pretty, json, yaml, csv, html, pdf, markdown, slack")
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")
	flags.StringVar(&options.Filter, "filter", "", "CEL expression for filtering table rows and tree nodes (e.g., \"status == 'active' && age > 30\")")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&options.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&options.PDF, "pdf", false, "Output in PDF format")
	flags.BoolVar(&options.Slack, "slack", false, "Output in Slack Block Kit JSON format")

	// Display structure flags (additive with format)
	flags.BoolVar(&options.Tree, "tree", false, "Display in tree structure (additive with format)")
	flags.BoolVar(&options.Table, "table", false, "Display in table structure (additive with format)")
}

// BindPFlags adds formatting flags to the provided pflag set (for cobra)
func BindPFlags(flags *pflag.FlagSet, options *FormatOptions) {
	flags.StringVar(&options.Format, "format", "", "Output format: pretty, json, yaml, csv, html, pdf, markdown, slack")
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")
	flags.StringVar(&options.Filter, "filter", "", "CEL expression for filtering table rows and tree nodes (e.g., \"status == 'active' && age > 30\")")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&options.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&options.PDF, "pdf", false, "Output in PDF format")
	flags.BoolVar(&options.Slack, "slack", false, "Output in Slack Block Kit JSON format")

	// Display structure flags (additive with format)
	flags.BoolVar(&options.Tree, "tree", false, "Display in tree structure (additive with format)")
	flags.BoolVar(&options.Table, "table", false, "Display in table structure (additive with format)")
}

// ResolveFormat resolves the output format from format-specific flags
func (options *FormatOptions) ResolveFormat() string {
	// logger.V(4).Infof("%+v", *options)
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
	} else if options.Slack {
		selectedFormat = append(selectedFormat, "slack")
	} else if options.Pretty {
		selectedFormat = append(selectedFormat, "pretty")
	}

	// If a format-specific flag was set, override the --format flag
	if len(selectedFormat) == 1 {
		options.Format = selectedFormat[0]
	} else {
		options.Format = "pretty" // Default format
	}

	return options.Format
}

func IsNoColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if v := strings.ToLower(os.Getenv("COLOR")); v == "no" || v == "false" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return true
	}

	// also scan os.Args for --no-color in case env vars are not set but the flag is used (e.g., in tests)
	for _, arg := range os.Args[1:] {
		if arg != "--no-color=false" && (arg == "--no-color" || arg == "-no-color") {
			return true
		}
	}
	return false
}

// ResolveNoColor checks env vars and sets NoColor accordingly.
// Respects: NO_COLOR (https://no-color.org/), COLOR=no|false, TERM=dumb.
// The --no-color CLI flag takes precedence (already set before this is called).
func (options *FormatOptions) ResolveNoColor() {
	if options.NoColor {
		return
	}

	if IsNoColor() {
		options.NoColor = true
	}

}

// IncreaseDepth returns a copy of FormatOptions with depth incremented by 1
func (o FormatOptions) IncreaseDepth() FormatOptions {
	o.depth++
	return o
}

// Depth returns the current depth (for internal use by formatters)
func (o FormatOptions) Depth() int {
	return o.depth
}
