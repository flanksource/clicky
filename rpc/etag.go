package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// contentETag returns a strong content validator (a quoted SHA-256 prefix) for
// b. Content-derived rather than time-derived so a regenerated document with
// identical bytes keeps revalidating, while any real change invalidates
// immediately without a cache-busting query parameter.
func contentETag(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// etagMatches reports whether a client's If-None-Match header — which may be a
// comma-separated list, or `*` — covers etag. The `W/` weak-validator prefix is
// ignored on both sides, per RFC 7232's weak comparison.
func etagMatches(ifNoneMatch, etag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}

	want := strings.TrimPrefix(etag, "W/")
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == want {
			return true
		}
	}
	return false
}
