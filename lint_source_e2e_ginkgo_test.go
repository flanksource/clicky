package clicky_test

import (
	"bytes"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("clicky lint source output", func() {
	It("renders the requested total number of source lines", func() {
		dir := writeLintFixtureModule(map[string]string{
			"bad.go": `
package fixture

import "github.com/flanksource/clicky/api"

var sourceBefore = "source-before"
var direct = api.Text{Content: "source-target"}
var sourceAfter = "source-after"
`,
		})

		run := func(sourceLines string) string {
			cmd := exec.Command(testBinaryPath, "lint", "--no-color", "--source", "--source-lines", sourceLines, ".")
			cmd.Dir = dir
			cmd.Env = append(clickyTestEnv(true), "GOWORK=off", "GOFLAGS=-mod=mod")

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			Expect(cmd.Run()).To(Succeed(), "source-rendered warnings should not fail: %s", stderr.String())
			return stdout.String()
		}

		oneLine := run("1")
		Expect(oneLine).To(ContainSubstring(`var direct = api.Text{Content: "source-target"}`))
		Expect(oneLine).To(MatchRegexp(`6.*│.*var direct`))
		Expect(oneLine).To(ContainSubstring("^"))
		Expect(oneLine).ToNot(ContainSubstring("source-before"))
		Expect(oneLine).ToNot(ContainSubstring("source-after"))

		threeLines := run("3")
		Expect(threeLines).To(ContainSubstring("source-before"))
		Expect(threeLines).To(ContainSubstring("source-target"))
		Expect(threeLines).To(ContainSubstring("source-after"))
	})

	It("rejects source modifiers without compatible pretty output", func() {
		for _, args := range [][]string{
			{"lint", "--source-lines=1", "."},
			{"lint", "--source", "--format=json", "."},
			{"lint", "--raw", "--source", "."},
		} {
			cmd := exec.Command(testBinaryPath, args...)
			cmd.Env = clickyTestEnv(true)

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			Expect(cmd.Run()).ToNot(Succeed())
			Expect(stderr.String()).To(ContainSubstring("source"))
		}
	})
})
