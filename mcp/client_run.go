package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

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
			toolArgs := args[1:]

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
			return executeRunServerCommand(cmd, registry, serverName, cfg, catalog, toolArgs)
		},
		ValidArgsFunction: completeRunArguments,
	}
	return cmd
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

// executeRunServerCommand builds the cached server subtree after the outer run
// command resolves its application-scoped registry.
func executeRunServerCommand(parent *cobra.Command, registry *ServerRegistry, serverName string, cfg ServerConfig, catalog *CatalogCache, args []string) error {
	serverCmd := &cobra.Command{
		Use:           serverName,
		Short:         fmt.Sprintf("Invoke tools exposed by the %s MCP server", serverName),
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: parent.CommandPath() + " " + serverName,
		},
	}
	serverCmd.SetIn(parent.InOrStdin())
	serverCmd.SetOut(parent.OutOrStdout())
	serverCmd.SetErr(parent.ErrOrStderr())
	for _, tool := range catalog.Tools {
		toolCmd, err := newRunToolCommand(registry, serverName, cfg, catalog, tool)
		if err != nil {
			return err
		}
		serverCmd.AddCommand(toolCmd)
	}
	serverCmd.SetArgs(args)
	return serverCmd.ExecuteContext(parent.Context())
}

// newRunToolCommand translates one cached MCP schema into a Cobra command.
func newRunToolCommand(registry *ServerRegistry, serverName string, cfg ServerConfig, catalog *CatalogCache, tool CachedTool) (*cobra.Command, error) {
	var jsonOutput bool
	var timeout time.Duration
	short := strings.TrimSpace(tool.Description)
	if firstLine, _, found := strings.Cut(short, "\n"); found {
		short = strings.TrimSpace(firstLine)
	}
	if short == "" {
		short = "Invoke the " + tool.Name + " MCP tool"
	}
	toolCmd := &cobra.Command{
		Use:           tool.Name,
		Short:         short,
		Long:          short,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	bindings, err := bindToolFlags(toolCmd, tool.InputSchema, parseMCPArgumentDescriptions(tool.Description))
	if err != nil {
		return nil, fmt.Errorf("build flags for tool %q: %w", tool.Name, err)
	}
	toolCmd.Flags().BoolVar(&jsonOutput, "json", false, "Print the complete MCP result as JSON")
	toolCmd.Flags().Bool("refresh", false, "Refresh the tool catalog before invocation")
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
	return toolCmd, nil
}

// parseMCPArgumentDescriptions recovers flag documentation from the common
// Args-style block used by servers that omit JSON Schema descriptions.
func parseMCPArgumentDescriptions(description string) map[string]string {
	lines := strings.Split(description, "\n")
	header := -1
	headerIndent := 0
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case "Args:", "Arguments:", "Parameters:":
			header = i
			headerIndent = leadingWhitespace(line)
		}
		if header >= 0 {
			break
		}
	}
	if header < 0 {
		return nil
	}

	descriptions := map[string]string{}
	entryIndent := -1
	current := ""
	for _, line := range lines[header+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := leadingWhitespace(line)
		if indent <= headerIndent {
			break
		}
		if entryIndent < 0 {
			entryIndent = indent
		}
		if indent == entryIndent {
			name, text, found := strings.Cut(trimmed, ":")
			if !found {
				current = ""
				continue
			}
			name = strings.TrimSpace(name)
			if typeStart := strings.IndexAny(name, " ("); typeStart >= 0 {
				name = name[:typeStart]
			}
			current = name
			descriptions[current] = strings.TrimSpace(text)
			continue
		}
		if current != "" && indent > entryIndent {
			descriptions[current] = strings.TrimSpace(descriptions[current] + " " + trimmed)
		}
	}
	if len(descriptions) == 0 {
		return nil
	}
	return descriptions
}

func leadingWhitespace(value string) int {
	return len(value) - len(strings.TrimLeft(value, " \t"))
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
	encodedLen := 0
	padding := 0
	seenPadding := false
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char == '\r' || char == '\n' {
			continue
		}
		encodedLen++
		if char == '=' {
			seenPadding = true
			padding++
			continue
		}
		if seenPadding || !isStdBase64Byte(char) {
			return 0
		}
	}
	if encodedLen%4 != 0 || padding > 2 {
		return 0
	}
	return base64.StdEncoding.DecodedLen(encodedLen) - padding
}

func isStdBase64Byte(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '+' || value == '/'
}
