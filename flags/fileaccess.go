package flags

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expanding an `@value` into the contents of a file or URL reads whatever the
// value names, on whichever machine is doing the expanding. On the CLI that
// machine is the operator's own; over RPC it is the *server*, and the value
// arrives from whoever made the request — so the same helper that is a
// convenience on one path is an arbitrary file read and an SSRF on the other.
//
// Expansion is therefore off unless a field asks for it:
//
//	Source string `flag:"source" clicky:"cli-file-read"`             // CLI only
//	Config string `flag:"config" clicky:"cli-file-read,rpc-file-read"` // both
//
// Without the tag an `@` value is passed through verbatim, which is also the
// right answer for content that legitimately starts with one — a Java
// annotation, an email address, an npm scope.
//
// Where expansion IS enabled, the paths below stay blocked. Request-supplied
// URL destinations are constrained by commons/http's defensive client.

// FileReadPolicy decides whether an `@` value may be expanded, and how strictly.
type FileReadPolicy struct {
	// Enabled is the field's opt-in for this path. False passes the value
	// through untouched.
	Enabled bool
	// Remote marks a value that arrived over the network. It tightens the URL
	// rules, because a server fetching a caller-supplied URL can reach hosts
	// the caller cannot.
	Remote bool
	ctx    context.Context
}

// deniedPrefixes are directory trees that hold credentials, kernel state or
// other process memory. /proc in particular exposes every process's environment,
// which is where secrets usually live.
var deniedPrefixes = []string{
	"/proc",
	"/sys",
	"/dev",
	"/root",
	"/etc/ssh",
	"/etc/pki",
	"/var/run/secrets",
	"/run/secrets",
	"/private/etc/ssh",
}

// deniedFiles are exact absolute paths worth naming individually.
var deniedFiles = []string{
	"/etc/passwd",
	"/etc/shadow",
	"/etc/gshadow",
	"/etc/sudoers",
	"/etc/master.passwd",
	"/private/etc/passwd",
	"/private/etc/master.passwd",
}

// deniedDirNames are directory names blocked wherever they appear, so a home
// directory does not have to be enumerated.
var deniedDirNames = []string{
	".ssh", ".aws", ".azure", ".gnupg", ".gcloud", ".kube", ".docker", ".config/gcloud",
}

// deniedBaseNames are filenames blocked wherever they appear.
var deniedBaseNames = []string{
	".netrc", ".npmrc", ".pypirc", ".htpasswd", ".git-credentials", ".dockercfg",
	"credentials", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "shadow",
}

// deniedExtensions are private-key and keystore formats.
var deniedExtensions = []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".ppk"}

// expandFileRef resolves an `@` value under the given policy. A value without
// the prefix, or a field that has not opted in, comes back unchanged.
func expandFileRef(value string, policy FileReadPolicy, load func(string, FileReadPolicy) (string, error)) (string, error) {
	if !strings.HasPrefix(value, "@") || !policy.Enabled {
		return value, nil
	}
	ref, err := resolveFileRef(strings.TrimPrefix(value, "@"))
	if err != nil {
		return "", err
	}
	return load(ref, policy)
}

func resolveFileRef(ref string) (string, error) {
	if isURL(ref) {
		return ref, nil
	}
	absolute, err := filepath.Abs(ref)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", ref, err)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return "", fmt.Errorf("resolving symlinks for %q: %w", ref, err)
	}
	if err := checkPath(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

func openValidatedFile(path string) (_ *os.File, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, file.Close())
		}
	}()

	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("revalidating %q after open: %w", path, err)
	}
	if err := checkPath(canonical); err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened file %q: %w", path, err)
	}
	current, err := os.Stat(canonical)
	if err != nil {
		return nil, fmt.Errorf("stat validated file %q: %w", canonical, err)
	}
	if !os.SameFile(opened, current) {
		return nil, fmt.Errorf("refusing to read %q: file changed while it was being validated", path)
	}
	return file, nil
}

func isURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// checkPath blocks credential stores and kernel state. The path is made
// absolute and cleaned first, so "a/../../etc/passwd" is judged by where it
// actually lands rather than how it was written.
func checkPath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", path, err)
	}
	absolute = filepath.Clean(absolute)
	lower := strings.ToLower(absolute)

	for _, denied := range deniedFiles {
		if lower == denied {
			return blockedPath(path, "a system credential file")
		}
	}
	for _, prefix := range deniedPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+"/") {
			return blockedPath(path, "under "+prefix)
		}
	}

	segments := strings.Split(lower, string(filepath.Separator))
	for _, segment := range segments {
		for _, denied := range deniedDirNames {
			// A nested name like ".config/gcloud" is matched on the joined path
			// rather than a single segment.
			if segment == denied {
				return blockedPath(path, "inside a "+denied+" directory")
			}
		}
	}
	for _, denied := range deniedDirNames {
		if strings.Contains(denied, "/") && strings.Contains(lower, string(filepath.Separator)+denied+string(filepath.Separator)) {
			return blockedPath(path, "inside a "+denied+" directory")
		}
	}

	base := filepath.Base(lower)
	for _, denied := range deniedBaseNames {
		if base == denied {
			return blockedPath(path, "a credential file")
		}
	}
	// .env, .env.local, .env.production — the whole family.
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return blockedPath(path, "an environment file")
	}
	for _, ext := range deniedExtensions {
		if strings.HasSuffix(base, ext) {
			return blockedPath(path, "a private key or keystore")
		}
	}
	return nil
}

func blockedPath(path, why string) error {
	return fmt.Errorf("refusing to read %q: it is %s. "+
		"@-expansion never reads credential stores, private keys or kernel state", path, why)
}
