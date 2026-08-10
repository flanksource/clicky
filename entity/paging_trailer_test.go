package entity

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// SetExportHeaders declares the trailer and the transport sends it, so the two
// have to read the same condition. A declared trailer that never arrives leaves
// a client waiting on it; an undeclared one is dropped on the floor. The whole
// matrix is walked because the disagreement would only ever show up in one
// corner of it.
func TestDeclaresTruncatedTrailer_AgreesWithSetExportHeaders(t *testing.T) {
	for _, scope := range []string{ScopePage, ScopeAll} {
		// More than one live ceiling: a second condition smuggled back into
		// SetExportHeaders is most likely to be a threshold, which a single
		// non-zero value would sit on one side of and never notice.
		for _, ceiling := range []int{0, 1, 500} {
			for _, truncated := range []bool{false, true} {
				name := fmt.Sprintf("scope=%s/ceiling=%d/truncated=%t", scope, ceiling, truncated)
				t.Run(name, func(t *testing.T) {
					req := PageRequest{Scope: scope, Format: "csv", Limit: 100}
					res := PageResponse{Mode: ModeStreaming, Ceiling: ceiling, Truncated: truncated}

					recorder := httptest.NewRecorder()
					SetExportHeaders(recorder, "rows", req, res)

					assert.Equal(t, DeclaresTruncatedTrailer(req, res),
						recorder.Header().Get("Trailer") == "X-Truncated",
						"the predicate and the header it gates must not disagree")
				})
			}
		}
	}
}

// The one case the trailer exists for, stated on its own so the matrix above
// cannot pass by agreeing on "never".
func TestDeclaresTruncatedTrailer_OnlyForACeilingThatMightStillBite(t *testing.T) {
	live := PageResponse{Ceiling: 500}
	all := PageRequest{Scope: ScopeAll}

	assert.True(t, DeclaresTruncatedTrailer(all, live))
	assert.False(t, DeclaresTruncatedTrailer(PageRequest{Scope: ScopePage}, live),
		"a page has no ceiling to discover")
	assert.False(t, DeclaresTruncatedTrailer(all, PageResponse{}),
		"an export with no ceiling has nothing left to find out")
	assert.False(t, DeclaresTruncatedTrailer(all, PageResponse{Ceiling: 500, Truncated: true}),
		"a cut already known has its answer in the headers")
}
