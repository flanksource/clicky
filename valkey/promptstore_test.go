package valkey_test

import (
	"time"

	"github.com/alicebob/miniredis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	valkeygo "github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/prompt"
	"github.com/flanksource/clicky/valkey"
)

var _ = Describe("Valkey PromptStore", func() {
	var (
		client valkeygo.Client
		mr     *miniredis.Miniredis
		store  prompt.Store
	)

	BeforeEach(func() {
		client, mr = newClient()
		store = valkey.NewPromptStore(client, valkey.PromptStoreConfig{KeyPrefix: "app:", Retention: time.Hour})
	})

	AfterEach(func() {
		client.Close()
		mr.Close()
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
