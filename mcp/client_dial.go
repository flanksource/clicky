package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

// Caller is the narrow MCP client surface used by command execution. Keeping
// this boundary small permits in-process integration tests without transport
// setup.
type Caller interface {
	ListTools(context.Context, mcpsdk.ListToolsRequest) (*mcpsdk.ListToolsResult, error)
	CallTool(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error)
	OnNotification(func(mcpsdk.JSONRPCNotification))
}

// ClientSession is an initialized MCP connection plus negotiated server and
// transport metadata.
type ClientSession struct {
	Caller        Caller
	Transport     string
	ServerName    string
	ServerVersion string
	close         func() error
	closeOnce     sync.Once
	closeErr      error
}

// Close terminates the client transport and its lifetime context.
func (s *ClientSession) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closeErr = s.close()
	})
	return s.closeErr
}

// Dial connects and initializes a server. For auto transport, preferred is
// tried first when it names a previously successful HTTP or SSE transport.
func Dial(ctx context.Context, name string, cfg ServerConfig, preferred ...string) (*ClientSession, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server %q: %w", name, err)
	}
	transports := []string{cfg.effectiveTransport()}
	if transports[0] == "auto" {
		transports = []string{"http", "sse"}
		if len(preferred) > 0 && (preferred[0] == "http" || preferred[0] == "sse") {
			other := "http"
			if preferred[0] == "http" {
				other = "sse"
			}
			transports = []string{preferred[0], other}
		}
	}

	var failures []string
	for _, transportName := range transports {
		session, err := dialTransport(ctx, cfg, transportName)
		if err == nil {
			return session, nil
		}
		if cfg.OAuth != nil && client.IsOAuthAuthorizationRequiredError(err) {
			return nil, fmt.Errorf("MCP server %q requires OAuth login; run `mcp login %s`: %w", name, name, err)
		}
		failures = append(failures, transportName+": "+err.Error())
	}
	return nil, fmt.Errorf("connect to MCP server %q: %s", name, strings.Join(failures, "; "))
}

func dialTransport(ctx context.Context, cfg ServerConfig, transportName string) (*ClientSession, error) {
	lifetime, cancel := context.WithCancelCause(ctx)
	var c *client.Client
	var err error
	var oauthConfig client.OAuthConfig
	if cfg.OAuth != nil {
		oauthConfig, err = oauthTransportConfig(ctx, cfg)
		if err != nil {
			cancel(err)
			return nil, err
		}
	}
	switch transportName {
	case "stdio":
		env := make([]string, 0, len(cfg.Env))
		for key, value := range cfg.Env {
			env = append(env, key+"="+value)
		}
		sort.Strings(env)
		c = client.NewClient(transport.NewStdio(cfg.Command, env, cfg.Args...))
	case "http":
		options := []transport.StreamableHTTPCOption{
			transport.WithHTTPHeaders(cfg.Headers),
			transport.WithHTTPTimeout(cfg.timeout()),
		}
		if cfg.OAuth != nil {
			c, err = client.NewOAuthStreamableHttpClient(cfg.URL, oauthConfig, options...)
		} else {
			c, err = client.NewStreamableHttpClient(cfg.URL, options...)
		}
	case "sse":
		options := []transport.ClientOption{client.WithHeaders(cfg.Headers)}
		if cfg.OAuth != nil {
			c, err = client.NewOAuthSSEClient(cfg.URL, oauthConfig, options...)
		} else {
			c, err = client.NewSSEMCPClient(cfg.URL, options...)
		}
	default:
		err = fmt.Errorf("unsupported transport %q", transportName)
	}
	if err != nil {
		cancel(err)
		return nil, err
	}
	startupExpired := make(chan struct{})
	startupTimer := time.AfterFunc(cfg.timeout(), func() {
		cancel(context.DeadlineExceeded)
		close(startupExpired)
	})
	startErr := c.Start(lifetime)
	if !startupTimer.Stop() {
		<-startupExpired
	}
	if startErr != nil {
		cancel(startErr)
		_ = c.Close()
		return nil, context.Cause(lifetime)
	}
	if err := context.Cause(lifetime); err != nil {
		_ = c.Close()
		return nil, err
	}
	if stderr, ok := client.GetStderr(c); ok {
		go func() {
			_, _ = io.Copy(io.Discard, stderr)
		}()
	}

	request := mcpsdk.InitializeRequest{}
	request.Params.ProtocolVersion = mcpsdk.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcpsdk.Implementation{Name: "clicky-cli", Version: "1"}
	initCtx, initCancel := context.WithTimeout(lifetime, cfg.timeout())
	result, err := c.Initialize(initCtx, request)
	initCancel()
	if err != nil {
		cancel(err)
		_ = c.Close()
		return nil, err
	}

	return &ClientSession{
		Caller: c, Transport: transportName,
		ServerName: result.ServerInfo.Name, ServerVersion: result.ServerInfo.Version,
		close: func() error {
			err := c.Close()
			cancel(context.Canceled)
			return err
		},
	}, nil
}

// FetchCatalog lists, filters, and normalizes the tools exposed by a session.
func FetchCatalog(ctx context.Context, cfg ServerConfig, session *ClientSession) ([]CachedTool, error) {
	callCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	result, err := session.Caller.ListTools(callCtx, mcpsdk.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	tools := make([]CachedTool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		if !toolIncluded(tool.Name, cfg.IncludeTools, cfg.ExcludeTools) {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("encode schema for tool %q: %w", tool.Name, err)
		}
		tools = append(tools, CachedTool{Name: tool.Name, Description: tool.Description, InputSchema: schema})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

func toolIncluded(name string, includes, excludes []string) bool {
	included := len(includes) == 0
	for _, pattern := range includes {
		if matched, _ := path.Match(pattern, name); matched {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, pattern := range excludes {
		if matched, _ := path.Match(pattern, name); matched {
			return false
		}
	}
	return true
}

// RefreshCatalog connects, fetches, and saves one catalog as a single owned
// operation.
func RefreshCatalog(ctx context.Context, registry *ServerRegistry, name string, cfg ServerConfig, preferred ...string) (*CatalogCache, error) {
	session, err := Dial(ctx, name, cfg, preferred...)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	tools, err := FetchCatalog(ctx, cfg, session)
	if err != nil {
		return nil, err
	}
	catalog := catalogFromSession(cfg, session, tools, time.Now())
	if err := SaveCatalog(registry, name, catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

func catalogFromSession(cfg ServerConfig, session *ClientSession, tools []CachedTool, fetchedAt time.Time) *CatalogCache {
	return &CatalogCache{
		Fingerprint: cfg.Fingerprint(), FetchedAt: fetchedAt, TTL: cfg.cacheTTL(),
		Transport: session.Transport, ServerName: session.ServerName,
		ServerVersion: session.ServerVersion, Tools: tools,
	}
}
