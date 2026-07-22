package mcp

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/clicky/rpc"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

const oauthCallbackTimeout = 5 * time.Minute

// OAuthLoginOptions supplies host UI hooks without coupling the reusable MCP
// client to a specific Cobra command or desktop environment.
type OAuthLoginOptions struct {
	OpenBrowser func(string) error
	Out         io.Writer
	NoBrowser   bool
}

type fileOAuthTokenStore struct {
	path string
}

type oauthCredentialFile struct {
	Token        *client.Token `json:"token,omitempty"`
	ClientSecret string        `json:"client_secret,omitempty"`
}

type oauthClientSecretFile struct {
	ClientSecret string `json:"client_secret"`
}

type oauthClientSecretStore interface {
	GetClientSecret(context.Context) (string, error)
	SaveClientSecret(context.Context, string) error
}

type oauthTokenClearer interface {
	ClearToken(context.Context) error
}

type oauthCredentialRemover interface {
	RemoveCredentials(context.Context) error
}

var oauthCredentialLocks sync.Map

func oauthCredentialLock(path string) *sync.Mutex {
	lock, _ := oauthCredentialLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (s *fileOAuthTokenStore) clientSecretPath() string {
	return s.path + ".client-secret"
}

func (s *fileOAuthTokenStore) readCredentials() (oauthCredentialFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return oauthCredentialFile{}, err
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil {
		return oauthCredentialFile{}, err
	}
	if _, wrapped := shape["token"]; wrapped || shape["client_secret"] != nil {
		var credentials oauthCredentialFile
		if err := json.Unmarshal(data, &credentials); err != nil {
			return oauthCredentialFile{}, err
		}
		return credentials, nil
	}
	var token client.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return oauthCredentialFile{}, err
	}
	return oauthCredentialFile{Token: &token}, nil
}

// GetToken loads the last access and refresh token written for one registered
// server. A missing file has the sentinel meaning expected by mcp-go.
func (s *fileOAuthTokenStore) GetToken(ctx context.Context) (*client.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lock := oauthCredentialLock(s.path)
	lock.Lock()
	defer lock.Unlock()
	credentials, err := s.readCredentials()
	if errors.Is(err, os.ErrNotExist) {
		return nil, transport.ErrNoToken
	}
	if err != nil {
		return nil, fmt.Errorf("read OAuth token: %w", err)
	}
	if credentials.Token == nil {
		return nil, transport.ErrNoToken
	}
	token := *credentials.Token
	return &token, nil
}

// SaveToken atomically persists refreshed credentials with private filesystem
// permissions so a refresh cannot leave a partially written token behind.
func (s *fileOAuthTokenStore) SaveToken(ctx context.Context, token *client.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("cannot save a nil OAuth token")
	}
	lock := oauthCredentialLock(s.path)
	lock.Lock()
	defer lock.Unlock()
	copy := *token
	if err := writeJSONAtomic(s.path, &copy); err != nil {
		return fmt.Errorf("save OAuth token: %w", err)
	}
	return nil
}

func (s *fileOAuthTokenStore) GetClientSecret(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	secretPath := s.clientSecretPath()
	lock := oauthCredentialLock(secretPath)
	lock.Lock()
	data, err := os.ReadFile(secretPath)
	lock.Unlock()
	if err == nil {
		var secret oauthClientSecretFile
		if err := json.Unmarshal(data, &secret); err != nil {
			return "", fmt.Errorf("parse OAuth client secret: %w", err)
		}
		return secret.ClientSecret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read OAuth client secret: %w", err)
	}

	// Older credential files stored the dynamic secret beside the token.
	lock = oauthCredentialLock(s.path)
	lock.Lock()
	defer lock.Unlock()
	credentials, err := s.readCredentials()
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read legacy OAuth client secret: %w", err)
	}
	return credentials.ClientSecret, nil
}

