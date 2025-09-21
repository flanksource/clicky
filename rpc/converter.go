package rpc

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Converter handles converting Cobra commands to generic RPC operations
type Converter struct {
	config *Config
}

// NewConverter creates a new RPC converter with the given configuration
func NewConverter(config *Config) *Converter {
	if config == nil {
		config = DefaultConfig()
	}
	return &Converter{
		config: config,
	}
}

// ConvertCommand converts a single Cobra command to an RPC operation
func (c *Converter) ConvertCommand(cmd *cobra.Command) (*RPCOperation, error) {
	cmdPath := getCommandPath(cmd)

	// Build input schema from flags
	schema := Schema{
		Type:       "object",
		Properties: make(map[string]Property),
		Required:   []string{},
	}

	var parameters []RPCParameter

	// Add positional arguments
	if cmd.Args != nil {
		argParam := RPCParameter{
			Name:        "args",
			Type:        "array",
			Description: "Positional arguments for the command",
			Required:    false,
			In:          "query",
		}
		parameters = append(parameters, argParam)

		schema.Properties["args"] = Property{
			Type:        "array",
			Description: "Positional arguments for the command",
		}
	}

	// Process flags
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		prop := Property{
			Description: flag.Usage,
		}

		param := RPCParameter{
			Name:        flag.Name,
			Description: flag.Usage,
			Required:    false,
			In:          "query",
		}

		// Determine type based on flag type
		switch flag.Value.Type() {
		case "bool":
			prop.Type = "boolean"
			param.Type = "boolean"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue == "true"
				param.Default = flag.DefValue == "true"
			}
		case "int", "int8", "int16", "int32", "int64":
			prop.Type = "integer"
			param.Type = "integer"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
				param.Default = flag.DefValue
			}
		case "float32", "float64":
			prop.Type = "number"
			param.Type = "number"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
				param.Default = flag.DefValue
			}
		default:
			prop.Type = "string"
			param.Type = "string"
			if flag.DefValue != "" {
				prop.Default = flag.DefValue
				param.Default = flag.DefValue
			}
		}

		schema.Properties[flag.Name] = prop
		parameters = append(parameters, param)

		// Check if flag is required (skip for boolean flags since they always have defaults)
		if flag.Value.Type() != "bool" && c.isFlagRequired(cmd, flag.Name) {
			schema.Required = append(schema.Required, flag.Name)
			param.Required = true
		}
	})

	// Generate HTTP method based on command semantics
	method := c.inferHTTPMethod(cmd, cmdPath)

	// Generate REST path if enabled
	path := ""
	if c.config.AutoGeneratePaths {
		path = c.generateRESTPath(cmdPath)
	}

	// Combine default tags with command-specific tags
	tags := append([]string{}, c.config.DefaultTags...)
	if parentName := c.getParentCommandName(cmd); parentName != "" {
		tags = append(tags, parentName)
	}

	operation := &RPCOperation{
		Name:        cmdPath,
		Description: cmd.Short,
		Parameters:  parameters,
		Schema:      schema,
		Command:     cmd,
		Path:        path,
		Method:      method,
		Tags:        tags,
	}

	return operation, nil
}

