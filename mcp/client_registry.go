package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultClientTimeout   = time.Minute
	defaultCatalogCacheTTL = time.Hour
)

var serverNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ServerConfig is the durable client-side description of one MCP server. Its
// transport fields follow the common mcpServers shape for interoperability.
type ServerConfig struct {
	Type         string            `json:"type,omitempty"`
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	URL          string            `json:"url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	IncludeTools []string          `json:"includeTools,omitempty"`
	ExcludeTools []string          `json:"excludeTools,omitempty"`
	CacheTTL     string            `json:"cacheTTL,omitempty"`
}

// Validate rejects ambiguous transport configurations before a process or
// network connection is attempted.
func (c ServerConfig) Validate() error {
	transport := c.effectiveTransport()
	if transport != "stdio" && transport != "sse" && transport != "http" && transport != "auto" {
		return fmt.Errorf("unsupported transport %q (want stdio, sse, http, or auto)", transport)
	}

	hasCommand := strings.TrimSpace(c.Command) != ""
	hasURL := strings.TrimSpace(c.URL) != ""
	if hasCommand == hasURL {
		return fmt.Errorf("configure exactly one of command or url")
	}
	if transport == "stdio" && !hasCommand {
		return fmt.Errorf("stdio transport requires a command")
	}
	if transport != "stdio" && hasCommand {
		return fmt.Errorf("transport %q requires a url, not a command", transport)
	}
	if len(c.Env) > 0 && !hasCommand {
		return fmt.Errorf("environment variables are only valid for stdio servers")
	}
	if len(c.Headers) > 0 && !hasURL {
		return fmt.Errorf("headers are only valid for remote servers")
	}
	if hasURL {
		parsed, err := url.ParseRequestURI(c.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("invalid MCP server URL %q", c.URL)
		}
	}
	if c.Timeout != "" {
		if d, err := time.ParseDuration(c.Timeout); err != nil || d <= 0 {
			return fmt.Errorf("invalid timeout %q", c.Timeout)
		}
	}
	if c.CacheTTL != "" {
		if d, err := time.ParseDuration(c.CacheTTL); err != nil || d <= 0 {
			return fmt.Errorf("invalid cache TTL %q", c.CacheTTL)
		}
	}
	for _, pattern := range append(append([]string{}, c.IncludeTools...), c.ExcludeTools...) {
		if _, err := path.Match(pattern, "tool"); err != nil {
			return fmt.Errorf("invalid tool glob %q: %w", pattern, err)
		}
	}
	return nil
}

// Fingerprint identifies fields that can change the remote tool catalog.
// Runtime timeout and cache lifetime choices deliberately do not invalidate it.
func (c ServerConfig) Fingerprint() string {
	payload := struct {
		Type         string            `json:"type"`
		Command      string            `json:"command"`
		Args         []string          `json:"args"`
		Env          map[string]string `json:"env"`
		URL          string            `json:"url"`
		Headers      map[string]string `json:"headers"`
		IncludeTools []string          `json:"includeTools"`
		ExcludeTools []string          `json:"excludeTools"`
	}{c.effectiveTransport(), c.Command, c.Args, c.Env, c.URL, c.Headers, c.IncludeTools, c.ExcludeTools}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c ServerConfig) effectiveTransport() string {
	if c.Type != "" {
		return c.Type
	}
	if strings.TrimSpace(c.Command) != "" {
		return "stdio"
	}
	return "auto"
}

func (c ServerConfig) timeout() time.Duration {
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
		return d
	}
	return defaultClientTimeout
}

func (c ServerConfig) cacheTTL() time.Duration {
	if d, err := time.ParseDuration(c.CacheTTL); err == nil && d > 0 {
		return d
	}
	return defaultCatalogCacheTTL
}

func (c ServerConfig) endpoint() string {
	if c.Command != "" {
		return strings.Join(append([]string{c.Command}, c.Args...), " ")
	}
	return c.URL
}

// ServerRegistry owns the mcpServers key in one application's client config.
// Unknown top-level keys survive every mutation without being decoded.
type ServerRegistry struct {
	appName string
	dir     string
}

// NewServerRegistry returns the registry namespaced to the host CLI name.
func NewServerRegistry(appName string) *ServerRegistry {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return newServerRegistryAt(appName, filepath.Join(configDir, appName, "mcp"))
}

func newServerRegistryAt(appName, dir string) *ServerRegistry {
	return &ServerRegistry{appName: appName, dir: dir}
}

func (r *ServerRegistry) configPath() string { return filepath.Join(r.dir, "config.json") }
func (r *ServerRegistry) cacheDir() string   { return filepath.Join(r.dir, "cache") }

// Get returns a named server and reports whether it is registered.
func (r *ServerRegistry) Get(name string) (ServerConfig, bool, error) {
	servers, _, err := r.load()
	if err != nil {
		return ServerConfig{}, false, err
	}
	cfg, ok := servers[name]
	return cfg, ok, nil
}

// List returns a name-sorted snapshot of the registered servers.
func (r *ServerRegistry) List() ([]string, map[string]ServerConfig, error) {
	servers, _, err := r.load()
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, servers, nil
}

// Add validates and atomically stores a server without disturbing foreign
// configuration keys.
func (r *ServerRegistry) Add(name string, cfg ServerConfig) error {
	if !serverNamePattern.MatchString(name) {
		return fmt.Errorf("invalid server name %q", name)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	servers, root, err := r.load()
	if err != nil {
		return err
	}
	servers[name] = cfg
	encoded, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root["mcpServers"] = encoded
	return writeJSONAtomic(r.configPath(), root)
}

// Remove deletes a server and its cached catalog.
func (r *ServerRegistry) Remove(name string) error {
	servers, root, err := r.load()
	if err != nil {
		return err
	}
	if _, ok := servers[name]; !ok {
		return fmt.Errorf("MCP server %q is not registered", name)
	}
	delete(servers, name)
	encoded, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root["mcpServers"] = encoded
	if err := writeJSONAtomic(r.configPath(), root); err != nil {
		return err
	}
	if err := os.Remove(catalogPath(r, name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove catalog cache: %w", err)
	}
	return nil
}

func (r *ServerRegistry) load() (map[string]ServerConfig, map[string]json.RawMessage, error) {
	data, err := os.ReadFile(r.configPath())
	if os.IsNotExist(err) {
		return map[string]ServerConfig{}, map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read MCP registry: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parse MCP registry: %w", err)
	}
	if root == nil {
		root = map[string]json.RawMessage{}
	}
	servers := map[string]ServerConfig{}
	if raw := root["mcpServers"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, nil, fmt.Errorf("parse MCP servers: %w", err)
		}
	}
	if servers == nil {
		servers = map[string]ServerConfig{}
	}
	for name := range servers {
		if !serverNamePattern.MatchString(name) {
			return nil, nil, fmt.Errorf("invalid server name %q in MCP registry", name)
		}
	}
	return servers, root, nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
