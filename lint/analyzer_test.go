package lint_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/flanksource/clicky/lint"
)

func TestLint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lint Suite")
}

var _ = Describe("clickylint", func() {
	It("detects bad api.Text usage patterns", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "bad")
		Expect(results).NotTo(BeEmpty())
	})

	It("allows good api.Text usage patterns", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "good")
		Expect(results).NotTo(BeEmpty())
	})

	It("flags direct stdout/stderr writes", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "directstdout/bad")
		Expect(results).NotTo(BeEmpty())
	})

	It("allows legitimate os.Stdin / inspection / buffer writes", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "directstdout/good")
		Expect(results).NotTo(BeEmpty())
	})

	It("honors the //clicky:allow-stdout file-level opt-out", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "directstdout/allowed")
		Expect(results).NotTo(BeEmpty())
	})

	It("skips main packages", func() {
		testdata := analysistest.TestData()
		results := analysistest.Run(GinkgoT(), testdata, lint.Analyzer, "directstdout/mainpkg")
		Expect(results).NotTo(BeEmpty())
	})
})
