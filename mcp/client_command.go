package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newClientAddCommand() *cobra.Command {
	var (
		transportName string
		environment   []string
		headers       []string
		includeTools  []string
		excludeTools  []string
		timeout       time.Duration
		cacheTTL      time.Duration
		noVerify      bool
	)
	cmd := &cobra.Command{
		Use:   "add <name> [--] <command> [args...] | add <name> <url>",
		Short: "Register an external MCP server",
		Args:  cobra.MinimumNArgs(2),
		Example: `  app mcp add local -- npx -y @modelcontextprotocol/server-everything
  app mcp add remote https://example.com/mcp --header 'Authorization: Bearer ...'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := ServerConfig{
				Type: transportName, Timeout: timeout.String(), CacheTTL: cacheTTL.String(),
				IncludeTools: includeTools, ExcludeTools: excludeTools,
			}
			if isRemoteEndpoint(args[1]) {
				if len(args) != 2 {
					return fmt.Errorf("remote MCP server accepts one URL argument")
				}
				cfg.URL = args[1]
			} else {
				cfg.Command = args[1]
				cfg.Args = append([]string(nil), args[2:]...)
				if cfg.Type == "auto" {
					cfg.Type = "stdio"
				}
			}
			var err error
			if cfg.Env, err = parseAssignments(environment, "environment variable", '='); err != nil {
				return err
			}
			if cfg.Headers, err = parseAssignments(headers, "header", ':'); err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}

			registry := NewServerRegistry(rootAppName(cmd))
			if _, exists, err := registry.Get(name); err != nil {
				return err
			} else if exists {
				return fmt.Errorf("MCP server %q is already registered; remove it before replacing it", name)
			}
			var catalog *CatalogCache
			if !noVerify {
				session, err := Dial(cmd.Context(), name, cfg)
				if err != nil {
					return err
				}
				tools, fetchErr := FetchCatalog(cmd.Context(), cfg, session)
				if fetchErr == nil {
					catalog = catalogFromSession(cfg, session, tools, time.Now())
				}
				_ = session.Close()
				if fetchErr != nil {
					return fetchErr
				}
			}
			if err := registry.Add(name, cfg); err != nil {
				return err
			}
			if catalog != nil {
				if err := SaveCatalog(registry, name, catalog); err != nil {
					_ = registry.Remove(name)
					return fmt.Errorf("save catalog: %w", err)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added MCP server %q (%s)\n", name, cfg.endpoint())
			return nil
		},
	}
	cmd.Flags().StringVar(&transportName, "transport", "auto", "Transport: stdio, sse, http, or auto")
	cmd.Flags().StringArrayVar(&environment, "env", nil, "Stdio environment variable KEY=VALUE (repeatable)")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "Remote HTTP header 'Name: Value' (repeatable)")
	cmd.Flags().StringSliceVar(&includeTools, "include-tool", nil, "Only cache tools matching these globs")
	cmd.Flags().StringSliceVar(&excludeTools, "exclude-tool", nil, "Exclude tools matching these globs")
	cmd.Flags().DurationVar(&timeout, "timeout", defaultClientTimeout, "Connection and invocation timeout")
	cmd.Flags().DurationVar(&cacheTTL, "cache-ttl", defaultCatalogCacheTTL, "Catalog cache lifetime")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Save without connecting or fetching the catalog")
	return cmd
}

type clientListRecord struct {
	Name      string       `json:"name"`
	Transport string       `json:"transport"`
	Endpoint  string       `json:"endpoint"`
	ToolCount int          `json:"toolCount"`
	CacheAge  string       `json:"cacheAge,omitempty"`
	Cache     string       `json:"cache"`
	Tools     []CachedTool `json:"tools,omitempty"`
}

func newClientListCommand() *cobra.Command {
	var refresh, showTools bool
	var format string
	cmd := &cobra.Command{
		Use:   "list [name]",
		Short: "List registered external MCP servers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := NewServerRegistry(rootAppName(cmd))
			names, servers, err := registry.List()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				cfg, ok := servers[args[0]]
				if !ok {
					return fmt.Errorf("MCP server %q is not registered", args[0])
				}
				names, servers = []string{args[0]}, map[string]ServerConfig{args[0]: cfg}
			}
			records := make([]clientListRecord, 0, len(names))
			for _, name := range names {
				cfg := servers[name]
				catalog, err := LoadCatalog(registry, name)
				if err != nil {
					return err
				}
				if refresh {
					preferred := ""
					if catalog != nil {
						preferred = catalog.Transport
					}
					catalog, err = RefreshCatalog(cmd.Context(), registry, name, cfg, preferred)
					if err != nil {
						return err
					}
				}
				records = append(records, makeListRecord(name, cfg, catalog, showTools))
			}
			return renderClientList(cmd.OutOrStdout(), records, format, showTools)
		},
		ValidArgsFunction: completeRegisteredServers,
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Connect and refresh tool catalogs")
	cmd.Flags().BoolVar(&showTools, "tools", false, "Include cached tool details")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format: pretty, json, or markdown")
	return cmd
}

func newClientRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an external MCP server and its cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry := NewServerRegistry(rootAppName(cmd))
			if err := registry.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed MCP server %q\n", args[0])
			return nil
		},
		ValidArgsFunction: completeRegisteredServers,
	}
}

func makeListRecord(name string, cfg ServerConfig, catalog *CatalogCache, tools bool) clientListRecord {
	record := clientListRecord{Name: name, Transport: cfg.effectiveTransport(), Endpoint: cfg.endpoint(), Cache: "missing"}
	if catalog == nil {
		return record
	}
	record.ToolCount = len(catalog.Tools)
	record.CacheAge = shortAge(time.Since(catalog.FetchedAt))
	record.Cache = "fresh"
	if catalog.Stale(cfg, time.Now()) {
		record.Cache = "stale"
	}
	if cfg.effectiveTransport() == "auto" && catalog.Transport != "" {
		record.Transport = "auto (" + catalog.Transport + ")"
	}
	if tools {
		record.Tools = catalog.Tools
	}
	return record
}

func renderClientList(out io.Writer, records []clientListRecord, format string, showTools bool) error {
	switch format {
	case "json":
		encoded, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(encoded))
		return err
	case "markdown":
		fmt.Fprintln(out, "| Name | Transport | Endpoint | Tools | Cache |")
		fmt.Fprintln(out, "| --- | --- | --- | ---: | --- |")
		for _, record := range records {
			fmt.Fprintf(out, "| %s | %s | %s | %d | %s |\n", record.Name, record.Transport, record.Endpoint, record.ToolCount, record.Cache)
			if showTools {
				for _, tool := range record.Tools {
					fmt.Fprintf(out, "| ↳ %s | | %s | | |\n", tool.Name, strings.ReplaceAll(tool.Description, "|", "\\|"))
				}
			}
		}
		return nil
	case "pretty":
		writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "NAME\tTRANSPORT\tENDPOINT\tTOOLS\tCACHE")
		for _, record := range records {
			cache := record.Cache
			if record.CacheAge != "" {
				cache += " (" + record.CacheAge + ")"
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\n", record.Name, record.Transport, record.Endpoint, record.ToolCount, cache)
			if showTools {
				for _, tool := range record.Tools {
					fmt.Fprintf(writer, "  %s\t\t%s\t\t\n", tool.Name, tool.Description)
				}
			}
		}
		return writer.Flush()
	default:
		return fmt.Errorf("unsupported format %q (want pretty, json, or markdown)", format)
	}
}

func completeRegisteredServers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	registry := NewServerRegistry(rootAppName(cmd))
	names, _, err := registry.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func isRemoteEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func parseAssignments(values []string, label string, separator byte) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(values))
	for _, value := range values {
		index := strings.IndexByte(value, separator)
		if index <= 0 {
			return nil, fmt.Errorf("invalid %s %q", label, value)
		}
		key := strings.TrimSpace(value[:index])
		if key == "" {
			return nil, fmt.Errorf("invalid %s %q", label, value)
		}
		result[key] = strings.TrimSpace(value[index+1:])
	}
	return result, nil
}

func shortAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	if age < time.Minute {
		return age.Round(time.Second).String()
	}
	if age < time.Hour {
		return age.Round(time.Minute).String()
	}
	return age.Round(time.Hour).String()
}
