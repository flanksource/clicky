package entitydemo

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	capchat "github.com/flanksource/captain/pkg/aichat"
	"github.com/flanksource/captain/pkg/aiflags"
	capapi "github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/clicky"
	clickyaichat "github.com/flanksource/clicky/aichat"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/markdown"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// newServeUICommand wires the clicky RPC executor together with the Vite
// webapp supplied by the entrypoint. The webapp consumes `@flanksource/clicky-ui`'s
// metadata-driven `EntityExplorerApp` against `/api/openapi.json` and `/api/v1/...`.
func newServeUICommand(options CommandOptions) *cobra.Command {
	var (
		host   string
		port   int
		dev    bool
		uiPort int
	)

	cmd := &cobra.Command{
		Use:   "serve-ui",
		Short: "Start the HTTP API and embedded operation-catalog UI",
		Long: `Start an HTTP server that exposes both the executor-backed OpenAPI endpoints
and the embedded React UI built from clicky-ui's metadata-driven entity explorer.

The API is served at /api/openapi.json + /api/v1/..., the UI at /. Build the
Vite frontend with ` + "`cd webapp && pnpm install && pnpm build`" + ` before
compiling the Go binary so the embedded assets are current.

With --dev, this command additionally launches the Vite dev server (HMR) from
webapp/, which resolves @flanksource/clicky-ui from the sibling checked-out
clicky-ui repo (../../../../clicky-ui/packages/ui/dist) and proxies /api back to
this Go process. Open the printed Vite URL to develop against local clicky-ui
source — no embedded rebuild needed. Requires a source checkout with pnpm
available and ` + "`pnpm install`" + ` already run in webapp/.`,
		Example: `  entity-demo serve-ui --port 8080
  entity-demo serve-ui --host 0.0.0.0 --port 9090
  entity-demo serve-ui --dev               # API + Vite HMR against local clicky-ui`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port: %d", port)
			}

			rootCmd := cmd.Root()
			openAPIConfig := &rpc.OpenAPIConfig{
				Title:       "Clicky Entity Example",
				Description: "Entity example app with embedded metadata-driven explorer UI.",
				Version:     "1.0.0",
			}
			serveConfig := &rpc.ServeConfig{
				Host:    host,
				Port:    port,
				Title:   openAPIConfig.Title,
				Version: openAPIConfig.Version,
				Executor: &rpc.ExecutorConfig{
					Enabled:    true,
					SkipPreRun: true,
					PathPrefix: "/api/v1",
				},
			}

			server := rpc.NewSwaggerServer(serveConfig, rootCmd, openAPIConfig)

			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			mux.HandleFunc("/api/examples/links", serveLinkExamples)
			mux.HandleFunc("/api/examples/markdown-preview", serveMarkdownPreview)

			// AI chat backend: the demo's own entity operations become tools.
			// Requires a provider key (ANTHROPIC_API_KEY / OPENAI_API_KEY /
			// GOOGLE_API_KEY); it fails loud on the first request otherwise.
			chatTools, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: rootCmd})
			if err != nil {
				return err
			}
			chat := capchat.NewService(capchat.ServiceOptions{
				Profile: capchat.RuntimeProfileProviderFunc(captainRuntimeProfile),
				Tools:   chatTools, Threads: capchat.FixedThreadStore(capchat.NewMemoryThreadStore()),
			})
			// Mount as a subtree so /api/chat, /api/chat/models and the thread
			// endpoints all resolve.
			mux.Handle("/api/chat", chat.Handler())
			mux.Handle("/api/chat/", chat.Handler())

			uiHandler, err := newWebappHandler(options.EmbeddedWebapp)
			if err != nil {
				return fmt.Errorf("load embedded webapp: %w", err)
			}
			mux.Handle("/", uiHandler)

			addr := fmt.Sprintf("%s:%d", host, port)
			httpSrv := &http.Server{
				Addr:        addr,
				Handler:     mux,
				ReadTimeout: 30 * time.Second,
				// No WriteTimeout: /api/chat streams SSE responses that stay open
				// well past any fixed deadline; a write timeout truncates them.
				IdleTimeout: 60 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				fmt.Fprintf(cmd.OutOrStdout(), "🚀 Entity UI listening on http://%s\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • UI:           http://%s/\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • OpenAPI JSON: http://%s/api/openapi.json\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • Executor API: http://%s/api/v1/...\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • AI Chat:      http://%s/api/chat\n", addr)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			if dev {
				vite, err := startViteDevServer(viteDevServerOptions{
					Context: ctx, Command: cmd, APIHost: host, APIPort: port, UIPort: uiPort,
					ResolveWebappDir: options.ResolveWebappDevDir,
				})
				if err != nil {
					return err
				}
				// ctx cancellation (Ctrl-C) kills the Vite process group via the
				// CommandContext Cancel hook; Wait reaps it on shutdown.
				defer func() { _ = vite.Wait() }()
			}

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return httpSrv.Shutdown(shutdownCtx)
			case err := <-errCh:
				return err
			}
		},
	}

	cmd.Flags().StringVar(&host, "host", "localhost", "Host to bind the server to")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to bind the server to")
	cmd.Flags().BoolVar(&dev, "dev", false, "Launch the Vite dev server (HMR) against the checked-out clicky-ui instead of using the embedded build")
	cmd.Flags().IntVar(&uiPort, "ui-port", 5173, "Port for the Vite dev server (only used with --dev)")

	return cmd
}

