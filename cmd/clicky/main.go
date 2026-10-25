package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/api/filter/gomplatefilter"
	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/flags/urlfetch"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/formatters/excel"
	"github.com/flanksource/clicky/formatters/pdf"
	"github.com/flanksource/clicky/internal/clirender"
)

// Build information (set by goreleaser)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	gomplatefilter.Use()
	urlfetch.Use()
	excel.Use()
	pdf.Use()
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var schemaFile string
	var inputFormat string
	var options formatters.FormatOptions

	rootCmd := &cobra.Command{
		Use:   "clicky",
		Short: "A CLI tool for formatting structured data and markdown",
		Long: `Clicky is a flexible CLI tool that formats structured data (JSON, YAML, etc.)
using YAML schema definitions. It supports multiple output formats including
pretty-printed tables, HTML, PDF, Markdown, and more. Schema-less stdin and
file input can be rendered with --input-format.

For backward compatibility, you can use the root command directly, or use the
'pretty' subcommand explicitly.`,
		Example: `  clicky --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  cat examples/kitchen-sink.md | clicky --input-format auto --format clicky-json
  clicky --input-format auto --format html examples/kitchen-sink.md
  clicky version`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && schemaFile == "" && inputFormat == "" && !hasStdinInput(cmd) {
				return fmt.Errorf("requires either a subcommand, data files with --schema flag, or --input-format for stdin")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			effectiveInputFormat := inputFormat
			if schemaFile == "" && effectiveInputFormat == "" && len(args) == 0 && hasStdinInput(cmd) {
				effectiveInputFormat = "auto"
			}
			if schemaFile == "" && effectiveInputFormat != "" {
				return clirender.RenderInput(args, effectiveInputFormat, renderOptions(cmd, options))
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required when using data files")
			}
			return clirender.RenderDataFiles(schemaFile, args, renderOptions(cmd, options))
		},
	}

	rootCmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema")
	rootCmd.Flags().StringVar(&inputFormat, "input-format", "", "Schema-less input format for stdin/files: auto, markdown/md, json, yaml/yml, text")
	formatters.BindPFlags(rootCmd.Flags(), &options)
	clicky.SetGroupedUsage(rootCmd)

	rootCmd.AddCommand(newPrettyCommand())
	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newSchemaCommand())
	rootCmd.AddCommand(newLintCommand())
	rootCmd.SetHelpCommand(newClickyHelpCommand())

	extensions.CobraExtensions(rootCmd).
		OpenAPICommand().
		MCPCommand()

	return rootCmd
}

func renderOptions(cmd *cobra.Command, options formatters.FormatOptions) clirender.Options {
	return clirender.Options{
		Stdin:  cmd.InOrStdin(),
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
		Format: options,
	}
}

func hasStdinInput(cmd *cobra.Command) bool {
	in := cmd.InOrStdin()
	if file, ok := in.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return false
		}
		return info.Mode()&os.ModeCharDevice == 0
	}
	return in != nil
}

func newPrettyCommand() *cobra.Command {
	var schemaFile string
	var options formatters.FormatOptions

	cmd := &cobra.Command{
		Use:   "pretty [flags] <file> [file...]",
		Short: "Format data files using a YAML schema",
		Long: `Format structured data files (JSON, YAML, etc.) using a YAML schema definition.

The pretty command is the main functionality of clicky, allowing you to transform
raw data into beautifully formatted output using customizable schemas.

For the full pretty-printing API guide, run:
  clicky help pretty`,
		Example: `  clicky pretty --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  clicky pretty --schema product-schema.yaml --format csv products.json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required")
			}
			return clirender.RenderDataFiles(schemaFile, args, renderOptions(cmd, options))
		},
	}

	cmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema (required)")
	if err := cmd.MarkFlagRequired("schema"); err != nil {
		panic(fmt.Sprintf("Failed to mark schema flag as required: %v", err))
	}
	formatters.BindPFlags(cmd.Flags(), &options)

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getVersionInfo())
		},
	}
}

func getVersionInfo() string {
	return fmt.Sprintf("clicky-schema %s (commit: %s, built: %s, go: %s)",
		version, commit, date, runtime.Version())
}
