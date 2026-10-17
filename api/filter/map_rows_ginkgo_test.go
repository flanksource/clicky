package filter

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MapRows", func() {
	It("compares numeric values without string coercion", func() {
		rows := []map[string]any{{"Amount": int64(9)}, {"Amount": int64(12)}}
		filtered, err := MapRows(rows, "Amount > 10")
		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(Equal([]map[string]any{{"Amount": int64(12)}}))
	})

	It("addresses SQL column keys containing spaces through row", func() {
		rows := []map[string]any{{"Total Amount": int64(12)}, {"Total Amount": int64(5)}}
		filtered, err := MapRows(rows, `row["Total Amount"] >= 10`)
		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(Equal([]map[string]any{{"Total Amount": int64(12)}}))
	})

	It("reports missing columns instead of dropping every row", func() {
		_, err := MapRows([]map[string]any{{"Amount": int64(12)}}, "Missing > 10")
		Expect(err).To(HaveOccurred())
	})
})
