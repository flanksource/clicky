package api

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The expectations below pin the output the golang.org/x/text/cases English
// title caser produced, so the stdlib implementation stays byte-identical.
var _ = Describe("Field name title casing", func() {
	DescribeTable("PrettifyFieldName",
		func(name, expected string) {
			Expect(PrettifyFieldName(name)).To(Equal(expected))
		},
		Entry("empty", "", ""),
		Entry("single lowercase word", "simple", "Simple"),
		Entry("camelCase", "firstName", "First Name"),
		Entry("PascalCase", "FirstName", "First Name"),
		Entry("snake_case", "first_name", "First Name"),
		Entry("kebab-case", "first-name", "First Name"),
		Entry("mixed separators", "created_at-utc", "Created At Utc"),
		Entry("leading and repeated separators", "__first__name_", "First Name"),
		Entry("only separators", "_-_", ""),
		Entry("trailing acronym", "userID", "User Id"),
		Entry("acronym prefix stays one word", "HTTPServer", "Httpserver"),
		Entry("all caps acronym", "URL", "Url"),
		Entry("acronym inside snake_case", "api_URL", "Api Url"),
		Entry("digit starts a word, next letter is titled", "1st_place", "1St Place"),
		Entry("uppercase after a digit is not a camelCase split", "ipv4Address", "Ipv4address"),
		Entry("letters after digits stay lowercase", "sha256Sum", "Sha256sum"),
		Entry("already spaced words", "first name", "First Name"),
		Entry("already spaced title case", "First Name", "First Name"),
		Entry("apostrophe keeps the word together", "don't_stop", "Don't Stop"),
		Entry("double apostrophe breaks the word", "a''b", "A''B"),
		Entry("dot keeps the word together", "file.name", "File.name"),
		Entry("double dot breaks the word", "a..b", "A..B"),
		Entry("colon keeps the word together", "ns:key", "Ns:key"),
		Entry("comma breaks the word", "a,b", "A,B"),
		Entry("slash breaks the word", "read/write", "Read/Write"),
		Entry("parentheses break the word", "size(mb)", "Size(Mb)"),
		Entry("accented latin", "éclair_über", "Éclair Über"),
		Entry("sharp s expands when titled", "ßeta", "Sseta"),
		Entry("sharp s mid-word is kept", "straße", "Straße"),
		Entry("ligature expands when titled", "ﬁle", "File"),
		Entry("greek uppercase is lowered then titled", "ΣΑΣ", "Σασ"),
		Entry("cyrillic", "имя_поля", "Имя Поля"),
		Entry("han is a word break", "日本abc", "日本Abc"),
		Entry("dz digraph titles to its titlecase form", "ǆx", "ǅx"),
		Entry("combining mark stays in the word", "i̇d", "İd"),
		Entry("zero width joiner stays in the word", "a‍b", "A‍b"),
	)

	DescribeTable("PrettyField.prettifyFieldName",
		func(name, expected string) {
			Expect(PrettyField{}.prettifyFieldName(name)).To(Equal(expected))
		},
		Entry("empty", "", ""),
		Entry("snake_case", "first_name", "First Name"),
		Entry("kebab-case", "first-name", "First Name"),
		Entry("camelCase is not split", "firstName", "Firstname"),
		Entry("acronym", "HTTPServer", "Httpserver"),
		Entry("only separators fall back to the raw name", "__", "__"),
		Entry("digit starts a word", "2fa_code", "2Fa Code"),
		Entry("already spaced words", "first name", "First Name"),
		Entry("unicode", "éclair_ßeta", "Éclair Sseta"),
	)
})
