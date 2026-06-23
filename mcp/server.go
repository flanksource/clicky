package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
)

// MCPServer implements the MCP protocol server using TaskManager for execution
type MCPServer struct {
	config         *Config
	registry       *ToolRegistry
	promptRegistry *PromptRegistry
	rootCmd        *cobra.Command
	verbose        bool
}

const maxSSEMessageBytes = 1 << 20

// NewMCPServer creates a new MCP server
func NewMCPServer(config *Config, rootCmd *cobra.Command) *MCPServer {
	promptRegistry := NewPromptRegistry(config)
	promptRegistry.LoadDefaults()

	// Try to load custom prompts (per-app, keyed off the root command name).
	promptsPath := GetPromptsPathFor(rootCmd.Name())
	if _, err := os.Stat(promptsPath); err == nil {
		if err := promptRegistry.LoadFromFile(promptsPath); err != nil {
			// Log error but continue with empty registry
			fmt.Printf("Warning: failed to load prompts from %s: %v\n", promptsPath, err)
		}
	}

	return &MCPServer{
		config:         config,
		registry:       NewToolRegistry(config),
		promptRegistry: promptRegistry,
		rootCmd:        rootCmd,
		verbose:        os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != "",
	}
}

// Initialize registers all commands with the tool registry
func (s *MCPServer) Initialize() error {
	return s.registry.RegisterCommandTree(s.rootCmd)
}

// Start starts the MCP server using the configured transport
func (s *MCPServer) Start(ctx context.Context) error {
	// Configure global task manager for this session
	clicky.SetGlobalMaxConcurrency(5) // Limit concurrent tool executions
	clicky.SetGlobalVerbose(s.verbose)

	// Configure retry for tool executions
	task.SetRetryConfig(clicky.RetryConfig{
		MaxRetries:      2,
		BaseDelay:       1 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		RetryableErrors: []string{"timeout", "temporary"},
	})

	switch s.config.Transport.Type {
	case "stdio":
		return s.startStdioServer(ctx)
	case "http", "sse":
		return s.startSSEServer(ctx)
	default:
		return fmt.Errorf("unsupported transport type: %s", s.config.Transport.Type)
	}
}

// startStdioServer starts the server using stdio transport
func (s *MCPServer) startStdioServer(ctx context.Context) error {
	lines := make(chan string)
	scanErr := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			scanErr <- err
		}
		close(lines)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-scanErr:
			return fmt.Errorf("stdin scan error: %w", err)
		case line, ok := <-lines:
			if !ok {
				return nil // EOF
			}
			if line == "" {
				continue
			}

			response, err := s.handleJSONRPCRequest(ctx, line)
			if err != nil {
				log.Printf("Error handling request: %v", err)
				continue
			}

			if response != nil {
				responseJSON, err := json.Marshal(response)
				if err != nil {
					log.Printf("Error marshaling response: %v", err)
					continue
				}

				fmt.Println(string(responseJSON))
			}
		}
	}
}

// sseSession holds the per-client state for an active SSE connection. The
// SSE GET handler owns the events channel and pumps events to the wire; the
// POST handler dispatches a JSON-RPC request and writes the response back
// onto events. closed prevents a slow client from blocking the handler
// after the stream has gone away.
type sseSession struct {
	id     string
	events chan []byte

	mu     sync.Mutex
	closed bool
}

func (s *sseSession) send(payload []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.events <- payload:
		return true
	default:
		return false
	}
}

func (s *sseSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.events)
}