func (s *fileOAuthTokenStore) SaveClientSecret(ctx context.Context, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.clientSecretPath()
	lock := oauthCredentialLock(path)
	lock.Lock()
	defer lock.Unlock()
	if secret == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		tokenLock := oauthCredentialLock(s.path)
		tokenLock.Lock()
		defer tokenLock.Unlock()
		credentials, err := s.readCredentials()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read legacy OAuth credentials: %w", err)
		}
		if credentials.ClientSecret == "" {
			return nil
		}
		if credentials.Token == nil {
			return os.Remove(s.path)
		}
		return writeJSONAtomic(s.path, credentials.Token)
	}
	return writeJSONAtomic(path, oauthClientSecretFile{ClientSecret: secret})
}

func (s *fileOAuthTokenStore) ClearToken(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if legacySecret, err := s.GetClientSecret(ctx); err != nil {
		return err
	} else if legacySecret != "" {
		if err := s.SaveClientSecret(ctx, legacySecret); err != nil {
			return err
		}
	}
	lock := oauthCredentialLock(s.path)
	lock.Lock()
	defer lock.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *fileOAuthTokenStore) RemoveCredentials(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, path := range []string{s.path, s.clientSecretPath()} {
		lock := oauthCredentialLock(path)
		lock.Lock()
		err := os.Remove(path)
		lock.Unlock()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func oauthTokenStore(cfg ServerConfig) (client.TokenStore, error) {
	if cfg.OAuth == nil || cfg.OAuth.TokenStore == nil {
		return nil, fmt.Errorf("OAuth token storage is required; set OAuth.TokenStore or load the server from a registry")
	}
	return cfg.OAuth.TokenStore, nil
}

func oauthTransportConfig(ctx context.Context, cfg ServerConfig) (client.OAuthConfig, error) {
	if cfg.OAuth == nil {
		return client.OAuthConfig{}, fmt.Errorf("OAuth is not configured")
	}
	store, err := oauthTokenStore(cfg)
	if err != nil {
		return client.OAuthConfig{}, err
	}
	secret, err := resolveOAuthSecret(cfg.OAuth.ClientSecret)
	if err != nil {
		return client.OAuthConfig{}, err
	}
	if secret == "" {
		if secretStore, ok := store.(oauthClientSecretStore); ok {
			secret, err = secretStore.GetClientSecret(ctx)
			if err != nil {
				return client.OAuthConfig{}, err
			}
		}
	}
	return client.OAuthConfig{
		ClientID:                     cfg.OAuth.ClientID,
		ClientSecret:                 secret,
		RedirectURI:                  cfg.OAuth.RedirectURI,
		Scopes:                       append([]string(nil), cfg.OAuth.Scopes...),
		TokenStore:                   store,
		AuthServerMetadataURL:        cfg.OAuth.AuthServerMetadataURL,
		ProtectedResourceMetadataURL: cfg.OAuth.ProtectedResourceMetadataURL,
		PKCEEnabled:                  true,
		HTTPClient:                   newOAuthHTTPClient(cfg.URL, cfg.OAuth.Issuer),
	}, nil
}

func resolveOAuthSecret(value string) (string, error) {
	if err := validateOAuthSecretReference(value); err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(value, "env:"):
		name := strings.TrimPrefix(value, "env:")
		if name == "" {
			return "", fmt.Errorf("OAuth client secret environment variable is empty")
		}
		secret, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("OAuth client secret environment variable %q is not set", name)
		}
		return secret, nil
	case strings.HasPrefix(value, "file:"):
		path := strings.TrimPrefix(value, "file:")
		if path == "" {
			return "", fmt.Errorf("OAuth client secret file path is empty")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read OAuth client secret: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	case value == "":
		return "", nil
	default:
		return "", fmt.Errorf("OAuth client secret must use an env:NAME or file:PATH reference")
	}
}

func validateOAuthSecretReference(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "env:") && strings.TrimPrefix(value, "env:") != "" {
		return nil
	}
	if strings.HasPrefix(value, "file:") && strings.TrimPrefix(value, "file:") != "" {
		return nil
	}
	return fmt.Errorf("OAuth client secret must use an env:NAME or file:PATH reference")
}

