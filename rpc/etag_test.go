package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentETag(t *testing.T) {
	tag := contentETag([]byte("openapi document"))

	assert.Equal(t, tag, contentETag([]byte("openapi document")), "same bytes must yield the same validator")
	assert.NotEqual(t, tag, contentETag([]byte("openapi document ")), "differing bytes must yield differing validators")
	assert.Regexp(t, `^"[0-9a-f]{32}"$`, tag, "a strong validator is a quoted opaque string")
}

func TestETagMatches(t *testing.T) {
	const tag = `"abc123"`

	tests := []struct {
		name        string
		ifNoneMatch string
		want        bool
	}{
		{name: "absent header", ifNoneMatch: "", want: false},
		{name: "exact", ifNoneMatch: tag, want: true},
		{name: "wildcard", ifNoneMatch: "*", want: true},
		{name: "weak validator", ifNoneMatch: `W/"abc123"`, want: true},
		{name: "list containing the tag", ifNoneMatch: `"other", ` + tag, want: true},
		{name: "list without the tag", ifNoneMatch: `"other", "another"`, want: false},
		{name: "unquoted", ifNoneMatch: "abc123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, etagMatches(tt.ifNoneMatch, tag))
		})
	}
}
