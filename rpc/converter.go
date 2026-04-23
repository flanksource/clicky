package rpc

import (
	"strings"

	"github.com/flanksource/clicky"
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
	// Extract parameter name from Use field if available
	positionalParamName := ""
	if cmd.Args != nil {
		positionalParamName = extractParameterName(cmd.Use)
		if positionalParamName != "" {
			// Add named path parameter for single positional arg
			argParam := RPCParameter{
				Name:        positionalParamName,
				Type:        "string",
				Description: "Positional argument from command",
				Required:    false,
				In:          "query", // Will be updated to "path" later if in URL
			}
			parameters = append(parameters, argParam)

			schema.Properties[positionalParamName] = Property{
				Type:        "string",
				Description: "Positional argument from command",
			}
		} else {
			// Fallback to generic args array
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
		path = c.generateRESTPath(cmd, cmdPath)
	}

	// Update parameter locations based on path
	c.updateParameterLocations(&parameters, path)

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

	if df := clicky.GetDataFunc(cmd); df != nil {
		operation.DataFunc = df
	}
	if lf := clicky.GetLookupFunc(cmd); lf != nil {
		operation.LookupFunc = lf
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
func (c *Converter) generateRESTPath(cmd *cobra.Command, cmdPath string) string {
	// Convert command path to REST path
	// e.g., "user create" -> "/api/v1/user"
	// e.g., "user get [id]" -> "/api/v1/user/{id}"
	// e.g., "policy recalculate <id>" -> "/api/v1/policy/{id}/recalculate"
	// e.g., "policy bulk-suspend <id> [id...]" -> "/api/v1/policy/{id}/bulk-suspend"

	parts := strings.Split(cmdPath, " ")

	// Build path with prefix
	pathParts := []string{c.config.PathPrefix}

	for i, part := range parts {
		// Skip the last part if it's a CRUD operation
		if i == len(parts)-1 {
			if isCRUDOperation(part) {
				partLower := strings.ToLower(part)
				// get/delete/inspect always take an {id} path parameter
				if partLower == "get" || partLower == "delete" || partLower == "inspect" {
					paramName := extractParameterName(cmd.Use)
					if paramName == "" {
						paramName = "id"
					}
					pathParts = append(pathParts, "{"+paramName+"}")
				}
				break
			}
		}

		// Use resource names as-is without pluralization
		if i < len(parts)-1 || !isCRUDOperation(part) {
			pathParts = append(pathParts, part)
		}
	}

	// For non-CRUD action commands with positional <id> args under a parent entity,
	// restructure to /entity/{id}/action pattern.
	// e.g., /api/v1/policy/recalculate with Use="recalculate <id>"
	//    -> /api/v1/policy/{id}/recalculate
	if cmd.Parent() != nil && !isCRUDOperation(cmd.Name()) {
		paramName := extractParameterName(cmd.Use)
		if paramName != "" {
			// Insert {id} before the action verb
			if len(pathParts) >= 2 {
				newParts := make([]string, 0, len(pathParts)+1)
				newParts = append(newParts, pathParts[:len(pathParts)-1]...)
				newParts = append(newParts, "{"+paramName+"}")
				newParts = append(newParts, pathParts[len(pathParts)-1:]...)
				pathParts = newParts
			}
		} else {
			// Check for common ID flags that should be path parameters
			parentName := cmd.Parent().Name()
			idFlagNames := []string{parentName, parentName + "Number", "id"}
			for _, flagName := range idFlagNames {
				if flag := cmd.Flags().Lookup(flagName); flag != nil {
					if len(pathParts) >= 2 {
						newParts := make([]string, 0, len(pathParts)+1)
						newParts = append(newParts, pathParts[:len(pathParts)-1]...)
						newParts = append(newParts, "{"+flagName+"}")
						newParts = append(newParts, pathParts[len(pathParts)-1:]...)
						pathParts = newParts
						break
					}
				}
			}
		}
	}

	return strings.Join(pathParts, "/")
}

// extractParameterName extracts parameter name from Use field
// e.g., "get [policyNumber]" -> "policyNumber"
// e.g., "get [id]" -> "id"
func extractParameterName(use string) string {
	// Try <param> first (e.g., "get <id>")
	start := strings.Index(use, "<")
	end := strings.Index(use, ">")
	if start != -1 && end != -1 && end > start {
		name := strings.TrimSpace(use[start+1 : end])
		if isValidParamName(name) {
			return name
		}
	}

	// Try [param] (e.g., "get [id]")
	start = strings.Index(use, "[")
	end = strings.Index(use, "]")
	if start != -1 && end != -1 && end > start {
		name := strings.TrimSpace(use[start+1 : end])
		if isValidParamName(name) {
			return name
		}
	}
	return ""
}

func isValidParamName(name string) bool {
	if name == "" {
		return false
	}
	// Reject variadic/body-style args like "key=value ...", "[args...]"
	if strings.Contains(name, "=") || strings.Contains(name, "...") || strings.Contains(name, " ") {
		return false
	}
	return true
}

// updateParameterLocations updates parameter "In" field to "path" for parameters that appear in the URL path
func (c *Converter) updateParameterLocations(parameters *[]RPCParameter, path string) {
	for i := range *parameters {
		param := &(*parameters)[i]
		// Check if parameter name appears in path as {paramName}
		if strings.Contains(path, "{"+param.Name+"}") {
			param.In = "path"
			param.Required = true // Path parameters are always required
		}
	}
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
