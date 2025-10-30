package text_test

import (
	"github.com/flanksource/clicky/text"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Built-in Processors", func() {
	Describe("RedactSecrets", func() {
		type testCase struct {
			input    string
			expected string
		}

		DescribeTable("default patterns",
			func(tc testCase) {
				processor := text.RedactSecrets()
				result, skip := processor(tc.input)

				Expect(skip).To(BeFalse())
				Expect(result).To(Equal(tc.expected))
			},
			Entry("password with colon", testCase{input: "password: secret", expected: "***"}),
			Entry("token with equals", testCase{input: "token=abc123", expected: "***"}),
			Entry("api_key with equals", testCase{input: "api_key=xyz789", expected: "***"}),
			Entry("PASSWORD uppercase", testCase{input: "PASSWORD=1234", expected: "***"}),
			Entry("auth with colon", testCase{input: "Auth: xyz", expected: "***"}),
			Entry("bearer token", testCase{input: "Bearer: token123", expected: "***"}),
		)

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
		})
	})

	Describe("RegexFilter", func() {
		type testCase struct {
			pattern      string
			invert       bool
			input        string
			expectedSkip bool
		}

		DescribeTable("filtering behavior",
			func(tc testCase) {
				processor := text.RegexFilter(tc.pattern, tc.invert)
				result, skip := processor(tc.input)

				Expect(skip).To(Equal(tc.expectedSkip))
				if !skip {
					Expect(result).To(Equal(tc.input))
				}
			},
			Entry("skip matching line (invert=false)", testCase{
				pattern:      "healthcheck",
				invert:       false,
				input:        "/healthcheck endpoint",
				expectedSkip: true,
			}),
			Entry("keep non-matching line (invert=false)", testCase{
				pattern:      "healthcheck",
				invert:       false,
				input:        "normal request",
				expectedSkip: false,
			}),
			Entry("keep matching line (invert=true)", testCase{
				pattern:      "ERROR",
				invert:       true,
				input:        "ERROR: something went wrong",
				expectedSkip: false,
			}),
			Entry("skip non-matching line (invert=true)", testCase{
				pattern:      "ERROR",
				invert:       true,
				input:        "INFO: all good",
				expectedSkip: true,
			}),
		)
	})

	Describe("AddPrefix", func() {
		type testCase struct {
			prefix   string
			input    string
			expected string
		}

		DescribeTable("adding prefixes",
			func(tc testCase) {
				processor := text.AddPrefix(tc.prefix)
				result, skip := processor(tc.input)

				Expect(skip).To(BeFalse())
				Expect(result).To(Equal(tc.expected))
			},
			Entry("simple prefix", testCase{
				prefix:   "[PREFIX] ",
				input:    "test line",
				expected: "[PREFIX] test line",
			}),
			Entry("timestamp prefix", testCase{
				prefix:   "2024-01-01 ",
				input:    "log message",
				expected: "2024-01-01 log message",
			}),
			Entry("empty input", testCase{
				prefix:   "[PREFIX] ",
				input:    "",
				expected: "[PREFIX] ",
			}),
		)
	})

	Describe("AddSuffix", func() {
		type testCase struct {
			suffix   string
			input    string
			expected string
		}

		DescribeTable("adding suffixes",
			func(tc testCase) {
				processor := text.AddSuffix(tc.suffix)
				result, skip := processor(tc.input)

				Expect(skip).To(BeFalse())
				Expect(result).To(Equal(tc.expected))
			},
			Entry("simple suffix", testCase{
				suffix:   " [SUFFIX]",
				input:    "test line",
				expected: "test line [SUFFIX]",
			}),
			Entry("newline suffix", testCase{
				suffix:   " [END]",
				input:    "log message",
				expected: "log message [END]",
			}),
			Entry("empty input", testCase{
				suffix:   " [SUFFIX]",
				input:    "",
				expected: " [SUFFIX]",
			}),
		)
	})
})