func captainRuntimeProfile(_ context.Context, selection capchat.RuntimeProfileSelection) (capchat.RuntimeProfile, error) {
	if selection.Ref != "" {
		return capchat.RuntimeProfile{}, capchat.RequestError(
			http.StatusBadRequest,
			fmt.Sprintf("runtime profile %q is not available; the entity demo serves its default profile", selection.Ref),
		)
	}
	defaults, err := aiflags.LoadDefaults()
	if err != nil {
		return capchat.RuntimeProfile{}, fmt.Errorf("load Captain AI defaults: %w", err)
	}
	model, err := aiflags.ApplyDefaults(capapi.Model{}, defaults)
	if err != nil {
		return capchat.RuntimeProfile{}, fmt.Errorf("resolve Captain AI defaults: %w", err)
	}
	// The profile is a resolved stack of named layers rather than a bare spec, so
	// even a single-layer demo declares its one layer explicitly.
	resolved, err := capapi.ResolveSpecLayers(capapi.SpecLayer{
		Name: "entity demo", Scope: capapi.SpecLayerGlobal,
		Spec: capapi.Spec{Model: model},
	})
	if err != nil {
		return capchat.RuntimeProfile{}, fmt.Errorf("resolve Captain runtime profile: %w", err)
	}
	return capchat.RuntimeProfile{
		Resolved: resolved,
		System: "You are an operator assistant for this entity demo " +
			"(stacks, clusters, teams). Prefer calling an operation over " +
			"guessing, and summarize results clearly.",
	}, nil
}

// startViteDevServer launches `pnpm dev` in webapp/ for --dev mode. It points
// Vite's /api proxy back at this Go process via CLICKY_EXAMPLE_API_URL (read by
// webapp/vite.config.ts), so the HMR dev server — which resolves
// @flanksource/clicky-ui from the sibling checked-out clicky-ui repo — talks to
// the live executor API. The returned command is bound to ctx: Ctrl-C kills the
// whole Vite process group (pnpm + its node/esbuild children) and the caller
// reaps it with Wait.
type viteDevServerOptions struct {
	Context          context.Context
	Command          *cobra.Command
	APIHost          string
	APIPort          int
	UIPort           int
	ResolveWebappDir func() (string, error)
}

