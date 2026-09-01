package rpc

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/clicky/entity"
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
	Host                     string
	Port                     int
	Title                    string
	Description              string
	Version                  string
	AutoRefresh              bool
	Open                     bool
	SkipHealth               bool            // Skip registering /health (caller provides their own)
	StructuredErrorResponses bool            // Return traceable ErrorResponse envelopes instead of legacy error formats
	HideErrorDetails         bool            // Hide unclassified details in structured error responses
	Executor                 *ExecutorConfig // Optional command execution configuration
}

// SwaggerServer serves API reference documentation for the OpenAPI specification.
type SwaggerServer struct {
	config       *ServeConfig
	rootCmd      *cobra.Command
	generator    *OpenAPIGenerator
	converterCfg *Config // Converter config used to build executor routes; reused when (re)generating the spec.
	server       *http.Server
	executor     *CommandExecutor // Optional command executor
	errorWriter  *entity.ErrorWriter

	// The spec is a pure function of rootCmd and converterCfg, both fixed at
	// construction, so each rendering is generated at most once and reused. A
	// large command tree renders into megabytes; doing that per request cost
	// seconds of server time and re-shipped an unchanged document every time.
	// Commands added to rootCmd after the first request are therefore not
	// reflected — register the whole tree before serving.
	specMu   sync.Mutex
	specDocs map[specFormat]*specDocument
	// specBase is the same document before any family path is added. A consumer
	// with families cannot memoize the encoded rendering, but the static portion
	// it is built from is still a pure function of rootCmd and converterCfg, so
	// only the family paths are re-derived per request.
	specBase *OpenAPISpec
}

// specFormat identifies a rendering of the OpenAPI document. Each rendering is
// a distinct entity with its own bytes and its own ETag.
type specFormat string

const (
	specFormatJSON specFormat = "json"
	specFormatYAML specFormat = "yaml"
)

// specDocument is a rendered OpenAPI document and its content validator.
type specDocument struct {
	body []byte
	etag string
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
	baseGenerator := NewOpenAPIGenerator(openAPIConfig)
	resolvedOpenAPIConfig := *baseGenerator.config
	resolvedOpenAPIConfig.StructuredErrorResponses = config.StructuredErrorResponses
	generator := NewOpenAPIGenerator(&resolvedOpenAPIConfig)

	server := &SwaggerServer{
		config:      config,
		rootCmd:     rootCmd,
		generator:   generator,
		errorWriter: entity.NewErrorWriter(entity.ErrorOptions{HideDetails: config.HideErrorDetails}),
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

// Executor returns the server's command executor, or nil when execution is not
// enabled. Callers can use it to invoke operations programmatically — resolve an
// operation with FindOperation(method, path) and run it via ExecuteCommand — so a
// stored operation descriptor can be replayed through the same code path the HTTP
// executor routes use.
func (s *SwaggerServer) Executor() *CommandExecutor {
	return s.executor
}

// Start starts the HTTP server
// RegisterRoutes registers all API routes onto the provided mux.
// This allows callers to compose the SwaggerServer routes with other handlers.
func (s *SwaggerServer) RegisterRoutes(mux *http.ServeMux) {
	// Exemption from the no-direct-ServeMux lint rule: that rule steers app code
	// onto the entity → rpc auto-routing surface, and this package IS that
	// surface — these registrations are the endpoints the rule points everyone
	// else at. (clicky lint already skips the clicky module; there is no
	// per-call marker for this rule.)
	mux.Handle("/api/openapi.json", s.tracedHandler("GET /api/openapi.json", http.HandlerFunc(s.handleOpenAPIJSON)))
	mux.Handle("/api/openapi.yaml", s.tracedHandler("GET /api/openapi.yaml", http.HandlerFunc(s.handleOpenAPIYAML)))
	mux.Handle("/api/entities", s.tracedHandler("GET /api/entities", http.HandlerFunc(s.handleEntities)))
	if !s.config.SkipHealth {
		mux.Handle("/health", s.tracedHandler("GET /health", http.HandlerFunc(s.handleHealth)))
	}

	if s.executor != nil {
		s.registerExecutionRoutes(mux)
	}
}

// HandleOpenAPIJSON serves the OpenAPI 3 spec as JSON. Exported so callers
// that compose their own mux (and want to wrap or override this endpoint)
// can re-mount it without going through RegisterRoutes.
func (s *SwaggerServer) HandleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	s.tracedHandler("GET /api/openapi.json", http.HandlerFunc(s.handleOpenAPIJSON)).ServeHTTP(w, r)
}

// HandleOpenAPIYAML serves the OpenAPI 3 spec as YAML. See HandleOpenAPIJSON.
func (s *SwaggerServer) HandleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	s.tracedHandler("GET /api/openapi.yaml", http.HandlerFunc(s.handleOpenAPIYAML)).ServeHTTP(w, r)
}

// HandleEntities serves the entity metadata endpoint. Exported so callers
// composing their own mux can register it independently of RegisterRoutes.
func (s *SwaggerServer) HandleEntities(w http.ResponseWriter, r *http.Request) {
	s.tracedHandler("GET /api/entities", http.HandlerFunc(s.handleEntities)).ServeHTTP(w, r)
}

