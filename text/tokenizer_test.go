package text

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// expectTokens compares tokens ignoring StartPos and EndPos
func expectTokens(actual, expected []Token) {
	Expect(len(actual)).To(Equal(len(expected)))
	for i := range actual {
		Expect(actual[i].Key).To(Equal(expected[i].Key), "Token %d: Key mismatch", i)
		Expect(actual[i].Separator).To(Equal(expected[i].Separator), "Token %d: Separator mismatch", i)
		Expect(actual[i].Value).To(Equal(expected[i].Value), "Token %d: Value mismatch", i)
		Expect(actual[i].QuoteChar).To(Equal(expected[i].QuoteChar), "Token %d: QuoteChar mismatch", i)
		if expected[i].ANSICode != "" {
			Expect(actual[i].ANSICode).To(Equal(expected[i].ANSICode), "Token %d: ANSICode mismatch", i)
		}
	}
}

var _ = Describe("Tokenizer", func() {
	Describe("Basic Tokenization", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("simple key-value pairs",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("password with equals", testCase{
				input: "password=secret",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "secret", QuoteChar: ""},
				},
			}),
			Entry("token with colon", testCase{
				input: "token: value",
				expected: []Token{
					{Key: "token", Separator: ": ", Value: "value", QuoteChar: ""},
				},
			}),
			Entry("password with space", testCase{
				input: "password secret",
				expected: []Token{
					{Key: "password", Separator: " ", Value: "secret", QuoteChar: ""},
				},
			}),
			Entry("case insensitive keyword", testCase{
				input: "PASSWORD=secret",
				expected: []Token{
					{Key: "PASSWORD", Separator: "=", Value: "secret", QuoteChar: ""},
				},
			}),
			Entry("api_key variant", testCase{
				input: "api_key=xyz",
				expected: []Token{
					{Key: "api_key", Separator: "=", Value: "xyz", QuoteChar: ""},
				},
			}),
		)
	})

	Describe("Quoted Values", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("single and double quotes",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("single quoted value", testCase{
				input: "password='secret value'",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "secret value", QuoteChar: "'"},
				},
			}),
			Entry("double quoted value", testCase{
				input: "token=\"secret value\"",
				expected: []Token{
					{Key: "token", Separator: "=", Value: "secret value", QuoteChar: "\""},
				},
			}),
			Entry("single quote with colon", testCase{
				input: "password: 'secret'",
				expected: []Token{
					{Key: "password", Separator: ": ", Value: "secret", QuoteChar: "'"},
				},
			}),
		)
	})

	Describe("ANSI Sequences", func() {
		It("should strip ANSI and extract tokens", func() {
			// ANSI is stripped before tokenization
			result := TokenizeLine("password \x1b[36msecret\x1b[0m")
			Expect(len(result)).To(Equal(1))
			Expect(result[0].Key).To(Equal("password"))
			Expect(result[0].Value).To(Equal("secret"))
			// QuoteChar is empty because ANSI was stripped
		})

		It("should handle ANSI-wrapped quoted values", func() {
			// ANSI wraps a quoted value: \x1b[37m'value'\x1b[0m
			result := TokenizeLine("password \x1b[37m'secret'\x1b[0m")
			Expect(len(result)).To(Equal(1))
			Expect(result[0].Key).To(Equal("password"))
			Expect(result[0].Value).To(Equal("secret"))
			Expect(result[0].QuoteChar).To(Equal("'"))
		})
	})

	Describe("Escaped Quotes", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("escape sequences",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("escaped single quote", testCase{
				input: "password='it\\'s secret'",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "it's secret", QuoteChar: "'"},
				},
			}),
			Entry("escaped double quote", testCase{
				input: "token=\"value with \\\"quotes\\\"\"",
				expected: []Token{
					{Key: "token", Separator: "=", Value: "value with \"quotes\"", QuoteChar: "\""},
				},
			}),
			Entry("escaped backslash", testCase{
				input: "password='path\\\\file'",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "path\\file", QuoteChar: "'"},
				},
			}),
		)
	})

	Describe("Nested Quotes", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("quotes within quotes",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("single inside double", testCase{
				input: "password=\"it's a secret\"",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "it's a secret", QuoteChar: "\""},
				},
			}),
			Entry("double inside single", testCase{
				input: "token='say \"hello\"'",
				expected: []Token{
					{Key: "token", Separator: "=", Value: "say \"hello\"", QuoteChar: "'"},
				},
			}),
		)
	})

	Describe("Unmatched Quotes", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("incomplete quotes",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("unclosed single quote", testCase{
				input: "password='unclosed",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "unclosed", QuoteChar: "'"},
				},
			}),
			Entry("unclosed double quote", testCase{
				input: "token=\"no end",
				expected: []Token{
					{Key: "token", Separator: "=", Value: "no end", QuoteChar: "\""},
				},
			}),
		)
	})

	Describe("Multi-Word Values", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("unquoted multi-word values",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("space-separated words", testCase{
				input: "password foo bar baz",
				expected: []Token{
					{Key: "password", Separator: " ", Value: "foo bar baz", QuoteChar: ""},
				},
			}),
			Entry("SQL statement", testCase{
				input: "ALTER USER postgres PASSWORD secret value here",
				expected: []Token{
					{Key: "PASSWORD", Separator: " ", Value: "secret value here", QuoteChar: ""},
				},
			}),
		)
	})

	Describe("Multiple Tokens", func() {
		type testCase struct {
			input    string
			expected []Token
		}

		DescribeTable("multiple key-value pairs",
			func(tc testCase) {
				result := TokenizeLine(tc.input)
				expectTokens(result, tc.expected)
			},
			Entry("two tokens with equals", testCase{
				input: "password=s1 token=s2",
				expected: []Token{
					{Key: "password", Separator: "=", Value: "s1", QuoteChar: ""},
					{Key: "token", Separator: "=", Value: "s2", QuoteChar: ""},
				},
			}),
			Entry("two tokens with colons", testCase{
				input: "password: s1 and token: s2",
				expected: []Token{
					{Key: "password", Separator: ": ", Value: "s1 and", QuoteChar: ""},
					{Key: "token", Separator: ": ", Value: "s2", QuoteChar: ""},
				},
			}),
		)
	})

	Describe("RebuildLine", func() {
		It("should rebuild line from tokens", func() {
			tokens := []Token{
				{Key: "password", Separator: "=", Value: "***", QuoteChar: ""},
			}
			result := RebuildLine("password=secret", tokens)
			Expect(result).To(Equal("password=***"))
		})

		It("should preserve quotes", func() {
			tokens := []Token{
				{Key: "password", Separator: "=", Value: "***", QuoteChar: "'"},
			}
			result := RebuildLine("password='secret'", tokens)
			Expect(result).To(Equal("password='***'"))
		})

		It("should handle ANSI in original", func() {
			// When original has ANSI, RebuildLine uses stripped version
			// ANSI preservation is handled by RedactSecrets via string replacement
			tokens := []Token{
				{Key: "password", Separator: " ", Value: "***", QuoteChar: ""},
			}
			result := RebuildLine("password \x1b[36msecret\x1b[0m", tokens)
			Expect(result).To(Equal("password ***"))
		})

		It("should handle multiple tokens", func() {
			tokens := []Token{
				{Key: "password", Separator: "=", Value: "***", QuoteChar: ""},
				{Key: "token", Separator: "=", Value: "***", QuoteChar: ""},
			}
			result := RebuildLine("password=s1 token=s2", tokens)
			Expect(result).To(Equal("password=*** token=***"))
		})

		It("should preserve non-tokenized portions", func() {
			tokens := []Token{
				{Key: "PASSWORD", Separator: " ", Value: "***", QuoteChar: "'"},
			}
			result := RebuildLine("ALTER USER postgres PASSWORD 'secret'", tokens)
			Expect(result).To(Equal("ALTER USER postgres PASSWORD '***'"))
		})
	})
})
