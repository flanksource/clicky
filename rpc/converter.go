package rpc

import (
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/flags"
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

	// Process flags. Use LocalNonPersistentFlags so only this command's own
	// operation flags become schema parameters — never the inherited persistent
	// globals (format/log-level/properties/config/company/entity/limit/...) that
	// cobra folds into Flags(), nor the command's own persistent flags which
	// exist to cascade to children rather than parameterize this operation.
	cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
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
		prop.Enum = flags.AllowedValues(flag)
		param.Enum = flags.AllowedValues(flag)

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
			// pflag renders an empty slice default as the literal "[]". Emitting
			// that as a schema default puts the two characters "[]" in every
			// generated form field, and a client that posts the pre-filled value
			// back sends a one-element slice containing "[]". A repeatable flag
			// with no default simply has none.
			if flag.DefValue != "" && (!isSliceFlag(flag) || flag.DefValue != "[]") {
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
		Command:     NewCobraExecutableCommand(cmd),
		Path:        path,
		Method:      method,
		Tags:        tags,
	}
	operation.Clicky = &ClickyOperationMeta{
		Command: strings.ReplaceAll(cmdPath, " ", "/"),
	}
	if meta := clicky.GetCommandOpenAPIMeta(cmd); meta != nil {
		operation.Clicky.SurfaceID = clickySurfaceID(meta.Entity, meta.Parent, meta.Admin)
		operation.Clicky.Entity = meta.Entity
		operation.Clicky.Parent = meta.Parent
		operation.Clicky.Aliases = append([]string(nil), meta.Aliases...)
		operation.Clicky.Admin = meta.Admin
		operation.Clicky.Icon = meta.Icon
		operation.Clicky.Path = meta.Path
		operation.Clicky.Title = meta.Title
		operation.Clicky.Verb = meta.Verb
		operation.Clicky.Scope = meta.Scope
		if meta.Verb == "" && meta.Entity != "" {
			operation.Clicky.Verb = "list"
			operation.Clicky.Scope = "collection"
		}
		operation.Clicky.ActionName = meta.ActionName
		operation.Clicky.IDParam = meta.IDParam
		operation.Clicky.SupportsLookup = meta.SupportsLookup
		operation.Clicky.SupportsFilterMode = meta.SupportsFilterMode
		hints := meta.ToolHints
		if hints.Group == "" {
			hints.Group = meta.ToolGroup
		}
		operation.ToolHints = hints
		operation.Clicky.ToolHints = hints
		if !hints.IsZero() {
			operation.Clicky.OpenAPIToolHints = &hints
		}
		operation.Clicky.Group = hints.Group
		operation.Group = hints.Group
	}

	if df := clicky.GetDataFunc(cmd); df != nil {
		operation.DataFunc = df
	}
	if cdf := clicky.GetContextDataFunc(cmd); cdf != nil {
		operation.ContextDataFunc = ContextDataFunc(cdf)
	}
	if lf := clicky.GetLookupFunc(cmd); lf != nil {
		operation.LookupFunc = lf
	}
	if clf := clicky.GetContextLookupFunc(cmd); clf != nil {
		operation.ContextLookupFunc = ContextLookupFunc(clf)
	}
	if meta := clicky.GetCommandResponseMeta(cmd); meta != nil {
		operation.ResponseType = meta.Type
		operation.ResponseArray = meta.Array
		operation.ResponsePaged = meta.Paged
		operation.ResponseEntityID = meta.EntityID
	}

	return operation, nil
}

