package valkey_test

import (
	"context"

	"github.com/alicebob/miniredis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	valkeygo "github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
	"github.com/flanksource/clicky/valkey"
)

var _ = Describe("Valkey cache Browser", func() {
	var (
		browser cache.Browser
		seed    func(cfg valkey.BrowserConfig)
		ctx     = context.Background()
		mr      *miniredis.Miniredis
	)

	// seed loads a small namespaced keyspace:
	//
	//	app:tx:1       string
	//	app:tx:2:meta  string
	//	app:plan:p1    hash
	//	app:queue      list
	//	app:tags       set
	//	app:scores     zset
	//	app:marker     string (JSON)
	//	other:noise    string outside the namespace
	BeforeEach(func() {
		var client valkeygo.Client
		client, mr = newClient()
		DeferCleanup(client.Close)
		DeferCleanup(mr.Close)
		mr.Set("app:tx:1", "one")
		mr.Set("app:tx:2:meta", "meta-value")
		mr.HSet("app:plan:p1", "name", "Plan One", "tier", "gold")
		mr.Lpush("app:queue", "second")
		mr.Lpush("app:queue", "first")
		mr.SAdd("app:tags", "beta", "alpha")
		mr.ZAdd("app:scores", 2.5, "bob")
		mr.ZAdd("app:scores", 1.5, "alice")
		mr.Set("app:marker", `{"hello":"world"}`)
		mr.Set("other:noise", "x")
		seed = func(cfg valkey.BrowserConfig) {
			cfg.KeyPrefix = "app:"
			browser = valkey.NewBrowser(client, cfg)
		}
		seed(valkey.BrowserConfig{})
	})

	It("groups the root level into prefix groups and sorted leaves", func() {
		resp, err := browser.Tree(ctx, cache.TreeRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Keys).To(Equal(7))
		Expect(resp.Truncated).To(BeFalse())
		Expect(resp.BytesSupported).To(BeTrue())
		for _, node := range resp.Nodes {
			if node.Key != "" {
				Expect(node.Bytes).To(BeNumerically(">", 0), "leaf %s should carry MEMORY USAGE", node.Key)
			} else {
				Expect(node.Bytes).To(BeNumerically(">", 0), "group %s should aggregate subkey MEMORY USAGE", node.Name)
			}
		}
		// Bytes values are miniredis-internal overhead constants; compare
		// everything else exactly.
		Expect(clearBytes(resp.Nodes)).To(Equal([]cache.TreeNode{
			{Name: "plan", Prefix: "plan:", Keys: 1, Children: 1},
			{Name: "tx", Prefix: "tx:", Keys: 2, Children: 2},
			{Name: "marker", Key: "marker", Keys: 1, Type: "string", TTLSeconds: -1},
			{Name: "queue", Key: "queue", Keys: 1, Type: "list", TTLSeconds: -1},
			{Name: "scores", Key: "scores", Keys: 1, Type: "zset", TTLSeconds: -1},
			{Name: "tags", Key: "tags", Keys: 1, Type: "set", TTLSeconds: -1},
		}))
	})

	It("aggregates every subkey's MEMORY USAGE into the group node", func() {
		root, err := browser.Tree(ctx, cache.TreeRequest{})
		Expect(err).NotTo(HaveOccurred())
		var txGroup int64
		for _, node := range root.Nodes {
			if node.Name == "tx" && node.Key == "" {
				txGroup = node.Bytes
			}
		}

		// tx: holds tx:1 (leaf) and tx:2:meta (under the tx:2 subgroup); the
		// group total must equal the sum of both, not just its direct leaf.
		tx1, err := browser.Key(ctx, "tx:1")
		Expect(err).NotTo(HaveOccurred())
		tx2meta, err := browser.Key(ctx, "tx:2:meta")
		Expect(err).NotTo(HaveOccurred())
		Expect(txGroup).To(Equal(tx1.Bytes + tx2meta.Bytes))
	})

	It("expands a group prefix into its own level", func() {
		resp, err := browser.Tree(ctx, cache.TreeRequest{Prefix: "tx:"})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Keys).To(Equal(2))
		Expect(clearBytes(resp.Nodes)).To(Equal([]cache.TreeNode{
			{Name: "2", Prefix: "tx:2:", Keys: 1, Children: 1},
			{Name: "1", Key: "tx:1", Keys: 1, Type: "string", TTLSeconds: -1},
		}))
	})

	It("caps a level at MaxChildren and flags truncation", func() {
		resp, err := browser.Tree(ctx, cache.TreeRequest{MaxChildren: 3})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Nodes).To(HaveLen(3))
		Expect(resp.Truncated).To(BeTrue())
		Expect(resp.Keys).To(Equal(7))
	})

	It("returns string key detail", func() {
		detail, err := browser.Key(ctx, "marker")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Bytes).To(BeNumerically(">", 0))
		detail.Bytes = 0
		Expect(detail).To(Equal(cache.KeyDetail{
			Key:        "marker",
			Type:       "string",
			TTLSeconds: -1,
			Length:     17,
			Value:      `{"hello":"world"}`,
		}))
	})

	It("caps long string values and reports the full length", func() {
		seed(valkey.BrowserConfig{MaxValueBytes: 4})
		detail, err := browser.Key(ctx, "tx:2:meta")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Value).To(Equal("meta"))
		Expect(detail.Length).To(BeEquivalentTo(len("meta-value")))
		Expect(detail.Truncated).To(BeTrue())
	})

	It("returns hash fields", func() {
		detail, err := browser.Key(ctx, "plan:p1")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Type).To(Equal("hash"))
		Expect(detail.Length).To(BeEquivalentTo(2))
		Expect(detail.Fields).To(Equal(map[string]string{"name": "Plan One", "tier": "gold"}))
	})

	It("returns list items in list order", func() {
		detail, err := browser.Key(ctx, "queue")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Type).To(Equal("list"))
		Expect(detail.Items).To(Equal([]string{"first", "second"}))
	})

	It("returns sorted set members", func() {
		detail, err := browser.Key(ctx, "tags")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Type).To(Equal("set"))
		Expect(detail.Items).To(Equal([]string{"alpha", "beta"}))
	})

	It("returns zset members with scores in rank order", func() {
		detail, err := browser.Key(ctx, "scores")
		Expect(err).NotTo(HaveOccurred())
		Expect(detail.Type).To(Equal("zset"))
		Expect(detail.Members).To(Equal([]cache.ScoredMember{
			{Member: "alice", Score: 1.5},
			{Member: "bob", Score: 2.5},
		}))
	})

	It("returns ErrKeyNotFound for a missing key", func() {
		_, err := browser.Key(ctx, "missing")
		Expect(err).To(MatchError(cache.ErrKeyNotFound))
	})

	It("searches by substring within the namespace only", func() {
		resp, err := browser.Search(ctx, cache.SearchRequest{Query: "tx"})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Truncated).To(BeFalse())
		Expect(keysOf(resp.Keys)).To(Equal([]string{"tx:1", "tx:2:meta"}))
	})

	It("flags truncated search results", func() {
		resp, err := browser.Search(ctx, cache.SearchRequest{Query: "tx", Limit: 1})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Keys).To(HaveLen(1))
		Expect(resp.Truncated).To(BeTrue())
	})

	It("clamps an unset search limit to MaxChildren", func() {
		seed(valkey.BrowserConfig{MaxChildren: 1})
		resp, err := browser.Search(ctx, cache.SearchRequest{Query: "tx"})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Keys).To(HaveLen(1))
		Expect(resp.Truncated).To(BeTrue())
	})

	It("deletes a single key", func() {
		resp, err := browser.DeleteKey(ctx, "marker")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Deleted).To(BeEquivalentTo(1))
		Expect(mr.Exists("app:marker")).To(BeFalse())
	})

	It("deletes a whole prefix without touching siblings", func() {
		resp, err := browser.DeletePrefix(ctx, "tx:")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Deleted).To(BeEquivalentTo(2))
		Expect(mr.Exists("app:tx:1")).To(BeFalse())
		Expect(mr.Exists("app:tx:2:meta")).To(BeFalse())
		Expect(mr.Exists("app:plan:p1")).To(BeTrue())
		Expect(mr.Exists("other:noise")).To(BeTrue())
	})

	It("fails fast instead of partially deleting when the scan is truncated", func() {
		seed(valkey.BrowserConfig{MaxScan: 1})
		_, err := browser.DeletePrefix(ctx, "tx:")
		Expect(err).To(MatchError(ContainSubstring("truncated at maxScan=1")))
		Expect(mr.Exists("app:tx:1")).To(BeTrue())
		Expect(mr.Exists("app:tx:2:meta")).To(BeTrue())
	})

	It("counts only namespaced keys in stats", func() {
		stats, err := browser.Stats(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(stats.Keys).To(BeEquivalentTo(7))
		Expect(stats.KeysTruncated).To(BeFalse())
	})
})

func clearBytes(nodes []cache.TreeNode) []cache.TreeNode {
	out := make([]cache.TreeNode, len(nodes))
	for i, n := range nodes {
		n.Bytes = 0
		out[i] = n
	}
	return out
}

func keysOf(nodes []cache.TreeNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Key)
	}
	return out
}
