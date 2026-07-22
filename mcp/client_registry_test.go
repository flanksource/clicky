package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestServerConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  ServerConfig
		ok   bool
	}{
		{"stdio", ServerConfig{Type: "stdio", Command: "node"}, true},
		{"implicit stdio", ServerConfig{Command: "node"}, true},
		{"auto remote", ServerConfig{Type: "auto", URL: "https://example.com/mcp"}, true},
		{"missing endpoint", ServerConfig{Type: "stdio"}, false},
		{"both endpoints", ServerConfig{Type: "stdio", Command: "node", URL: "https://example.com"}, false},
		{"env on remote", ServerConfig{Type: "http", URL: "https://example.com", Env: map[string]string{"A": "b"}}, false},
		{"headers on stdio", ServerConfig{Type: "stdio", Command: "node", Headers: map[string]string{"X": "y"}}, false},
		{"bad URL", ServerConfig{Type: "http", URL: "file:///tmp/socket"}, false},
		{"bad timeout", ServerConfig{Type: "stdio", Command: "node", Timeout: "soon"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.ok && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
}

func TestServerConfigFingerprint(t *testing.T) {
	base := ServerConfig{Type: "stdio", Command: "node", Args: []string{"server.js"}, Timeout: "10s", CacheTTL: "1h"}
	cosmetic := base
	cosmetic.Timeout = "2m"
	cosmetic.CacheTTL = "5m"
	if base.Fingerprint() != cosmetic.Fingerprint() {
		t.Fatal("timeout and cache TTL must not change fingerprint")
	}
	behavioral := base
	behavioral.Args = []string{"other.js"}
	if base.Fingerprint() == behavioral.Fingerprint() {
		t.Fatal("command arguments must change fingerprint")
	}
}

func TestServerRegistryRoundTripAndPreservesForeignKeys(t *testing.T) {
	dir := t.TempDir()
	r := newServerRegistryAt("captain", dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.configPath(), []byte(`{"theme":{"name":"dark"},"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{Type: "stdio", Command: "node", Args: []string{"server.js"}}
	if err := r.Add("demo", cfg); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Get("demo")
	if err != nil || !ok || !reflect.DeepEqual(got, cfg) {
		t.Fatalf("Get() = %#v, %v, %v", got, ok, err)
	}
	names, _, err := r.List()
	if err != nil || !reflect.DeepEqual(names, []string{"demo"}) {
		t.Fatalf("List() = %v, %v", names, err)
	}

	data, err := os.ReadFile(r.configPath())
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root["theme"]), "dark") {
		t.Fatalf("foreign key was not preserved: %s", data)
	}

	if err := r.Remove("demo"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove("demo"); err == nil {
		t.Fatal("removing an unknown server should fail")
	}
}

func TestCatalogCacheLifecycle(t *testing.T) {
	r := newServerRegistryAt("captain", t.TempDir())
	cfg := ServerConfig{Type: "stdio", Command: "node", CacheTTL: "1h"}
	now := time.Now().UTC()
	catalog := &CatalogCache{
		Fingerprint: cfg.Fingerprint(), FetchedAt: now, TTL: time.Hour,
		Transport: "stdio", Tools: []CachedTool{{Name: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}
	if err := SaveCatalog(r, "demo", catalog); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCatalog(r, "demo")
	if err != nil || loaded == nil || loaded.Tools[0].Name != "echo" {
		t.Fatalf("LoadCatalog() = %#v, %v", loaded, err)
	}
	if loaded.Stale(cfg, now.Add(59*time.Minute)) {
		t.Fatal("fresh catalog reported stale")
	}
	if !loaded.Stale(cfg, now.Add(time.Hour)) {
		t.Fatal("expired catalog reported fresh")
	}
	changed := cfg
	changed.Args = []string{"different"}
	if !loaded.Stale(changed, now) {
		t.Fatal("fingerprint mismatch reported fresh")
	}
	if _, err := os.Stat(catalogPath(r, "demo") + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("atomic cache write left a temporary file")
	}
}

func TestLoadCatalogHealsCorruption(t *testing.T) {
	r := newServerRegistryAt("captain", t.TempDir())
	path := catalogPath(r, "broken")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCatalog(r, "broken")
	if err != nil || got != nil {
		t.Fatalf("LoadCatalog() = %#v, %v", got, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("corrupt cache file was not removed")
	}
}
