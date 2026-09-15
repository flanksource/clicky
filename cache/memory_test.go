package cache_test

import (
	"context"
	"errors"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/cache"
)

var _ = Describe("In-memory cache.Store", func() {
	var (
		ctx   context.Context
		store cache.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = cache.NewMemory()
	})

	Describe("strings", func() {
		It("reports an absent key as ErrKeyNotFound", func() {
			_, err := store.Get(ctx, "missing")
			Expect(errors.Is(err, cache.ErrKeyNotFound)).To(BeTrue())
		})

		It("round-trips a value and isolates the caller's slice", func() {
			in := []byte("hello")
			Expect(store.Set(ctx, "k", in, 0)).To(Succeed())
			in[0] = 'J' // mutate after Set: the store must hold its own copy
			got, err := store.Get(ctx, "k")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]byte("hello")))
		})

		It("treats a non-positive ttl as no expiry", func() {
			Expect(store.Set(ctx, "k", []byte("v"), 0)).To(Succeed())
			Consistently(func() error {
				_, err := store.Get(ctx, "k")
				return err
			}, 60*time.Millisecond, 15*time.Millisecond).Should(Succeed())
		})

		It("expires a value lazily once its ttl has passed", func() {
			Expect(store.Set(ctx, "k", []byte("v"), 20*time.Millisecond)).To(Succeed())
			Eventually(func() bool {
				_, err := store.Get(ctx, "k")
				return errors.Is(err, cache.ErrKeyNotFound)
			}, time.Second, 5*time.Millisecond).Should(BeTrue())
		})

		It("deletes a key", func() {
			Expect(store.Set(ctx, "k", []byte("v"), 0)).To(Succeed())
			Expect(store.Del(ctx, "k")).To(Succeed())
			_, err := store.Get(ctx, "k")
			Expect(errors.Is(err, cache.ErrKeyNotFound)).To(BeTrue())
		})
	})

	Describe("MGet", func() {
		It("yields values the reader can change without changing the store", func() {
			Expect(store.Set(ctx, "k", []byte("hello"), 0)).To(Succeed())
			for entry, err := range store.MGet(ctx, slices.Values([]string{"k"})) {
				Expect(err).NotTo(HaveOccurred())
				entry.Value[0] = 'J'
			}
			got, err := store.Get(ctx, "k")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]byte("hello")))
		})

		// A reader commonly writes back what it read; a store still locked while
		// the loop body runs would deadlock on that write.
		It("lets the reader use the store between entries", func() {
			Expect(store.Set(ctx, "a", []byte("1"), 0)).To(Succeed())
			Expect(store.Set(ctx, "b", []byte("2"), 0)).To(Succeed())
			done := make(chan []string)
			go func() {
				defer GinkgoRecover()
				var seen []string
				for entry, err := range store.MGet(ctx, slices.Values([]string{"a", "b"})) {
					Expect(err).NotTo(HaveOccurred())
					Expect(store.Set(ctx, entry.Key+"-seen", entry.Value, 0)).To(Succeed())
					seen = append(seen, entry.Key)
				}
				done <- seen
			}()

			Eventually(done, time.Second).Should(Receive(Equal([]string{"a", "b"})))
		})

		It("reports an expired key as not found", func() {
			Expect(store.Set(ctx, "k", []byte("v"), 10*time.Millisecond)).To(Succeed())
			time.Sleep(20 * time.Millisecond)
			for entry, err := range store.MGet(ctx, slices.Values([]string{"k"})) {
				Expect(err).NotTo(HaveOccurred())
				Expect(entry).To(Equal(cache.Entry{Key: "k"}))
			}
		})
	})

	Describe("sorted sets", func() {
		// Two members tie at score 3 (c, d) so ordering specs also pin the
		// equal-score tie-break, which the in-memory and valkey backends must share.
		BeforeEach(func() {
			for _, m := range []struct {
				member string
				score  float64
			}{{"a", 1}, {"b", 2}, {"c", 3}, {"d", 3}} {
				Expect(store.ZAdd(ctx, "z", m.score, m.member)).To(Succeed())
			}
		})

		It("orders ZRevRange by score desc, breaking ties on member desc", func() {
			got, err := store.ZRevRange(ctx, "z", 0, -1)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"d", "c", "b", "a"}))
		})

		It("returns ZRangeByScore ascending, inclusive on both ends", func() {
			got, err := store.ZRangeByScore(ctx, "z", cache.Inclusive(2), cache.Inclusive(3))
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"b", "c", "d"}))
		})

		It("honours an exclusive lower bound", func() {
			got, err := store.ZRangeByScore(ctx, "z", cache.Exclusive(2), cache.PosInf)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"c", "d"}))
		})

		It("keeps the edge member when ZRemRangeByScore upper is exclusive", func() {
			// Drop everything strictly below score 2: removes only "a" (score 1).
			Expect(store.ZRemRangeByScore(ctx, "z", cache.NegInf, cache.Exclusive(2))).To(Succeed())
			got, err := store.ZRangeByScore(ctx, "z", cache.NegInf, cache.PosInf)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"b", "c", "d"}))
		})

		It("trims to the newest N with a negative ZRemRangeByRank stop", func() {
			// Keep the newest 2: ZRemRangeByRank(0, -(2+1)).
			Expect(store.ZRemRangeByRank(ctx, "z", 0, -3)).To(Succeed())
			got, err := store.ZRangeByScore(ctx, "z", cache.NegInf, cache.PosInf)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"c", "d"}))
		})

		It("leaves a set under the cap untouched (ZRemRangeByRank no-op)", func() {
			Expect(store.ZRemRangeByRank(ctx, "z", 0, -100)).To(Succeed())
			got, err := store.ZRangeByScore(ctx, "z", cache.NegInf, cache.PosInf)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(4))
		})

		It("promotes a string key to a sorted set, dropping the stale string", func() {
			Expect(store.Set(ctx, "k", []byte("v"), 0)).To(Succeed())
			Expect(store.ZAdd(ctx, "k", 1, "m")).To(Succeed())
			_, err := store.Get(ctx, "k")
			Expect(errors.Is(err, cache.ErrKeyNotFound)).To(BeTrue())
			got, err := store.ZRevRange(ctx, "k", 0, -1)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal([]string{"m"}))
		})

		It("drops the key once its last member is removed", func() {
			for _, m := range []string{"a", "b", "c", "d"} {
				Expect(store.ZRem(ctx, "z", m)).To(Succeed())
			}
			got, err := store.ZRevRange(ctx, "z", 0, -1)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeEmpty())
		})
	})
})
