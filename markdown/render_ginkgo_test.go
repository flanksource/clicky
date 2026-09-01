package markdown_test

import (
	"strings"

	"github.com/flanksource/clicky/markdown"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("terminal Markdown rendering", func() {
	const richMarkdown = `# Database sizes

Use **pg_database** with *live* sizes and ~~stale rows~~.

> Run against a replica.

!!! warning Production
    Review the target first.

- [x] Connect
- [ ] Verify
  - Nested check

1. Measure
2. Sort

| Database | Size |
| :--- | ---: |
| app | 42 GB |

` + "```sql\n" + `SELECT datname, pg_database_size(datname)
FROM pg_database
ORDER BY 2 DESC;
` + "```\n\n```unknown-language\nplain <value>\n```\n"

	It("preserves document structure without Markdown source delimiters in plain text", func() {
		doc, err := markdown.ParseString(richMarkdown)
		Expect(err).NotTo(HaveOccurred())

		plain := doc.String()
		Expect(plain).To(And(
			ContainSubstring("Database sizes"),
			ContainSubstring("> Run against a replica."),
			ContainSubstring("!!! warning Production"),
			ContainSubstring("- [x] Connect"),
			ContainSubstring("- [ ] Verify"),
			ContainSubstring("  - Nested check"),
			ContainSubstring("1. Measure"),
			ContainSubstring("Database"),
			ContainSubstring("42 GB"),
			ContainSubstring("SELECT datname"),
			ContainSubstring("plain <value>"),
		))
		Expect(plain).NotTo(ContainSubstring("```"))
		Expect(plain).NotTo(ContainSubstring("\x1b["))
	})

	It("styles prose and syntax-highlights known fenced languages in ANSI output", func() {
		doc, err := markdown.ParseString(richMarkdown)
		Expect(err).NotTo(HaveOccurred())

		ansi := doc.ANSI()
		Expect(ansi).To(And(
			ContainSubstring("Database sizes"),
			ContainSubstring("SELECT"),
			ContainSubstring("plain <value>"),
			ContainSubstring("\x1b["),
		))
		Expect(ansi).NotTo(ContainSubstring("```"))
	})

	It("keeps canonical Markdown and semantic HTML renderers intact", func() {
		doc, err := markdown.ParseString(richMarkdown)
		Expect(err).NotTo(HaveOccurred())

		Expect(doc.Markdown()).To(And(
			ContainSubstring("# Database sizes"),
			ContainSubstring("```sql"),
			ContainSubstring("| Database | Size |"),
		))
		Expect(doc.HTML()).To(And(
			ContainSubstring("<h1>Database sizes</h1>"),
			ContainSubstring("<table>"),
			ContainSubstring("admonition-warning"),
		))
	})

	It("renders empty and plain documents without decoration", func() {
		empty, err := markdown.ParseString("")
		Expect(err).NotTo(HaveOccurred())
		Expect(empty.String()).To(BeEmpty())
		Expect(empty.ANSI()).To(BeEmpty())

		plain, err := markdown.ParseString("A plain response.\n")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(plain.String())).To(Equal("A plain response."))
		Expect(strings.TrimSpace(plain.ANSI())).To(Equal("A plain response."))
	})
})