// startSSEServer starts the server using SSE transport. It exposes:
//   - GET  /sse                          — opens an event stream and emits an
//     `endpoint` event whose data is the relative URL the client should POST
//     JSON-RPC requests to.
//   - POST /messages?session_id=<uuid>   — dispatches one JSON-RPC request
//     and pushes the response back onto the corresponding SSE stream as a
//     `message` event.
//
// This matches the MCP SSE transport contract used by Claude Desktop and
// other clients.
func (s *MCPServer) startSSEServer(ctx context.Context) error {
	addr := s.config.Transport.Address
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := s.config.Transport.Port
	if port == 0 {
		port = 8080
	}
	listenAddr := fmt.Sprintf("%s:%d", addr, port)

	sessions := &sync.Map{} // session_id -> *sseSession

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		session := &sseSession{
			id:     uuid.NewString(),
			events: make(chan []byte, 16),
		}
		sessions.Store(session.id, session)
		defer func() {
			sessions.Delete(session.id)
			session.close()
		}()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		endpoint := fmt.Sprintf("/messages?session_id=%s", session.id)
		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpoint)
		flusher.Flush()

		clientGone := r.Context().Done()
		serverGone := ctx.Done()
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case <-clientGone:
				return
			case <-serverGone:
				return
			case <-keepalive.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case payload, ok := <-session.events:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		sessionID := r.URL.Query().Get("session_id")
		raw, ok := sessions.Load(sessionID)
		if !ok {
			http.Error(w, "unknown session_id", http.StatusNotFound)
			return
		}
		session := raw.(*sseSession)

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSSEMessageBytes))
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		response, err := s.handleJSONRPCRequest(r.Context(), string(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)

		if response == nil {
			return
		}
		payload, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling SSE response: %v", err)
			return
		}
		if !session.send(payload) {
			log.Printf("Failed to deliver SSE response for session %s", sessionID)
		}
	})

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "MCP SSE server listening on http://%s/sse\n", listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("SSE server failed: %w", err)
	}
	return nil
}

func sameOriginOrNoOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// JSONRPCRequest represents an MCP JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents an MCP JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents an MCP JSON-RPC error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// handleJSONRPCRequest processes a JSON-RPC request and returns a response
func (s *MCPServer) handleJSONRPCRequest(ctx context.Context, requestJSON string) (*JSONRPCResponse, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
		}, nil
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "prompts/get":
		return s.handlePromptsGet(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}, nil
	}
}

// handleInitialize handles the MCP initialize request
func (s *MCPServer) handleInitialize(req JSONRPCRequest) (*JSONRPCResponse, error) {
	capabilities := map[string]interface{}{
		"tools": map[string]interface{}{
			"listChanged": false,
		},
		"prompts": map[string]interface{}{
			"listChanged": false,
		},
	}

	serverInfo := map[string]interface{}{
		"name":    s.config.Name,
		"version": s.config.Version,
	}

	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    capabilities,
		"serverInfo":      serverInfo,
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, nil
}

// handleToolsList handles the MCP tools/list request
func (s *MCPServer) handleToolsList(req JSONRPCRequest) (*JSONRPCResponse, error) {
	response := s.registry.ToListResponse()

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  response,
	}, nil
}

// ToolCallParams represents the parameters for a tools/call request
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in MCP
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// handleToolsCall handles the MCP tools/call request
func (s *MCPServer) handleToolsCall(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	var params ToolCallParams
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	// Get the tool
	tool, exists := s.registry.GetTool(params.Name)
	if !exists {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Tool not found: %s", params.Name),
			},
		}, nil
	}

	// Execute the tool using TaskManager
	result, err := s.executeToolWithTaskManager(ctx, tool, params.Arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolCallResult{
				Content: []ContentBlock{
					{
						Type: "text",
						Text: fmt.Sprintf("Tool execution failed: %v", err),
					},
				},
				IsError: true,
			},
		}, nil
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, nil
}

