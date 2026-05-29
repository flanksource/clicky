package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Severity", func() {
	It("round-trips known severity words through ParseSeverity/String", func() {
		for word, sev := range map[string]Severity{
			"note":     SeverityNote,
			"info":     SeverityInfo,
			"tip":      SeverityTip,
			"warning":  SeverityWarning,
			"danger":   SeverityDanger,
			"error":    SeverityDanger,
			"critical": SeverityDanger,
		} {
			Expect(ParseSeverity(word)).To(Equal(sev), "ParseSeverity(%q)", word)
		}
		Expect(SeverityWarning.String()).To(Equal("warning"))
	})

	It("falls back to note for unknown words", func() {
		Expect(ParseSeverity("nonsense")).To(Equal(SeverityNote))
	})
})

var _ = Describe("Admonition", func() {
	build := func() Admonition {
		return Admonition{
			Severity: SeverityWarning,
			Title:    Text{Content: "Net income changed sign"},
			Body:     Text{Content: "review the statement"},
		}
	}

	It("renders the !!! header with severity and title in text formats", func() {
		a := build()
		Expect(a.String()).To(HavePrefix("!!! warning Net income changed sign"))
		Expect(a.Markdown()).To(HavePrefix("!!! warning Net income changed sign"))
		Expect(a.ANSI()).To(ContainSubstring("Net income changed sign"))
	})

	It("indents the body four spaces in text and markdown", func() {
		a := build()
		Expect(a.String()).To(ContainSubstring("    review the statement"))
		Expect(a.Markdown()).To(ContainSubstring("    review the statement"))
	})

	It("renders an admonition div keyed by severity in HTML", func() {
		a := build()
		Expect(a.HTML()).To(ContainSubstring(`class="admonition admonition-warning"`))
		Expect(a.HTML()).To(ContainSubstring("Net income changed sign"))
	})

	It("renders header only when body is nil", func() {
		a := Admonition{Severity: SeverityNote, Title: Text{Content: "heads up"}}
		Expect(a.String()).To(Equal("!!! note heads up"))
	})
})
