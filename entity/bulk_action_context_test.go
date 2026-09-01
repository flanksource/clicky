package entity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("context-aware bulk actions", func() {
	type selection struct {
		Database string `flag:"database"`
	}

	It("passes request context to selected and filtered handlers", func() {
		type contextKey string
		const key contextKey = "request"
		ctx := context.WithValue(context.Background(), key, "tenant-x")
		var selectedContext, filteredContext string

		action := BulkActionWithFilterAndContext(
			"evict",
			func(ctx context.Context, ids []string, _ map[string]string) ([]string, error) {
				selectedContext, _ = ctx.Value(key).(string)
				return ids, nil
			},
			func(ctx context.Context, opts selection, _ map[string]string) ([]string, error) {
				filteredContext, _ = ctx.Value(key).(string)
				return []string{opts.Database}, nil
			},
		)
		info := action.bulkActionInfo(func(flags map[string]string) (any, error) {
			return selection{Database: flags["database"]}, nil
		})

		selected, err := info.ContextDataFunc(ctx, nil, []string{"0x01"})
		Expect(err).NotTo(HaveOccurred())
		Expect(selected).To(Equal([]string{"0x01"}))
		Expect(selectedContext).To(Equal("tenant-x"))

		filtered, err := info.ContextFilterFunc(ctx, map[string]string{"database": "acme"}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(Equal([]string{"acme"}))
		Expect(filteredContext).To(Equal("tenant-x"))
	})
})
