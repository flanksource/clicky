package flags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noLoad fails the test if a read is attempted; the guard must reject before
// anything touches the disk or the network.
func noLoad(t *testing.T) func(string, FileReadPolicy) (string, error) {
	t.Helper()
	return func(ref string, _ FileReadPolicy) (string, error) {
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

func TestExpandFileRef_BlocksSymlinkToProtectedPath(t *testing.T) {
	dir := t.TempDir()
	protectedDir := filepath.Join(dir, ".ssh")
	if err := os.Mkdir(protectedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(protectedDir, "config")
	if err := os.WriteFile(protected, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "document.txt")
	if err := os.Symlink(protected, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := expandFileRef("@"+link, FileReadPolicy{Enabled: true, Remote: true}, fetchFileOrURL); err == nil {
		t.Fatal("expected a symlink to a protected path to be refused")
	}
}

func TestExpandFileRef_BlocksSymlinkSwapAfterValidation(t *testing.T) {
	dir := t.TempDir()
	protectedDir := filepath.Join(dir, ".ssh")
	if err := os.Mkdir(protectedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(protectedDir, "config")
	if err := os.WriteFile(protected, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(dir, "document.txt")
	if err := os.WriteFile(document, []byte("public"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := expandFileRef("@"+document, FileReadPolicy{Enabled: true, Remote: true}, func(validated string, policy FileReadPolicy) (string, error) {
		if err := os.Remove(validated); err != nil {
			return "", err
		}
		if err := os.Symlink(protected, validated); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		return fetchFileOrURL(validated, policy)
	})
	if err == nil {
		t.Fatal("expected a file replaced by a symlink after validation to be refused")
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