// ConvertCommandTree recursively converts a command and its subcommands
func (c *Converter) ConvertCommandTree(rootCmd *cobra.Command) (*RPCService, error) {
	var operations []RPCOperation

	err := c.walkCommands(rootCmd, func(cmd *cobra.Command) error {
		if c.shouldConvertCommand(cmd) {
			op, err := c.ConvertCommand(cmd)
			if err != nil {
				return err
			}
			operations = append(operations, *op)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Get service name from root command
	serviceName := rootCmd.Name()
	if serviceName == "" {
		serviceName = "api"
	}

	service := &RPCService{
		Name:        serviceName,
		Version:     "1.0.0",
		Description: rootCmd.Long,
		Operations:  operations,
	}

	return service, nil
}

// shouldConvertCommand determines if a command should be converted to an RPC operation
func (c *Converter) shouldConvertCommand(cmd *cobra.Command) bool {
	// Skip root command and commands without Run function
	if cmd.Parent() == nil || (cmd.Run == nil && cmd.RunE == nil) {
		return false
	}
	return true
}

// walkCommands recursively walks through all commands
func (c *Converter) walkCommands(cmd *cobra.Command, fn func(*cobra.Command) error) error {
	// Process current command
	if err := fn(cmd); err != nil {
		return err
	}

	// Process subcommands
	for _, subCmd := range cmd.Commands() {
		if err := c.walkCommands(subCmd, fn); err != nil {
			return err
		}
	}

	return nil
}

// inferHTTPMethod attempts to infer appropriate HTTP method from command semantics
func (c *Converter) inferHTTPMethod(cmd *cobra.Command, cmdPath string) string {
	cmdLower := strings.ToLower(cmdPath)

	// Check for common CRUD patterns
	if strings.Contains(cmdLower, "get") || strings.Contains(cmdLower, "list") ||
	   strings.Contains(cmdLower, "show") || strings.Contains(cmdLower, "describe") {
		return "GET"
	}

	if strings.Contains(cmdLower, "create") || strings.Contains(cmdLower, "add") ||
	   strings.Contains(cmdLower, "new") {
		return "POST"
	}

	if strings.Contains(cmdLower, "update") || strings.Contains(cmdLower, "edit") ||
	   strings.Contains(cmdLower, "modify") || strings.Contains(cmdLower, "set") {
		return "PUT"
	}

	if strings.Contains(cmdLower, "delete") || strings.Contains(cmdLower, "remove") ||
	   strings.Contains(cmdLower, "destroy") {
		return "DELETE"
	}

	// Default to configured method
	return c.config.DefaultMethod
}

// generateRESTPath generates a REST API path from command hierarchy
func (c *Converter) generateRESTPath(cmdPath string) string {
	// Convert command path to REST path
	// e.g., "user create" -> "/api/v1/user"
	// e.g., "config set" -> "/api/v1/config"

	parts := strings.Split(cmdPath, " ")

	// Build path with prefix
	pathParts := []string{c.config.PathPrefix}

	for i, part := range parts {
		// Skip the last part if it's a CRUD operation
		if i == len(parts)-1 {
			if isCRUDOperation(part) {
				break
			}
		}

		// Use resource names as-is without pluralization
		if i < len(parts)-1 || !isCRUDOperation(part) {
			pathParts = append(pathParts, part)
		}
	}

	return strings.Join(pathParts, "/")
}

// isCRUDOperation checks if a command part represents a CRUD operation
func isCRUDOperation(part string) bool {
	crudOps := []string{"create", "get", "list", "update", "delete", "show", "describe", "set", "add", "remove"}
	partLower := strings.ToLower(part)
	for _, op := range crudOps {
		if partLower == op {
			return true
		}
	}
	return false
}

// pluralize applies simple pluralization rules (can be enhanced)
func pluralize(word string) string {
	if strings.HasSuffix(word, "s") {
		return word
	}
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	return word + "s"
}

// getParentCommandName returns the name of the parent command for tagging
func (c *Converter) getParentCommandName(cmd *cobra.Command) string {
	if cmd.Parent() != nil && cmd.Parent().Parent() != nil {
		return cmd.Parent().Name()
	}
	return ""
}

// isFlagRequired checks if a flag is marked as required using reflection
func (c *Converter) isFlagRequired(cmd *cobra.Command, flagName string) bool {
	// Look up the flag in the command's flag set
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		// Try persistent flags if not found in local flags
		flag = cmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			return false
		}
	}

	// Check if the flag has been marked as required by looking for Cobra's
	// internal required flag annotation. When MarkFlagRequired is called,
	// Cobra adds the "cobra_annotation_bash_completion_one_required_flag"
	// annotation to the flag.
	if flag.Annotations != nil {
		_, isRequired := flag.Annotations["cobra_annotation_bash_completion_one_required_flag"]
		return isRequired
	}

	return false
}

// getCommandPath returns the full command path (e.g., "status", "ai cache")
func getCommandPath(cmd *cobra.Command) string {
	if cmd.Parent() == nil {
		return cmd.Name()
	}

	parts := []string{}
	for c := cmd; c.Parent() != nil; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}

	return strings.Join(parts, " ")
}