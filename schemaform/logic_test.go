package schemaform_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/schemaform"
)

func mustParse(raw string) *schemaform.Schema {
	GinkgoHelper()
	s, err := schemaform.Parse(json.RawMessage(raw))
	Expect(err).NotTo(HaveOccurred())
	return s
}

func fixture(name string) json.RawMessage {
	GinkgoHelper()
	raw, err := os.ReadFile("testdata/" + name)
	Expect(err).NotTo(HaveOccurred())
	return raw
}

var _ = Describe("ResolveControl", func() {
	DescribeTable("picks the clicky-ui control for a property",
		func(prop string, want schemaform.Kind) {
			Expect(schemaform.ResolveControl(mustParse(prop)).Kind).To(Equal(want))
		},
		Entry("lookup wins over enum", `{"type":"string","enum":["a"],"x-clicky-lookup":{"filter":"env"}}`, schemaform.KindLookup),
		Entry("format json", `{"type":"object","format":"json"}`, schemaform.KindJSON),
		Entry("multi-type union", `{"type":["string","number"]}`, schemaform.KindUnion),
		Entry("enum", `{"type":"string","enum":["a","b"]}`, schemaform.KindEnum),
		Entry("enum in an anyOf branch", `{"anyOf":[{"enum":["a","b"]},{"type":"string","pattern":"^\\$\\{"}]}`, schemaform.KindEnum),
		Entry("template-or-integer renders as a number", `{"anyOf":[{"type":"string","pattern":"^\\$\\{"},{"type":"integer"}]}`, schemaform.KindNumber),
		Entry("date", `{"type":"string","format":"date"}`, schemaform.KindDate),
		Entry("date-time", `{"type":"string","format":"date-time"}`, schemaform.KindDate),
		Entry("textarea", `{"type":"string","format":"textarea"}`, schemaform.KindTextarea),
		Entry("markdown", `{"type":"string","format":"md"}`, schemaform.KindMarkdown),
		Entry("boolean", `{"type":"boolean"}`, schemaform.KindBoolean),
		Entry("integer", `{"type":"integer"}`, schemaform.KindNumber),
		Entry("array of strings is a tag list", `{"type":"array","items":{"type":"string"}}`, schemaform.KindTags),
		Entry("array of enum items is a multi choice", `{"type":"array","items":{"type":"string","enum":["a","b"]}}`, schemaform.KindMultiEnum),
		Entry("array of objects", `{"type":"array","items":{"type":"object","properties":{"a":{"type":"string"}}}}`, schemaform.KindObjectList),
		Entry("typed map", `{"type":"object","additionalProperties":{"type":"string"}}`, schemaform.KindStringMap),
		Entry("fixed object", `{"type":"object","properties":{"a":{"type":"string"}}}`, schemaform.KindObject),
		Entry("bare object is an open map", `{"type":"object"}`, schemaform.KindStringMap),
		Entry("untyped falls back to string", `{}`, schemaform.KindString),
	)

	It("labels enum options as Label (value)", func() {
		control := schemaform.ResolveControl(mustParse(`{"type":"string","enum":["01","02"],"x-enum-labels":{"01":"Add"}}`))
		Expect(control.Options).To(Equal([]schemaform.Option{
			{Value: "01", Label: "Add (01)", Raw: "01"},
			{Value: "02", Label: "02", Raw: "02"},
		}))
	})
})

var _ = Describe("Parse", func() {
	It("keeps properties in document order", func() {
		s := mustParse(`{"type":"object","properties":{"zeta":{},"alpha":{},"mid":{}}}`)
		Expect(s.PropertyOrder()).To(Equal([]string{"zeta", "alpha", "mid"}))
	})

	It("inlines a local $ref, letting sibling keywords win", func() {
		s := mustParse(`{
			"$defs": {"env": {"type": "string", "title": "Environment", "enum": ["dev", "prod"]}},
			"type": "object",
			"properties": {"target": {"$ref": "#/$defs/env", "title": "Target"}}
		}`)
		target := s.Properties["target"]
		Expect(*target.Title).To(Equal("Target"))
		Expect(target.Enum).To(Equal([]any{"dev", "prod"}))
	})

	It("breaks a recursive $ref with a JSON editor stub", func() {
		s := mustParse(`{
			"$defs": {"node": {"type": "object", "properties": {"child": {"$ref": "#/$defs/node"}}}},
			"$ref": "#/$defs/node"
		}`)
		Expect(schemaform.ResolveControl(s.Properties["child"]).Kind).To(Equal(schemaform.KindJSON))
	})

	It("fails loudly on a pointer that resolves to nothing", func() {
		_, err := schemaform.Parse(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"#/$defs/missing"}}}`))
		Expect(err).To(MatchError(ContainSubstring(`segment "$defs" not found`)))
	})
})

var _ = Describe("Effective", func() {
	It("merges the then branch when the if matches and the else branch otherwise", func() {
		s, err := schemaform.Parse(fixture("conditionals.json"))
		Expect(err).NotTo(HaveOccurred())

		adding := schemaform.Effective(s, map[string]any{"MemberClassGroupOption": "01"})
		Expect(adding.Map["NewMemberClassGroup"].Hidden).To(BeFalse())
		Expect(adding.Map["CurrentMemberClassGroup"].Hidden).To(BeTrue())
		Expect(adding.Required).To(ConsistOf("MemberClassGroupOption", "NewMemberClassGroup"))

		updating := schemaform.Effective(s, map[string]any{"MemberClassGroupOption": "02"})
		Expect(updating.Map["CurrentMemberClassGroup"].Hidden).To(BeFalse())
		Expect(updating.Map["ChangeReason"].Hidden).To(BeFalse())
		Expect(updating.Required).To(ConsistOf("MemberClassGroupOption", "CurrentMemberClassGroup"))
	})
})

var _ = Describe("ApplyDefaults", func() {
	It("lets a defaulted discriminator activate the defaults of its branch, keeping existing values", func() {
		s := mustParse(`{
			"type": "object",
			"properties": {
				"kind": {"type": "string", "enum": ["cron", "event"], "default": "cron"},
				"note": {"type": "string", "default": "nightly"},
				"retry": {"type": "object", "properties": {"limit": {"type": "integer", "default": 3}}}
			},
			"allOf": [{
				"if": {"properties": {"kind": {"const": "cron"}}, "required": ["kind"]},
				"then": {"properties": {"schedule": {"type": "string", "default": "@daily"}}}
			}]
		}`)
		Expect(schemaform.ApplyDefaults(s, map[string]any{"note": ""})).To(Equal(map[string]any{
			"kind":     "cron",
			"note":     "",
			"schedule": "@daily",
			"retry":    map[string]any{"limit": float64(3)},
		}))
	})
})
