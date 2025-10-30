package text_test

import (
	"github.com/flanksource/clicky/text"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Built-in Processors", func() {
	Describe("RedactSecrets", func() {
		It("should redact default secret patterns", func() {
			processor := text.RedactSecrets()

			result, skip := processor("password=secret123")
			Expect(skip).To(BeFalse())
			Expect(result).To(ContainSubstring("***"))
			Expect(result).ToNot(ContainSubstring("secret123"))
		})

		It("should redact custom patterns", func() {
			processor := text.RedactSecrets("api.*?=\\S+")

			result, skip := processor("api_key=abc123 other=data")
			Expect(skip).To(BeFalse())
			Expect(result).To(ContainSubstring("***"))
			Expect(result).ToNot(ContainSubstring("abc123"))
		})

		It("should never skip lines", func() {
			processor := text.RedactSecrets()

			_, skip := processor("password=secret")
			Expect(skip).To(BeFalse())

			_, skip = processor("normal line")
			Expect(skip).To(BeFalse())
		})

		It("should return same string if no secrets found", func() {
			processor := text.RedactSecrets()

			input := "normal log line"
			result, skip := processor(input)
			Expect(skip).To(BeFalse())
			Expect(result).To(Equal(input))
		})
	})

	Describe("RegexFilter", func() {
		It("should skip matching lines when invert=false", func() {
			processor := text.RegexFilter("healthcheck", false)

			result, skip := processor("/healthcheck endpoint")
			Expect(skip).To(BeTrue())

			result, skip = processor("normal request")
			Expect(skip).To(BeFalse())
			Expect(result).To(Equal("normal request"))
		})

		It("should skip non-matching lines when invert=true", func() {
			processor := text.RegexFilter("ERROR", true)

			result, skip := processor("ERROR: something went wrong")
			Expect(skip).To(BeFalse())
			Expect(result).To(Equal("ERROR: something went wrong"))

			result, skip = processor("INFO: all good")
			Expect(skip).To(BeTrue())
		})

		It("should return original string reference", func() {
			processor := text.RegexFilter("test", false)

			input := "normal line"
			result, skip := processor(input)
			Expect(skip).To(BeFalse())
			Expect(result).To(BeIdenticalTo(input))
		})
	})

	Describe("AddPrefix", func() {
		It("should add prefix to line", func() {
			processor := text.AddPrefix("[PREFIX] ")

			result, skip := processor("test line")
			Expect(skip).To(BeFalse())
			Expect(result).To(Equal("[PREFIX] test line"))
		})

		It("should never skip lines", func() {
			processor := text.AddPrefix("[PREFIX] ")

			_, skip := processor("test")
			Expect(skip).To(BeFalse())
		})
	})

	Describe("AddSuffix", func() {
		It("should add suffix to line", func() {
			processor := text.AddSuffix(" [SUFFIX]")

			result, skip := processor("test line")
			Expect(skip).To(BeFalse())
			Expect(result).To(Equal("test line [SUFFIX]"))
		})

		It("should never skip lines", func() {
			processor := text.AddSuffix(" [SUFFIX]")

			_, skip := processor("test")
			Expect(skip).To(BeFalse())
		})
	})
})
