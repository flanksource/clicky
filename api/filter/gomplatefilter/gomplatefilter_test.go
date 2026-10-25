package gomplatefilter_test

import (
	"testing"

	"github.com/flanksource/clicky/api/filter"
	"github.com/flanksource/clicky/api/filter/celfilter"
	"github.com/flanksource/clicky/api/filter/gomplatefilter"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("gomplatefilter", func() {
	rows := []map[string]any{{"Amount": int64(12)}, {"Amount": int64(5)}}
	const gomplateOnly = "text(Amount) == '12'"

	ginkgo.It("adds gomplate CEL functions to filter expressions", func() {
		gomplatefilter.Use()
		filtered, err := filter.MapRows(rows, gomplateOnly)
		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(Equal(rows[:1]))
	})

	ginkgo.It("the plain CEL adapter rejects gomplate functions", func() {
		celfilter.Use()
		_, err := filter.MapRows(rows, gomplateOnly)
		Expect(err).To(MatchError(ContainSubstring("undeclared reference to 'text'")))
	})
})

func TestGomplateFilter(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Gomplate Filter Suite")
}
