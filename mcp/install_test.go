package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installTest captures one (client, global, transport) install permutation
// plus the assertions that should hold on the resulting file.
type installTest struct {
	name      string
	client    string
	global    bool
	transport string
	url       string

	// expectedRel is the path the install should write to, relative to either
	// home (if global) or cwd (if !global).
	expectedRel func(home, cwd string) string

	// assertContent runs against the bytes the install wrote.
	assertContent func(t *testing.T, content []byte, entry serverEntry)
}

func TestWriteClientConfig(t *testing.T) {
	cases := []installTest{
		{
			name:   "claude project stdio",
			client: "claude", global: false, transport: "stdio",
			expectedRel:   func(_, cwd string) string { return filepath.Join(cwd, ".mcp.json") },
			assertContent: assertJSONServerEntry("mcpServers", "stdio"),
		},
		{
			name:   "claude global stdio",
			client: "claude", global: true, transport: "stdio",
			expectedRel:   func(home, _ string) string { return filepath.Join(home, ".claude.json") },
			assertContent: assertJSONServerEntry("mcpServers", "stdio"),
		},
		{
			name:   "claude global sse",
			client: "claude", global: true, transport: "sse",
			url:           "http://127.0.0.1:9000/sse",
			expectedRel:   func(home, _ string) string { return filepath.Join(home, ".claude.json") },
			assertContent: assertJSONServerEntry("mcpServers", "sse"),
		},
		{
			name:   "gemini project stdio",
			client: "gemini", global: false, transport: "stdio",
			expectedRel:   func(_, cwd string) string { return filepath.Join(cwd, ".gemini", "settings.json") },
			assertContent: assertJSONServerEntry("mcpServers", "stdio"),
		},
		{
			name:   "copilot project stdio",
			client: "copilot", global: false, transport: "stdio",
			expectedRel:   func(_, cwd string) string { return filepath.Join(cwd, ".vscode", "mcp.json") },
			assertContent: assertJSONServerEntry("servers", "stdio"),
		},
		{
			name:   "copilot global sse",
			client: "copilot", global: true, transport: "sse",
			url:           "http://127.0.0.1:9000/sse",
			expectedRel:   func(home, _ string) string { return filepath.Join(home, ".config", "Code", "User", "mcp.json") },
			assertContent: assertJSONServerEntry("servers", "sse"),
		},
		{
			name:   "cursor project stdio",
			client: "cursor", global: false, transport: "stdio",
			expectedRel:   func(_, cwd string) string { return filepath.Join(cwd, ".cursor", "mcp.json") },
			assertContent: assertJSONServerEntry("mcpServers", "stdio"),
		},
		{
			name:   "codex global stdio",
			client: "codex", global: true, transport: "stdio",
			expectedRel: func(home, _ string) string { return filepath.Join(home, ".codex", "config.toml") },
			assertContent: func(t *testing.T, content []byte, entry serverEntry) {
				s := string(content)
				if !strings.Contains(s, "[mcp_servers."+entry.Name+"]") {
					t.Fatalf("missing TOML section header: %s", s)
				}
				if !strings.Contains(s, `command = "`+entry.Command+`"`) {
					t.Fatalf("missing command line: %s", s)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, cwd := withTempHomeAndCwd(t)
			entry := serverEntry{
				Name:      "myapp",
				Command:   "/usr/local/bin/myapp",
				Args:      []string{"mcp", "serve"},
				Transport: tc.transport,
				URL:       tc.url,
			}
			path, err := writeClientConfig(tc.client, tc.global, entry)
			if err != nil {
				t.Fatalf("writeClientConfig: %v", err)
			}
			want := tc.expectedRel(home, cwd)
			if !pathsEqual(path, want) {
				t.Fatalf("path = %q, want %q", path, want)
			}
			data, err := os.ReadFile(want)
			if err != nil {
				t.Fatalf("expected install at %s, got read error: %v", want, err)
			}
			tc.assertContent(t, data, entry)
		})
	}
}

func TestWriteCodexTOMLReplacesExistingSection(t *testing.T) {
	home, _ := withTempHomeAndCwd(t)
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	prior := `model = "gpt-5"

[mcp_servers.myapp]
command = "/old/path"
args = ["mcp", "serve"]

[mcp_servers.other]
command = "/keep/me"
args = ["x"]
`
	if err := os.WriteFile(path, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := serverEntry{
		Name:      "myapp",
		Command:   "/new/path",
		Args:      []string{"mcp", "serve"},
		Transport: "stdio",
	}
	if err := writeCodexTOML(path, entry); err != nil {
		t.Fatalf("writeCodexTOML: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)

	if !strings.Contains(s, `model = "gpt-5"`) {
		t.Fatalf("dropped unrelated top-level keys: %s", s)
	}
	if !strings.Contains(s, "[mcp_servers.other]") || !strings.Contains(s, `command = "/keep/me"`) {
		t.Fatalf("dropped unrelated section: %s", s)
	}
	if strings.Contains(s, `command = "/old/path"`) {
		t.Fatalf("did not replace stale command: %s", s)
	}
	if !strings.Contains(s, `command = "/new/path"`) {
		t.Fatalf("missing replacement command: %s", s)
	}
}

func TestClientConfigPathRejectsUnknownClient(t *testing.T) {
	if got := clientConfigPath("nope", false); got != "" {
		t.Fatalf("clientConfigPath(unknown) = %q, want empty", got)
	}
}

// assertJSONServerEntry returns a check that the JSON file uses the given
// top-level key, contains an entry named "myapp", and has the expected
// transport-specific fields.
func assertJSONServerEntry(topKey, transport string) func(t *testing.T, content []byte, entry serverEntry) {
	return func(t *testing.T, content []byte, entry serverEntry) {
		var parsed map[string]any
		if err := json.Unmarshal(content, &parsed); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, content)
		}
		servers, ok := parsed[topKey].(map[string]any)
		if !ok {
			t.Fatalf("missing top-level key %q in %v", topKey, parsed)
		}
		raw, ok := servers[entry.Name].(map[string]any)
		if !ok {
			t.Fatalf("missing server %q under %s", entry.Name, topKey)
		}
		if raw["type"] != transport {
			t.Fatalf("type = %v, want %s", raw["type"], transport)
		}
		switch transport {
		case "stdio":
			if raw["command"] != entry.Command {
				t.Fatalf("command = %v, want %s", raw["command"], entry.Command)
			}
			args, _ := raw["args"].([]any)
			if len(args) != len(entry.Args) {
				t.Fatalf("args = %v, want %v", args, entry.Args)
			}
		case "sse":
			if raw["url"] != entry.URL {
				t.Fatalf("url = %v, want %s", raw["url"], entry.URL)
			}
		}
	}
}

// withTempHomeAndCwd points $HOME at a fresh tempdir and chdirs into a
// second tempdir so install runs are fully sandboxed. Restores both on
// cleanup.
func withTempHomeAndCwd(t *testing.T) (home, cwd string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)

	cwd = t.TempDir()
	prior, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prior) })
	return home, cwd
}

// pathsEqual compares two paths after Abs + Clean + symlink resolution.
// Symlink resolution matters on macOS where /tmp is a symlink to
// /private/tmp, so an Abs of "./foo" inside a TempDir won't textually
// match the TempDir path returned to the test.
func pathsEqual(a, b string) bool {
	return canonicalPath(a) == canonicalPath(b)
}

func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	// File may not exist yet (e.g. expected path before write); resolve
	// the deepest existing parent and re-join the leaf.
	dir, leaf := filepath.Split(abs)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Clean(filepath.Join(resolved, leaf))
	}
	return filepath.Clean(abs)
}