// executeToolWithTaskManager executes a tool using the TaskManager
func (s *MCPServer) executeToolWithTaskManager(ctx context.Context, tool *ToolDefinition, args map[string]interface{}) (*ToolCallResult, error) {
	// Check for user confirmation if required
	if s.config.Security.RequireConfirmation {
		if !s.confirmToolExecution(tool.Name, args) {
			return &ToolCallResult{
				Content: []ContentBlock{
					{
						Type: "text",
						Text: "Tool execution canceled by user",
					},
				},
				IsError: true,
			}, nil
		}
	}

	// Prepare timeout
	timeout := time.Duration(s.config.Security.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Capture output
	var output strings.Builder
	var errorOutput strings.Builder

	// Create a task for the tool execution
	task := clicky.StartTask(fmt.Sprintf("MCP: %s", tool.Name),
		func(ctx flanksourceContext.Context, t *clicky.Task) (interface{}, error) {
			if tool.Command == nil {
				return nil, fmt.Errorf("tool command not available")
			}

			t.SetName(fmt.Sprintf("Executing: %s", tool.Name))

			// Run through the transport-neutral handle, which serializes the
			// global stdout/stderr capture, resets state, and applies the
			// arguments + server-wide format overrides.
			flags, posArgs := splitMCPArgs(args)
			stdout, stderr, cmdErr := tool.Command.Execute(ctx, entity.ExecuteOptions{
				Args:            posArgs,
				Flags:           flags,
				FormatOverrides: formatOverrides(s.config.Tools.Format),
			})
			if stdout != "" {
				output.WriteString(stdout)
				t.Infof("Output: %s", stdout)
			}
			if stderr != "" {
				errorOutput.WriteString(stderr)
				t.Warnf("Error: %s", stderr)
			}

			if cmdErr != nil {
				if _, err := t.FailedWithError(cmdErr); err != nil {
					// Log error but continue
					fmt.Printf("Warning: failed to record command failure: %v\n", err)
				}
				return nil, cmdErr
			}

			t.Success()
			return nil, nil
		},
		clicky.WithTimeout(timeout),
	)

	// Wait for task completion
	<-task.Context().Done()

	// Build result
	content := []ContentBlock{}

	if output.Len() > 0 {
		content = append(content, ContentBlock{
			Type: "text",
			Text: output.String(),
		})
	}

	isError := false
	if task.Error() != nil || errorOutput.Len() > 0 {
		isError = true
		errorText := errorOutput.String()
		if task.Error() != nil && errorText == "" {
			errorText = task.Error().Error()
		}

		if errorText != "" {
			content = append(content, ContentBlock{
				Type: "text",
				Text: fmt.Sprintf("Error: %s", errorText),
			})
		}
	}

	// Audit logging
	if s.config.Security.AuditLog {
		status := "success"
		if isError {
			status = "failed"
		}
		log.Printf("MCP tool executed: %s with args: %v (%s)", tool.Name, args, status)
	}

	return &ToolCallResult{
		Content: content,
		IsError: isError,
	}, nil
}

// confirmToolExecution prompts for user confirmation
func (s *MCPServer) confirmToolExecution(toolName string, args map[string]interface{}) bool {
	// In stdio mode, we can't easily prompt for confirmation
	// This would need to be handled by the client
	// For now, we'll auto-approve if confirmation is required
	// In a real implementation, this would send a confirmation request to the client
	return true
}

// formatOverrides translates the MCP server's configured format into best-effort
// flag values (format / no-color) applied before tool execution. Returns nil when
// no format is configured.
func formatOverrides(opts *formatters.FormatOptions) map[string]string {
	if opts == nil {
		return nil
	}
	out := map[string]string{}
	if name := formatName(opts); name != "" {
		out["format"] = name
	}
	if opts.NoColor {
		out["no-color"] = "true"
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// formatName picks the canonical format name from FormatOptions.
// Boolean toggles win over the Format string when both are set.
func formatName(opts *formatters.FormatOptions) string {
	switch {
	case opts.Markdown:
		return "markdown"
	case opts.JSON:
		return "json"
	case opts.YAML:
		return "yaml"
	case opts.CSV:
		return "csv"
	case opts.HTML:
		return "html"
	case opts.PDF:
		return "pdf"
	case opts.Slack:
		return "slack"
	case opts.Pretty:
		return "pretty"
	}
	return strings.TrimSpace(strings.ToLower(opts.Format))
}

// splitMCPArgs splits a model-provided MCP argument map into positional args
// (the "args" key) and string-valued flags (every other key).
func splitMCPArgs(args map[string]interface{}) (map[string]string, []string) {
	flags := map[string]string{}
	var posArgs []string
	for key, value := range args {
		if key == "args" {
			if argArray, ok := value.([]interface{}); ok {
				for _, arg := range argArray {
					posArgs = append(posArgs, fmt.Sprintf("%v", arg))
				}
			}
			continue
		}
		flags[key] = fmt.Sprintf("%v", value)
	}
	return flags, posArgs
}

// handlePromptsList handles the MCP prompts/list request
func (s *MCPServer) handlePromptsList(req JSONRPCRequest) (*JSONRPCResponse, error) {
	prompts := s.promptRegistry.List()

	response := &ListPromptsResponse{
		Prompts: make([]Prompt, len(prompts)),
	}

	for i, p := range prompts {
		response.Prompts[i] = *p
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  response,
	}, nil
}

// PromptsGetParams represents the parameters for a prompts/get request
type PromptsGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// handlePromptsGet handles the MCP prompts/get request
func (s *MCPServer) handlePromptsGet(req JSONRPCRequest) (*JSONRPCResponse, error) {
	var params PromptsGetParams
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	// Get the regular prompt
	prompt, exists := s.promptRegistry.Get(params.Name)
	if !exists {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Prompt not found: %s", params.Name),
			},
		}, nil
	}

	// Apply arguments and get response
	response := prompt.ToMCPResponse(params.Arguments)

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  response,
	}, nil
}
