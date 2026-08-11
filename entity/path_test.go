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
		// PathSeparator is the wire delimiter, so a producer that never declares it
		// still cannot smuggle it inside a segment — JoinPath would emit a path the
		// consumer splits differently.
		{"path separator splits even when not declared", "jms/api.incoming", ".", []string{"jms", "api", "incoming"}},
		{"path separator splits with no delimiters declared", "jms/api", "", []string{"jms", "api"}},
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
	for _, input := range []struct {
		name       string
		delimiters string
	}{
		{"jms.incoming.disbursements", "./"},
		// A name carrying PathSeparator under a different producer delimiter must
		// not gain hierarchy levels on the way through JoinPath.
		{"jms/api.incoming", "."},
		{"jms/api", ""},
	} {
		t.Run(input.name, func(t *testing.T) {
			segments := SplitPath(input.name, input.delimiters)
			assert.Equal(t, segments, SplitPath(JoinPath(segments), PathSeparator))
		})
	}
}
