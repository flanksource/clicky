package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionRoutesPreserveServeMuxSpecialWildcards(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		matched bool
		params  map[string]string
	}{
		{pattern: "/files/{path...}", path: "/files/a/b", matched: true, params: map[string]string{"path": "a/b"}},
		{pattern: "/api/{$}", path: "/api/", matched: true, params: map[string]string{}},
		{pattern: "/api/{$}", path: "/api/item", matched: false, params: map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.pattern+test.path, func(t *testing.T) {
			sanitized, ok := sanitizePathParams(test.pattern)
			require.True(t, ok)
			assert.Equal(t, test.pattern, sanitized)
			assert.Equal(t, test.matched, matchTemplatePath(test.pattern, test.path))
			params, err := extractPathParams(test.pattern, test.path)
			require.NoError(t, err)
			assert.Equal(t, test.params, params)
		})
	}

	_, ok := sanitizePathParams("/files/{path*}")
	assert.False(t, ok, "unsupported wildcard syntax must be rejected instead of rewritten")
}