func startViteDevServer(options viteDevServerOptions) (*exec.Cmd, error) {
	if options.ResolveWebappDir == nil {
		return nil, fmt.Errorf("webapp source directory resolver is required for --dev")
	}
	webappDir, err := options.ResolveWebappDir()
	if err != nil {
		return nil, err
	}
	apiURL := fmt.Sprintf("http://%s:%d", options.APIHost, options.APIPort)

	// `pnpm exec vite` runs the dev server binary directly so --port/--strictPort
	// reach Vite; `pnpm dev -- ...` would forward them past Vite's arg parser and
	// it would silently fall back to the default port.
	vite := exec.CommandContext(options.Context, "pnpm", "exec", "vite", "--port", strconv.Itoa(options.UIPort), "--strictPort")
	vite.Dir = webappDir
	vite.Env = append(os.Environ(), "CLICKY_EXAMPLE_API_URL="+apiURL)
	vite.Stdout = options.Command.OutOrStdout()
	vite.Stderr = options.Command.ErrOrStderr()
	// Run Vite in its own process group so we can signal the whole tree; the
	// default CommandContext kill only targets pnpm and would orphan node/vite.
	vite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	vite.Cancel = func() error {
		if vite.Process == nil {
			return nil
		}
		return syscall.Kill(-vite.Process.Pid, syscall.SIGTERM)
	}
	vite.WaitDelay = 5 * time.Second

	if err := vite.Start(); err != nil {
		return nil, fmt.Errorf("start vite dev server in %s (is pnpm installed and `pnpm install` run there?): %w", webappDir, err)
	}

	fmt.Fprintf(options.Command.OutOrStdout(), "   • Dev UI (Vite): http://localhost:%d/  (HMR, clicky-ui from ../../../../clicky-ui, /api → %s)\n", options.UIPort, apiURL)
	return vite, nil
}

// newWebappHandler returns an http.Handler that serves the embedded Vite
// build. Unknown paths fall back to index.html so the React router can
// handle client-side routes on a full page load.
func newWebappHandler(webapp fs.FS) (http.Handler, error) {
	if webapp == nil {
		return nil, fmt.Errorf("embedded webapp filesystem is required")
	}
	sub, err := fs.Sub(webapp, "webapp/dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(r.URL.Path, "/")
		if requested == "" {
			serveIndex(w, sub)
			return
		}
		if _, err := fs.Stat(sub, requested); err != nil {
			// File not found: assume a SPA route and return index.html.
			if !looksLikeAssetRequest(requested) {
				serveIndex(w, sub)
				return
			}
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func serveIndex(w http.ResponseWriter, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "webapp index.html missing — run `pnpm build` in webapp/", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func serveLinkExamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := clicky.Format(linkExamplesDocument(), clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render link examples: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json+clicky")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(payload))
}

func serveMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	source, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read markdown: %v", err), http.StatusBadRequest)
		return
	}
	doc, err := markdown.ParseString(string(source))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse markdown: %v", err), http.StatusBadRequest)
		return
	}

	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "clicky-json"
	}
	format = normalizeMarkdownPreviewFormat(format)
	if !markdownPreviewFormats[format] {
		http.Error(w, fmt.Sprintf("unsupported format: %s", format), http.StatusBadRequest)
		return
	}

	if format == "excel" {
		serveMarkdownPreviewExcel(w, markdownPreviewRows(doc))
		return
	}

	payloadData := any(doc)
	if format == "csv" {
		payloadData = markdownPreviewRows(doc)
	}
	payload, err := clicky.Format(payloadData, clicky.FormatOptions{Format: format})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render %s preview: %v", format, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", markdownPreviewContentType(format))
	if _, err := w.Write([]byte(payload)); err != nil {
		fmt.Fprintf(os.Stderr, "write markdown preview response: %v\n", err)
	}
}

func serveMarkdownPreviewExcel(w http.ResponseWriter, rows []markdownPreviewRow) {
	tmpDir, err := os.MkdirTemp("", "clicky-markdown-preview-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create excel preview: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	output := filepath.Join(tmpDir, "markdown-preview.xlsx")
	manager := formatters.NewFormatManager()
	if err := manager.ExcelToFile(rows, output); err != nil {
		http.Error(w, fmt.Sprintf("failed to render excel preview: %v", err), http.StatusInternalServerError)
		return
	}
	data, err := os.ReadFile(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read excel preview: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", clicky.FormatToContentType("excel"))
	w.Header().Set("Content-Disposition", `inline; filename="markdown-preview.xlsx"`)
	_, _ = w.Write(data)
}

// markdownPreviewFormats is the set of formats the preview endpoint accepts
// (after normalization). It mirrors clicky's known render formats so a bad
// client-supplied ?format= is rejected as 400 instead of surfacing as a 500.
var markdownPreviewFormats = map[string]bool{
	"clicky-json": true,
	"json":        true,
	"yaml":        true,
	"yml":         true,
	"csv":         true,
	"html":        true,
	"html-react":  true,
	"html-static": true,
	"markdown":    true,
	"pdf":         true,
	"slack":       true,
	"excel":       true,
	"tree":        true,
	"pretty":      true,
}
