package exec

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shell detection", func() {
	DescribeTable("detects shell operators",
		func(command string, expected bool) {
			Expect(ContainsShellOperators(command)).To(Equal(expected))
		},
		Entry("pipe", "echo foo | grep bar", true),
		Entry("output redirect", "echo foo > file.txt", true),
		Entry("input redirect", "cat < file.txt", true),
		Entry("stderr redirect", "command 2> error.log", true),
		Entry("and", "command1 && command2", true),
		Entry("or", "command1 || command2", true),
		Entry("semicolon", "command1; command2", true),
		Entry("backticks", "echo `date`", true),
		Entry("substitution", "echo $(date)", true),
		Entry("simple", "echo hello world", false),
		Entry("arguments", "/usr/bin/command --option=value", false),
	)

	It("runs pipes", func() {
		process := NewExec("echo hello | tr h H").Run()
		Expect(process.IsOK()).To(BeTrue())
		Expect(process.GetStdout()).To(Equal("Hello\n"))
	})

	It("runs redirects", func() {
		process := NewExec("echo error >&2 | cat").Run()
		Expect(process.IsOK()).To(BeTrue())
		Expect(process.GetStderr()).To(ContainSubstring("error"))
	})

	It("runs and expressions", func() {
		process := NewExec("echo first && echo second").Run()
		Expect(process.IsOK()).To(BeTrue())
		Expect(process.GetStdout()).To(Equal("first\nsecond\n"))
	})

	It("runs or expressions", func() {
		process := NewExec("false || echo fallback").Run()
		Expect(process.IsOK()).To(BeTrue())
		Expect(process.GetStdout()).To(Equal("fallback\n"))
	})

	It("runs command substitutions", func() {
		process := NewExec("echo result: $(echo nested)").Run()
		Expect(process.IsOK()).To(BeTrue())
		Expect(process.GetStdout()).To(Equal("result: nested\n"))
	})
})
