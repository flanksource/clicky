package valkey_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"time"

	"github.com/alicebob/miniredis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	valkeygo "github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/clicky/prompt"
	"github.com/flanksource/clicky/valkey"
)

// base is a fixed reference time so specs never depend on wall-clock now.
var base = time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

func newClient() (valkeygo.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	Expect(err).NotTo(HaveOccurred())
	client, err := valkeygo.NewClient(valkeygo.ClientOption{
		InitAddress:  []string{mr.Addr()},
		DisableCache: true,
	})
	Expect(err).NotTo(HaveOccurred())
	return client, mr
}

// The prompt and metrics stores are written once over cache.Store. These specs
// run the same behaviour against both backends — the dependency-free in-memory
// one and the valkey adapter over miniredis — so the two can never silently
// diverge the way the old hand-forked implementations did.
var _ = Describe("Domain stores over cache.Store", func() {
	for _, name := range []string{"memory", "valkey"} {
		name := name
		Context(name+" backend", func() {
			var (
				kv      cache.Store
				cleanup func()
			)

			BeforeEach(func() {
				switch name {
				case "memory":
					kv, cleanup = cache.NewMemory(), func() {}
				case "valkey":
					client, mr := newClient()
					kv, cleanup = valkey.NewStore(client), func() { client.Close(); mr.Close() }
				}
			})

			AfterEach(func() { cleanup() })

			Describe("prompt.Store", func() {
				var store prompt.Store

				BeforeEach(func() {
					store = prompt.NewStore(kv, prompt.StoreConfig{KeyPrefix: "app:", Retention: time.Hour})
				})

				It("round-trips a snapshot and filters by owner and labels", func() {
					Expect(store.Set(prompt.PromptSnapshot{
						ID: "a", Owner: "todo-1", State: "pending",
						Labels:    map[string]string{"session": "s1"},
						CreatedAt: base.Format(time.RFC3339),
					})).To(Succeed())
					Expect(store.Set(prompt.PromptSnapshot{
						ID: "b", Owner: "todo-2", State: "pending",
						Labels:    map[string]string{"session": "s2"},
						CreatedAt: base.Add(time.Minute).Format(time.RFC3339),
					})).To(Succeed())

					got, ok := store.Get("a")
					Expect(ok).To(BeTrue())
					Expect(got.Owner).To(Equal("todo-1"))

					owned := store.List(prompt.Filter{Owner: "todo-1"})
					Expect(owned).To(HaveLen(1))
					Expect(owned[0].ID).To(Equal("a"))

					bySession := store.List(prompt.Filter{Labels: map[string]string{"session": "s2"}})
					Expect(bySession).To(HaveLen(1))
					Expect(bySession[0].ID).To(Equal("b"))
				})

				It("lists newest first", func() {
					Expect(store.Set(prompt.PromptSnapshot{ID: "old", CreatedAt: base.Format(time.RFC3339)})).To(Succeed())
					Expect(store.Set(prompt.PromptSnapshot{ID: "new", CreatedAt: base.Add(time.Hour).Format(time.RFC3339)})).To(Succeed())
					all := store.List(prompt.Filter{})
					Expect(all).To(HaveLen(2))
					Expect(all[0].ID).To(Equal("new"))
				})

				It("removes a deleted snapshot from the index", func() {
					Expect(store.Set(prompt.PromptSnapshot{ID: "gone", CreatedAt: base.Format(time.RFC3339)})).To(Succeed())
					Expect(store.Delete("gone")).To(Succeed())
					_, ok := store.Get("gone")
					Expect(ok).To(BeFalse())
					Expect(store.List(prompt.Filter{})).To(BeEmpty())
				})
			})

			Describe("metrics.Timeseries", func() {
				var ts metrics.Timeseries

				BeforeEach(func() {
					ts = metrics.NewStore(kv, metrics.StoreConfig{KeyPrefix: "app:", Retention: time.Hour})
				})

				It("round-trips recorded points within a query range, ascending", func() {
					for i, v := range []float64{1, 2, 3, 4, 5} {
						Expect(ts.Record(metrics.RecordRequest{
							ID:    "cpu",
							At:    base.Add(time.Duration(i) * time.Minute),
							Value: v,
						})).To(Succeed())
					}

					got, err := ts.Query(metrics.QueryRequest{
						ID:    "cpu",
						Since: base.Add(time.Minute),
						Until: base.Add(3 * time.Minute),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal([]metrics.Point{
						{At: base.Add(time.Minute), Value: 2},
						{At: base.Add(2 * time.Minute), Value: 3},
						{At: base.Add(3 * time.Minute), Value: 4},
					}))
				})

				It("trims points older than the retention window on record", func() {
					ts = metrics.NewStore(kv, metrics.StoreConfig{KeyPrefix: "app:", Retention: 10 * time.Minute})
					Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base.Add(-time.Hour), Value: 1})).To(Succeed())
					Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base, Value: 2})).To(Succeed())

					got, err := ts.Query(metrics.QueryRequest{ID: "cpu"})
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal([]metrics.Point{{At: base, Value: 2}}))
				})

				It("caps retained points at MaxPoints, keeping the newest", func() {
					ts = metrics.NewStore(kv, metrics.StoreConfig{KeyPrefix: "app:", Retention: time.Hour, MaxPoints: 3})
					for i, v := range []float64{1, 2, 3, 4, 5} {
						Expect(ts.Record(metrics.RecordRequest{
							ID:    "cpu",
							At:    base.Add(time.Duration(i) * time.Second),
							Value: v,
						})).To(Succeed())
					}
					got, err := ts.Query(metrics.QueryRequest{ID: "cpu"})
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal([]metrics.Point{
						{At: base.Add(2 * time.Second), Value: 3},
						{At: base.Add(3 * time.Second), Value: 4},
						{At: base.Add(4 * time.Second), Value: 5},
					}))
				})

				It("returns an empty slice for an unknown metric", func() {
					got, err := ts.Query(metrics.QueryRequest{ID: "missing"})
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(BeEmpty())
				})
			})
		})
	}
})

