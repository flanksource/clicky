package valkey_test

import (
	"context"
	"errors"
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
