package formatters

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/filter"
	flagmeta "github.com/flanksource/clicky/flags"
)

// knownFormats is the set of format names accepted in --format specs
// (both as a bare stdout format and inside "format=file" sinks).
var knownFormats = map[string]bool{
	"pretty":      true,
	"json":        true,
	"ndjson":      true,
	"toon":        true,
	"yaml":        true,
	"yml":         true,
	"clicky-json": true,
	"csv":         true,
	"html":        true,
	"html-react":  true,
	"html-static": true,
	"markdown":    true,
	"llm":         true,
	"md":          true,
	"pdf":         true,
	"slack":       true,
	"excel":       true,
	"xlsx":        true,
	"tree":        true,
}

const FormatSpecHelp = "Output format. Use one stdout format (pretty|json|ndjson|toon|yaml|yml|csv|markdown|md|llm|html|html-static|html-react|clicky-json|pdf|slack|excel|xlsx|tree), " +
	"or comma-separated format=file sinks such as 'pretty,json=out.json,markdown=summary.md'."

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

// FormatOptions contains options for formatting operations
type FormatOptions struct {
	Context    context.Context   `json:"-" yaml:"-"`
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
	LLM      bool `json:"llm,omitempty"`
	Pretty   bool `json:"pretty,omitempty"`
	HTML     bool `json:"html,omitempty"`
	PDF      bool `json:"pdf,omitempty"`
	Slack    bool `json:"slack,omitempty"`

	// Display structure flags (additive with format flags)
	Tree  bool `json:"tree,omitempty"`  // Display in tree structure
	Table bool `json:"table,omitempty"` // Display in table structure

	// Paging options
	Page    int          `json:"page,omitempty"`  // Current page (1-indexed)
	Limit   int          `json:"limit,omitempty"` // Items per page
	LLMPage *LLMPageInfo `json:"-"`

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
	legacy := []string{}
	for _, candidate := range []struct {
		selected bool
		name     string
	}{{o.JSON, "json"}, {o.YAML, "yaml"}, {o.CSV, "csv"}, {o.Markdown, "markdown"}, {o.LLM, "llm"}, {o.Pretty, "pretty"}, {o.HTML, "html"}, {o.PDF, "pdf"}, {o.Slack, "slack"}} {
		if candidate.selected {
			legacy = append(legacy, candidate.name)
		}
	}
	if len(legacy) > 1 || (o.Format != "" && len(legacy) > 0) {
		return fmt.Errorf("choose one output format: --format cannot be combined with legacy format flags")
	}
	if o.Tree && o.Table {
		return fmt.Errorf("--tree and --table cannot be combined")
	}
	if err := filter.Validate(o.Filter); err != nil {
		return fmt.Errorf("invalid --filter: %w", err)
	}
	spec := strings.TrimSpace(o.Format)
	if spec == "" {
		if len(legacy) > 0 || o.Output != "" {
			format := "pretty"
			if len(legacy) > 0 {
				format = legacy[0]
			}
			if incompatibleStructure(o.Tree, o.Table, format) {
				return fmt.Errorf("--tree or --table is incompatible with --format %s", format)
			}
			o.Sinks = []FormatSink{{Format: format, File: o.Output}}
		}
		return nil
	}

	var stdoutCount int
	parsed := make([]FormatSink, 0)
	files := make(map[string]bool)
	for _, rawPart := range strings.Split(spec, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return fmt.Errorf("invalid --format %q: empty entry", spec)
		}
		name, file, hasFile := strings.Cut(part, "=")
		name = canonicalFormat(strings.ToLower(strings.TrimSpace(name)))
		file = strings.TrimSpace(file)
		if name == "" {
			return fmt.Errorf("invalid --format entry %q: empty format name", part)
		}
		if !knownFormats[name] {
			if _, registered := GetCustomFormatter(name); !registered {
				return fmt.Errorf("invalid --format entry %q: unknown format %q", part, name)
			}
		}
		if hasFile && file == "" {
			return fmt.Errorf("invalid --format entry %q: empty file path after '='", part)
		}
		if incompatibleStructure(o.Tree, o.Table, name) {
			return fmt.Errorf("--%s is incompatible with --format %s", map[bool]string{true: "tree", false: "table"}[o.Tree], name)
		}
		if hasFile {
			absolute, err := filepath.Abs(file)
			if err != nil {
				return fmt.Errorf("invalid output path %q: %w", file, err)
			}
			if files[absolute] {
				return fmt.Errorf("invalid --format: two sinks target the same output file %q", file)
			}
			files[absolute] = true
		}
		if !hasFile {
			stdoutCount++
			if stdoutCount > 1 {
				return fmt.Errorf("invalid --format %q: more than one stdout format specified", spec)
			}
		}
		parsed = append(parsed, FormatSink{Format: name, File: file})
	}
	if o.Output != "" {
		if len(parsed) != 1 || parsed[0].File != "" {
			return fmt.Errorf("--output requires a single stdout --format entry")
		}
		parsed[0].File = o.Output
	}
	o.Sinks = parsed
	return nil
}

