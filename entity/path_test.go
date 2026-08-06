package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitPath(t *testing.T) {
	const dotSlash = "./"

	tests := []struct {
		name       string
		input      string
		delimiters string
		want       []string
	}{
		{"flat name is a root leaf", "jaeger", dotSlash, []string{"jaeger"}},
		{"dots nest", "jms.incoming.disbursements", dotSlash, []string{"jms", "incoming", "disbursements"}},
		{"slashes nest", "logs/api", dotSlash, []string{"logs", "api"}},
		{"mixed delimiters", "a.b/c", dotSlash, []string{"a", "b", "c"}},
		// A hyphen is a legitimate name character, not a separator: splitting it
		// would shatter "remote-debugger" into a bogus two-level hierarchy.
		{"hyphen is not a separator", "remote-debugger.sql-xevent", dotSlash, []string{"remote-debugger", "sql-xevent"}},
		{"repeated separators collapse", "a//b", dotSlash, []string{"a", "b"}},
		{"leading and trailing separators drop", ".a.b.", dotSlash, []string{"a", "b"}},
		{"empty name has no path", "", dotSlash, nil},
		{"separator-only name has no path", "...", dotSlash, nil},
		{"no delimiters declared means no split", "jms.incoming", "", []string{"jms.incoming"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, SplitPath(test.input, test.delimiters))
		})
	}
}

func TestJoinPath(t *testing.T) {
	assert.Equal(t, "jms/incoming/disbursements", JoinPath([]string{"jms", "incoming", "disbursements"}))
	assert.Equal(t, "jms", JoinPath([]string{"jms"}))
	assert.Equal(t, "", JoinPath(nil))
	assert.Equal(t, "a/b", JoinPath([]string{"a", "", "b"}), "empty segments are dropped")
}

func TestSplitPathRoundTripsThroughJoinPath(t *testing.T) {
	segments := SplitPath("jms.incoming.disbursements", "./")
	assert.Equal(t, segments, SplitPath(JoinPath(segments), PathSeparator))
}
