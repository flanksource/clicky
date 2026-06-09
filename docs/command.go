package docs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewCommand creates the docs command group with default configuration.
func NewCommand() *cobra.Command {
	return NewCommandWithConfig(nil)
}

// NewCommandWithConfig creates the docs command group. cfg supplies optional
// host defaults (title, intro, excluded commands, controller depth).
func NewCommandWithConfig(cfg *DocsConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate CLI and clicky-ui documentation",
		Long: `Generate a markdown CLI reference and a clicky-ui surface catalog from
this CLI's command tree.

With --output-dir, one markdown file per high-level command controller is written
directly into that directory. Generated reference pages are refreshed on every
run; hand-editable starter pages are written once and preserved unless --force is
passed.`,
	}
	cmd.AddCommand(newGenerateCommand(cfg))
	return cmd
}

func newGenerateCommand(cfg *DocsConfig) *cobra.Command {
	var (
		outputFile string
		outputDir  string
		format     string
		force      bool
		titleFlag  string
		descFlag   string
		depthFlag  int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate CLI + UI docs to a file, stdout, or a directory",
		Example: `  myapp docs generate                          # markdown to stdout
  myapp docs generate -o REFERENCE.md          # single file
  myapp docs generate --format json            # structured model as JSON
  myapp docs generate --output-dir ./docs      # one markdown file per controller, flat in ./docs
  myapp docs generate --output-dir ./docs --depth 2   # include grandchild commands
  myapp docs generate --output-dir ./docs --force   # also overwrite starter pages`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile != "" && outputDir != "" {
				return fmt.Errorf("--output and --output-dir cannot be used together")
			}

			effective := mergeConfig(cfg, titleFlag, descFlag, depthFlag, cmd.Flags().Changed("depth"))

			model, err := BuildModel(cmd.Root(), effective)
			if err != nil {
				return fmt.Errorf("failed to build docs model: %w", err)
			}

			if outputDir != "" {
				return runScaffold(cmd, model, outputDir, force)
			}

			content, err := RenderSingleFile(model, format)
			if err != nil {
				return err
			}
			return writeSingle(cmd, content, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write single-file output to this path (default: stdout)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Emit one markdown file per controller directly into this directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "Single-file format: markdown, json, yaml")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite write-once starter pages when writing to a directory")
	cmd.Flags().StringVar(&titleFlag, "title", "", "Override the docs title")
	cmd.Flags().StringVar(&descFlag, "description", "", "Override the docs description")
	cmd.Flags().IntVar(&depthFlag, "depth", defaultDepth, "Command levels below each high-level controller to document (1=controller + direct subcommands, 0=unlimited)")

	return cmd
}

func runScaffold(cmd *cobra.Command, model *Model, dir string, force bool) error {
	result, err := Scaffold(model, dir, force)
	if err != nil {
		return fmt.Errorf("failed to write docs: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Wrote docs to %s:\n", dir)
	for _, a := range result.Actions {
		fmt.Fprintf(out, "  %-12s %s\n", a.Status, a.Path)
	}
	return nil
}

func writeSingle(cmd *cobra.Command, content, outputFile string) error {
	if outputFile == "" {
		fmt.Fprint(cmd.OutOrStdout(), content)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputFile, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Documentation written to %s\n", outputFile)
	return nil
}

// mergeConfig overlays --title/--description/--depth flags onto the host
// config. depthChanged distinguishes an explicit --depth=0 (unlimited) from the
// flag's default, so the host config's Depth is only overridden when the user
// passed the flag.
func mergeConfig(base *DocsConfig, titleFlag, descFlag string, depthFlag int, depthChanged bool) *DocsConfig {
	merged := DocsConfig{}
	if base != nil {
		merged = *base
	}
	if titleFlag != "" {
		merged.Title = titleFlag
	}
	if descFlag != "" {
		merged.Description = descFlag
	}
	if depthChanged {
		if depthFlag <= 0 {
			merged.Depth = unlimitedDepth
		} else {
			merged.Depth = depthFlag
		}
	}
	return &merged
}
