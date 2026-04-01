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
})
