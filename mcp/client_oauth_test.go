package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestOAuthLoginUsesOIDCDiscoveryAndRefreshes(t *testing.T) {
	tests := []struct {
		name             string
		metadataOverride bool
		scopes           []string
		wantScope        string
	}{
		{name: "discovered", scopes: []string{"mcp.read"}, wantScope: "mcp.read"},
		{name: "explicit-metadata", metadataOverride: true, scopes: []string{"mcp.read"}, wantScope: "mcp.read"},
		{name: "protected-resource-scopes", wantScope: "openid profile email"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testOAuthLoginUsesOIDCDiscoveryAndRefreshes(t, test.metadataOverride, test.scopes, test.wantScope)
		})
	}
}

func testOAuthLoginUsesOIDCDiscoveryAndRefreshes(t *testing.T, metadataOverride bool, scopes []string, wantScope string) {
	var baseURL string
	var oidcDiscovered, registered, authorizationExchanged, refreshed atomic.Bool

	sdkServer := mcpserver.NewMCPServer("oidc-test", "1.0.0", mcpserver.WithToolCapabilities(true))
	sdkServer.AddTool(
		mcpsdk.NewTool("echo", mcpsdk.WithDescription("Echo a message")),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ok"), nil
		},
	)
	mcpHandler := mcpserver.NewStreamableHTTPServer(
		sdkServer,
		mcpserver.WithStateLess(true),
		mcpserver.WithDisableStreaming(true),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization != "Bearer access-1" && authorization != "Bearer access-2" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+baseURL+`/oauth-resource"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	mux.HandleFunc("/oauth-resource", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"resource":              baseURL + "/mcp",
			"authorization_servers": []string{baseURL + "/tenant"},
			"scopes_supported":      []string{"openid", "profile", "email"},
		})
	})
	mux.HandleFunc("/.well-known/openid-configuration/tenant", func(w http.ResponseWriter, r *http.Request) {
		oidcDiscovered.Store(true)
		writeTestJSON(w, map[string]any{
			"issuer":                                baseURL + "/tenant",
			"authorization_endpoint":                baseURL + "/tenant/authorize",
			"token_endpoint":                        baseURL + "/tenant/token",
			"registration_endpoint":                 baseURL + "/tenant/register",
			"response_types_supported":              []string{"code"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"none"},
		})
	})
	mux.HandleFunc("/tenant/register", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			RedirectURIs            []string `json:"redirect_uris"`
			TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
			Resource                string   `json:"resource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(request.RedirectURIs) != 1 || request.TokenEndpointAuthMethod != "none" || request.Resource != baseURL+"/mcp" {
			http.Error(w, "invalid client registration", http.StatusBadRequest)
			return
		}
		registered.Store(true)
		w.WriteHeader(http.StatusCreated)
		writeTestJSON(w, map[string]any{"client_id": "clicky-test", "client_secret": "dynamic-secret"})
	})
	mux.HandleFunc("/tenant/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("client_id") != "clicky-test" || r.Form.Get("client_secret") != "dynamic-secret" || r.Form.Get("resource") != baseURL+"/mcp" {
			http.Error(w, "invalid token request", http.StatusBadRequest)
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "test-code" || r.Form.Get("code_verifier") == "" {
				http.Error(w, "invalid authorization code exchange", http.StatusBadRequest)
				return
			}
			authorizationExchanged.Store(true)
			writeTestJSON(w, map[string]any{
				"access_token": "access-1", "token_type": "bearer", "refresh_token": "refresh-1", "expires_in": 3600,
			})
		case "refresh_token":
			if r.Form.Get("refresh_token") != "refresh-1" {
				http.Error(w, "invalid refresh token", http.StatusBadRequest)
				return
			}
			refreshed.Store(true)
			writeTestJSON(w, map[string]any{
				"access_token": "access-2", "token_type": "Bearer", "refresh_token": "refresh-1", "expires_in": 3600,
			})
		default:
			http.Error(w, "unsupported grant", http.StatusBadRequest)
		}
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()
	baseURL = httpServer.URL

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	registry := NewServerRegistry("testapp")
	oauthConfig := &OAuthClientConfig{Scopes: append([]string(nil), scopes...)}
	if metadataOverride {
		oauthConfig.AuthServerMetadataURL = baseURL + "/.well-known/openid-configuration/tenant"
	}
	cfg, err := registry.bind("private", ServerConfig{
		Type: "http", URL: baseURL + "/mcp", OAuth: oauthConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	var authorizationQuery url.Values
	openBrowser := func(rawURL string) error {
		authorizationURL, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		authorizationQuery = authorizationURL.Query()
		callbackURL := authorizationQuery.Get("redirect_uri") + "?" + url.Values{
			"code":  []string{"test-code"},
			"state": []string{authorizationQuery.Get("state")},
		}.Encode()
		response, err := http.Get(callbackURL)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("OAuth callback returned %s", response.Status)
		}
		return nil
	}
	loggedIn, err := loginOAuth(context.Background(), registry, "private", cfg, OAuthLoginOptions{
		OpenBrowser: openBrowser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !oidcDiscovered.Load() || !registered.Load() || !authorizationExchanged.Load() {
		t.Fatalf("OAuth flow incomplete: oidc=%v registration=%v exchange=%v", oidcDiscovered.Load(), registered.Load(), authorizationExchanged.Load())
	}
	if authorizationQuery.Get("code_challenge_method") != "S256" || authorizationQuery.Get("code_challenge") == "" {
		t.Fatalf("authorization query did not use PKCE: %v", authorizationQuery)
	}
	if authorizationQuery.Get("resource") != baseURL+"/mcp" || authorizationQuery.Get("scope") != wantScope {
		t.Fatalf("authorization query = %v", authorizationQuery)
	}
	if strings.Join(loggedIn.OAuth.Scopes, " ") != wantScope {
		t.Fatalf("stored OAuth scopes = %v, want %q", loggedIn.OAuth.Scopes, wantScope)
	}
	if loggedIn.OAuth.ClientID != "clicky-test" || !loggedIn.OAuth.DynamicallyRegistered || loggedIn.OAuth.RedirectURI != "" {
		t.Fatalf("login did not retain client registration: %#v", loggedIn.OAuth)
	}
	if loggedIn.OAuth.ProtectedResourceMetadataURL != baseURL+"/oauth-resource" {
		t.Fatalf("protected resource metadata URL = %q", loggedIn.OAuth.ProtectedResourceMetadataURL)
	}
	if err := registry.Add("private", loggedIn); err != nil {
		t.Fatal(err)
	}
	configData, err := os.ReadFile(registry.configPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configData), "dynamic-secret") {
		t.Fatal("dynamic client secret was written to the registry")
	}

	reloaded, ok, err := registry.Get("private")
	if err != nil || !ok {
		t.Fatalf("reload = %#v, %v, %v", reloaded, ok, err)
	}
	session, err := Dial(context.Background(), "private", reloaded)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := FetchCatalog(context.Background(), reloaded, session)
	_ = session.Close()
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools = %#v, %v", tools, err)
	}

	store, err := oauthTokenStore(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	secretStore, ok := store.(oauthClientSecretStore)
	if !ok {
		t.Fatalf("client secret store = %T", store)
	}
	secret, err := secretStore.GetClientSecret(context.Background())
	if err != nil || secret != "dynamic-secret" {
		t.Fatalf("dynamic client secret = %q, %v", secret, err)
	}
	if err := store.SaveToken(context.Background(), &mcpclient.Token{
		AccessToken: "expired", TokenType: "Bearer", RefreshToken: "refresh-1", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	refreshedSession, err := Dial(context.Background(), "private", reloaded)
	if err != nil {
		t.Fatal(err)
	}
	_ = refreshedSession.Close()
	if !refreshed.Load() {
		t.Fatal("expired token was not refreshed")
	}

	fileStore, ok := reloaded.OAuth.TokenStore.(*fileOAuthTokenStore)
	if !ok {
		t.Fatalf("token store = %T", reloaded.OAuth.TokenStore)
	}
	info, err := os.Stat(fileStore.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("OAuth token permissions = %o", info.Mode().Perm())
	}
	secretInfo, err := os.Stat(fileStore.clientSecretPath())
	if err != nil {
		t.Fatal(err)
	}
	if secretInfo.Mode().Perm() != 0o600 {
		t.Fatalf("OAuth client secret permissions = %o", secretInfo.Mode().Perm())
	}
	token, err := store.GetToken(context.Background())
	if err != nil || token.AccessToken != "access-2" {
		t.Fatalf("refreshed token = %#v, %v", token, err)
	}

	output, err := executeMCPCommand("logout", "private")
	if err != nil || !strings.Contains(output, "Logged out") {
		t.Fatalf("logout: %v\n%s", err, output)
	}
	if _, err := store.GetToken(context.Background()); err == nil {
		t.Fatal("logout retained the OAuth token")
	}
	secret, err = secretStore.GetClientSecret(context.Background())
	if err != nil || secret != "dynamic-secret" {
		t.Fatalf("logout removed dynamic client secret: %q, %v", secret, err)
	}
	output, err = executeMCPCommandWithClientOptions(
		context.Background(), ClientOptions{OpenBrowser: openBrowser}, "login", "private",
	)
	if err != nil || !strings.Contains(output, "Logged in") {
		t.Fatalf("login: %v\n%s", err, output)
	}
	reloaded, ok, err = registry.Get("private")
	if err != nil || !ok || reloaded.OAuth.RedirectURI != "" {
		t.Fatalf("configuration after login = %#v, %v, %v", reloaded, ok, err)
	}
}

func TestOAuthCallbackCapturesAuthorizationResponse(t *testing.T) {
	listener, redirectURI, path, err := openOAuthCallbackListener("")
	if err != nil {
		t.Fatal(err)
	}
	result, server := receiveOAuthCallback(listener, path, "expected")
	defer server.Close()
	response, err := http.Post(redirectURI+"?code=code&state=expected", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("invalid method status = %s", response.Status)
	}
	response, err = http.Get(redirectURI + "?code=code&state=unexpected")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid state status = %s", response.Status)
	}
	response, err = http.Get(redirectURI + "?code=code&state=expected")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	callback := <-result
	if callback.code != "code" || callback.state != "expected" {
		t.Fatalf("callback = %#v", callback)
	}
}

func TestResolveOAuthSecret(t *testing.T) {
	t.Setenv("CLICKY_TEST_OAUTH_SECRET", "env-secret")
	path := t.TempDir() + "/secret"
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for input, want := range map[string]string{
		"env:CLICKY_TEST_OAUTH_SECRET": "env-secret",
		"file:" + path:                 "file-secret",
	} {
		got, err := resolveOAuthSecret(input)
		if err != nil || got != want {
			t.Errorf("resolveOAuthSecret(%q) = %q, %v", input, got, err)
		}
	}
	if _, err := resolveOAuthSecret("env:CLICKY_TEST_MISSING_SECRET"); err == nil || !strings.Contains(err.Error(), "is not set") {
		t.Fatalf("missing environment secret error = %v", err)
	}
	if _, err := resolveOAuthSecret("literal"); err == nil || !strings.Contains(err.Error(), "env:NAME") {
		t.Fatalf("literal secret error = %v", err)
	}
}

func TestDialSupportsPublicTokenStoreAndStaticHeaders(t *testing.T) {
	for _, test := range []struct {
		name       string
		configure  func(*ServerConfig)
		authorized func(*http.Request) bool
	}{
		{
			name:      "public",
			configure: func(*ServerConfig) {},
			authorized: func(r *http.Request) bool {
				return r.Header.Get("Authorization") == "" && r.Header.Get("X-API-Key") == ""
			},
		},
		{
			name:      "static header",
			configure: func(cfg *ServerConfig) { cfg.Headers = map[string]string{"X-API-Key": "static-secret"} },
			authorized: func(r *http.Request) bool {
				return r.Header.Get("X-API-Key") == "static-secret"
			},
		},
		{
			name: "public OAuth token store",
			configure: func(cfg *ServerConfig) {
				store := mcpclient.NewMemoryTokenStore()
				if err := store.SaveToken(context.Background(), &mcpclient.Token{AccessToken: "direct-token", TokenType: "Bearer"}); err != nil {
					t.Fatal(err)
				}
				cfg.OAuth = &OAuthClientConfig{ClientID: "direct-client", TokenStore: store}
			},
			authorized: func(r *http.Request) bool {
				return r.Header.Get("Authorization") == "Bearer direct-token"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sdkServer := newOAuthTestMCPServer()
			mcpHandler := mcpserver.NewStreamableHTTPServer(
				sdkServer, mcpserver.WithStateLess(true), mcpserver.WithDisableStreaming(true),
			)
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !test.authorized(r) {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				mcpHandler.ServeHTTP(w, r)
			}))
			defer httpServer.Close()
			cfg := ServerConfig{Type: "http", URL: httpServer.URL + "/mcp"}
			test.configure(&cfg)
			session, err := Dial(context.Background(), "direct", cfg)
			if err != nil {
				t.Fatal(err)
			}
			_ = session.Close()
		})
	}
}

