package entitydemo

import (
	"io/fs"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/docs"
	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/mcp"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

type CommandOptions struct {
	EmbeddedWebapp      fs.FS
	ResolveWebappDevDir func() (string, error)
}

func NewCommand(options CommandOptions) *cobra.Command {
	store := newDemoStore()
	rootCmd := &cobra.Command{
		Use:   "entity-demo",
		Short: "Entity example covering clicky entity generation and served execution",
		Long: `A self-contained example showing how clicky entities can power both a CLI
and the executor-backed OpenAPI serve mode from the same registrations.`,
	}

	registerEntities(store)
	registerSubCommands(store)
	clicky.GenerateCLI(rootCmd)

	openAPIConfig := &rpc.OpenAPIConfig{
		Title:       "Clicky Entity Example",
		Description: "Entity example app covering CRUD, actions, filters, admin views, nested parents, and executor-backed serve mode.",
		Version:     "1.0.0",
		Tags: []rpc.OpenAPITag{
			{Name: "stack", Description: "Stack entity operations"},
			{Name: "catalog", Description: "Nested catalog entity operations"},
			{Name: "admin", Description: "Administrative entity operations"},
		},
	}
	extensions.CobraExtensions(rootCmd).
		OpenAPICommandWithConfig(openAPIConfig).
		ServeCommandWithConfig(openAPIConfig)

	extensions.CobraExtensions(rootCmd).DocsCommandWithConfig(&docs.DocsConfig{
		Title:       "Clicky Entity Example",
		Description: "Entity example app with a CLI reference and clicky-ui surface catalog.",
		Exclude:     []string{"serve-ui"},
	})

	rootCmd.AddCommand(newServeUICommand(options))
	rootCmd.AddCommand(
		mcp.NewMcpServer(rootCmd).
			AutoExpose().
			WithExclude("serve", "serve-ui").
			IgnoreParams("*", "--host", "--port").
			WithFormat(formatters.FormatOptions{Markdown: true, NoColor: true}).
			Command(),
	)
	return rootCmd
}
