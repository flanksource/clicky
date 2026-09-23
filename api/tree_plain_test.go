//go:build !wasm

package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func plainTestNode(label string, children ...TextTree) TextTree {
	return TextTree{Node: Text{Content: label}, Children: children}
}

var plainTreeFixtures = map[string]TextTree{
	"single node": plainTestNode("root"),
	"nested children": plainTestNode("root",
		plainTestNode("child-a", plainTestNode("leaf")),
		plainTestNode("child-b")),
	"rootless with several children": {Children: []TextTree{
		plainTestNode("one", plainTestNode("one-a")),
		plainTestNode("two"),
	}},
	"rootless with one child collapses into the child": {Children: []TextTree{
		plainTestNode("only", plainTestNode("leaf")),
	}},
	"multi-line labels with blank lines": plainTestNode("title\n\nsubtitle",
		plainTestNode("first\nsecond", plainTestNode("deep\n  indented")),
		plainTestNode("last\nline")),
	"tabbed labels": plainTestNode("key\tvalue",
		plainTestNode("a\tb\nc\td")),
	"unicode labels": plainTestNode("日本",
		plainTestNode("βeta", plainTestNode("é"))),
	"nodeless child keeps its connector": plainTestNode("root",
		TextTree{Children: []TextTree{plainTestNode("a"), plainTestNode("b")}},
		plainTestNode("after")),
	"styled labels": {
		Node: Text{Content: "root", Style: "text-blue-600 font-bold"},
		Children: []TextTree{
			{Node: Text{Content: "red\nwrapped", Style: "text-red-600"}},
			{Node: Text{Content: "green", Style: "text-green-500"}},
		},
	},
}

var _ = Describe("plainTree", func() {
	It("renders nothing for an empty tree", func() {
		Expect(TextTree{}.plainTree(false)).To(BeEmpty())
		Expect(TextTree{}.plainTree(true)).To(BeEmpty())
	})

	It("draws connectors with continuation gutters under multi-line labels", func() {
		tree := plainTestNode("root",
			plainTestNode("first\nsecond", plainTestNode("leaf")),
			plainTestNode("last\nline"))

		Expect(tree.plainTree(false)).To(Equal(plainLines(
			"root",
			"├── first",
			"│   second",
			"│   ╰── leaf",
			"╰── last",
			"    line",
		)))
	})

	It("keeps each label's ANSI styling", func() {
		Expect(plainTreeFixtures["styled labels"].plainTree(true)).To(ContainSubstring("\x1b["))
	})

	for name, tree := range plainTreeFixtures {
		It("matches the native plain render for "+name, func() {
			Expect(tree.plainTree(false)).To(Equal(tree.String()))
		})
		It("matches the native ANSI render for "+name, func() {
			Expect(tree.plainTree(true)).To(Equal(tree.ANSI()))
		})
	}
})