// mgetEntry is one MGet yield, flattened so a whole read compares in one
// assertion.
type mgetEntry struct {
	Key   string
	Value string
	Found bool
	Err   error
}

func collectMGet(entries iter.Seq2[cache.Entry, error]) []mgetEntry {
	var out []mgetEntry
	for entry, err := range entries {
		out = append(out, mgetEntry{Key: entry.Key, Value: string(entry.Value), Found: entry.Found, Err: err})
	}
	return out
}

// counted yields keys and counts how many the reader pulled.
func counted(keys []string, pulled *int) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, key := range keys {
			*pulled++
			if !yield(key) {
				return
			}
		}
	}
}

var _ = Describe("cache.Store MGet", func() {
	for _, name := range []string{"memory", "valkey"} {
		Context(name+" backend", func() {
			var (
				ctx context.Context
				kv  cache.Store
			)

			BeforeEach(func() {
				ctx = context.Background()
				switch name {
				case "memory":
					kv = cache.NewMemory()
				case "valkey":
					client, mr := newClient()
					DeferCleanup(func() { client.Close(); mr.Close() })
					kv = valkey.NewStore(client)
				}
			})

			It("yields every key in the order asked, a key holding no value as not found", func() {
				Expect(kv.Set(ctx, "a", []byte("1"), 0)).To(Succeed())
				Expect(kv.Set(ctx, "c", []byte("3"), 0)).To(Succeed())
				Expect(kv.ZAdd(ctx, "z", 1, "member")).To(Succeed())

				Expect(collectMGet(kv.MGet(ctx, slices.Values([]string{"a", "b", "z", "c", "a"})))).To(Equal([]mgetEntry{
					{Key: "a", Value: "1", Found: true},
					{Key: "b"},
					{Key: "z"},
					{Key: "c", Value: "3", Found: true},
					{Key: "a", Value: "1", Found: true},
				}))
			})

			It("reads a key set larger than one round trip in order", func() {
				var keys []string
				var expected []mgetEntry
				for n := range 250 {
					key := fmt.Sprintf("key-%03d", n)
					keys = append(keys, key)
					if n%7 == 0 {
						expected = append(expected, mgetEntry{Key: key})
						continue
					}
					Expect(kv.Set(ctx, key, []byte(fmt.Sprint(n)), 0)).To(Succeed())
					expected = append(expected, mgetEntry{Key: key, Value: fmt.Sprint(n), Found: true})
				}

				Expect(collectMGet(kv.MGet(ctx, slices.Values(keys)))).To(Equal(expected))
			})

			It("stops pulling keys once the reader stops", func() {
				keys := make([]string, 250)
				for n := range keys {
					keys[n] = fmt.Sprintf("key-%03d", n)
				}
				pulled := 0
				for entry, err := range kv.MGet(ctx, counted(keys, &pulled)) {
					Expect(err).NotTo(HaveOccurred())
					Expect(entry.Key).To(Equal("key-000"))
					break
				}

				Expect(pulled).To(BeNumerically("<", len(keys)))
			})

			It("yields nothing for no keys", func() {
				Expect(collectMGet(kv.MGet(ctx, slices.Values([]string(nil))))).To(BeEmpty())
			})
		})
	}
})

