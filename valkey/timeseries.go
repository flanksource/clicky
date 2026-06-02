// Package valkey implements clicky's metrics.Timeseries on top of a
// valkey/redis sorted set, one ZSET per metric ID. It lives in a separate Go
// module so the valkey-go dependency stays out of the root clicky module:
// callers that only need the in-process store (metrics.NewMemory) never pull
// it in.
//
// Each point is stored as a ZSET member "<unixMillis>:<value>" with the
// millisecond timestamp as the score (see metrics.EncodeMember). Range queries
// are ZRANGEBYSCORE; retention is enforced point-precisely on every Record via
// ZREMRANGEBYSCORE plus an EXPIRE backstop on the key.
package valkey

import (
	"context"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/metrics"
)

// opTimeout caps any single round-trip so recording or querying metrics never
// stalls the caller. Mirrors oipa-cli's cache l2OpTimeout.
const opTimeout = 250 * time.Millisecond

const defaultRetention = 7 * 24 * time.Hour

// Config tunes the valkey-backed store.
type Config struct {
	// KeyPrefix is prepended to every key so metrics share the caller's
	// namespace (e.g. an app-wide prefix). The full key is
	// KeyPrefix + "metric:" + id.
	KeyPrefix string
	// Retention drops points older than this on each Record. Zero -> 7d.
	Retention time.Duration
}

type store struct {
	client    valkey.Client
	keyPrefix string
	retention time.Duration
}

// New returns a metrics.Timeseries backed by client. The client is owned by
// the caller (New does not close it), so an app can share its existing
// connection rather than opening a second one.
func New(client valkey.Client, cfg Config) metrics.Timeseries {
	if cfg.Retention <= 0 {
		cfg.Retention = defaultRetention
	}
	return &store{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
		retention: cfg.Retention,
	}
}

func (s *store) key(id string) string {
	return s.keyPrefix + "metric:" + id
}

func (s *store) Record(req metrics.RecordRequest) error {
	at := req.At
	if at.IsZero() {
		at = time.Now()
	}
	key := s.key(req.ID)
	point := metrics.Point{At: at, Value: req.Value}
	cutoffMs := at.Add(-s.retention).UnixMilli()

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	cmds := valkey.Commands{
		s.client.B().Zadd().Key(key).ScoreMember().
			ScoreMember(float64(at.UnixMilli()), metrics.EncodeMember(point)).Build(),
		// Exclusive lower bound "(<cutoff>" keeps a point recorded exactly at
		// the cutoff edge.
		s.client.B().Zremrangebyscore().Key(key).
			Min("-inf").Max("(" + strconv.FormatInt(cutoffMs, 10)).Build(),
		s.client.B().Expire().Key(key).Seconds(int64(s.retention.Seconds())).Build(),
	}
	for _, resp := range s.client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) Query(req metrics.QueryRequest) ([]metrics.Point, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	members, err := s.client.Do(ctx, s.client.B().Zrangebyscore().Key(s.key(req.ID)).
		Min(scoreBound(req.Since, "-inf")).
		Max(scoreBound(req.Until, "+inf")).Build()).AsStrSlice()
	if err != nil {
		return nil, err
	}
	points := make([]metrics.Point, 0, len(members))
	for _, m := range members {
		p, err := metrics.ParseMember(m)
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

// scoreBound renders a ZRANGEBYSCORE bound: a zero time falls back to the
// unbounded sentinel ("-inf"/"+inf"), otherwise the unix-millis score.
func scoreBound(t time.Time, unbounded string) string {
	if t.IsZero() {
		return unbounded
	}
	return strconv.FormatInt(t.UnixMilli(), 10)
}
