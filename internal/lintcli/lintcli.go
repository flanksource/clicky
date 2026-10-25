// Package lintcli implements the argument handling and output of the
// `clicky lint` command on top of the lint package.
package lintcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/lint"
	"github.com/spf13/pflag"
)

type Options struct {
	Format       string
	NoColor      bool
	Raw          bool
	SummaryLimit int
	Severity     []string
	Source       bool
	SourceLines  int
}

type ExitError struct {
	Result *lint.Result
}

func (e ExitError) Error() string {
	if e.Result == nil {
		return "clickylint found issues"
	}
	parts := []string{}
	if count := e.Result.ErrorCount(); count > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", count))
	}
	if count := e.Result.WarningCount(); count > 0 {
		parts = append(parts, fmt.Sprintf("%d warnings", count))
	}
	if count := len(e.Result.Errors); count > 0 {
		parts = append(parts, fmt.Sprintf("%d load errors", count))
	}
	return "clickylint found " + strings.Join(parts, ", ")
}

func DefaultOptions() Options {
	return Options{
		Format:       "pretty",
		SummaryLimit: 5,
		SourceLines:  1,
	}
}

func BindFlags(flags *pflag.FlagSet, opts *Options) {
	flags.StringVar(&opts.Format, "format", opts.Format, "Output format: pretty or json")
	flags.BoolVar(&opts.NoColor, "no-color", opts.NoColor, "Disable ANSI color output")
	flags.BoolVar(&opts.Raw, "raw", opts.Raw, "Use the raw go/analysis singlechecker driver")
	flags.IntVar(&opts.SummaryLimit, "summary-limit", opts.SummaryLimit, "Maximum file locations to show per rule")
	flags.BoolVar(&opts.Source, "source", opts.Source, "Show source excerpts beneath each displayed location")
	flags.IntVar(&opts.SourceLines, "source-lines", opts.SourceLines, "Total source excerpt lines per location (requires --source)")
	flags.StringArrayVar(&opts.Severity, "severity", opts.Severity,
		"Re-level a rule, as <rule-id>=error|warning (repeatable). Adopt a rule as advice before making it a gate.")
}

// Run parses the non-raw lint arguments, lints the packages and renders the
// result to w, returning ExitError when error-level diagnostics were found.
func Run(w io.Writer, args []string) error {
	opts, packages, err := ParseArgs(args)
	if err != nil {
		return err
	}
	severity, err := ParseSeverityOverrides(opts.Severity)
	if err != nil {
		return err
	}
	result, err := lint.Run(lint.RunOptions{
		Packages:     packages,
		IncludeTests: true,
		Severity:     severity,
	})
	if err != nil {
		return err
	}
	if err := Render(w, result, opts); err != nil {
		return err
	}
	if result.HasErrors() {
		return ExitError{Result: result}
	}
	return nil
}

// ParseSeverityOverrides reads the --severity flags into the map the runner
// takes. Rule IDs are validated by the runner against the real catalogue, so a
// typo surfaces there rather than being silently dropped here.
func ParseSeverityOverrides(values []string) (map[string]lint.Severity, error) {
	if len(values) == 0 {
		return nil, nil
	}
	overrides := make(map[string]lint.Severity, len(values))
	for _, value := range values {
		id, level, found := strings.Cut(value, "=")
		if !found {
			return nil, fmt.Errorf("invalid --severity %q (expected <rule-id>=error|warning)", value)
		}
		id, level = strings.TrimSpace(id), strings.ToLower(strings.TrimSpace(level))
		switch lint.Severity(level) {
		case lint.SeverityError, lint.SeverityWarning:
			overrides[id] = lint.Severity(level)
		default:
			return nil, fmt.Errorf("invalid severity %q for rule %q (expected error or warning)", level, id)
		}
	}
	return overrides, nil
}

func ParseArgs(args []string) (Options, []string, error) {
	opts := DefaultOptions()
	flags := pflag.NewFlagSet("lint", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	BindFlags(flags, &opts)
	if err := flags.Parse(args); err != nil {
		return opts, nil, err
	}
	if opts.Raw {
		return opts, nil, fmt.Errorf("--raw must be handled before lint flag parsing")
	}
	if opts.SourceLines < 1 {
		return opts, nil, fmt.Errorf("--source-lines must be greater than zero")
	}
	if flags.Changed("source-lines") && !opts.Source {
		return opts, nil, fmt.Errorf("--source-lines requires --source")
	}
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "pretty"
	}
	switch opts.Format {
	case "pretty", "json":
	default:
		return opts, nil, fmt.Errorf("unsupported lint format %q (expected pretty or json)", opts.Format)
	}
	if opts.Source && opts.Format != "pretty" {
		return opts, nil, fmt.Errorf("--source is only supported with --format pretty")
	}
	packages := flags.Args()
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	return opts, packages, nil
}

func Render(w io.Writer, result *lint.Result, opts Options) error {
	if opts.Format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	view := lint.NewSummaryView(result, opts.SummaryLimit)
	if opts.Source {
		if err := view.LoadSource(opts.SourceLines); err != nil {
			return err
		}
	}
	formatOpts := formatters.FormatOptions{
		Format:  "tree",
		NoColor: opts.NoColor,
	}
	formatOpts.ResolveNoColor()
	out, err := clicky.Format(view, formatOpts)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_, err = fmt.Fprint(w, out)
	return err
}