func incompatibleStructure(tree, table bool, format string) bool {
	switch format {
	case "json", "ndjson", "yaml", "toon", "clicky-json":
		return tree || table
	case "csv", "excel":
		return tree
	default:
		return false
	}
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
		if opt.Context != nil {
			merged.Context = opt.Context
		}
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
		if opt.LLMPage != nil {
			merged.LLMPage = opt.LLMPage
		}
		if opt.LLM {
			merged.LLM = true
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
	flags.StringVar(&options.Format, "format", "", FormatSpecHelp)
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")
	flags.StringVar(&options.Filter, "filter", "", "CEL expression for filtering table rows and tree nodes (e.g., \"status == 'active' && age > 30\")")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.LLM, "llm", false, "Output up to 25 rows in Markdown with paging information")
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
	flags.StringVar(&options.Format, "format", "", FormatSpecHelp)
	flags.StringVar(&options.Output, "output", "", "Output file pattern (optional, uses stdout if not specified)")
	flags.BoolVar(&options.NoColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&options.DumpSchema, "dump-schema", false, "Dump the schema to stderr for debugging")
	flags.StringVar(&options.Filter, "filter", "", "CEL expression for filtering table rows and tree nodes (e.g., \"status == 'active' && age > 30\")")

	// Format-specific flags (mutually exclusive)
	flags.BoolVar(&options.JSON, "json", false, "Output in JSON format")
	flags.BoolVar(&options.YAML, "yaml", false, "Output in YAML format")
	flags.BoolVar(&options.CSV, "csv", false, "Output in CSV format")
	flags.BoolVar(&options.Markdown, "markdown", false, "Output in Markdown format")
	flags.BoolVar(&options.LLM, "llm", false, "Output up to 25 rows in Markdown with paging information")
	flags.BoolVar(&options.Pretty, "pretty", false, "Output in pretty format (default)")
	flags.BoolVar(&options.HTML, "html", false, "Output in HTML format")
	flags.BoolVar(&options.PDF, "pdf", false, "Output in PDF format")
	flags.BoolVar(&options.Slack, "slack", false, "Output in Slack Block Kit JSON format")

	// Display structure flags (additive with format)
	flags.BoolVar(&options.Tree, "tree", false, "Display in tree structure (additive with format)")
	flags.BoolVar(&options.Table, "table", false, "Display in table structure (additive with format)")
	for _, name := range []string{
		"format", "output", "no-color", "dump-schema", "filter",
		"json", "yaml", "csv", "markdown", "llm", "pretty", "html", "pdf", "slack", "tree", "table",
	} {
		flagmeta.SetCategory(flags.Lookup(name), flagmeta.CategoryFormatting)
	}
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
	} else if options.LLM {
		selectedFormat = append(selectedFormat, "llm")
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

// stdoutIsTerminal reports whether formatted output, which MustPrint writes to
// stdout, lands on a terminal. A variable so tests can stand in a terminal.
var stdoutIsTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// ResolveNoColor checks env vars and stdout and sets NoColor accordingly.
// Respects: NO_COLOR (https://no-color.org/), COLOR=no|false, TERM=dumb, and a
// stdout that is not a terminal (`cmd > out.txt`, `cmd | less`) unless
// CLICKY_FORCE_INTERACTIVE is set.
// The --no-color CLI flag takes precedence (already set before this is called).
func (options *FormatOptions) ResolveNoColor() {
	if options.NoColor {
		return
	}

	if IsNoColor() || (!stdoutIsTerminal() && os.Getenv("CLICKY_FORCE_INTERACTIVE") == "") {
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
