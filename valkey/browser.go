package valkey

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
)

// BrowserConfig tunes the valkey-backed cache.Browser.
type BrowserConfig struct {
	// KeyPrefix is the physical namespace shared with the rest of the app
	// (the same prefix the cache writes with). It is stripped from every key
	// the browser returns and re-applied on lookups, so the API speaks
	// logical keys only.
	KeyPrefix string
	// Separator splits keys into tree segments. Default ":".
	Separator string
	// MaxScan caps the number of keys a single tree/search/stats request
	// scans. Default 100000.
	MaxScan int
	// MaxChildren caps the number of nodes returned per tree level. Default
	// 1000.
	MaxChildren int
	// MaxValueBytes caps the string value returned by Key. Default 256KiB.
	MaxValueBytes int
	// MaxItems caps the collection items returned by Key. Default 1000.
	MaxItems int
	// OpTimeout bounds a whole browser request. Scans over large keyspaces
	// take longer than single-key ops, so this is deliberately more generous
	// than the timeseries opTimeout. Default 10s.
	OpTimeout time.Duration
}

func (c BrowserConfig) withDefaults() BrowserConfig {
	if c.Separator == "" {
		c.Separator = ":"
	}
	if c.MaxScan <= 0 {
		c.MaxScan = 100_000
	}
	if c.MaxChildren <= 0 {
		c.MaxChildren = 1_000
	}
	if c.MaxValueBytes <= 0 {
		c.MaxValueBytes = 256 << 10
	}
	if c.MaxItems <= 0 {
		c.MaxItems = 1_000
	}
	if c.OpTimeout <= 0 {
		c.OpTimeout = 10 * time.Second
	}
	return c
}

type browser struct {
	client valkey.Client
	cfg    BrowserConfig
	// memoryUnsupported latches once the server rejects MEMORY USAGE
	// (miniredis, older forks) so later requests skip the probe and report
	// BytesSupported=false instead of failing.
	memoryUnsupported atomic.Bool
}

// NewBrowser returns a cache.Browser over client. The client is owned by the
// caller (NewBrowser does not close it), so an app can share its existing
// connection.
func NewBrowser(client valkey.Client, cfg BrowserConfig) cache.Browser {
	return &browser{client: client, cfg: cfg.withDefaults()}
}

func (b *browser) opCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, b.cfg.OpTimeout)
}

// globEscape neutralises glob metacharacters in user-supplied fragments so a
// prefix like "tx:[0]" matches literally inside a SCAN MATCH pattern.
func globEscape(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch r {
		case '*', '?', '[', ']', '\\':
			out.WriteByte('\\')
		}
		out.WriteRune(r)
	}
	return out.String()
}

// scan iterates SCAN MATCH pattern until the cursor is exhausted or max keys
// were collected; the second return reports whether the scan stopped early.
func (b *browser) scan(ctx context.Context, pattern string, max int) ([]string, bool, error) {
	var keys []string
	var cursor uint64
	for {
		entry, err := b.client.Do(ctx, b.client.B().Scan().Cursor(cursor).
			Match(pattern).Count(1000).Build()).AsScanEntry()
		if err != nil {
			return nil, false, err
		}
		keys = append(keys, entry.Elements...)
		if len(keys) >= max {
			return keys[:max], entry.Cursor != 0 || len(keys) > max, nil
		}
		if entry.Cursor == 0 {
			return keys, false, nil
		}
		cursor = entry.Cursor
	}
}