func TestDialOAuthSSE(t *testing.T) {
	sseHandler := mcpserver.NewSSEServer(newOAuthTestMCPServer())
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sse-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sseHandler.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	store := mcpclient.NewMemoryTokenStore()
	if err := store.SaveToken(context.Background(), &mcpclient.Token{AccessToken: "sse-token", TokenType: "Bearer"}); err != nil {
		t.Fatal(err)
	}
	cfg := ServerConfig{
		Type: "sse", URL: httpServer.URL + "/sse",
		OAuth: &OAuthClientConfig{ClientID: "sse-client", TokenStore: store},
	}
	session, err := Dial(context.Background(), "sse", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := FetchCatalog(context.Background(), cfg, session)
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools = %#v, %v", tools, err)
	}
}

func TestDialMissingOAuthTokenRequestsLogin(t *testing.T) {
	cfg := ServerConfig{
		Type: "http", URL: "http://127.0.0.1:1/mcp",
		OAuth: &OAuthClientConfig{ClientID: "missing-token", TokenStore: mcpclient.NewMemoryTokenStore()},
	}
	_, err := Dial(context.Background(), "private", cfg)
	if err == nil || !strings.Contains(err.Error(), "mcp login private") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileOAuthTokenStoreMigratesLegacyClientSecret(t *testing.T) {
	store := &fileOAuthTokenStore{path: t.TempDir() + "/oauth.json"}
	legacy := oauthCredentialFile{
		Token: &mcpclient.Token{
			AccessToken: "old-token", TokenType: "Bearer", RefreshToken: "refresh-token",
		},
		ClientSecret: "dynamic-secret",
	}
	if err := writeJSONAtomic(store.path, legacy); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveToken(context.Background(), &mcpclient.Token{
		AccessToken: "new-token", TokenType: "Bearer", RefreshToken: "refresh-token",
	}); err != nil {
		t.Fatal(err)
	}

	secret, err := store.GetClientSecret(context.Background())
	if err != nil || secret != "dynamic-secret" {
		t.Fatalf("client secret = %q, %v", secret, err)
	}
	token, err := store.GetToken(context.Background())
	if err != nil || token.AccessToken != "new-token" {
		t.Fatalf("token = %#v, %v", token, err)
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "client_secret") {
		t.Fatalf("legacy token file was not migrated: %s", data)
	}
}

func TestFileOAuthTokenStoreConcurrentWrites(t *testing.T) {
	store := &fileOAuthTokenStore{path: t.TempDir() + "/oauth.json"}
	errors := make(chan error, 32)
	for i := range 32 {
		go func() {
			errors <- store.SaveToken(context.Background(), &mcpclient.Token{
				AccessToken: fmt.Sprintf("token-%d", i), TokenType: "Bearer",
			})
		}()
	}
	for range 32 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.GetToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".oauth.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary credential files remain: %v", matches)
	}
}

func newOAuthTestMCPServer() *mcpserver.MCPServer {
	server := mcpserver.NewMCPServer("oauth-test", "1.0.0", mcpserver.WithToolCapabilities(true))
	server.AddTool(
		mcpsdk.NewTool("echo", mcpsdk.WithDescription("Echo a message")),
		func(context.Context, mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return mcpsdk.NewToolResultText("ok"), nil
		},
	)
	return server
}

func writeTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
