// Package cache defines clicky's cache contracts and a dependency-free
// in-process backend for them:
//
//   - Store (store.go) is a minimal Redis-shaped key/value surface — the
//     strings and sorted sets that the domain stores (prompt.Store,
//     metrics.Timeseries) are written once against. NewMemory is the in-process
//     implementation; valkey.NewStore is the cross-process one.
//   - Browser (this file) is a read/admin API over a whole cache keyspace
//     (valkey/redis or anything shaped like it): a prefix tree, per-key detail,
//     search, whole-keyspace stats, and key/prefix deletion.
//
// The package mirrors the metrics split: the interfaces plus the HTTP handler
// live here in the root module, while the valkey-backed implementations live in
// their own module (github.com/flanksource/clicky/valkey) so the root module
// never pulls in client dependencies.
package cache

import (
	"context"
	"errors"
)

// ErrKeyNotFound is returned by Browser.Key when the key does not exist. The
// HTTP handler maps it to a 404.
var ErrKeyNotFound = errors.New("cache: key not found")

// TreeRequest asks for one level of the key tree under Prefix. Keys are split
// on the backend's separator (":" for valkey); each distinct next segment
// becomes either a group node (more segments follow) or a leaf (an actual
// key).
type TreeRequest struct {
	// Prefix is the logical prefix to expand, e.g. "tx:". Empty expands the
	// root. It must include the trailing separator when pointing at a group.
	Prefix string
	// MaxChildren caps the number of nodes returned for the level. Zero uses
	// the backend default.
	MaxChildren int
}

// TreeNode is one entry in a tree level: either a group of keys sharing the
// next path segment, or a single leaf key.
type TreeNode struct {
	// Name is the display segment: the next path segment for groups, the
	// remaining key tail for leaves.
	Name string `json:"name"`
	// Prefix is set on group nodes only: the full logical prefix including
	// the trailing separator, ready to feed back into TreeRequest.Prefix.
	Prefix string `json:"prefix,omitempty"`
	// Key is set on leaf nodes only: the full logical key.
	Key string `json:"key,omitempty"`
	// Keys is the total number of keys under this node (1 for a leaf).
	Keys int `json:"keys"`
	// Children is the number of distinct child segments under a group node.
	Children int `json:"children,omitempty"`
	// Type is the value type of a leaf: string|hash|list|set|zset|stream.
	Type string `json:"type,omitempty"`
	// TTLSeconds is the leaf's remaining TTL; -1 means no expiry.
	TTLSeconds int64 `json:"ttlSeconds,omitempty"`
	// Bytes is the MEMORY USAGE of the node when the server supports it: the
	// leaf's own size, or for a group the aggregated size of every key
	// beneath it.
	Bytes int64 `json:"bytes,omitempty"`
}

// TreeResponse is one expanded tree level.
type TreeResponse struct {
	Prefix string     `json:"prefix"`
	Nodes  []TreeNode `json:"nodes"`
	// Keys is the total number of keys scanned under Prefix, including keys
	// rolled up into group nodes.
	Keys int `json:"keys"`
	// Truncated is set when the node list or the underlying scan hit a cap;
	// counts are then lower bounds.
	Truncated bool `json:"truncated,omitempty"`
	// BytesSupported reports whether per-key byte sizes were available
	// (MEMORY USAGE); when false every Bytes field is absent, not zero.
	BytesSupported bool `json:"bytesSupported"`
}

// ScoredMember is one zset member with its score.
type ScoredMember struct {
	Member string  `json:"member"`
	Score  float64 `json:"score"`
}

// KeyDetail is the full detail for a single key. Exactly one of Value,
// Fields, Items, Members is populated according to Type.
type KeyDetail struct {
	Key        string `json:"key"`
	Type       string `json:"type"`
	TTLSeconds int64  `json:"ttlSeconds"`
	// Bytes is MEMORY USAGE when supported, otherwise omitted.
	Bytes int64 `json:"bytes,omitempty"`
	// Length is the full value length: STRLEN, HLEN, LLEN, SCARD or ZCARD.
	Length  int64             `json:"length"`
	Value   string            `json:"value,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
	Items   []string          `json:"items,omitempty"`
	Members []ScoredMember    `json:"members,omitempty"`
	// Truncated is set when the returned value was capped (string bytes or
	// collection items); Length still reports the full size.
	Truncated bool `json:"truncated,omitempty"`
}

// SearchRequest is a substring search over the whole keyspace.
type SearchRequest struct {
	Query string
	// Limit caps the number of returned keys. Zero uses the backend default.
	Limit int
}

// SearchResponse lists matching keys as leaf nodes.
type SearchResponse struct {
	Keys      []TreeNode `json:"keys"`
	Truncated bool       `json:"truncated,omitempty"`
}

// Stats is a whole-keyspace overview. Keys is always populated; the
// server-info fields are populated only when the backend's INFO command
// succeeded — when it failed or is unsupported, InfoError carries the reason
// so the caller can render the gap honestly instead of showing zeros.
type Stats struct {
	Keys int64 `json:"keys"`
	// KeysTruncated is set when Keys was counted by a capped scan (namespaced
	// keyspaces) and is therefore a lower bound.
	KeysTruncated    bool   `json:"keysTruncated,omitempty"`
	UsedMemoryBytes  int64  `json:"usedMemoryBytes,omitempty"`
	MaxMemoryBytes   int64  `json:"maxMemoryBytes,omitempty"`
	Hits             int64  `json:"hits,omitempty"`
	Misses           int64  `json:"misses,omitempty"`
	EvictedKeys      int64  `json:"evictedKeys,omitempty"`
	ExpiredKeys      int64  `json:"expiredKeys,omitempty"`
	ConnectedClients int64  `json:"connectedClients,omitempty"`
	Version          string `json:"version,omitempty"`
	UptimeSeconds    int64  `json:"uptimeSeconds,omitempty"`
	InfoError        string `json:"infoError,omitempty"`
}

// DeleteResponse reports how many keys a delete removed.
type DeleteResponse struct {
	Deleted int64 `json:"deleted"`
}

// Browser is the backend contract the HTTP handler serves. All keys and
// prefixes are logical: implementations strip any physical namespace prefix
// before returning and re-apply it on lookups.
type Browser interface {
	Tree(ctx context.Context, req TreeRequest) (TreeResponse, error)
	Key(ctx context.Context, key string) (KeyDetail, error)
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
	Stats(ctx context.Context) (Stats, error)
	DeleteKey(ctx context.Context, key string) (DeleteResponse, error)
	DeletePrefix(ctx context.Context, prefix string) (DeleteResponse, error)
}