// HandleHealth serves the liveness probe. Exported for the same reason as
// HandleEntities.
func (s *SwaggerServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	s.tracedHandler("GET /health", http.HandlerFunc(s.handleHealth)).ServeHTTP(w, r)
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

	// Register routes. Direct mux.Handle is the rpc serving layer's job — see
	// the exemption note on RegisterRoutes.
	mux.Handle("/", s.traceHandler("GET /", http.HandlerFunc(s.handleSwaggerUI)))
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
		s.writeError(w, r, entity.NewStatusError(http.StatusNotFound, "not_found", "page not found"))
		return
	}

	// Read and parse the HTML template
	htmlContent, err := assets.ReadFile("assets/index.html")
	if err != nil {
		s.writeError(w, r, fmt.Errorf("read template: %w", err))
		return
	}

	tmpl, err := template.New("swagger").Parse(string(htmlContent))
	if err != nil {
		s.writeError(w, r, fmt.Errorf("parse template: %w", err))
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
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		s.writeError(w, r, fmt.Errorf("render template: %w", err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body.Bytes())
}

// renderSpec returns the requested rendering of the OpenAPI document,
// generating and encoding it on first use. Only successful renderings are
// memoized, so a failure is retried rather than served forever.
func (s *SwaggerServer) renderSpec(format specFormat) (*specDocument, error) {
	s.specMu.Lock()
	defer s.specMu.Unlock()

	if doc := s.specDocs[format]; doc != nil {
		return doc, nil
	}

	spec, err := s.generator.GenerateFromCobraWithConfig(s.rootCmd, s.converterCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OpenAPI spec: %w", err)
	}

	doc, err := encodeSpec(spec, format)
	if err != nil {
		return nil, err
	}
	if s.specDocs == nil {
		s.specDocs = make(map[specFormat]*specDocument, 2)
	}
	s.specDocs[format] = doc
	return doc, nil
}

// encodeSpec renders a document and its content validator.
func encodeSpec(spec *OpenAPISpec, format specFormat) (*specDocument, error) {
	var body []byte
	var err error
	if format == specFormatYAML {
		body, err = yaml.Marshal(spec)
	} else {
		body, err = json.Marshal(spec)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode OpenAPI spec as %s: %w", format, err)
	}
	return &specDocument{body: body, etag: contentETag(body)}, nil
}

// specDocument returns the document to serve this request.
//
// The memoized rendering is a pure function of the command tree, which a
// dynamic-entity family is not part of: its instances come and go while the
// server runs, so a document that described them would be stale the moment it
// was cached. A consumer with no families therefore never leaves the cache,
// and one with families pays a rendering per request for a document that is
// true when it is read — which is the point of registering a family at all.
func (s *SwaggerServer) specDocument(r *http.Request, format specFormat) (*specDocument, error) {
	families := entity.GetDynamicEntityFamilies()
	if len(families) == 0 {
		return s.renderSpec(format)
	}

	base, err := s.baseSpec()
	if err != nil {
		return nil, err
	}
	spec := cloneSpecForFamilies(base)
	if err := s.addFamilyPaths(r.Context(), spec, families); err != nil {
		return nil, err
	}
	return encodeSpec(spec, format)
}

// baseSpec renders the family-independent document once and keeps it, so a
// consumer with families pays the command-tree walk on the first request only.
func (s *SwaggerServer) baseSpec() (*OpenAPISpec, error) {
	s.specMu.Lock()
	defer s.specMu.Unlock()

	if s.specBase != nil {
		return s.specBase, nil
	}
	spec, err := s.generator.GenerateFromCobraWithConfig(s.rootCmd, s.converterCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OpenAPI spec: %w", err)
	}
	s.specBase = spec
	return spec, nil
}

// cloneSpecForFamilies copies exactly what addFamilyPaths writes to — the path
// map and the surface list — so a request's instances never reach the cached
// base. Everything else is shared: nothing on this path mutates it.
func cloneSpecForFamilies(base *OpenAPISpec) *OpenAPISpec {
	spec := *base
	spec.Paths = maps.Clone(base.Paths)
	if spec.Paths == nil {
		spec.Paths = make(map[string]OpenAPIPath)
	}
	if base.Clicky != nil {
		meta := *base.Clicky
		meta.Surfaces = slices.Clone(base.Clicky.Surfaces)
		spec.Clicky = &meta
	}
	return &spec
}

// serveSpec writes a rendered OpenAPI document, answering a matching
// If-None-Match with a 304 instead of re-shipping the whole spec.
func (s *SwaggerServer) serveSpec(w http.ResponseWriter, r *http.Request, format specFormat, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	doc, err := s.specDocument(r, format)
	if err != nil {
		if s.structuredErrorResponses() {
			s.writeError(w, r, err)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// no-cache, not no-store: the client keeps the body but must revalidate,
	// and the ETag lets that revalidation cost a 304 rather than a re-download.
	w.Header().Set("ETag", doc.etag)
	w.Header().Set("Cache-Control", "no-cache")

	if etagMatches(r.Header.Get("If-None-Match"), doc.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if _, err := w.Write(doc.body); err != nil {
		// Log error but response already started
		fmt.Printf("Warning: failed to write OpenAPI %s response: %v\n", format, err)
	}
}

// handleOpenAPIJSON serves the OpenAPI specification in JSON format
func (s *SwaggerServer) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	s.serveSpec(w, r, specFormatJSON, "application/json")
}

// handleOpenAPIYAML serves the OpenAPI specification in YAML format
func (s *SwaggerServer) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	s.serveSpec(w, r, specFormatYAML, "text/yaml")
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

	body, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		s.writeError(w, r, fmt.Errorf("encode health response: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