type oauthRoundTripper struct {
	base           http.RoundTripper
	resource       string
	expectedIssuer string
}

func newOAuthHTTPClient(resource, expectedIssuer string) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &oauthRoundTripper{
			base: http.DefaultTransport, resource: resource, expectedIssuer: expectedIssuer,
		},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many OAuth HTTP redirects")
			}
			if err := validateSecureHTTPURL(request.URL.String(), "OAuth redirect target"); err != nil {
				return err
			}
			if len(via) > 0 && !sameOrigin(via[0].URL, request.URL) {
				return fmt.Errorf("OAuth HTTP redirect changed origin from %s to %s", via[0].URL, request.URL)
			}
			return nil
		},
	}
}

// RoundTrip enforces transport security and supplies the MCP resource
// indicator even when mcp-go uses an explicit authorization metadata URL.
func (t *oauthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := validateSecureHTTPURL(request.URL.String(), "OAuth endpoint"); err != nil {
		return nil, err
	}
	request, err := addOAuthResourceToRequest(request, t.resource)
	if err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if t.expectedIssuer != "" && request.Method == http.MethodGet && response.StatusCode >= 200 && response.StatusCode < 300 {
		if err := validateMetadataResponseIssuer(response, t.expectedIssuer); err != nil {
			response.Body.Close()
			return nil, err
		}
	}
	return response, nil
}

func addOAuthResourceToRequest(request *http.Request, resource string) (*http.Request, error) {
	if resource == "" || request.Method != http.MethodPost || request.Body == nil {
		return request, nil
	}
	contentType := strings.ToLower(request.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") && !strings.HasPrefix(contentType, "application/json") {
		return request, nil
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("read OAuth request body: %w", err)
	}
	_ = request.Body.Close()
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, fmt.Errorf("parse OAuth form body: %w", err)
		}
		if values.Get("grant_type") == "" {
			return cloneRequestBody(request, body), nil
		}
		values.Set("resource", resource)
		return cloneRequestBody(request, []byte(values.Encode())), nil
	}
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, fmt.Errorf("parse OAuth JSON body: %w", err)
	}
	if _, registration := object["client_name"]; !registration {
		return cloneRequestBody(request, body), nil
	}
	object["resource"] = resource
	body, err = json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode OAuth JSON body: %w", err)
	}
	return cloneRequestBody(request, body), nil
}

func cloneRequestBody(request *http.Request, body []byte) *http.Request {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return clone
}

func validateMetadataResponseIssuer(response *http.Response, expected string) error {
	const maxMetadataSize = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataSize+1))
	if err != nil {
		return fmt.Errorf("read OAuth metadata response: %w", err)
	}
	response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxMetadataSize {
		return fmt.Errorf("OAuth metadata response exceeds %d bytes", maxMetadataSize)
	}
	var metadata struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil || metadata.Issuer == "" {
		return nil
	}
	if metadata.Issuer != expected {
		return fmt.Errorf("OAuth metadata issuer %q does not match expected issuer %q", metadata.Issuer, expected)
	}
	return nil
}

