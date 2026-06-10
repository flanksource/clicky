package cache_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/cache"
)

// fakeBrowser records the last request it saw and replies with canned data so
// the specs can assert parameter plumbing and JSON encoding without a real
// backend.
type fakeBrowser struct {
	lastTree   cache.TreeRequest
	lastKey    string
	lastSearch cache.SearchRequest
	lastDelete string
	keyErr     error
}

func (f *fakeBrowser) Tree(_ context.Context, req cache.TreeRequest) (cache.TreeResponse, error) {
	f.lastTree = req
	return cache.TreeResponse{
		Prefix: req.Prefix,
		Nodes: []cache.TreeNode{
			{Name: "tx", Prefix: "tx:", Keys: 12, Children: 12},
			{Name: "marker", Key: "marker", Keys: 1, Type: "string", TTLSeconds: -1},
		},
		Keys:           13,
		BytesSupported: true,
	}, nil
}

func (f *fakeBrowser) Key(_ context.Context, key string) (cache.KeyDetail, error) {
	f.lastKey = key
	if f.keyErr != nil {
		return cache.KeyDetail{}, f.keyErr
	}
	return cache.KeyDetail{Key: key, Type: "string", TTLSeconds: -1, Length: 5, Value: "hello"}, nil
}

func (f *fakeBrowser) Search(_ context.Context, req cache.SearchRequest) (cache.SearchResponse, error) {
	f.lastSearch = req
	return cache.SearchResponse{Keys: []cache.TreeNode{{Name: "tx:1", Key: "tx:1", Keys: 1, Type: "string"}}}, nil
}

func (f *fakeBrowser) Stats(context.Context) (cache.Stats, error) {
	return cache.Stats{Keys: 13, Version: "7.2.0"}, nil
}

func (f *fakeBrowser) DeleteKey(_ context.Context, key string) (cache.DeleteResponse, error) {
	f.lastDelete = key
	return cache.DeleteResponse{Deleted: 1}, nil
}

func (f *fakeBrowser) DeletePrefix(_ context.Context, prefix string) (cache.DeleteResponse, error) {
	f.lastDelete = prefix
	return cache.DeleteResponse{Deleted: 12}, nil
}

var _ = Describe("Cache HTTP handler", func() {
	var (
		browser *fakeBrowser
		server  *httptest.Server
	)

	BeforeEach(func() {
		browser = &fakeBrowser{}
		mux := http.NewServeMux()
		cache.RegisterRoutes(mux, browser, "/api/v1")
		server = httptest.NewServer(mux)
		DeferCleanup(server.Close)
	})

	get := func(path string) (*http.Response, map[string]any) {
		GinkgoHelper()
		resp, err := http.Get(server.URL + path)
		Expect(err).NotTo(HaveOccurred())
		var body map[string]any
		if resp.Header.Get("Content-Type") == "application/json" {
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		}
		resp.Body.Close()
		return resp, body
	}

	del := func(path string) (*http.Response, map[string]any) {
		GinkgoHelper()
		req, err := http.NewRequest(http.MethodDelete, server.URL+path, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		var body map[string]any
		if resp.Header.Get("Content-Type") == "application/json" {
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		}
		resp.Body.Close()
		return resp, body
	}

	It("serves a tree level and threads prefix and max through", func() {
		resp, body := get("/api/v1/cache/tree?prefix=tx%3A&max=50")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(browser.lastTree).To(Equal(cache.TreeRequest{Prefix: "tx:", MaxChildren: 50}))
		Expect(body["keys"]).To(BeEquivalentTo(13))
		Expect(body["bytesSupported"]).To(BeTrue())
		Expect(body["nodes"]).To(HaveLen(2))
	})

	It("rejects a non-numeric max", func() {
		resp, _ := get("/api/v1/cache/tree?max=lots")
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("serves key detail with the key taken from the query", func() {
		resp, body := get("/api/v1/cache/key?key=tx%3Aabc%2Fdef")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(browser.lastKey).To(Equal("tx:abc/def"))
		Expect(body["value"]).To(Equal("hello"))
	})

	It("maps ErrKeyNotFound to 404", func() {
		browser.keyErr = cache.ErrKeyNotFound
		resp, _ := get("/api/v1/cache/key?key=missing")
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("requires key on the key endpoint", func() {
		resp, _ := get("/api/v1/cache/key")
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("serves search results", func() {
		resp, body := get("/api/v1/cache/search?q=tx&limit=10")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(browser.lastSearch).To(Equal(cache.SearchRequest{Query: "tx", Limit: 10}))
		Expect(body["keys"]).To(HaveLen(1))
	})

	It("requires q on the search endpoint", func() {
		resp, _ := get("/api/v1/cache/search")
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("serves stats", func() {
		resp, body := get("/api/v1/cache/stats")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(body["keys"]).To(BeEquivalentTo(13))
		Expect(body["version"]).To(Equal("7.2.0"))
	})

	It("deletes a key", func() {
		resp, body := del("/api/v1/cache/key?key=tx%3A1")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(browser.lastDelete).To(Equal("tx:1"))
		Expect(body["deleted"]).To(BeEquivalentTo(1))
	})

	It("deletes a prefix but refuses an empty one", func() {
		resp, body := del("/api/v1/cache/prefix?prefix=tx%3A")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(body["deleted"]).To(BeEquivalentTo(12))

		resp, _ = del("/api/v1/cache/prefix")
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})
})