func (b *browser) Tree(ctx context.Context, req cache.TreeRequest) (cache.TreeResponse, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()

	pattern := globEscape(b.cfg.KeyPrefix+req.Prefix) + "*"
	keys, truncated, err := b.scan(ctx, pattern, b.cfg.MaxScan)
	if err != nil {
		return cache.TreeResponse{}, err
	}

	// Byte sizes for every scanned key, so each group node can report the
	// aggregated MEMORY USAGE of all keys beneath it (not just its direct
	// leaves). nil when the backend has no MEMORY USAGE support.
	bytesByKey, err := b.memoryUsage(ctx, keys)
	if err != nil {
		return cache.TreeResponse{}, err
	}

	sep := b.cfg.Separator
	type group struct {
		keys     int
		children map[string]struct{}
		bytes    int64
	}
	groups := map[string]*group{}
	var leaves []string
	for _, physical := range keys {
		logical := strings.TrimPrefix(physical, b.cfg.KeyPrefix)
		tail := strings.TrimPrefix(logical, req.Prefix)
		idx := strings.Index(tail, sep)
		if idx < 0 || tail == "" {
			leaves = append(leaves, logical)
			continue
		}
		seg := tail[:idx]
		g := groups[seg]
		if g == nil {
			g = &group{children: map[string]struct{}{}}
			groups[seg] = g
		}
		g.keys++
		g.bytes += bytesByKey[physical]
		rest := tail[idx+len(sep):]
		if next := strings.Index(rest, sep); next >= 0 {
			g.children[rest[:next]] = struct{}{}
		} else {
			g.children[rest] = struct{}{}
		}
	}

	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	sort.Strings(leaves)

	nodes := make([]cache.TreeNode, 0, len(groupNames)+len(leaves))
	for _, name := range groupNames {
		g := groups[name]
		node := cache.TreeNode{
			Name:     name,
			Prefix:   req.Prefix + name + sep,
			Keys:     g.keys,
			Children: len(g.children),
		}
		if bytesByKey != nil {
			node.Bytes = g.bytes
		}
		nodes = append(nodes, node)
	}
	maxChildren := req.MaxChildren
	if maxChildren <= 0 || maxChildren > b.cfg.MaxChildren {
		maxChildren = b.cfg.MaxChildren
	}
	leafBudget := maxChildren - len(nodes)
	if len(nodes) > maxChildren {
		nodes = nodes[:maxChildren]
		leafBudget = 0
		truncated = true
	}
	if leafBudget < len(leaves) {
		leaves = leaves[:max(leafBudget, 0)]
		truncated = true
	}

	leafNodes, err := b.leafNodes(ctx, req.Prefix, leaves, bytesByKey)
	if err != nil {
		return cache.TreeResponse{}, err
	}
	nodes = append(nodes, leafNodes...)

	return cache.TreeResponse{
		Prefix:         req.Prefix,
		Nodes:          nodes,
		Keys:           len(keys),
		Truncated:      truncated,
		BytesSupported: !b.memoryUnsupported.Load(),
	}, nil
}

// memoryUsage pipelines MEMORY USAGE for every physical key and returns a
// physical-key→bytes map. It returns a nil map (and no error) when the backend
// does not support MEMORY USAGE, latching memoryUnsupported so callers skip the
// probe and report BytesSupported=false. A key that vanished between the scan
// and this call (TTL race) contributes nothing rather than failing the level.
func (b *browser) memoryUsage(ctx context.Context, physical []string) (map[string]int64, error) {
	if b.memoryUnsupported.Load() || len(physical) == 0 {
		return nil, nil
	}
	cmds := make(valkey.Commands, 0, len(physical))
	for _, key := range physical {
		cmds = append(cmds, b.client.B().MemoryUsage().Key(key).Build())
	}
	resps := b.client.DoMulti(ctx, cmds...)
	out := make(map[string]int64, len(physical))
	for i, key := range physical {
		bytes, err := resps[i].AsInt64()
		switch {
		case err == nil:
			out[key] = bytes
		case valkey.IsValkeyNil(err):
			// Key expired between scan and probe; skip it.
		case isUnknownCommand(err):
			b.memoryUnsupported.Store(true)
			return nil, nil
		default:
			return nil, err
		}
	}
	return out, nil
}

