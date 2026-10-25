package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/internal/clihelp"
	"github.com/flanksource/clicky/internal/lintcli"
)

func newClickyHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Use "clicky help pretty" for the full pretty-printing API guide.`,
		Args: cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 1 && args[0] == "pretty" {
				cobra.CheckErr(clihelp.RenderPrettyGuide(cmd.OutOrStdout(), cmd.Root()))
				return
			}

			target, _, err := cmd.Root().Find(args)
			if target == nil || err != nil {
				cmd.Printf("Unknown help topic %#q\n", args)
				cobra.CheckErr(cmd.Root().Usage())
				return
			}

			target.InitDefaultHelpFlag()
			target.InitDefaultVersionFlag()
			cobra.CheckErr(target.Help())
		},
	}
}

func newLintCommand() *cobra.Command {
	helpOpts := lintcli.DefaultOptions()
	cmd := &cobra.Command{
		Use:   "lint [flags] [packages...]",
		Short: "Run the clicky API linter on Go packages (defaults to ./...)",
		Long: `Run the clicky API linter (clickylint) on the given Go packages.

Detects bad usage patterns of the clicky text API, such as incorrect
composite literal usage of api.Text, direct stdout writes, and calling
.ANSI()/.HTML()/.Markdown()/.String() inside Pretty/render-builder code.

By default, clicky lint renders a colored tree summary grouped by rule and
affected file. Use --source to include source excerpts in the summary and
--source-lines to control their total line count. Use --format json for
structured output, or --raw / -- for the underlying go/analysis driver flags
(-json, -fix, -c=N, etc.).

If no packages are given, defaults to ./... in the current directory.`,
		Example: `  clicky lint ./...
  clicky lint ./pkg/foo ./pkg/bar
  clicky lint --summary-limit 2 ./...
  clicky lint --source --source-lines 3 ./...
  clicky lint --format json ./...
  clicky lint --severity manual-cobra-command=warning ./...
  clicky lint -- -json ./...`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rawArgs, raw := lintcli.RawArgs(args); raw {
				if lintcli.SourceFlagPresent(args) {
					return fmt.Errorf("--source and --source-lines cannot be used with raw analyzer output")
				}
				return lintcli.RunRaw(rawArgs)
			}
			if lintcli.HelpRequested(args) {
				return cmd.Help()
			}
			return lintcli.Run(cmd.OutOrStdout(), args)
		},
	}
	lintcli.BindFlags(cmd.Flags(), &helpOpts)
	return cmd
}