// These specs are valkey-specific: they reach into miniredis to confirm the
// adapter renders the wire commands (TTL, nil) the way the in-memory backend's
// semantics imply.
var _ = Describe("valkey.NewStore adapter", func() {
	var (
		client valkeygo.Client
		mr     *miniredis.Miniredis
		kv     cache.Store
	)

	BeforeEach(func() {
		client, mr = newClient()
		kv = valkey.NewStore(client)
	})

	AfterEach(func() {
		client.Close()
		mr.Close()
	})

	It("maps a missing key to cache.ErrKeyNotFound", func() {
		_, err := kv.Get(context.Background(), "nope")
		Expect(errors.Is(err, cache.ErrKeyNotFound)).To(BeTrue())
	})

	It("yields an MGet read failure once, and nothing after it", func() {
		mr.Close()
		entries := collectMGet(kv.MGet(context.Background(), slices.Values([]string{"a", "b"})))

		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Err).To(HaveOccurred())
		Expect(entries[0].Key).To(BeEmpty())
	})

	// miniredis answers CLUSTER SLOTS, so newClient is a cluster client that
	// reads keys of different slots separately; a standalone server reads a
	// batch in one MGET. The server is its own and the client holds one
	// connection, so neither another client nor a lazily opened connection's
	// handshake reaches the count.
	It("reads a batch of keys in one MGET round trip on a standalone server", func() {
		ctx := context.Background()
		server, err := miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(server.Close)
		single, err := valkeygo.NewClient(valkeygo.ClientOption{
			InitAddress: []string{server.Addr()}, DisableCache: true, ForceSingleClient: true, PipelineMultiplex: -1,
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(single.Close)
		standalone := valkey.NewStore(single)
		Expect(standalone.Set(ctx, "a", []byte("1"), 0)).To(Succeed())
		before := server.CommandCount()
		Expect(collectMGet(standalone.MGet(ctx, slices.Values([]string{"a", "b", "c"})))).To(Equal([]mgetEntry{
			{Key: "a", Value: "1", Found: true}, {Key: "b"}, {Key: "c"},
		}))

		Expect(server.CommandCount() - before).To(Equal(1))
	})

	It("sets a ttl that miniredis observes", func() {
		Expect(kv.Set(context.Background(), "k", []byte("v"), time.Hour)).To(Succeed())
		Expect(mr.TTL("k")).To(Equal(time.Hour))
	})

	It("clears the ttl (PERSIST) when Expire gets a non-positive ttl", func() {
		ctx := context.Background()
		Expect(kv.Set(ctx, "k", []byte("v"), time.Hour)).To(Succeed())
		// A non-positive ttl must remove the expiry, matching the in-memory store —
		// not collapse to a ~1ms PEXPIRE that reaps the key almost immediately.
		Expect(kv.Expire(ctx, "k", 0)).To(Succeed())
		Expect(mr.TTL("k")).To(Equal(time.Duration(0)))
	})

	It("expires the metric key the timeseries store writes", func() {
		ts := metrics.NewStore(kv, metrics.StoreConfig{KeyPrefix: "app:", Retention: time.Hour})
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base, Value: 1})).To(Succeed())
		Expect(mr.TTL("app:metric:cpu")).To(Equal(time.Hour))
	})
})
