// Tests operations that stream events, so a subscription can be an operation
// rather than a hand-written route the generated surfaces know nothing about.
package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky/entity"
)

// streamRecorder counts flushes so a test can tell streaming from buffering.
type streamRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (s *streamRecorder) Flush() {
	s.flushes++
	s.ResponseRecorder.Flush()
}

func TestStreamOperationSendsEachEventAsItHappens(t *testing.T) {
	op := &RPCOperation{
		Name: "trace-run events", Path: "/api/v1/trace-runs/events", Method: http.MethodGet,
		StreamFunc: func(_ context.Context, _ map[string]string, _ []string, send entity.StreamSend) error {
			for _, id := range []string{"one", "two", "three"} {
				if err := send(entity.StreamEvent{Data: map[string]string{"id": id}}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	recorder := &streamRecorder{ResponseRecorder: httptest.NewRecorder()}
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil), op)

	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	for _, id := range []string{"one", "two", "three"} {
		assert.Containsf(t, body, `"id":"`+id+`"`, "event %q must reach the client", id)
	}
	assert.GreaterOrEqual(t, recorder.flushes, 3, "each event must be pushed, not buffered to the end")
}

func TestStreamOperationNamesItsEvents(t *testing.T) {
	op := &RPCOperation{
		Name: "task stream", Path: "/api/v1/tasks/stream", Method: http.MethodGet,
		StreamFunc: func(_ context.Context, _ map[string]string, _ []string, send entity.StreamSend) error {
			return send(entity.StreamEvent{Name: "progress", ID: "7", Data: map[string]int{"done": 3}})
		},
	}

	recorder := httptest.NewRecorder()
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/stream", nil), op)

	body := recorder.Body.String()
	assert.Contains(t, body, "event: progress")
	assert.Contains(t, body, "id: 7")
	assert.Contains(t, body, `"done":3`)
}

// A stream that fails after it has begun cannot change its status, so the
// reason travels as an event the client can act on rather than vanishing.
func TestStreamOperationReportsAFailureMidStream(t *testing.T) {
	op := &RPCOperation{
		Name: "trace-run events", Path: "/api/v1/trace-runs/events", Method: http.MethodGet,
		StreamFunc: func(_ context.Context, _ map[string]string, _ []string, send entity.StreamSend) error {
			if err := send(entity.StreamEvent{Data: "first"}); err != nil {
				return err
			}
			return assert.AnError
		},
	}

	recorder := httptest.NewRecorder()
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil), op)

	require.Equal(t, http.StatusOK, recorder.Code, "the status was already sent with the first event")
	assert.Contains(t, recorder.Body.String(), "event: error")
	assert.Contains(t, recorder.Body.String(), assert.AnError.Error())
}

// A stream that fails before sending anything can still answer properly.
func TestStreamOperationThatFailsImmediatelyAnswersWithAStatus(t *testing.T) {
	op := &RPCOperation{
		Name: "trace-run events", Path: "/api/v1/trace-runs/events", Method: http.MethodGet,
		StreamFunc: func(context.Context, map[string]string, []string, entity.StreamSend) error {
			return entity.NewStatusError(http.StatusNotFound, "not_found", "no such trace run")
		},
	}

	recorder := httptest.NewRecorder()
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil), op)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Header().Get("Content-Type"), "event-stream")
}

// The client going away ends the stream; it is not an error.
func TestStreamOperationStopsWhenTheClientLeaves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sent := 0
	op := &RPCOperation{
		Name: "trace-run events", Path: "/api/v1/trace-runs/events", Method: http.MethodGet,
		StreamFunc: func(ctx context.Context, _ map[string]string, _ []string, send entity.StreamSend) error {
			for {
				if err := send(entity.StreamEvent{Data: sent}); err != nil {
					return err
				}
				sent++
				if sent == 2 {
					cancel()
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
		},
	}

	recorder := httptest.NewRecorder()
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil).WithContext(ctx), op)

	assert.Equal(t, 2, sent)
	assert.NotContains(t, recorder.Body.String(), "event: error", "a client hanging up is not a failure")
}

func TestStreamedStringsAreSentVerbatim(t *testing.T) {
	op := &RPCOperation{
		Name: "log tail", Path: "/api/v1/logs/tail", Method: http.MethodGet,
		StreamFunc: func(_ context.Context, _ map[string]string, _ []string, send entity.StreamSend) error {
			return send(entity.StreamEvent{Data: "plain line"})
		},
	}

	recorder := httptest.NewRecorder()
	serveStream(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/logs/tail", nil), op)

	assert.Contains(t, recorder.Body.String(), "data: plain line")
	assert.False(t, strings.Contains(recorder.Body.String(), `data: "plain line"`),
		"a string is already a line; it must not be re-quoted as JSON")
}
