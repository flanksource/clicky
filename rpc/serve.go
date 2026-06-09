package rpc

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed assets/*
var assets embed.FS

// ExecutorConfig holds configuration for command execution
type ExecutorConfig struct {
	Enabled    bool   // Enable dynamic command execution
	SkipPreRun bool   // Skip pre-run hooks during execution
	PathPrefix string // Path prefix for execution endpoints (defaults to /api/v1)
}

// ServeConfig holds configuration for the OpenAPI serve command
type ServeConfig struct {
	Host        string
	Port        int
	Title       string
	Description string
	Version     string
	AutoRefresh bool
	Open        bool
	SkipHealth  bool            // Skip registering /health (caller provides their own)
	Executor    *ExecutorConfig // Optional command execution configuration
}

// SwaggerServer serves API reference documentation for the OpenAPI specification.
type SwaggerServer struct {
	config       *ServeConfig
	rootCmd      *cobra.Command
	generator    *OpenAPIGenerator
	converterCfg *Config // Converter config used to build executor routes; reused when (re)generating the spec.
	server       *http.Server
	executor     *CommandExecutor // Optional command executor
}

// TemplateData holds data for rendering the HTML template
type TemplateData struct {
	Title       string
	Description string
	Version     string
	Timestamp   string
	AutoRefresh bool
}

// NewSwaggerServer creates a new OpenAPI documentation server
func NewSwaggerServer(config *ServeConfig, rootCmd *cobra.Command, openAPIConfig *OpenAPIConfig) *SwaggerServer {
	generator := NewOpenAPIGenerator(openAPIConfig)

	server := &SwaggerServer{
		config:    config,
		rootCmd:   rootCmd,
		generator: generator,
	}

	// Initialize executor if enabled. Honor ExecutorConfig.PathPrefix so the
	// converter — which is the source of both the OpenAPI paths and the
	// ServeMux patterns — mounts the executor under the configured prefix
	// instead of the hardcoded "/api/v1". Without this override the prefix
	// only affected the runtime executor and the registered routes silently
	// stayed under "/api/v1", colliding with anything else mounted there.
	if config.Executor != nil && config.Executor.Enabled {
		server.converterCfg = DefaultConfig()
		if prefix := strings.TrimRight(config.Executor.PathPrefix, "/"); prefix != "" {
			server.converterCfg.PathPrefix = prefix
		}
		converter := NewConverter(server.converterCfg)
		service, err := converter.ConvertCommandTree(rootCmd)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to build executor command tree: %v\n", err)
		} else {
			server.executor = NewCommandExecutor(service, config.Executor)
		}
	}

	return server
}

// Start starts the HTTP server
// RegisterRoutes registers all API routes onto the provided mux.
// This allows callers to compose the SwaggerServer routes with other handlers.
func (s *SwaggerServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/openapi.json", s.handleOpenAPIJSON)
	mux.HandleFunc("/api/openapi.yaml", s.handleOpenAPIYAML)
	mux.HandleFunc("/api/entities", s.handleEntities)
	if !s.config.SkipHealth {
		mux.HandleFunc("/health", s.handleHealth)
	}

	if s.executor != nil {
		s.registerExecutionRoutes(mux)
	}
}

// HandleOpenAPIJSON serves the OpenAPI 3 spec as JSON. Exported so callers
// that compose their own mux (and want to wrap or override this endpoint)
// can re-mount it without going through RegisterRoutes.
func (s *SwaggerServer) HandleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	s.handleOpenAPIJSON(w, r)
}

// HandleOpenAPIYAML serves the OpenAPI 3 spec as YAML. See HandleOpenAPIJSON.
func (s *SwaggerServer) HandleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	s.handleOpenAPIYAML(w, r)
}

// HandleEntities serves the entity metadata endpoint. Exported so callers
// composing their own mux can register it independently of RegisterRoutes.
func (s *SwaggerServer) HandleEntities(w http.ResponseWriter, r *http.Request) {
	s.handleEntities(w, r)
}

// HandleHealth serves the liveness probe. Exported for the same reason as
// HandleEntities.
func (s *SwaggerServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	s.handleHealth(w, r)
}

// ConverterConfig returns the converter Config used to build the executor's
// routes (path prefix, default method, etc.). Callers that re-run
// GenerateFromCobraWithConfig outside this server — e.g. to merge a separate
// OpenAPI document with the executor spec — should pass this value so the
// regenerated paths match the actually-registered ServeMux patterns. Returns
// nil when the server was constructed without an executor.
func (s *SwaggerServer) ConverterConfig() *Config {
	return s.converterCfg
}

// RegisterExecutionRoutes registers only the dynamic executor patterns
// derived from the RPC service (one method+path per operation). Exported so
// callers that want everything from RegisterRoutes except the openapi
// handlers (because they intend to wrap /api/openapi.json with a merge
// across multiple specs) can opt in route-by-route.
func (s *SwaggerServer) RegisterExecutionRoutes(mux *http.ServeMux) {
	if s.executor != nil {
		s.registerExecutionRoutes(mux)
	}
}

func (s *SwaggerServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/", s.handleSwaggerUI)
	s.RegisterRoutes(mux)

	// Create server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		fmt.Printf("🚀 OpenAPI documentation server starting on http://%s\n", addr)
		fmt.Printf("📖 API explorer available at: http://%s/\n", addr)
		fmt.Printf("📄 OpenAPI JSON spec: http://%s/api/openapi.json\n", addr)
		fmt.Printf("📄 OpenAPI YAML spec: http://%s/api/openapi.yaml\n", addr)
		if !s.config.SkipHealth {
			fmt.Printf("💊 Health check: http://%s/health\n", addr)
		}
		fmt.Println("Press Ctrl+C to stop the server")

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Open browser if requested
	if s.config.Open {
		go func() {
			time.Sleep(500 * time.Millisecond) // Give server time to start
			url := fmt.Sprintf("http://%s", addr)
			if err := OpenBrowser(url); err != nil {
				fmt.Printf("⚠️  Failed to open browser: %v\n", err)
				fmt.Printf("📖 Please manually open: %s\n", url)
			} else {
				fmt.Printf("🌐 Opening browser: %s\n", url)
			}
		}()
	}

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		fmt.Println("\n🛑 Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}

// handleSwaggerUI serves the API explorer HTML page.
func (s *SwaggerServer) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Read and parse the HTML template
	htmlContent, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read template: %v", err), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("swagger").Parse(string(htmlContent))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := TemplateData{
		Title:       s.config.Title,
		Description: s.config.Description,
		Version:     s.config.Version,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05 UTC"),
		AutoRefresh: s.config.AutoRefresh,
	}

	// Set content type and render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleOpenAPIJSON serves the OpenAPI specification in JSON format
func (s *SwaggerServer) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	spec, err := s.generator.GenerateFromCobraWithConfig(s.rootCmd, s.converterCfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI spec: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spec); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleOpenAPIYAML serves the OpenAPI specification in YAML format
func (s *SwaggerServer) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	spec, err := s.generator.GenerateFromCobraWithConfig(s.rootCmd, s.converterCfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI spec: %v", err), http.StatusInternalServerError)
		return
	}

	yamlData, err := yaml.Marshal(spec)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal YAML: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	if _, err := w.Write(yamlData); err != nil {
		// Log error but response already started
		fmt.Printf("Warning: failed to write YAML response: %v\n", err)
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Server    string `json:"server"`
	Version   string `json:"version"`
}

// handleHealth serves a simple health check endpoint
func (s *SwaggerServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Server:    "OpenAPI Documentation Server",
		Version:   s.config.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(health); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode health response: %v", err), http.StatusInternalServerError)
	}
}

// OpenBrowser opens the default browser to the specified URL
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// DefaultServeConfig returns default configuration for the serve command
func DefaultServeConfig() *ServeConfig {
	return &ServeConfig{
		Host:        "localhost",
		Port:        8080,
		Title:       "CLI API",
		Description: "Generated API documentation from CLI commands",
		Version:     "1.0.0",
		AutoRefresh: false,
		Open:        false,
	}
}

// newOpenAPIServeCommand creates the serve subcommand
func newOpenAPIServeCommand(defaultConfig *OpenAPIConfig) *cobra.Command {
	serveConfig := DefaultServeConfig()

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP server with interactive API documentation",
		Long: `Start an HTTP server that serves interactive API documentation for the CLI.

This command generates an OpenAPI specification from the current CLI command structure
and serves it through a web interface. The documentation is
generated dynamically and reflects the current state of the CLI commands.`,
		Example: `  myapp openapi serve
  myapp openapi serve --port 3000 --open
  myapp openapi serve --title "My API" --description "Custom API documentation"
  myapp openapi serve --host 0.0.0.0 --port 8080 --auto-refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the root command to convert
			rootCmd := cmd.Root()

			// Validate configuration
			if serveConfig.Port < 1 || serveConfig.Port > 65535 {
				return fmt.Errorf("invalid port number: %d (must be between 1 and 65535)", serveConfig.Port)
			}

			if strings.TrimSpace(serveConfig.Host) == "" {
				return fmt.Errorf("host cannot be empty")
			}

			// Create OpenAPI config from serve config and defaults
			openAPIConfig := &OpenAPIConfig{
				Title:       serveConfig.Title,
				Description: serveConfig.Description,
				Version:     serveConfig.Version,
			}

			// Apply defaults if provided
			if defaultConfig != nil {
				if defaultConfig.Contact != nil {
					openAPIConfig.Contact = defaultConfig.Contact
				}
				if defaultConfig.License != nil {
					openAPIConfig.License = defaultConfig.License
				}
				if len(defaultConfig.Servers) > 0 {
					openAPIConfig.Servers = defaultConfig.Servers
				}
				if len(defaultConfig.Tags) > 0 {
					openAPIConfig.Tags = defaultConfig.Tags
				}
			}

			// Create and start the server
			server := NewSwaggerServer(serveConfig, rootCmd, openAPIConfig)
			return server.Start(cmd.Context())
		},
	}

	// Add flags
	cmd.Flags().StringVar(&serveConfig.Host, "host", serveConfig.Host, "Host to bind the server to")
	cmd.Flags().IntVarP(&serveConfig.Port, "port", "p", serveConfig.Port, "Port to bind the server to")
	cmd.Flags().StringVar(&serveConfig.Title, "title", serveConfig.Title, "API documentation title")
	cmd.Flags().StringVar(&serveConfig.Description, "description", serveConfig.Description, "API documentation description")
	cmd.Flags().StringVar(&serveConfig.Version, "version", serveConfig.Version, "API version")
	cmd.Flags().BoolVar(&serveConfig.AutoRefresh, "auto-refresh", serveConfig.AutoRefresh, "Enable auto-refresh of documentation")
	cmd.Flags().BoolVar(&serveConfig.Open, "open", serveConfig.Open, "Automatically open browser")

	// Add executor flags
	var enableExecutor bool
	var skipPreRun bool
	cmd.Flags().BoolVar(&enableExecutor, "enable-executor", false, "Enable dynamic command execution via HTTP endpoints")
	cmd.Flags().BoolVar(&skipPreRun, "skip-pre-run", true, "Skip pre-run hooks during command execution")

	// Set up executor config when flags are parsed
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if enableExecutor {
			serveConfig.Executor = &ExecutorConfig{
				Enabled:    enableExecutor,
				SkipPreRun: skipPreRun,
				PathPrefix: "/api/v1",
			}
		}
		return nil
	}

	return cmd
}

// registerExecutionRoutes registers dynamic command execution routes based on the RPC service
func (s *SwaggerServer) registerExecutionRoutes(mux *http.ServeMux) {
	if s.executor == nil || s.executor.service == nil {
		fmt.Printf("⚠️  Warning: Executor has no service; no routes registered\n")
		return
	}

	registered := make(map[string]string)
	routeCount := 0

	registerRoute := func(method, path, opName string) bool {
		sanitized, ok := sanitizePathParams(path)
		if !ok {
			fmt.Printf("⚠️  Warning: Skipping route with invalid path params: %s %s\n", method, path)
			return false
		}

		pattern := method + " " + sanitized
		if existingOp, found := registered[pattern]; found {
			fmt.Printf("⚠️  Warning: Duplicate endpoint detected\n")
			fmt.Printf("    Path: %s\n", pattern)
			fmt.Printf("    Already registered by: %s\n", existingOp)
			fmt.Printf("    Skipping: %s\n", opName)
			return false
		}

		registered[pattern] = opName
		mux.HandleFunc(pattern, s.handleExecuteCommand)
		routeCount++
		return true
	}

	// Register a route for each operation
	for _, op := range s.executor.service.Operations {
		path := op.Path
		method := strings.ToUpper(op.Method)

		// Replace spaces in path segments with hyphens
		path = strings.ReplaceAll(path, " ", "-")
		registerRoute(method, path, op.Name)

		if !hasLookup(&op) {
			continue
		}

		registerRoute(http.MethodHead, path, op.Name+" lookup")
		if !strings.EqualFold(method, http.MethodGet) {
			registerRoute(http.MethodGet, path, op.Name+" lookup")
		}
	}
	fmt.Printf("✅ Registered %d executor routes\n", routeCount)
}

// sanitizePathParams replaces invalid characters in {param} names with underscores
// and spaces with hyphens, since Go's ServeMux requires alphanumeric wildcard names.
// Returns the sanitized path and true, or the original path and false if any param
// name is still invalid after sanitization.
func sanitizePathParams(path string) (string, bool) {
	offset := 0
	for {
		start := strings.Index(path[offset:], "{")
		if start == -1 {
			break
		}
		start += offset
		end := strings.Index(path[start+1:], "}")
		if end == -1 {
			return path, false
		}
		end += start + 1
		name := path[start+1 : end]
		sanitized := sanitizeWildcard(name)
		if !isValidWildcard(sanitized) {
			return path, false
		}
		path = path[:start+1] + sanitized + path[end:]
		offset = start + 1 + len(sanitized) + 1
	}
	return path, true
}

func sanitizeWildcard(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func isValidWildcard(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// executeCommandCore contains the core execution logic without HTTP handling
// Returns: (data, metadata, statusCode, error)
func (s *SwaggerServer) executeCommandCore(r *http.Request) (any, *ExecutionResponse, int, error) {
	// Find the operation for this path and method
	op := s.executor.FindOperation(r.Method, r.URL.Path)
	if op == nil {
		resp := &ExecutionResponse{
			Success: false,
			Error:   fmt.Sprintf("No operation found for %s %s", r.Method, r.URL.Path),
		}
		return resp, resp, http.StatusNotFound, fmt.Errorf("operation not found")
	}

	// Extract request parameters
	req, err := s.executor.ExtractRequestFromHTTP(r, op)
	if err != nil {
		resp := &ExecutionResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to extract parameters: %v", err),
			Input:   req,
		}
		return resp, resp, http.StatusBadRequest, err
	}

	// Validate parameters
	if err := s.executor.ValidateParameters(req, op); err != nil {
		resp := &ExecutionResponse{
			Success: false,
			Error:   fmt.Sprintf("Parameter validation failed: %v", err),
			Input:   req,
		}
		return resp, resp, http.StatusBadRequest, err
	}

	// Execute the command
	data, metadata, err := s.executor.ExecuteCommand(op, req)
	if err != nil {
		// Return metadata for error headers
		return data, metadata, http.StatusInternalServerError, err
	}

	// Return both data and metadata
	return data, metadata, http.StatusOK, nil
}

// handleExecuteCommand is the main handler for command execution
func (s *SwaggerServer) handleExecuteCommand(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	lookupRequested := isLookupRequest(r)
	op := s.executor.FindOperation(r.Method, r.URL.Path)
	if lookupRequested {
		if !hasLookup(op) {
			op = s.executor.FindLookupOperation(r.Method, r.URL.Path)
		}
		if !hasLookup(op) {
			http.Error(w, fmt.Sprintf("No lookup found for %s %s", r.Method, r.URL.Path), http.StatusNotFound)
			return
		}
		s.handleLookupCommand(w, r, op)
		return
	}

	if op == nil {
		http.Error(w, fmt.Sprintf("No operation found for %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		return
	}

	// Execute command and get data + metadata
	data, metadata, statusCode, _ := s.executeCommandCore(r)

	// Add execution metadata headers
	if metadata != nil {
		w.Header().Set("X-CLI-Command", metadata.CLI)
		w.Header().Set("X-Exit-Code", strconv.Itoa(metadata.ExitCode))
		w.Header().Set("X-Execution-Success", strconv.FormatBool(metadata.Success))
		if metadata.Error != "" {
			w.Header().Set("X-Error", metadata.Error)
		}
		if metadata.Stderr != "" {
			w.Header().Set("X-Stderr", metadata.Stderr)
		}
	}

	// Format and write response body using FormatManager (defaults to json).
	// Structured wire formats serialize the ExecutionResponse envelope so the
	// rendered command output is addressable via `output`; render formats emit
	// the command's parsed data directly.
	opts := extractFormatOpts(r)
	body := data
	// Substitute the envelope only for stdout-capture commands, whose payload
	// lives in metadata.Output. DataFunc operations (entity list/get) carry their
	// payload in `data`; serializing the envelope there would drop the result.
	if isStructuredWireFormat(opts.Format) && metadata != nil && !metadata.DataIsStructured {
		body = metadata
	}
	s.writeFormattedResponse(w, r, body, opts, statusCode)
}

func isLookupRequest(r *http.Request) bool {
	if strings.EqualFold(r.Method, http.MethodHead) {
		return true
	}
	return r.URL.Query().Get("__lookup") == "filters"
}

// hasLookup reports whether an operation can serve a filter-metadata lookup
// request, via either its plain or context-aware lookup func.
func hasLookup(op *RPCOperation) bool {
	return op != nil && (op.LookupFunc != nil || op.ContextLookupFunc != nil)
}

func (s *SwaggerServer) handleLookupCommand(w http.ResponseWriter, r *http.Request, op *RPCOperation) {
	req, err := s.executor.ExtractRequestFromHTTP(r, op)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract parameters: %v", err), http.StatusBadRequest)
		return
	}

	// __lookup_filter / __lookup_q drive per-filter server-side search. They are
	// not declared operation parameters, so ExtractRequestFromHTTP drops them;
	// forward them into the flag map for buildLookupFunc to consume.
	if v := r.URL.Query().Get("__lookup_filter"); v != "" {
		req.Flags["__lookup_filter"] = v
	}
	if v := r.URL.Query().Get("__lookup_q"); v != "" {
		req.Flags["__lookup_q"] = v
	}

	var data any
	if op.ContextLookupFunc != nil {
		data, err = op.ContextLookupFunc(r.Context(), req.Flags, req.Args)
	} else {
		data, err = op.LookupFunc(req.Flags, req.Args)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json+clicky")
	w.WriteHeader(http.StatusOK)

	if strings.EqualFold(r.Method, http.MethodHead) {
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode lookup response: %v", err), http.StatusInternalServerError)
	}
}

// writeFormattedResponse formats data using the FormatManager and writes it as
// the raw response body with the appropriate Content-Type.
func (s *SwaggerServer) writeFormattedResponse(w http.ResponseWriter, r *http.Request, data any, opts formatOptions, statusCode int) {
	if paged, ok := data.(clicky.Paged); ok {
		page := paged.PageMetadata()
		w.Header().Set("X-Total-Count", strconv.FormatInt(page.Total, 10))
		w.Header().Set("X-Page-Limit", strconv.Itoa(page.Limit))
		w.Header().Set("X-Page-Offset", strconv.Itoa(page.Offset))
		if shouldFormatPagedRows(opts.Format) {
			data = paged.PageRows()
		}
	}

	manager := formatters.NewFormatManager()
	output, err := manager.FormatWithContext(r, formatters.FormatOptions{
		Format: opts.Format,
		Page:   opts.Page,
		Limit:  opts.Limit,
	}, data)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "format error: %v", err) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", formatToContentType(opts.Format))
	if filename := sanitizedAttachmentFilename(r.URL.Query().Get("filename")); filename != "" {
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
	}
	w.WriteHeader(statusCode)
	w.Write([]byte(output)) //nolint:errcheck
}

func sanitizedAttachmentFilename(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	base := path.Base(strings.ReplaceAll(raw, "\\", "/"))
	base = strings.Trim(base, ". ")
	if base == "" || base == "." || base == "/" {
		return ""
	}

	var out strings.Builder
	lastDash := false
	for _, r := range base {
		safe := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' ||
			r == '_' ||
			r == '-'
		if safe {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}

	filename := strings.Trim(out.String(), ".-")
	if len(filename) > 160 {
		filename = filename[:160]
		filename = strings.Trim(filename, ".-")
	}
	return filename
}

func shouldFormatPagedRows(format string) bool {
	switch format {
	case "json", "clicky-json", "yaml", "yml":
		return false
	default:
		return true
	}
}

type formatOptions struct {
	Format string
	Page   int
	Limit  int
}

func extractFormatOpts(r *http.Request) formatOptions {
	opts := formatOptions{}

	if p := r.URL.Query().Get("page"); p != "" {
		opts.Page, _ = strconv.Atoi(p)
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		opts.Limit, _ = strconv.Atoi(l)
	}

	// A `format` of pretty/tree is a command *render* format (it controls how
	// the executed command renders its Output), not an HTTP wire format. When
	// the client also sends an Accept header asking for a structured format, the
	// Accept header wins for the wire format and the render format flows to the
	// command unchanged.
	if f := r.URL.Query().Get("format"); f != "" && !isRenderFormat(f) {
		opts.Format = f
		return opts
	}

	accept := r.Header.Get("Accept")
	if accept != "" {
		for _, part := range strings.Split(accept, ",") {
			ct := strings.TrimSpace(strings.Split(part, ";")[0])
			switch strings.ToLower(ct) {
			case "application/json":
				opts.Format = "json"
				return opts
			case "application/clicky+json", "application/json+clicky":
				opts.Format = "clicky-json"
				return opts
			case "application/yaml", "text/yaml", "application/x-yaml":
				opts.Format = "yaml"
				return opts
			case "text/csv", "application/csv":
				opts.Format = "csv"
				return opts
			case "text/html", "application/xhtml+xml":
				opts.Format = "html"
				return opts
			case "text/markdown":
				opts.Format = "markdown"
				return opts
			case "application/pdf":
				opts.Format = "pdf"
				return opts
			case "text/plain":
				opts.Format = "pretty"
				return opts
			}
		}
	}

	opts.Format = "json"
	return opts
}

// isRenderFormat reports whether format is a human-facing render format used to
// style a command's Output (vs. a structured HTTP wire format).
func isRenderFormat(format string) bool {
	switch format {
	case "pretty", "tree":
		return true
	default:
		return false
	}
}

// isStructuredWireFormat reports whether format serializes the ExecutionResponse
// envelope as structured data (so Output/Success/ExitCode are addressable).
func isStructuredWireFormat(format string) bool {
	switch format {
	case "json", "clicky-json", "yaml", "yml":
		return true
	default:
		return false
	}
}

func formatToContentType(format string) string {
	switch format {
	case "clicky-json":
		return "application/json+clicky"
	case "yaml", "yml":
		return "application/yaml"
	case "csv":
		return "text/csv; charset=utf-8"
	case "html", "html-react":
		return "text/html; charset=utf-8"
	case "markdown", "md":
		return "text/markdown; charset=utf-8"
	case "pdf":
		return "application/pdf"
	case "pretty", "tree":
		return "text/plain; charset=utf-8"
	default:
		return "application/json"
	}
}
