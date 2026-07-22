package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type runPolicy struct {
	allow []string
	deny  []string
}

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run <server> [tool] [tool-flags]",
		Short:              "Invoke a registered MCP tool with typed flags",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (len(args) == 1 && args[0] == "--help") {
				return renderRunServers(cmd)
			}
			serverName := args[0]
			policy, toolArgs, err := parseRunPolicyArgs(args[1:])
			if err != nil {
				return err
			}

			registry := NewServerRegistry(rootAppName(cmd))
			cfg, ok, err := registry.Get(serverName)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("MCP server %q is not registered", serverName)
			}
			forceRefresh := containsRawFlag(toolArgs, "--refresh")
			catalog, err := loadRunCatalog(cmd.Context(), registry, serverName, cfg, forceRefresh)
			if err != nil {
				return err
			}
			tools := permittedTools(serverName, catalog.Tools, policy)
			if len(toolArgs) == 0 || (len(toolArgs) == 1 && toolArgs[0] == "--help") {
				return renderRunTools(cmd, serverName, tools)
			}
			toolName := toolArgs[0]
			tool, ok := findCachedTool(tools, toolName)
			if !ok {
				if _, exists := findCachedTool(catalog.Tools, toolName); exists {
					return fmt.Errorf("tool %q is not permitted by this shortcut", toolName)
				}
				return fmt.Errorf("MCP server %q has no cached tool %q", serverName, toolName)
			}
			return executeEphemeralTool(cmd, registry, serverName, cfg, catalog, tool, toolArgs[1:])
		},
		ValidArgsFunction: completeRunArguments,
	}
	return cmd
}

func parseRunPolicyArgs(args []string) (runPolicy, []string, error) {
	policy := runPolicy{}
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 {
		return policy, args, nil
	}
	for i := 0; i < separator; i++ {
		switch args[i] {
		case "--allow-tool", "--deny-tool":
			if i+1 >= separator {
				return runPolicy{}, nil, fmt.Errorf("%s requires a pattern", args[i])
			}
			if args[i] == "--allow-tool" {
				policy.allow = append(policy.allow, args[i+1])
			} else {
				policy.deny = append(policy.deny, args[i+1])
			}
			i++
		default:
			return runPolicy{}, nil, fmt.Errorf("unknown shortcut policy option %q", args[i])
		}
	}
	return policy, args[separator+1:], nil
}

func loadRunCatalog(ctx context.Context, registry *ServerRegistry, name string, cfg ServerConfig, refresh bool) (*CatalogCache, error) {
	catalog, err := LoadCatalog(registry, name)
	if err != nil {
		return nil, err
	}
	if catalog != nil && !refresh {
		return catalog, nil
	}
	preferred := ""
	if catalog != nil {
		preferred = catalog.Transport
	}
	return RefreshCatalog(ctx, registry, name, cfg, preferred)
}

func executeEphemeralTool(parent *cobra.Command, registry *ServerRegistry, serverName string, cfg ServerConfig, catalog *CatalogCache, tool CachedTool, args []string) error {
	var jsonOutput, refresh bool
	var timeout time.Duration
	toolCmd := &cobra.Command{
		Use:           tool.Name,
		Short:         tool.Description,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	bindings, err := bindToolFlags(toolCmd, tool.InputSchema)
	if err != nil {
		return fmt.Errorf("build flags for tool %q: %w", tool.Name, err)
	}
	toolCmd.Flags().BoolVar(&jsonOutput, "json", false, "Print the complete MCP result as JSON")
	toolCmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh the tool catalog before invocation")
	toolCmd.Flags().DurationVar(&timeout, "timeout", cfg.timeout(), "Invocation timeout")
	toolCmd.RunE = func(cmd *cobra.Command, positional []string) error {
		arguments, err := assembleArguments(cmd, bindings)
		if err != nil {
			return err
		}
		runtimeCfg := cfg
		runtimeCfg.Timeout = timeout.String()
		result, err := invokeMCPTool(cmd.Context(), registry, serverName, runtimeCfg, catalog, tool.Name, arguments)
		if err != nil {
			return err
		}
		return renderCallToolResult(cmd.OutOrStdout(), cmd.ErrOrStderr(), tool.Name, result, jsonOutput)
	}
	toolCmd.SetIn(parent.InOrStdin())
	toolCmd.SetOut(parent.OutOrStdout())
	toolCmd.SetErr(parent.ErrOrStderr())
	toolCmd.SetArgs(args)
	return toolCmd.ExecuteContext(parent.Context())
}

func invokeMCPTool(ctx context.Context, registry *ServerRegistry, serverName string, cfg ServerConfig, catalog *CatalogCache, toolName string, arguments map[string]any) (*mcpsdk.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	operation := clicky.StartTask("MCP: "+serverName+"/"+toolName, func(taskCtx flanksourceContext.Context, progress *task.Task) (*mcpsdk.CallToolResult, error) {
		linkedCtx, cancelLinked := context.WithCancel(taskCtx)
		stopParentCancel := context.AfterFunc(ctx, cancelLinked)
		defer func() {
			stopParentCancel()
			cancelLinked()
		}()
		preferred := ""
		if catalog != nil {
			preferred = catalog.Transport
		}
		session, err := Dial(linkedCtx, serverName, cfg, preferred)
		if err != nil {
			return nil, err
		}
		defer session.Close()
		session.Caller.OnNotification(func(notification mcpsdk.JSONRPCNotification) {
			handleMCPNotification(progress, notification)
		})

		request := mcpsdk.CallToolRequest{}
		request.Params.Name = toolName
		request.Params.Arguments = arguments
		request.Params.Meta = &mcpsdk.Meta{ProgressToken: serverName + "/" + toolName}
		callCtx, cancel := context.WithTimeout(linkedCtx, cfg.timeout())
		result, err := session.Caller.CallTool(callCtx, request)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("call tool %q: %w", toolName, err)
		}
		if !result.IsError {
			if tools, refreshErr := FetchCatalog(linkedCtx, cfg, session); refreshErr != nil {
				progress.Warnf("catalog refresh failed: %v", refreshErr)
			} else if saveErr := SaveCatalog(registry, serverName, catalogFromSession(cfg, session, tools, time.Now())); saveErr != nil {
				progress.Warnf("catalog cache write failed: %v", saveErr)
			}
		}
		return result, nil
	}, clicky.WithTaskTimeout(cfg.timeout()))
	return operation.GetResult()
}