func clickySurfaceID(entity string, parent string, admin bool) string {
	if entity == "" {
		return ""
	}

	parts := []string{"entity", entity}
	if parent != "" {
		parts = append(parts, "parent", parent)
	}
	if admin {
		parts = append(parts, "admin")
	}

	return strings.Join(parts, ":")
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
	// A command the author marked local-only administers the process rather than
	// serving a resource. Publishing the CLI published these too, so an
	// unauthenticated request could run a schema migration or start a nested
	// server.
	if clicky.IsLocalOnly(cmd) {
		return false
	}
	// Skip an entity's `list` subcommand when its parent entity-root is itself
	// runnable as a list shortcut (parent has RunE, an entity annotation, and
	// no operation verb). Both produce the same GET /api/v1/<entity>, and the
	// parent is the canonical endpoint that mirrors the CLI.
	if meta := clicky.GetCommandOpenAPIMeta(cmd); meta != nil && meta.Verb == "list" {
		parent := cmd.Parent()
		if parent != nil && (parent.RunE != nil || parent.Run != nil) {
			if parentMeta := clicky.GetCommandOpenAPIMeta(parent); parentMeta != nil && parentMeta.Verb == "" && parentMeta.Entity != "" {
				return false
			}
		}
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
	if meta := clicky.GetCommandOpenAPIMeta(cmd); meta != nil {
		if meta.Method != "" {
			return strings.ToUpper(meta.Method)
		}
		// Entity-root command (annotated with entity name but no operation verb).
		// It mirrors the CLI: typing `xero-cli accounts` lists, so the matching
		// REST endpoint is GET /api/v1/accounts. Without this, the cmdPath
		// "accounts" matches no CRUD keyword below and falls through to
		// DefaultMethod (POST), colliding with the entity's `create` subcommand.
		if meta.Verb == "" && meta.Entity != "" {
			return "GET"
		}
		// An annotated operation states its verb, so use it rather than guessing
		// from the command path. The guess below matches CRUD keywords anywhere
		// in that path, which silently misreads any entity whose *name* contains
		// one: "target update" contains "get", so it inferred GET and collided
		// with the entity's own list route. "budget delete" and "asset set" fail
		// the same way.
		if method := methodForVerb(meta.Verb); method != "" {
			return method
		}
	}

	// Match the last segment — the verb — rather than searching the whole path.
	// A command path carries its parents' names, and matching anywhere in it
	// reads a CRUD keyword out of a noun: "target scan" contains "get", so the
	// action that starts a scan was published as a safe, prefetchable GET.
	verb := strings.ToLower(cmdPath)
	if index := strings.LastIndex(verb, " "); index != -1 {
		verb = verb[index+1:]
	}

	switch verb {
	case "get", "list", "show", "describe":
		return "GET"
	case "create", "add", "new":
		return "POST"
	case "update", "edit", "modify", "set":
		return "PUT"
	case "delete", "remove", "destroy":
		return "DELETE"
	}

	// Anything else does something rather than returning something, so the
	// configured default (POST) is the safe reading.
	return c.config.DefaultMethod
}

// methodForVerb maps an entity operation verb to its HTTP method. An empty
// result means the verb is not one of the CRUD four — a custom action, say —
// and the caller should fall back to inferring one.
func methodForVerb(verb string) string {
	switch strings.ToLower(verb) {
	case "list", "get":
		return "GET"
	case "create":
		return "POST"
	case "update":
		return "PUT"
	case "delete":
		return "DELETE"
	default:
		return ""
	}
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
	// Only applies when the command is nested under a parent entity command
	// (not at the top level under the root command).
	//
	// Actions declared with WithOptionalID are exempt: they are invocable
	// without an id (collection-level summaries like `activity overview`), so
	// inserting an {id} segment would make the no-id REST call (which the
	// frontend issues) fall through to the entity's get-by-id route. Keep the
	// flat /entity/action path for those.
	if meta := clicky.GetCommandOpenAPIMeta(cmd); meta != nil && meta.OptionalID {
		return strings.Join(pathParts, "/")
	}
	// A multi-operand action (e.g. `diff <a> <b>`) compares two instances and
	// has no single entity id to lift into the path — both operands are body
	// args. Lifting the first into {id} produces /entity/{a}/diff, which the
	// flat-path frontend never calls. Keep it flat; only single-positional
	// actions (`recalculate <id>`) restructure to /entity/{id}/action.
	if positionalOperandCount(cmd.Use) > 1 {
		return strings.Join(pathParts, "/")
	}
	if cmd.Parent() != nil && cmd.Parent().Parent() != nil && !isCRUDOperation(cmd.Name()) {
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

// positionalOperandCount counts the valid positional operands declared in a
// command's Use string — both <required> and [optional] forms. Variadic /
// body-style tokens (`[args...]`, `key=value`) and the conventional `[flags]`
// placeholder are not operands and don't count. Used to distinguish a single-id
// action (`recalculate <id>`, count 1, lifts to /entity/{id}/action) from a
// multi-operand action (`diff <a> <b>`, count 2, stays flat).
func positionalOperandCount(use string) int {
	count := 0
	for _, open := range []struct{ l, r byte }{{'<', '>'}, {'[', ']'}} {
		s := use
		for {
			start := strings.IndexByte(s, open.l)
			if start == -1 {
				break
			}
			end := strings.IndexByte(s[start:], open.r)
			if end == -1 {
				break
			}
			end += start
			name := strings.TrimSpace(s[start+1 : end])
			if name != "flags" && isValidParamName(name) {
				count++
			}
			s = s[end+1:]
		}
	}
	return count
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

// isSliceFlag reports whether a flag holds multiple values, i.e. whether pflag
// would render its zero value as "[]".
func isSliceFlag(flag *pflag.Flag) bool {
	_, ok := flag.Value.(pflag.SliceValue)
	return ok
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
