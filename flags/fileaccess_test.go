package flags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noLoad fails the test if a read is attempted; the guard must reject before
// anything touches the disk or the network.
func noLoad(t *testing.T) func(string) (string, error) {
	t.Helper()
	return func(ref string) (string, error) {
		t.Fatalf("expansion reached the loader for %q, which should have been refused or passed through", ref)
		return "", nil
	}
}

func TestExpandFileRef_PassesThroughWithoutOptIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := expandFileRef("@"+path, FileReadPolicy{}, noLoad(t))
	if err != nil {
		t.Fatalf("expandFileRef: %v", err)
	}
	if got != "@"+path {
		t.Errorf("got %q; want the value unchanged", got)
	}
}

// A leading @ is ordinary content — a Java annotation, an email address, an npm
// scope — so a field that has not opted in must never reinterpret it.
func TestExpandFileRef_LeavesAtPrefixedContentAlone(t *testing.T) {
	for _, value := range []string{"@Override public class X {}", "@user@example.com", "@scope/pkg"} {
		got, err := expandFileRef(value, FileReadPolicy{}, noLoad(t))
		if err != nil {
			t.Fatalf("expandFileRef(%q): %v", value, err)
		}
		if got != value {
			t.Errorf("expandFileRef(%q) = %q; want it unchanged", value, got)
		}
	}
}

func TestExpandFileRef_ReadsWhenOptedIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := expandFileRef("@"+path, FileReadPolicy{Enabled: true}, fetchFileOrURL)
	if err != nil {
		t.Fatalf("expandFileRef: %v", err)
	}
	if got != "contents" {
		t.Errorf("got %q; want the file contents", got)
	}
}

func TestCheckPath_BlocksCredentialStores(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/home/someone"
	}
	blocked := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/proc/self/environ",
		"/sys/class/net",
		"/root/.bashrc",
		"/var/run/secrets/kubernetes.io/serviceaccount/token",
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".aws", "credentials"),
		filepath.Join(home, ".kube", "config"),
		filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"),
		filepath.Join(home, "project", ".env"),
		filepath.Join(home, "project", ".env.production"),
		filepath.Join(home, "project", ".netrc"),
		filepath.Join(home, "certs", "server.pem"),
		filepath.Join(home, "certs", "server.key"),
		filepath.Join(home, "certs", "store.jks"),
	}
	for _, path := range blocked {
		if err := checkPath(path); err == nil {
			t.Errorf("checkPath(%q) = nil; want a refusal", path)
		}
	}
}

// Judged by where the path lands, not how it is spelled. Absolute bases keep
// the assertion independent of the test's working directory.
func TestCheckPath_BlocksTraversalOntoAProtectedPath(t *testing.T) {
	for _, path := range []string{
		"/var/tmp/../../etc/passwd",
		"/tmp/./../etc/shadow",
		"/var/lib/../../proc/self/environ",
	} {
		if err := checkPath(path); err == nil {
			t.Errorf("checkPath(%q) = nil; a traversal onto a protected path must be refused", path)
		}
	}
}

func TestCheckPath_AllowsOrdinaryDocuments(t *testing.T) {
	dir := t.TempDir()
	allowed := []string{
		filepath.Join(dir, "Patched.java"),
		filepath.Join(dir, "policies.csv"),
		filepath.Join(dir, "notes.md"),
		filepath.Join(dir, "envoy.yaml"),
	}
	for _, path := range allowed {
		if err := checkPath(path); err != nil {
			t.Errorf("checkPath(%q) = %v; want it allowed", path, err)
		}
	}
}

// Metadata endpoints hand out cloud credentials to anything on the right
// network, so they are refused on both the CLI and the RPC path.
func TestCheckURL_BlocksInstanceMetadataEverywhere(t *testing.T) {
	for _, raw := range []string{
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		"http://metadata.google.internal/computeMetadata/v1/",
	} {
		for _, policy := range []FileReadPolicy{{Enabled: true}, {Enabled: true, Remote: true}} {
			if err := checkURL(raw, policy); err == nil {
				t.Errorf("checkURL(%q, remote=%v) = nil; want a refusal", raw, policy.Remote)
			}
		}
	}
}

// A request-supplied URL must not make the server reach networks the caller
// cannot; the same URL typed on the CLI is the operator's own business.
func TestCheckURL_BlocksInternalHostsOnlyForRemoteValues(t *testing.T) {
	internal := []string{
		"http://localhost:8080/x",
		"http://127.0.0.1/x",
		"http://10.0.0.5/x",
		"http://192.168.1.10/x",
		"http://172.16.4.4/x",
		"http://db.internal/x",
	}
	for _, raw := range internal {
		if err := checkURL(raw, FileReadPolicy{Enabled: true, Remote: true}); err == nil {
			t.Errorf("checkURL(%q, remote) = nil; want a refusal", raw)
		}
		if err := checkURL(raw, FileReadPolicy{Enabled: true}); err != nil {
			t.Errorf("checkURL(%q, cli) = %v; want it allowed on the CLI", raw, err)
		}
	}
}

func TestCheckURL_AllowsPublicHosts(t *testing.T) {
	for _, raw := range []string{"https://example.com/list.txt", "https://raw.githubusercontent.com/o/r/main/f"} {
		if err := checkURL(raw, FileReadPolicy{Enabled: true, Remote: true}); err != nil {
			t.Errorf("checkURL(%q) = %v; want it allowed", raw, err)
		}
	}
}

func TestParseFileReadTag(t *testing.T) {
	cases := []struct {
		tag      string
		cli, rpc bool
	}{
		{"", false, false},
		{"cli-file-read", true, false},
		{"rpc-file-read", false, true},
		{"cli-file-read rpc-file-read", true, true},
		{"cli-file-read,rpc-file-read", true, true},
		// Tokens belonging to the schema vocabulary are ignored here.
		{"type=k8s-url-selector,title=URL,cli-file-read", true, false},
		{"title=Source", false, false},
	}
	for _, tc := range cases {
		cli, rpc := parseFileReadTag(tc.tag)
		if cli != tc.cli || rpc != tc.rpc {
			t.Errorf("parseFileReadTag(%q) = (%v, %v); want (%v, %v)", tc.tag, cli, rpc, tc.cli, tc.rpc)
		}
	}
}

// The refusal has to say which path was rejected and why, or an operator
// hitting it on a legitimate file has nothing to act on.
func TestBlockedPathErrorNamesThePathAndReason(t *testing.T) {
	err := checkPath("/etc/passwd")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "/etc/passwd") || !strings.Contains(err.Error(), "credential") {
		t.Errorf("error = %v; want it to name the path and the reason", err)
	}
}
