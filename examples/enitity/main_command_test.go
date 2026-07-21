package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("main.go", func() {
	It("runs the serve command when passed directly to go run", func() {
		cmd := exec.Command("go", "run", "main.go", "serve", "--help")
		output, err := cmd.CombinedOutput()

		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Usage:"))
		Expect(string(output)).To(ContainSubstring("entity-demo serve"))
	})
})
