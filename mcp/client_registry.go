package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
)

const (
	defaultClientTimeout   = time.Minute
	defaultCatalogCacheTTL = time.Hour
)

var serverNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ServerConfig is the durable client-side description of one MCP server. Its
// transport fields follow the common mcpServers shape for interoperability.
type ServerConfig struct {
	Type         string             `json:"type,omitempty"`
	Command      string             `json:"command,omitempty"`
	Args         []string           `json:"args,omitempty"`
	Env          map[string]string  `json:"env,omitempty"`
	URL          string             `json:"url,omitempty"`
	Headers      map[string]string  `json:"headers,omitempty"`
	OAuth        *OAuthClientConfig `json:"oauth,omitempty"`
	Timeout      string             `json:"timeout,omitempty"`
	IncludeTools []string           `json:"includeTools,omitempty"`
	ExcludeTools []string           `json:"excludeTools,omitempty"`
	CacheTTL     string             `json:"cacheTTL,omitempty"`
}

// OAuthClientConfig is the durable, non-token portion of a remote server's
// OAuth setup. ClientSecret accepts only env:NAME and file:PATH references so
// pre-registered secrets stay out of arguments and registry files.
type OAuthClientConfig struct {
	ClientID                     string   `json:"clientId,omitempty"`
	ClientSecret                 string   `json:"clientSecret,omitempty"`
	ClientName                   string   `json:"clientName,omitempty"`
	Scopes                       []string `json:"scopes,omitempty"`
	RedirectURI                  string   `json:"redirectUri,omitempty"`
	AuthServerMetadataURL        string   `json:"authServerMetadataUrl,omitempty"`
	ProtectedResourceMetadataURL string   `json:"protectedResourceMetadataUrl,omitempty"`
	Issuer                       string   `json:"issuer,omitempty"`
	DynamicallyRegistered        bool     `json:"dynamicallyRegistered,omitempty"`

	// TokenStore is required when an OAuth configuration is passed directly to
	// Dial. Registry-loaded configurations receive a private file-backed store.
	TokenStore client.TokenStore `json:"-"`
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
	if c.OAuth != nil && !hasURL {
		return fmt.Errorf("OAuth is only valid for remote servers")
	}
	if c.OAuth != nil {
		if err := validateSecureHTTPURL(c.URL, "OAuth MCP server URL"); err != nil {
			return err
		}
		if c.OAuth.ClientSecret != "" && c.OAuth.ClientID == "" {
			return fmt.Errorf("OAuth client secret requires a client ID")
		}
		if err := validateOAuthSecretReference(c.OAuth.ClientSecret); err != nil {
			return err
		}
		for name := range c.Headers {
			if strings.EqualFold(name, "Authorization") {
				return fmt.Errorf("OAuth cannot be combined with an Authorization header")
			}
		}
		for _, scope := range c.OAuth.Scopes {
			if strings.TrimSpace(scope) == "" {
				return fmt.Errorf("OAuth scopes cannot be empty")
			}
		}
		if c.OAuth.RedirectURI != "" {
			if err := validateOAuthRedirectURI(c.OAuth.RedirectURI); err != nil {
				return err
			}
		}
		if c.OAuth.AuthServerMetadataURL != "" {
			if err := validateSecureHTTPURL(c.OAuth.AuthServerMetadataURL, "OAuth authorization server metadata URL"); err != nil {
				return err
			}
		}
		if c.OAuth.ProtectedResourceMetadataURL != "" {
			if err := validateSecureHTTPURL(c.OAuth.ProtectedResourceMetadataURL, "OAuth protected resource metadata URL"); err != nil {
				return err
			}
			if err := validateResourceMetadataOrigin(c.URL, c.OAuth.ProtectedResourceMetadataURL); err != nil {
				return err
			}
		}
		if c.OAuth.Issuer != "" {
			if err := validateSecureHTTPURL(c.OAuth.Issuer, "OAuth issuer URL"); err != nil {
				return err
			}
		}
	}
	if hasURL {
		if err := validateHTTPURL(c.URL, "MCP server URL"); err != nil {
			return err
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
		Type         string             `json:"type"`
		Command      string             `json:"command"`
		Args         []string           `json:"args"`
		Env          map[string]string  `json:"env"`
		URL          string             `json:"url"`
		Headers      map[string]string  `json:"headers"`
		OAuth        *OAuthClientConfig `json:"oauth"`
		IncludeTools []string           `json:"includeTools"`
		ExcludeTools []string           `json:"excludeTools"`
	}{c.effectiveTransport(), c.Command, c.Args, c.Env, c.URL, c.Headers, c.OAuth, c.IncludeTools, c.ExcludeTools}
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

func validateHTTPURL(value, label string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid %s %q", label, value)
	}
	return nil
}

func validateSecureHTTPURL(value, label string) error {
	if err := validateHTTPURL(value, label); err != nil {
		return err
	}
	parsed, _ := url.Parse(value)
	if parsed.Scheme == "https" || (parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return nil
	}
	return fmt.Errorf("invalid %s %q: use https except for loopback addresses", label, value)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateServerName(name string) error {
	if !serverNamePattern.MatchString(name) {
		return fmt.Errorf("invalid server name %q", name)
	}
	return nil
}

// ServerRegistry owns the mcpServers key in one application's client config.
// Unknown top-level keys survive every mutation without being decoded.
type ServerRegistry struct {
	appName string
	dir     string
}

// NewServerRegistry returns the registry namespaced to the host CLI name.
//
// XDG_CONFIG_HOME is honored before os.UserConfigDir on every platform, not
// only where the standard library reads it. On macOS os.UserConfigDir answers
// ~/Library/Application Support and ignores the variable outright, so a caller
// that set it — a test isolating itself into a temp directory, a user keeping
// one config tree across machines — silently got the real config instead.
func NewServerRegistry(appName string) *ServerRegistry {
	return newServerRegistryAt(appName, filepath.Join(userConfigDir(), appName, "mcp"))
}

func userConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func newServerRegistryAt(appName, dir string) *ServerRegistry {
	return &ServerRegistry{appName: appName, dir: dir}
}

func (r *ServerRegistry) configPath() string { return filepath.Join(r.dir, "config.json") }
func (r *ServerRegistry) cacheDir() string   { return filepath.Join(r.dir, "cache") }
func (r *ServerRegistry) oauthDir() string   { return filepath.Join(r.dir, "oauth") }

func (r *ServerRegistry) bind(name string, cfg ServerConfig) (ServerConfig, error) {
	if err := validateServerName(name); err != nil {
		return ServerConfig{}, err
	}
	if cfg.OAuth != nil {
		oauth := *cfg.OAuth
		oauth.TokenStore = &fileOAuthTokenStore{path: filepath.Join(r.oauthDir(), name+".json")}
		cfg.OAuth = &oauth
	}
	return cfg, nil
}

// Get returns a named server and reports whether it is registered.
func (r *ServerRegistry) Get(name string) (ServerConfig, bool, error) {
	if err := validateServerName(name); err != nil {
		return ServerConfig{}, false, err
	}
	servers, _, err := r.load()
	if err != nil {
		return ServerConfig{}, false, err
	}
	cfg, ok := servers[name]
	if !ok {
		return ServerConfig{}, false, nil
	}
	cfg, err = r.bind(name, cfg)
	return cfg, true, err
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
		servers[name], err = r.bind(name, servers[name])
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Strings(names)
	return names, servers, nil
}

// Add validates and atomically stores a server without disturbing foreign
// configuration keys.
func (r *ServerRegistry) Add(name string, cfg ServerConfig) error {
	if err := validateServerName(name); err != nil {
		return err
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
	if err := validateServerName(name); err != nil {
		return err
	}
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
	credentialPath := filepath.Join(r.oauthDir(), name+".json")
	for _, path := range []string{credentialPath, credentialPath + ".client-secret"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove OAuth credentials: %w", err)
		}
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}