func sameOrigin(first, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func addOAuthResourceToAuthorizationURL(rawURL, resource string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if err := validateSecureHTTPURL(parsed.String(), "OAuth authorization endpoint"); err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("resource", resource)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func validateAuthorizationServerMetadata(metadata *transport.AuthServerMetadata, expectedIssuer string) error {
	if metadata == nil || metadata.Issuer == "" {
		return fmt.Errorf("OAuth authorization server metadata omits issuer")
	}
	if expectedIssuer != "" && metadata.Issuer != expectedIssuer {
		return fmt.Errorf("OAuth metadata issuer %q does not match expected issuer %q", metadata.Issuer, expectedIssuer)
	}
	for label, value := range map[string]string{
		"issuer": metadata.Issuer, "authorization endpoint": metadata.AuthorizationEndpoint,
		"token endpoint": metadata.TokenEndpoint, "registration endpoint": metadata.RegistrationEndpoint,
	} {
		if value == "" && label == "registration endpoint" {
			continue
		}
		if value == "" {
			return fmt.Errorf("OAuth authorization server metadata omits %s", label)
		}
		if err := validateSecureHTTPURL(value, "OAuth "+label); err != nil {
			return err
		}
	}
	return nil
}

func validateOAuthRedirectURI(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return fmt.Errorf("invalid OAuth redirect URI %q: use an http loopback URL", value)
	}
	if parsed.Port() == "" {
		return fmt.Errorf("invalid OAuth redirect URI %q: an explicit port is required", value)
	}
	if parsed.User != nil {
		return fmt.Errorf("invalid OAuth redirect URI %q: user information is not allowed", value)
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("invalid OAuth redirect URI %q: host must be localhost or a loopback address", value)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid OAuth redirect URI %q: query and fragment are not allowed", value)
	}
	return nil
}

func openOAuthCallbackListener(configured string) (net.Listener, string, string, error) {
	if configured == "" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, "", "", fmt.Errorf("listen for OAuth callback: %w", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		return listener, fmt.Sprintf("http://127.0.0.1:%d/callback", port), "/callback", nil
	}
	if err := validateOAuthRedirectURI(configured); err != nil {
		return nil, "", "", err
	}
	parsed, _ := url.Parse(configured)
	bindHost := parsed.Hostname()
	if bindHost == "localhost" {
		bindHost = "127.0.0.1"
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(bindHost, parsed.Port()))
	if err != nil {
		return nil, "", "", fmt.Errorf("listen on OAuth redirect URI %q: %w", configured, err)
	}
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	return listener, configured, path, nil
}

type oauthCallbackResult struct {
	code  string
	state string
	err   error
}

// receiveOAuthCallback owns the short-lived loopback server. Requests with an
// unexpected method, path, or state never consume the pending authorization.
func receiveOAuthCallback(listener net.Listener, path, expectedState string) (<-chan oauthCallbackResult, *http.Server) {
	result := make(chan oauthCallbackResult, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "OAuth callback requires GET", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		state := query.Get("state")
		if subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		callback := oauthCallbackResult{code: query.Get("code"), state: state}
		if oauthError := query.Get("error"); oauthError != "" {
			callback.err = fmt.Errorf("OAuth authorization failed: %s: %s", oauthError, query.Get("error_description"))
		} else if callback.code == "" {
			callback.err = fmt.Errorf("OAuth callback did not include an authorization code")
		}
		select {
		case result <- callback:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "Authorization complete. You can close this window.\n")
		default:
			http.Error(w, "Authorization callback already received", http.StatusConflict)
		}
	})
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
	}()
	return result, server
}

func probeOAuthResourceMetadata(ctx context.Context, cfg ServerConfig) string {
	probe := cfg
	probe.OAuth = nil
	transports := []string{probe.effectiveTransport()}
	if transports[0] == "auto" {
		transports = []string{"http", "sse"}
	}
	for _, transportName := range transports {
		session, err := dialTransport(ctx, probe, transportName)
		if session != nil {
			_ = session.Close()
		}
		if metadataURL := client.GetResourceMetadataURL(err); metadataURL != "" {
			return metadataURL
		}
	}
	return ""
}

func validateResourceMetadataOrigin(serverURL, metadataURL string) error {
	server, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	metadata, err := url.Parse(metadataURL)
	if err != nil {
		return err
	}
	if !strings.EqualFold(server.Scheme, metadata.Scheme) || !strings.EqualFold(server.Host, metadata.Host) {
		return fmt.Errorf("OAuth resource metadata URL %q does not share the MCP server origin", metadataURL)
	}
	return nil
}

