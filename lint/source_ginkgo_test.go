package lint_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/lint"
)

var _ = Describe("lint summary source excerpts", func() {
	It("centers the requested total line count on the diagnostic", func() {
		result := &lint.Result{
			Linter:  "clickylint",
			WorkDir: ".",
			Violations: []lint.Violation{{
				File: "testdata/source-window.txt", Line: 3, Column: 4, Rule: "example", Message: "example; details",
			}},
		}
		view := lint.NewSummaryView(result, 5)

		Expect(view.LoadSource(3)).To(Succeed())
		output := sourceLocationText(view)
		Expect(output).To(ContainSubstring("second source line"))
		Expect(output).To(ContainSubstring("third source line"))
		Expect(output).To(ContainSubstring("fourth source line"))
		Expect(output).ToNot(ContainSubstring("first source line"))
		Expect(output).ToNot(ContainSubstring("fifth source line"))
	})

	It("shifts the window at the start of a file", func() {
		result := &lint.Result{
			Linter:  "clickylint",
			WorkDir: ".",
			Violations: []lint.Violation{{
				File: "testdata/source-window.txt", Line: 1, Column: 1, Rule: "example", Message: "example; details",
			}},
		}
		view := lint.NewSummaryView(result, 5)

		Expect(view.LoadSource(3)).To(Succeed())
		output := sourceLocationText(view)
		Expect(output).To(ContainSubstring("first source line"))
		Expect(output).To(ContainSubstring("second source line"))
		Expect(output).To(ContainSubstring("third source line"))
	})

	It("shifts the window before the trailing newline at the end of a file", func() {
		result := &lint.Result{
			Linter:  "clickylint",
			WorkDir: ".",
			Violations: []lint.Violation{{
				File: "testdata/source-window.txt", Line: 5, Column: 1, Rule: "example", Message: "example; details",
			}},
		}
		view := lint.NewSummaryView(result, 5)

		Expect(view.LoadSource(3)).To(Succeed())
		output := sourceLocationText(view)
		Expect(output).To(ContainSubstring("third source line"))
		Expect(output).To(ContainSubstring("fourth source line"))
		Expect(output).To(ContainSubstring("fifth source line"))
		Expect(output).ToNot(ContainSubstring("second source line"))
	})

	It("fails when a diagnostic source cannot be read", func() {
		result := &lint.Result{
			Linter:  "clickylint",
			WorkDir: ".",
			Violations: []lint.Violation{{
				File: "testdata/missing-source.go", Line: 1, Column: 1, Rule: "example", Message: "example; details",
			}},
		}

		Expect(lint.NewSummaryView(result, 5).LoadSource(1)).To(MatchError(ContainSubstring("missing-source.go")))
	})
})

func sourceLocationText(view *lint.SummaryView) string {
	location := view.GetChildren()[0].GetChildren()[0].GetChildren()[0]
	return strings.TrimSpace(location.Pretty().String())
}