func handleMCPNotification(progress *task.Task, notification mcpsdk.JSONRPCNotification) {
	encoded, err := json.Marshal(notification)
	if err != nil {
		return
	}
	switch notification.Method {
	case "notifications/progress":
		var update mcpsdk.ProgressNotification
		if json.Unmarshal(encoded, &update) != nil {
			return
		}
		if update.Params.Total > 0 {
			progress.SetProgress(int(update.Params.Progress), int(update.Params.Total))
		}
		if update.Params.Message != "" {
			progress.SetDescription(update.Params.Message)
		}
	case "notifications/message":
		var message mcpsdk.LoggingMessageNotification
		if json.Unmarshal(encoded, &message) != nil {
			return
		}
		text := notificationText(message.Params.Data)
		switch message.Params.Level {
		case mcpsdk.LoggingLevelDebug:
			progress.Debugf("%s", text)
		case mcpsdk.LoggingLevelWarning, mcpsdk.LoggingLevelNotice:
			progress.Warnf("%s", text)
		case mcpsdk.LoggingLevelError, mcpsdk.LoggingLevelCritical, mcpsdk.LoggingLevelAlert, mcpsdk.LoggingLevelEmergency:
			progress.Errorf("%s", text)
		default:
			progress.Infof("%s", text)
		}
	}
}

func renderCallToolResult(out, errOut io.Writer, toolName string, result *mcpsdk.CallToolResult, jsonOutput bool) error {
	if jsonOutput {
		destination := out
		if result.IsError {
			destination = errOut
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(destination, string(encoded))
		if err != nil {
			return err
		}
		if result.IsError {
			return fmt.Errorf("MCP tool %q returned an error", toolName)
		}
		return nil
	}

	destination := out
	if result.IsError {
		destination = errOut
	}
	for _, content := range result.Content {
		switch value := content.(type) {
		case mcpsdk.TextContent:
			fmt.Fprintln(destination, prettyJSONText(destination, value.Text))
		case mcpsdk.ImageContent:
			fmt.Fprintf(destination, "[image %s, %d bytes]\n", value.MIMEType, decodedSize(value.Data))
		case mcpsdk.AudioContent:
			fmt.Fprintf(destination, "[audio %s, %d bytes]\n", value.MIMEType, decodedSize(value.Data))
		case mcpsdk.EmbeddedResource:
			encoded, _ := json.Marshal(value.Resource)
			fmt.Fprintln(destination, string(encoded))
		default:
			encoded, _ := json.Marshal(value)
			fmt.Fprintln(destination, string(encoded))
		}
	}
	if result.IsError {
		return fmt.Errorf("MCP tool %q returned an error", toolName)
	}
	return nil
}

func prettyJSONText(out io.Writer, value string) string {
	file, ok := out.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) || !json.Valid([]byte(value)) {
		return value
	}
	var formatted any
	if json.Unmarshal([]byte(value), &formatted) != nil {
		return value
	}
	encoded, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return value
	}
	return string(encoded)
}

func renderRunServers(cmd *cobra.Command) error {
	registry := NewServerRegistry(rootAppName(cmd))
	names, _, err := registry.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No external MCP servers registered. Use 'mcp add' first.")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Registered MCP servers:")
	for _, name := range names {
		fmt.Fprintln(cmd.OutOrStdout(), "  "+name)
	}
	return nil
}

func renderRunTools(cmd *cobra.Command, serverName string, tools []CachedTool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Tools cached for %s:\n", serverName)
	for _, tool := range tools {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %s\n", tool.Name, tool.Description)
	}
	return nil
}

func completeRunArguments(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	registry := NewServerRegistry(rootAppName(cmd))
	if len(args) == 0 {
		names, _, err := registry.List()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 1 {
		catalog, err := LoadCatalog(registry, args[0])
		if err != nil || catalog == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(catalog.Tools))
		for _, tool := range catalog.Tools {
			names = append(names, tool.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func permittedTools(server string, tools []CachedTool, policy runPolicy) []CachedTool {
	result := make([]CachedTool, 0, len(tools))
	for _, tool := range tools {
		fullName := "mcp__" + server + "__" + tool.Name
		allowed := len(policy.allow) == 0 || matchesAnyGlob(fullName, policy.allow)
		if allowed && !matchesAnyGlob(fullName, policy.deny) {
			result = append(result, tool)
		}
	}
	return result
}

func matchesAnyGlob(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, value); matched {
			return true
		}
	}
	return false
}

func findCachedTool(tools []CachedTool, name string) (CachedTool, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return CachedTool{}, false
}

func containsRawFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || arg == name+"=true" {
			return true
		}
	}
	return false
}

func notificationText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

func decodedSize(value string) int {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return 0
	}
	return len(data)
}
