package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Heading ANSI styles", func() {
	It("preserves authored colors in heading content", func() {
		heading := Heading{Level: 2, Content: Text{}.Append("Prompt", "text-blue-600")}

		Expect(heading.Content.ANSI()).To(ContainSubstring("38;2;"))
		Expect(heading.ANSI()).To(ContainSubstring(heading.Content.ANSI()))
		Expect(heading.ANSI()).To(HavePrefix("\x1b[1m"))
		Expect(heading.String()).To(Equal("Prompt"))
	})

	It("preserves styles of nested content and keeps String uncolored", func() {
		heading := Heading{Level: 1, Content: Text{}.Append("Prompt ").Add(Text{}.Append("run", "italic text-blue-600"))}

		Expect(heading.Content.ANSI()).To(ContainSubstring("\x1b[3;38;2;"))
		Expect(heading.ANSI()).To(ContainSubstring(heading.Content.ANSI()))
		Expect(heading.String()).To(Equal("Prompt run"))
	})
})
