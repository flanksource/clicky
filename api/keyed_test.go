package api

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Keyed", func() {
	It("renders identically to the wrapped value in every format", func() {
		inner := Text{Content: "hello", Style: "font-bold"}
		k := Keyed{Key: "greeting", Value: inner}
		Expect(k.String()).To(Equal(inner.String()))
		Expect(k.ANSI()).To(Equal(inner.ANSI()))
		Expect(k.HTML()).To(Equal(inner.HTML()))
		Expect(k.Markdown()).To(Equal(inner.Markdown()))
	})

	It("marshals JSON as a single-field object keyed by Key", func() {
		k := Keyed{Key: "greeting", Value: Text{Content: "hi"}}
		raw, err := json.Marshal(k)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(raw)).To(Equal(`{"greeting":"hi"}`))
	})

	It("delegates JSON to a wrapped value that marshals itself", func() {
		table := TextTable{
			Headers:    TextList{Text{Content: "A"}},
			FieldNames: []string{"a"},
			Rows:       []TableRow{{"a": TypedValue{Textable: Text{Content: "1"}}}},
		}
		k := Keyed{Key: "rows", Value: table}
		raw, err := json.Marshal(k)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring(`"rows":[`))
		Expect(string(raw)).To(ContainSubstring(`"a":"1"`))
	})
})
