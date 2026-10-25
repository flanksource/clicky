// Package clirender renders the clicky CLI's data inputs: schema-backed data
// files and schema-less stdin/file input, to stdout, stderr or file sinks.
package clirender

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/commons/logger"
)

// Options carries the command's streams and resolved format options.
type Options struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Format formatters.FormatOptions
}

type inputRenderer struct {
	Options
	manager     *formatters.FormatManager
	inputFormat string
}

// RenderInput reads schema-less stdin (when files is empty) or files, parses
// each according to inputFormat, and renders it through the Clicky formatter
// manager into every configured sink.
func RenderInput(files []string, inputFormat string, opts Options) error {
	normalized, err := normalizeInputFormat(inputFormat)
	if err != nil {
		return err
	}
	opts.Format.ResolveNoColor()
	opts.Format.Format = opts.Format.ResolveFormat()
	if err := opts.Format.ParseFormatSpec(); err != nil {
		return err
	}

	r := inputRenderer{Options: opts, manager: formatters.NewFormatManager(), inputFormat: normalized}
	if len(files) == 0 {
		data, err := io.ReadAll(opts.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		return r.render(data, "stdin")
	}

	for _, filename := range files {
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}
		if err := r.render(data, filename); err != nil {
			return fmt.Errorf("error processing %s: %w", filename, err)
		}
	}
	return nil
}

func (r inputRenderer) render(data []byte, sourceName string) error {
	parsed, detectedFormat, err := parseInputData(data, sourceName, r.inputFormat)
	if err != nil {
		return err
	}
	logger.Tracef("Parsed %s as %s\n", sourceName, detectedFormat)

	sinks := r.Format.Sinks
	if len(sinks) == 0 {
		sinks = []formatters.FormatSink{{Format: r.Format.ResolveFormat()}}
	}
	for _, sink := range sinks {
		if err := r.renderSink(parsed, sourceName, sink); err != nil {
			return err
		}
	}
	return nil
}

func (r inputRenderer) renderSink(value any, sourceName string, sink formatters.FormatSink) error {
	sinkOpts := r.Format
	sinkOpts.Sinks = nil
	sinkOpts.Format = sink.Format
	sinkOpts.Output = sink.File
	if sinkOpts.Output == "" {
		sinkOpts.Output = r.Format.Output
	}

	switch sinkOpts.Output {
	case "", "stdout", "-", "/dev/stdout":
		return r.renderStream(r.Stdout, sinkOpts, value)
	case "stderr", "/dev/stderr":
		return r.renderStream(r.Stderr, sinkOpts, value)
	default:
		return r.renderFile(resolveInputOutputPath(sinkOpts.Output, sourceName, sinkOpts.Format), sinkOpts, value)
	}
}

func (r inputRenderer) renderStream(w io.Writer, sinkOpts formatters.FormatOptions, value any) error {
	sinkOpts.Output = ""
	if isFileOnlyFormat(sinkOpts.Format) {
		return fmt.Errorf("%s output requires --output or a format=file sink", sinkOpts.Format)
	}
	output, err := r.manager.FormatWithOptions(sinkOpts, value)
	if err != nil {
		return fmt.Errorf("failed to format %s output: %w", sinkOpts.Format, err)
	}
	if sinkOpts.Format == "pretty" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	_, err = fmt.Fprint(w, output)
	return err
}

func (r inputRenderer) renderFile(outputFile string, sinkOpts formatters.FormatOptions, value any) error {
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if isFileOnlyFormat(sinkOpts.Format) {
		sinkOpts.Output = outputFile
		if err := r.manager.FormatToFile(sinkOpts, value); err != nil {
			return fmt.Errorf("failed to write %s output: %w", sinkOpts.Format, err)
		}
		return nil
	}
	sinkOpts.Output = ""
	output, err := r.manager.FormatWithOptions(sinkOpts, value)
	if err != nil {
		return fmt.Errorf("failed to format %s output: %w", sinkOpts.Format, err)
	}
	if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}
	logger.Tracef("Output written to %s\n", outputFile)
	return nil
}