func fetchProtectedResourceIssuer(ctx context.Context, httpClient *http.Client, metadataURL, resource string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpsdk.LATEST_PROTOCOL_VERSION)
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch OAuth protected resource metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch OAuth protected resource metadata: status %s", response.Status)
	}
	var metadata transport.OAuthProtectedResource
	decoder := json.NewDecoder(io.LimitReader(response.Body, (1<<20)+1))
	if err := decoder.Decode(&metadata); err != nil {
		return "", fmt.Errorf("decode OAuth protected resource metadata: %w", err)
	}
	if metadata.Resource == "" || !oauthResourceEqual(metadata.Resource, resource) {
		return "", fmt.Errorf("OAuth protected resource %q does not match MCP server %q", metadata.Resource, resource)
	}
	if len(metadata.AuthorizationServers) == 0 {
		return "", nil
	}
	issuer := metadata.AuthorizationServers[0]
	if err := validateSecureHTTPURL(issuer, "OAuth issuer URL"); err != nil {
		return "", err
	}
	return issuer, nil
}

func oauthResourceEqual(first, second string) bool {
	a, errA := url.Parse(first)
	b, errB := url.Parse(second)
	if errA != nil || errB != nil || !sameOrigin(a, b) {
		return first == second
	}
	return strings.TrimSuffix(a.EscapedPath(), "/") == strings.TrimSuffix(b.EscapedPath(), "/") &&
		a.RawQuery == b.RawQuery && a.Fragment == b.Fragment && a.User.String() == b.User.String()
}

