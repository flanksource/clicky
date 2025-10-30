package text_test

import (
	"github.com/flanksource/clicky/api"
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

		var sql = "ALTER USER postgres PASSWORD 'rtRl4h1gahQvspXxOxtKcr41kVYeRQKG'"

		DescribeTable("default patterns",
			func(tc testCase) {
				processor := text.RedactSecrets()
				result, skip := processor(tc.input)

				Expect(skip).To(BeFalse())
				Expect(result).To(Equal(tc.expected))
			},
			Entry("password with colon", testCase{input: "password: secret", expected: "password: ***"}),
			Entry("token with equals", testCase{input: "token=abc123", expected: "token=***"}),
			Entry("api_key with equals", testCase{input: "api_key=xyz789", expected: "api_key=***"}),
			Entry("PASSWORD uppercase", testCase{input: "PASSWORD=1234", expected: "PASSWORD=***"}),
			Entry("auth with colon", testCase{input: "Auth: xyz", expected: "Auth: ***"}),
			Entry("bearer token", testCase{input: "Bearer: token123", expected: "Bearer: ***"}),
			Entry("password with quoted value", testCase{input: "password 'too redact'", expected: "password '***'"}),
			Entry("sql with quoted value", testCase{input: sql, expected: "ALTER USER postgres PASSWORD '***'"}),
			Entry("sql with quoted value", testCase{input: api.CodeBlock("sql", sql).ANSI(), expected: api.CodeBlock("sql", "ALTER USER postgres PASSWORD '***'").ANSI()}),

			Entry("token with double quotes", testCase{input: "token=\"secret value\"", expected: "token=\"***\""}),
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

	Describe("RedactValues", func() {
		type testCase struct {
			values   []string
			input    string
			expected string
		}

		DescribeTable("redacting known values",
			func(tc testCase) {
				processor := text.RedactValues(tc.values...)
				result, skip := processor(tc.input)

				Expect(skip).To(BeFalse())
				Expect(result).To(Equal(tc.expected))
			},
			Entry("single value", testCase{
				values:   []string{"secret123"},
				input:    "password=secret123",
				expected: "password=***",
			}),
			Entry("value in middle of text", testCase{
				values:   []string{"mysecret"},
				input:    "the token is mysecret and done",
				expected: "the token is *** and done",
			}),
			Entry("multiple occurrences", testCase{
				values:   []string{"abc"},
				input:    "abc is abc twice",
				expected: "*** is *** twice",
			}),
			Entry("multiple values", testCase{
				values:   []string{"secret1", "secret2"},
				input:    "password=secret1 token=secret2",
				expected: "password=*** token=***",
			}),
			Entry("no match", testCase{
				values:   []string{"secret"},
				input:    "normal log line",
				expected: "normal log line",
			}),
			Entry("quoted value", testCase{
				values:   []string{"my secret"},
				input:    "password='my secret'",
				expected: "password='***'",
			}),
		)

		It("should never skip lines", func() {
			processor := text.RedactValues("secret")
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
