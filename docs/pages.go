package docs

import "fmt"

// Page keys.
const (
	pageIndex          = "index"
	pageUISurfaces     = "ui-surfaces"
	pageGettingStarted = "getting-started"
	pageIntegration    = "clicky-ui-integration"
	// controllerPagePrefix namespaces the one-page-per-controller reference
	// files so their keys never collide with the starter/UI page keys.
	controllerPagePrefix = "commands/"
)

// buildPages assembles the full set of docs-site pages from the model. Two
// tiers: Generated pages (one per controller, ui-surfaces) are rewritten every
// run; starter pages (index, getting-started, clicky-ui-integration) are
// write-once. Each high-level controller gets its own reference page.
func buildPages(m *Model) []Page {
	pages := []Page{
		{
			Key:         pageIndex,
			Title:       m.Title,
			Description: firstLine(m.Description),
			Order:       0,
			Body:        renderIndex(m),
			Generated:   false,
		},
		{
			Key:         pageGettingStarted,
			Title:       "Getting Started",
			Description: "Install and run " + m.Title + ".",
			Order:       1,
			Body:        renderGettingStarted(m),
			Generated:   false,
		},
	}

	for i, ctrl := range m.Controllers {
		pages = append(pages, Page{
			Key:         controllerPagePrefix + ctrl.Name,
			Title:       ctrl.Name,
			Description: controllerDescription(ctrl),
			Order:       100 + i, // reference pages cluster after the starters
			Body:        RenderController(ctrl),
			Generated:   true,
		})
	}

	pages = append(pages,
		Page{
			Key:         pageUISurfaces,
			Title:       "UI Surface Catalog",
			Description: "clicky-ui surfaces, operations, and parameter widget roles.",
			Order:       200,
			Body:        RenderUISurfaces(m),
			Generated:   true,
		},
		Page{
			Key:         pageIntegration,
			Title:       "clicky-ui Integration",
			Description: "Wire " + m.Title + " into the clicky-ui web explorer.",
			Order:       201,
			Body:        renderIntegration(m),
			Generated:   false,
		},
	)
	return pages
}

func controllerDescription(ctrl ControllerDoc) string {
	if ctrl.Short != "" {
		return firstLine(ctrl.Short)
	}
	return fmt.Sprintf("Commands under `%s`.", ctrl.Name)
}

func renderIndex(m *Model) string {
	md := &mdBuilder{}
	md.heading(1, m.Title)
	md.para(m.Description)
	md.heading(2, "Documentation")
	md.line("- [Getting Started](./getting-started) — install and run.")
	md.line("- [UI Surface Catalog](./ui-surfaces) — what the web explorer exposes.")
	md.line("- [clicky-ui Integration](./clicky-ui-integration) — serve the web UI.")
	md.blank()

	md.heading(2, "Command Reference")
	for _, ctrl := range m.Controllers {
		line := fmt.Sprintf("- [%s](./%s)", ctrl.Name, controllerPagePrefix+ctrl.Name)
		if ctrl.Short != "" {
			line += " — " + firstLine(ctrl.Short)
		}
		md.line(line)
	}
	md.blank()
	return md.String()
}

func renderGettingStarted(m *Model) string {
	md := &mdBuilder{}
	md.heading(1, "Getting Started")
	md.para("This page is a starter you can edit; it is not overwritten by regenerating the docs unless you pass `--force`.")
	md.heading(2, "Install")
	md.codeBlock("sh", fmt.Sprintf("# install the %s CLI\n# (replace with your distribution method)", m.Title))
	md.heading(2, "Run")
	md.codeBlock("sh", m.Title+" --help")
	return md.String()
}

func renderIntegration(m *Model) string {
	md := &mdBuilder{}
	md.heading(1, "clicky-ui Integration")
	md.para("This page is a starter you can edit; regenerating the docs does not overwrite it unless you pass `--force`.")
	md.para("clicky-ui is a React component library that renders a metadata-driven explorer over a CLI's operations. It discovers operations from the OpenAPI spec and invokes them over HTTP.")

	md.heading(2, "Endpoints")
	md.table(
		[]string{"Path", "Purpose"},
		[][]string{
			{code("/api/openapi.json"), "OpenAPI spec (with x-clicky extensions) the UI reads to build the catalog."},
			{code("/api/v1/..."), "Executor endpoints the UI calls to run operations."},
			{code("/"), "The embedded React explorer."},
		},
	)

	md.heading(2, "Serving the UI")
	md.para("Wire the executor-backed OpenAPI server and the embedded explorer from the same command registrations:")
	md.codeBlock("go", integrationSnippet)

	md.heading(2, "Response envelope")
	md.para("Executor responses are wrapped in an `ExecutionResponse` envelope carrying `success`, `stdout`/`stderr`, `exit_code`, the structured `output`, and a `cli` field with the equivalent command for reproduction.")

	md.heading(2, "Surfaces")
	md.para("See the [UI Surface Catalog](./ui-surfaces) for the operations this CLI exposes, their endpoints, and parameter widget roles.")
	return md.String()
}

const integrationSnippet = `serveConfig := &rpc.ServeConfig{
    Host: host, Port: port,
    Title: "` + "" + `", Version: "1.0.0",
    Executor: &rpc.ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"},
}
server := rpc.NewSwaggerServer(serveConfig, rootCmd, openAPIConfig)

mux := http.NewServeMux()
server.RegisterRoutes(mux)        // /api/openapi.json + /api/v1/...
mux.Handle("/", uiHandler)        // embedded clicky-ui explorer
http.ListenAndServe(addr, mux)`

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