// loginOAuth performs one authorization-code flow and returns the durable
// configuration additions produced by discovery and dynamic registration.
func loginOAuth(ctx context.Context, registry *ServerRegistry, name string, cfg ServerConfig, options OAuthLoginOptions) (ServerConfig, error) {
	if err := validateServerName(name); err != nil {
		return cfg, err
	}
	if cfg.OAuth == nil {
		return cfg, fmt.Errorf("MCP server %q is not configured for OAuth", name)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	if options.Out == nil {
		options.Out = io.Discard
	}
	if options.OpenBrowser == nil {
		options.OpenBrowser = rpc.OpenBrowser
	}

	oauthSettings := *cfg.OAuth
	cfg.OAuth = &oauthSettings
	var err error
	cfg, err = registry.bind(name, cfg)
	if err != nil {
		return cfg, err
	}
	configuredRedirectURI := cfg.OAuth.RedirectURI
	listener, redirectURI, callbackPath, err := openOAuthCallbackListener(configuredRedirectURI)
	if err != nil {
		return cfg, err
	}
	cfg.OAuth.RedirectURI = redirectURI
	state, err := client.GenerateState()
	if err != nil {
		_ = listener.Close()
		return cfg, fmt.Errorf("generate OAuth state: %w", err)
	}
	callback, callbackServer := receiveOAuthCallback(listener, callbackPath, state)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = callbackServer.Shutdown(shutdownCtx)
	}()

	if cfg.OAuth.ProtectedResourceMetadataURL == "" {
		if metadataURL := probeOAuthResourceMetadata(ctx, cfg); metadataURL != "" {
			if err := validateResourceMetadataOrigin(cfg.URL, metadataURL); err != nil {
				return cfg, err
			}
			cfg.OAuth.ProtectedResourceMetadataURL = metadataURL
		}
	}
	if cfg.OAuth.ProtectedResourceMetadataURL != "" {
		discoveredIssuer, err := fetchProtectedResourceIssuer(
			ctx, newOAuthHTTPClient(cfg.URL, cfg.OAuth.Issuer), cfg.OAuth.ProtectedResourceMetadataURL, cfg.URL,
		)
		if err != nil {
			return cfg, err
		}
		if cfg.OAuth.Issuer != "" && discoveredIssuer != "" && cfg.OAuth.Issuer != discoveredIssuer {
			return cfg, fmt.Errorf("OAuth protected resource issuer %q does not match configured issuer %q", discoveredIssuer, cfg.OAuth.Issuer)
		}
		if discoveredIssuer != "" {
			cfg.OAuth.Issuer = discoveredIssuer
		}
	}

	sdkConfig, err := oauthTransportConfig(ctx, cfg)
	if err != nil {
		return cfg, err
	}
	handler := transport.NewOAuthHandler(sdkConfig)
	handler.SetBaseURL(cfg.URL)
	if cfg.OAuth.ProtectedResourceMetadataURL != "" {
		handler.SetProtectedResourceMetadataURL(cfg.OAuth.ProtectedResourceMetadataURL)
	}
	metadata, err := handler.GetServerMetadata(ctx)
	if err != nil {
		return cfg, fmt.Errorf("discover OAuth authorization server metadata: %w", err)
	}
	if err := validateAuthorizationServerMetadata(metadata, cfg.OAuth.Issuer); err != nil {
		return cfg, err
	}
	if cfg.OAuth.Issuer == "" {
		cfg.OAuth.Issuer = metadata.Issuer
		if roundTripper, ok := sdkConfig.HTTPClient.Transport.(*oauthRoundTripper); ok {
			roundTripper.expectedIssuer = metadata.Issuer
		}
	}
	if cfg.OAuth.ClientID == "" {
		clientName := cfg.OAuth.ClientName
		if clientName == "" {
			clientName = registry.appName
		}
		if err := handler.RegisterClient(ctx, clientName); err != nil {
			return cfg, fmt.Errorf("register OAuth client: %w", err)
		}
		cfg.OAuth.ClientID = handler.GetClientID()
		cfg.OAuth.DynamicallyRegistered = true
		if secret := handler.GetClientSecret(); secret != "" {
			secretStore, ok := cfg.OAuth.TokenStore.(oauthClientSecretStore)
			if !ok {
				return cfg, fmt.Errorf("OAuth server returned a dynamic client secret, but the token store cannot persist it")
			}
			if err := secretStore.SaveClientSecret(ctx, secret); err != nil {
				return cfg, fmt.Errorf("save dynamically registered OAuth client secret: %w", err)
			}
		}
	}

	verifier, err := client.GenerateCodeVerifier()
	if err != nil {
		return cfg, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	challenge := client.GenerateCodeChallenge(verifier)
	authorizationURL, err := handler.GetAuthorizationURL(ctx, state, challenge)
	if err != nil {
		return cfg, fmt.Errorf("build OAuth authorization URL: %w", err)
	}
	authorizationURL, err = addOAuthResourceToAuthorizationURL(authorizationURL, cfg.URL)
	if err != nil {
		return cfg, fmt.Errorf("add MCP resource to OAuth authorization URL: %w", err)
	}
	fmt.Fprintf(options.Out, "Open this URL to authorize %s:\n%s\n", name, authorizationURL)
	if !options.NoBrowser {
		if err := options.OpenBrowser(authorizationURL); err != nil {
			fmt.Fprintf(options.Out, "Could not open a browser automatically: %v\n", err)
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, oauthCallbackTimeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return cfg, fmt.Errorf("wait for OAuth callback: %w", waitCtx.Err())
	case result := <-callback:
		if result.err != nil {
			return cfg, result.err
		}
		if err := handler.ProcessAuthorizationResponse(ctx, result.code, result.state, verifier); err != nil {
			return cfg, fmt.Errorf("exchange OAuth authorization code: %w", err)
		}
	}
	// RFC 8252 requires authorization servers to accept variable loopback
	// ports, so generated callback ports must not become durable configuration.
	if configuredRedirectURI == "" {
		cfg.OAuth.RedirectURI = ""
	}
	return cfg, nil
}

func clearOAuthToken(ctx context.Context, cfg ServerConfig) error {
	store, err := oauthTokenStore(cfg)
	if err != nil {
		return err
	}
	clearer, ok := store.(oauthTokenClearer)
	if !ok {
		return fmt.Errorf("OAuth token store does not support logout")
	}
	if err := clearer.ClearToken(ctx); err != nil {
		return fmt.Errorf("remove OAuth token: %w", err)
	}
	return nil
}

func removeOAuthCredentials(ctx context.Context, cfg ServerConfig) error {
	store, err := oauthTokenStore(cfg)
	if err != nil {
		return nil
	}
	remover, ok := store.(oauthCredentialRemover)
	if !ok {
		return nil
	}
	if err := remover.RemoveCredentials(ctx); err != nil {
		return fmt.Errorf("remove OAuth credentials: %w", err)
	}
	return nil
}
