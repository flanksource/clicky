package clicky_test

import (
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api/filter"
	"github.com/flanksource/clicky/api/filter/celfilter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Format --filter adapter", func() {
	rows := []map[string]any{{"name": "alpha", "amount": int64(12)}, {"name": "beta", "amount": int64(5)}}
	options := clicky.FormatOptions{Format: "json", Filter: "amount > 10"}

	AfterEach(func() {
		filter.SetAdapter(nil)
	})

	It("fails with ErrNoAdapter when the application configured no filter engine", func() {
		_, err := clicky.Format(rows, options)
		Expect(err).To(MatchError(filter.ErrNoAdapter))
	})

	It("filters rows once the CEL adapter is configured", func() {
		celfilter.Use()
		out, err := clicky.Format(rows, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("alpha"))
		Expect(out).NotTo(ContainSubstring("beta"))
	})
})
