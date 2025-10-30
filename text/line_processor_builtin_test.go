package text_test

import (
	"github.com/flanksource/clicky/text"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Built-in Processors", func() {
	Describe("RedactSecrets", func() {
		type testCase struct {
			input            string
			expectedContains string
			expectedSkip     bool
			notContains      string
		}

		DescribeTable("default patterns",
			func(tc testCase) {
				processor := text.RedactSecrets()
				result, skip := processor(tc.input)

				Expect(skip).To(Equal(tc.expectedSkip))
				if tc.expectedContains != "" {
					Expect(result).To(ContainSubstring(tc.expectedContains))
				}
				if tc.notContains != "" {
					Expect(result).ToNot(ContainSubstring(tc.notContains))
				}
			},
			Entry("redacts password", testCase{
				input:            "password=secret123",
				expectedContains: "***",
				notContains:      "secret123",
			}),
			Entry("redacts token", testCase{
				input:            "token=abc123",
				expectedContains: "***",
				notContains:      "abc123",
			}),
			Entry("redacts api_key", testCase{
				input:            "api_key=xyz789",
				expectedContains: "***",
				notContains:      "xyz789",
			}),
			Entry("passes through normal text", testCase{
				input:            "normal log line",
				expectedContains: "normal log line",
			}),
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
