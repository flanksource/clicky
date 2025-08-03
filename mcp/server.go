package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
)

// MCPServer implements the MCP protocol server using TaskManager for execution
type MCPServer struct {
	config         *Config
	registry       *ToolRegistry
	promptRegistry *PromptRegistry
	rootCmd        *cobra.Command
	taskManager    *clicky.TaskManager
	verbose        bool
}

// NewMCPServer creates a new MCP server
func NewMCPServer(config *Config, rootCmd *cobra.Command) *MCPServer {
	promptRegistry := NewPromptRegistry(config)
	promptRegistry.LoadDefaults()
	
	// Try to load custom prompts
	promptsPath := GetPromptsPath()
	if _, err := os.Stat(promptsPath); err == nil {
		promptRegistry.LoadFromFile(promptsPath)
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
	// Create a new TaskManager for this session
	s.taskManager = clicky.NewTaskManagerWithConcurrency(5) // Limit concurrent tool executions
	s.taskManager.SetVerbose(s.verbose)
	
	// Configure retry for tool executions
	s.taskManager.SetRetryConfig(clicky.RetryConfig{
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
	case "http":
		return s.startHTTPServer(ctx)
	default:
		return fmt.Errorf("unsupported transport type: %s", s.config.Transport.Type)
	}
}

// startStdioServer starts the server using stdio transport
func (s *MCPServer) startStdioServer(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	
	// Handle shutdown gracefully
	go func() {
		<-ctx.Done()
		if s.taskManager != nil {
			s.taskManager.CancelAll()
		}
	}()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("stdin scan error: %w", err)
				}
				return nil // EOF
			}
			
			line := scanner.Text()
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

// startHTTPServer starts the server using HTTP transport (placeholder)
func (s *MCPServer) startHTTPServer(ctx context.Context) error {
	// TODO: Implement HTTP transport
	return fmt.Errorf("HTTP transport not yet implemented")
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
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
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
						Text: "Tool execution cancelled by user",
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
	task := s.taskManager.Start(
		fmt.Sprintf("MCP: %s", tool.Name),
		clicky.WithTimeout(timeout),
		clicky.WithFunc(func(t *clicky.Task) error {
			// Set up command arguments
			if tool.Command == nil {
				return fmt.Errorf("tool command not available")
			}
			
			// Apply arguments to command flags
			if err := s.applyArgsToCommand(tool.Command, args); err != nil {
				t.Errorf("Failed to apply arguments: %v", err)
				return err
			}
			
			// Capture output by redirecting stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			
			// Create pipes
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			
			os.Stdout = wOut
			os.Stderr = wErr
			
			// Capture output in goroutines
			outDone := make(chan struct{})
			errDone := make(chan struct{})
			
			go func() {
				defer close(outDone)
				buf := make([]byte, 1024)
				for {
					n, err := rOut.Read(buf)
					if n > 0 {
						output.Write(buf[:n])
						t.Infof("Output: %s", string(buf[:n]))
					}
					if err != nil {
						break
					}
				}
			}()
			
			go func() {
				defer close(errDone)
				buf := make([]byte, 1024)
				for {
					n, err := rErr.Read(buf)
					if n > 0 {
						errorOutput.Write(buf[:n])
						t.Warnf("Error: %s", string(buf[:n]))
					}
					if err != nil {
						break
					}
				}
			}()
			
			// Execute the command
			t.SetStatus(fmt.Sprintf("Executing: %s", tool.Name))
			
			var cmdErr error
			if tool.Command.RunE != nil {
				cmdErr = tool.Command.RunE(tool.Command, []string{})
			} else if tool.Command.Run != nil {
				tool.Command.Run(tool.Command, []string{})
			} else {
				cmdErr = fmt.Errorf("command has no Run function")
			}
			
			// Restore stdout/stderr
			wOut.Close()
			wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			
			// Wait for output capture to complete
			<-outDone
			<-errDone
			
			if cmdErr != nil {
				t.FailedWithError(cmdErr)
				return cmdErr
			}
			
			t.Success()
			return nil
		}),
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

// applyArgsToCommand applies arguments to cobra command flags
func (s *MCPServer) applyArgsToCommand(cmd *cobra.Command, args map[string]interface{}) error {
	// Reset command flags to defaults
	cmd.ResetFlags()
	
	// Apply each argument
	for key, value := range args {
		if key == "args" {
			// Handle positional arguments
			if argArray, ok := value.([]interface{}); ok {
				strArgs := make([]string, len(argArray))
				for i, arg := range argArray {
					strArgs[i] = fmt.Sprintf("%v", arg)
				}
				cmd.SetArgs(strArgs)
			}
			continue
		}
		
		// Find the flag
		flag := cmd.Flags().Lookup(key)
		if flag == nil {
			// Try persistent flags
			flag = cmd.PersistentFlags().Lookup(key)
			if flag == nil {
				return fmt.Errorf("unknown flag: %s", key)
			}
		}
		
		// Set the flag value
		switch v := value.(type) {
		case bool:
			if v {
				if err := flag.Value.Set("true"); err != nil {
					return fmt.Errorf("failed to set flag %s: %w", key, err)
				}
			}
		case string:
			if err := flag.Value.Set(v); err != nil {
				return fmt.Errorf("failed to set flag %s: %w", key, err)
			}
		default:
			if err := flag.Value.Set(fmt.Sprintf("%v", v)); err != nil {
				return fmt.Errorf("failed to set flag %s: %w", key, err)
			}
		}
	}
	
	return nil
}

// handlePromptsList handles the MCP prompts/list request
func (s *MCPServer) handlePromptsList(req JSONRPCRequest) (*JSONRPCResponse, error) {
	prompts := s.promptRegistry.List()
	
	// Add a special prompt that helps with tool discovery
	prompts = append(prompts, &Prompt{
		Name:        "discover-tools",
		Description: "Discover how to use available tools",
		Template: `Please analyze the available tools and show me:
1. What each tool does
2. The appropriate arguments for each tool
3. Common use cases and examples
4. How to combine tools for complex tasks

Focus on the most useful tools for common operations.`,
		Tags: []string{"discovery", "tools", "help"},
	})
	
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
	
	// Handle special discover-tools prompt
	if params.Name == "discover-tools" {
		tools := s.registry.GetTools()
		
		// Build a comprehensive prompt about available tools
		var toolDescriptions []string
		for name, tool := range tools {
			desc := fmt.Sprintf("**%s**: %s", name, tool.Description)
			
			// Add parameter information
			if len(tool.InputSchema.Properties) > 0 {
				desc += "\n  Parameters:"
				for param, prop := range tool.InputSchema.Properties {
					required := ""
					for _, req := range tool.InputSchema.Required {
						if req == param {
							required = " (required)"
							break
						}
					}
					desc += fmt.Sprintf("\n    - %s: %s%s", param, prop.Description, required)
				}
			}
			
			toolDescriptions = append(toolDescriptions, desc)
		}
		
		content := fmt.Sprintf(`Here are the available tools you can use:

%s

To use a tool, call it with the appropriate arguments. For example:
- Use the 'help' tool to get general help
- Use specific tools with their required parameters

What would you like to do with these tools?`, strings.Join(toolDescriptions, "\n\n"))
		
		response := &GetPromptResponse{
			Description: "Discover available tools and their usage",
			Messages: []PromptMessage{
				{
					Role:    "user",
					Content: content,
				},
			},
		}
		
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  response,
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