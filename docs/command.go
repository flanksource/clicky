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
// host defaults (title, intro, excluded commands, default provider).
func NewCommandWithConfig(cfg *DocsConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate CLI and clicky-ui documentation",
		Long: `Generate a markdown CLI reference and a clicky-ui surface catalog from
this CLI's command tree, or scaffold a docs site around them.

With --output-dir, docs are written as a docs-site folder for a provider
(default: astro / Astro Starlight). Generated reference pages are refreshed on
every run; hand-editable starter pages are written once and preserved unless
--force is passed.`,
	}
	cmd.AddCommand(newGenerateCommand(cfg))
	return cmd
}

func newGenerateCommand(cfg *DocsConfig) *cobra.Command {
	var (
		outputFile string
		outputDir  string
		format     string
		provider   string
		basePath   string
		force      bool
		titleFlag  string
		descFlag   string
		depthFlag  int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate CLI + UI docs to a file, stdout, or a docs-site directory",
		Example: `  myapp docs generate                          # markdown to stdout
  myapp docs generate -o REFERENCE.md          # single file
  myapp docs generate --format json            # structured model as JSON
  myapp docs generate --output-dir ./docs      # scaffold astro docs site
  myapp docs generate --output-dir ./site --base-path reference   # into an existing site
  myapp docs generate --output-dir ./docs --force   # also overwrite starter pages`,
		RunE: func(cmd *cobra.Command, args []string) error {
			effective := mergeConfig(cfg, titleFlag, descFlag, depthFlag, cmd.Flags().Changed("depth"))

			model, err := BuildModel(cmd.Root(), effective)
			if err != nil {
				return fmt.Errorf("failed to build docs model: %w", err)
			}

			if outputDir != "" {
				return runScaffold(cmd, model, outputDir, providerName(effective, provider, cmd), basePath, force)
			}

			content, err := RenderSingleFile(model, format)
			if err != nil {
				return err
			}
			return writeSingle(cmd, content, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write single-file output to this path (default: stdout)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Scaffold a docs-site directory at this path")
	cmd.Flags().StringVar(&format, "format", "markdown", "Single-file format: markdown, json, yaml")
	cmd.Flags().StringVar(&provider, "provider", "", "Docs-site provider for --output-dir (default: astro)")
	cmd.Flags().StringVar(&basePath, "base-path", "", "Subdirectory within the provider's content root to write pages into (e.g. reference)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite write-once starter pages when scaffolding")
	cmd.Flags().StringVar(&titleFlag, "title", "", "Override the docs title")
	cmd.Flags().StringVar(&descFlag, "description", "", "Override the docs description")
	cmd.Flags().IntVar(&depthFlag, "depth", defaultDepth, "Command levels below each high-level controller to document (1=controller + direct subcommands, 0=unlimited)")

	return cmd
}

func runScaffold(cmd *cobra.Command, model *Model, dir, providerName, basePath string, force bool) error {
	provider, err := providerFor(providerName, basePath)
	if err != nil {
		return err
	}
	result, err := Scaffold(model, dir, provider, force)
	if err != nil {
		return fmt.Errorf("failed to scaffold docs: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Scaffolded %s docs in %s:\n", provider.Name(), dir)
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

// providerName resolves the provider: explicit --provider wins, then config
// default, then "astro".
func providerName(cfg *DocsConfig, flag string, cmd *cobra.Command) string {
	if cmd.Flags().Changed("provider") && flag != "" {
		return flag
	}
	return cfg.defaultProvider()
}
