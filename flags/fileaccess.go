package flags

import (
	"fmt"
	"net"
	"net/url"
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
// Where expansion IS enabled, the paths and hosts below stay blocked. An
// opt-in says "this field names a document", not "this field may read the
// host's secrets".

// FileReadPolicy decides whether an `@` value may be expanded, and how strictly.
type FileReadPolicy struct {
	// Enabled is the field's opt-in for this path. False passes the value
	// through untouched.
	Enabled bool
	// Remote marks a value that arrived over the network. It tightens the URL
	// rules, because a server fetching a caller-supplied URL can reach hosts
	// the caller cannot.
	Remote bool
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
func expandFileRef(value string, policy FileReadPolicy, load func(string) (string, error)) (string, error) {
	if !strings.HasPrefix(value, "@") || !policy.Enabled {
		return value, nil
	}
	ref := strings.TrimPrefix(value, "@")
	if err := checkFileRef(ref, policy); err != nil {
		return "", err
	}
	return load(ref)
}

// checkFileRef rejects a reference that names protected state.
func checkFileRef(ref string, policy FileReadPolicy) error {
	if isURL(ref) {
		return checkURL(ref, policy)
	}
	return checkPath(ref)
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

// checkURL blocks the addresses a caller should not be able to make someone
// else's process reach: cloud instance metadata above all, which hands out
// credentials to anything that asks from the right network.
func checkURL(raw string, policy FileReadPolicy) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parsing URL %q: %w", raw, err)
	}
	host := strings.ToLower(parsed.Hostname())

	// Metadata endpoints are blocked on both paths: there is no legitimate
	// reason to load a flag value from one, and it is the highest-value target.
	if host == "metadata.google.internal" || host == "metadata" || strings.HasPrefix(host, "169.254.") {
		return fmt.Errorf("refusing to fetch %q: instance metadata endpoints are never readable through @-expansion", raw)
	}
	if !policy.Remote {
		return nil
	}
	// A server fetching on behalf of a caller must stay off the networks that
	// caller cannot reach directly.
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return blockedHost(raw, host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return blockedHost(raw, host)
		}
	}
	return nil
}

func blockedHost(raw, host string) error {
	return fmt.Errorf("refusing to fetch %q: %s is on a loopback, private or internal network, "+
		"which a request-supplied URL may not reach", raw, host)
}