// leafNodes pipelines TYPE+TTL for each logical key and renders leaf TreeNodes
// in the given order, attaching per-key MEMORY USAGE from bytesByKey (nil when
// the backend has no support). Name is the key tail relative to stripPrefix so
// a tree level displays segments, not full keys.
func (b *browser) leafNodes(ctx context.Context, stripPrefix string, logical []string, bytesByKey map[string]int64) ([]cache.TreeNode, error) {
	if len(logical) == 0 {
		return nil, nil
	}
	const perKey = 2
	cmds := make(valkey.Commands, 0, len(logical)*perKey)
	for _, key := range logical {
		physical := b.cfg.KeyPrefix + key
		cmds = append(cmds,
			b.client.B().Type().Key(physical).Build(),
			b.client.B().Ttl().Key(physical).Build())
	}
	resps := b.client.DoMulti(ctx, cmds...)

	nodes := make([]cache.TreeNode, 0, len(logical))
	for i, key := range logical {
		typ, err := resps[i*perKey].ToString()
		if err != nil {
			return nil, err
		}
		ttl, err := resps[i*perKey+1].AsInt64()
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(key, stripPrefix)
		if name == "" {
			name = key
		}
		node := cache.TreeNode{Name: name, Key: key, Keys: 1, Type: typ, TTLSeconds: ttl}
		if bytesByKey != nil {
			node.Bytes = bytesByKey[b.cfg.KeyPrefix+key]
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// isUnknownCommand matches the server error for a command the backend does
// not implement (e.g. MEMORY on miniredis).
func isUnknownCommand(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unknown command")
}

func (b *browser) Search(ctx context.Context, req cache.SearchRequest) (cache.SearchResponse, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()

	limit := req.Limit
	if limit <= 0 {
		limit = min(100, b.cfg.MaxChildren)
	}
	if limit > b.cfg.MaxChildren {
		limit = b.cfg.MaxChildren
	}
	pattern := globEscape(b.cfg.KeyPrefix) + "*" + globEscape(req.Query) + "*"
	// Scan one past the limit so truncation is reported without a full pass.
	keys, scanTruncated, err := b.scan(ctx, pattern, limit+1)
	if err != nil {
		return cache.SearchResponse{}, err
	}
	truncated := scanTruncated || len(keys) > limit
	if len(keys) > limit {
		keys = keys[:limit]
	}
	bytesByKey, err := b.memoryUsage(ctx, keys)
	if err != nil {
		return cache.SearchResponse{}, err
	}
	logical := make([]string, 0, len(keys))
	for _, k := range keys {
		logical = append(logical, strings.TrimPrefix(k, b.cfg.KeyPrefix))
	}
	sort.Strings(logical)
	nodes, err := b.leafNodes(ctx, "", logical, bytesByKey)
	if err != nil {
		return cache.SearchResponse{}, err
	}
	return cache.SearchResponse{Keys: nodes, Truncated: truncated}, nil
}

func (b *browser) DeleteKey(ctx context.Context, key string) (cache.DeleteResponse, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()
	deleted, err := b.client.Do(ctx, b.client.B().Del().Key(b.cfg.KeyPrefix+key).Build()).AsInt64()
	if err != nil {
		return cache.DeleteResponse{}, err
	}
	return cache.DeleteResponse{Deleted: deleted}, nil
}

func (b *browser) DeletePrefix(ctx context.Context, prefix string) (cache.DeleteResponse, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()
	pattern := globEscape(b.cfg.KeyPrefix+prefix) + "*"
	keys, truncated, err := b.scan(ctx, pattern, b.cfg.MaxScan)
	if err != nil {
		return cache.DeleteResponse{}, err
	}
	if truncated {
		return cache.DeleteResponse{}, fmt.Errorf("prefix delete %q truncated at maxScan=%d; refine prefix or raise cap", pattern, b.cfg.MaxScan)
	}
	// One DEL per key, pipelined: valkey-go rejects multi-key DELs whose keys
	// hash to different slots, and a browser prefix routinely spans slots.
	var deleted int64
	for batch := range slices.Chunk(keys, 512) {
		cmds := make(valkey.Commands, 0, len(batch))
		for _, key := range batch {
			cmds = append(cmds, b.client.B().Del().Key(key).Build())
		}
		for _, resp := range b.client.DoMulti(ctx, cmds...) {
			n, err := resp.AsInt64()
			if err != nil {
				return cache.DeleteResponse{}, err
			}
			deleted += n
		}
	}
	return cache.DeleteResponse{Deleted: deleted}, nil
}
