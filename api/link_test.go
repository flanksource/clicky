package api

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// shortAndLong implements both PrettyShort and Pretty; cells must pick the short form.
type shortAndLong struct{}

func (shortAndLong) PrettyShort() Textable { return Text{Content: "short"} }
func (shortAndLong) Pretty() Text          { return Text{Content: "long"} }

// longOnly implements only Pretty; cells fall back to it.
type longOnly struct{}

func (longOnly) Pretty() Text { return Text{Content: "long"} }

var _ = Describe("Link", func() {
	Describe("MarshalJSON", func() {
		It("emits a structured object with href, text, and payload", func() {
			link := NewLink("/entity/plan/abc").
				Text("Life Plan").
				WithJSON(map[string]any{"id": "abc", "name": "Life Plan", "kind": "plan"})

			raw, err := json.Marshal(link)
			Expect(err).NotTo(HaveOccurred())

			var shape map[string]any
			Expect(json.Unmarshal(raw, &shape)).To(Succeed())
			Expect(shape["href"]).To(Equal("/entity/plan/abc"))
			Expect(shape["text"]).To(Equal("Life Plan"))
			payload, ok := shape["json"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(payload["id"]).To(Equal("abc"))
			Expect(payload["name"]).To(Equal("Life Plan"))
			Expect(payload["kind"]).To(Equal("plan"))
		})

		It("omits href, target, tooltip, and json when unset", func() {
			raw, err := json.Marshal(NewLink("").Text("plain"))
			Expect(err).NotTo(HaveOccurred())

			var shape map[string]any
			Expect(json.Unmarshal(raw, &shape)).To(Succeed())
			Expect(shape).To(HaveKeyWithValue("text", "plain"))
			Expect(shape).NotTo(HaveKey("href"))
			Expect(shape).NotTo(HaveKey("target"))
			Expect(shape).NotTo(HaveKey("tooltip"))
			Expect(shape).NotTo(HaveKey("json"))
		})

		It("emits the tooltip when one is set", func() {
			link := NewLink("/x").Text("name").WithTooltip(Text{Content: "the-guid"})
			raw, err := json.Marshal(link)
			Expect(err).NotTo(HaveOccurred())

			var shape map[string]any
			Expect(json.Unmarshal(raw, &shape)).To(Succeed())
			Expect(shape["tooltip"]).To(Equal("the-guid"))
		})
	})

	Describe("PrettyShort cell preference", func() {
		It("prefers PrettyShort over Pretty for a cell value", func() {
			Expect(convertToTextable(shortAndLong{}).String()).To(Equal("short"))
		})

		It("falls back to Pretty when PrettyShort is not implemented", func() {
			Expect(convertToTextable(longOnly{}).String()).To(Equal("long"))
		})
	})

	Describe("text rendering is unaffected by the payload", func() {
		It("renders String/Markdown/HTML the same with or without JSON", func() {
			base := NewLink("/x").Text("name")
			withJSON := base.WithJSON(map[string]any{"id": "1"})

			Expect(withJSON.String()).To(Equal(base.String()))
			Expect(withJSON.Markdown()).To(Equal(base.Markdown()))
			Expect(withJSON.HTML()).To(Equal(base.HTML()))
		})
	})
})
