package valkey

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/cache"
)

func (b *browser) Key(ctx context.Context, key string) (cache.KeyDetail, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()

	physical := b.cfg.KeyPrefix + key
	typ, err := b.client.Do(ctx, b.client.B().Type().Key(physical).Build()).ToString()
	if err != nil {
		return cache.KeyDetail{}, err
	}
	if typ == "none" {
		return cache.KeyDetail{}, cache.ErrKeyNotFound
	}

	detail := cache.KeyDetail{Key: key, Type: typ}
	if detail.TTLSeconds, err = b.client.Do(ctx, b.client.B().Ttl().Key(physical).Build()).AsInt64(); err != nil {
		return cache.KeyDetail{}, err
	}
	if !b.memoryUnsupported.Load() {
		bytes, err := b.client.Do(ctx, b.client.B().MemoryUsage().Key(physical).Build()).AsInt64()
		switch {
		case err == nil:
			detail.Bytes = bytes
		case isUnknownCommand(err):
			b.memoryUnsupported.Store(true)
		default:
			return cache.KeyDetail{}, err
		}
	}
	if err := b.loadValue(ctx, physical, &detail); err != nil {
		return cache.KeyDetail{}, err
	}
	return detail, nil
}

// loadValue fills Length plus the type-appropriate value field, capping
// strings at MaxValueBytes and collections at MaxItems.
func (b *browser) loadValue(ctx context.Context, physical string, d *cache.KeyDetail) error {
	c := b.client
	maxItems := int64(b.cfg.MaxItems)
	var err error
	switch d.Type {
	case "string":
		if d.Length, err = c.Do(ctx, c.B().Strlen().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		d.Value, err = c.Do(ctx, c.B().Getrange().Key(physical).
			Start(0).End(int64(b.cfg.MaxValueBytes)-1).Build()).ToString()
		if err != nil {
			return err
		}
		d.Truncated = d.Length > int64(b.cfg.MaxValueBytes)
	case "hash":
		if d.Length, err = c.Do(ctx, c.B().Hlen().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		fields, err := c.Do(ctx, c.B().Hgetall().Key(physical).Build()).AsStrMap()
		if err != nil {
			return err
		}
		d.Fields = capFields(fields, b.cfg.MaxItems)
		d.Truncated = d.Length > maxItems
	case "list":
		if d.Length, err = c.Do(ctx, c.B().Llen().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		if d.Items, err = c.Do(ctx, c.B().Lrange().Key(physical).
			Start(0).Stop(maxItems-1).Build()).AsStrSlice(); err != nil {
			return err
		}
		d.Truncated = d.Length > maxItems
	case "set":
		if d.Length, err = c.Do(ctx, c.B().Scard().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		items, err := c.Do(ctx, c.B().Smembers().Key(physical).Build()).AsStrSlice()
		if err != nil {
			return err
		}
		sort.Strings(items)
		if int64(len(items)) > maxItems {
			items = items[:maxItems]
		}
		d.Items = items
		d.Truncated = d.Length > maxItems
	case "zset":
		if d.Length, err = c.Do(ctx, c.B().Zcard().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		scores, err := c.Do(ctx, c.B().Zrange().Key(physical).
			Min("0").Max(strconv.FormatInt(maxItems-1, 10)).Withscores().Build()).AsZScores()
		if err != nil {
			return err
		}
		d.Members = make([]cache.ScoredMember, 0, len(scores))
		for _, s := range scores {
			d.Members = append(d.Members, cache.ScoredMember{Member: s.Member, Score: s.Score})
		}
		d.Truncated = d.Length > maxItems
	case "stream":
		if d.Length, err = c.Do(ctx, c.B().Xlen().Key(physical).Build()).AsInt64(); err != nil {
			return err
		}
		// Stream entries are not rendered; Length and metadata are enough
		// for the browser to show what the key is.
	}
	return nil
}

// capFields bounds a hash to max entries deterministically (sorted by field
// name) so truncation is stable across requests.
func capFields(fields map[string]string, max int) map[string]string {
	if len(fields) <= max {
		return fields
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	capped := make(map[string]string, max)
	for _, name := range names[:max] {
		capped[name] = fields[name]
	}
	return capped
}

func (b *browser) Stats(ctx context.Context) (cache.Stats, error) {
	ctx, cancel := b.opCtx(ctx)
	defer cancel()

	var stats cache.Stats
	if b.cfg.KeyPrefix == "" {
		keys, err := b.client.Do(ctx, b.client.B().Dbsize().Build()).AsInt64()
		if err != nil {
			return cache.Stats{}, err
		}
		stats.Keys = keys
	} else {
		// DBSIZE counts the whole DB; a namespaced browser only owns
		// KeyPrefix, so count by scan and report the cap honestly.
		keys, truncated, err := b.scan(ctx, globEscape(b.cfg.KeyPrefix)+"*", b.cfg.MaxScan)
		if err != nil {
			return cache.Stats{}, err
		}
		stats.Keys = int64(len(keys))
		stats.KeysTruncated = truncated
	}

	info, err := b.client.Do(ctx, b.client.B().Info().Build()).ToString()
	if err != nil {
		// INFO is unsupported on some embedded backends (miniredis); the
		// keyspace numbers above are still valid, so surface the gap in the
		// payload instead of failing the whole overview.
		stats.InfoError = err.Error()
		return stats, nil
	}
	applyInfo(&stats, info)
	return stats, nil
}

// applyInfo copies the fields the overview renders out of an INFO dump.
func applyInfo(stats *cache.Stats, info string) {
	fields := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, value, ok := strings.Cut(line, ":"); ok {
			fields[name] = value
		}
	}
	asInt := func(name string) int64 {
		v, _ := strconv.ParseInt(fields[name], 10, 64)
		return v
	}
	stats.UsedMemoryBytes = asInt("used_memory")
	stats.MaxMemoryBytes = asInt("maxmemory")
	stats.Hits = asInt("keyspace_hits")
	stats.Misses = asInt("keyspace_misses")
	stats.EvictedKeys = asInt("evicted_keys")
	stats.ExpiredKeys = asInt("expired_keys")
	stats.ConnectedClients = asInt("connected_clients")
	stats.UptimeSeconds = asInt("uptime_in_seconds")
	stats.Version = fields["redis_version"]
	if stats.Version == "" {
		stats.Version = fields["valkey_version"]
	}
}
