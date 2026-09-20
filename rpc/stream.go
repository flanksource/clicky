// Server-sent events for operations that stream, so a subscription reaches
// clients through the generated surface rather than a hand-written route.
package rpc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flanksource/clicky/entity"
)

// serveStream runs a streaming operation as an SSE response.
//
// Nothing is written until the operation produces its first event, so an
// operation that fails immediately can still answer with a status. Once the
// first event is out the status line is gone, and a later failure travels as an
// `error` event instead — a client that has already begun reading learns why
// the stream stopped rather than seeing it end silently.
func serveStream(w http.ResponseWriter, r *http.Request, op *RPCOperation) {
	flags, args := streamInputs(r, op)
	stream := &sseStream{writer: w, controller: http.NewResponseController(w)}

	err := op.StreamFunc(r.Context(), flags, args, stream.send)
	switch {
	case err == nil, errors.Is(err, r.Context().Err()) && r.Context().Err() != nil:
		// Done, or the client hung up. Neither is a failure.
	case !stream.started:
		writeStreamStatus(w, err)
		return
	default:
		stream.sendError(err)
	}
	if stream.started {
		stream.flush()
	}
}

// streamInputs reads the operation's declared parameters out of the request.
// A stream is a GET in practice, so query parameters and path values are the
// whole input; there is no body to decode.
func streamInputs(r *http.Request, op *RPCOperation) (map[string]string, []string) {
	flags := map[string]string{}
	for _, param := range op.Parameters {
		if value := r.URL.Query().Get(param.Name); value != "" {
			flags[param.Name] = value
		}
	}
	var args []string
	for _, param := range op.Parameters {
		if param.In != "path" {
			continue
		}
		if value := r.PathValue(param.Name); value != "" {
			flags[param.Name] = value
			args = append(args, value)
		}
	}
	return flags, args
}

// writeStreamStatus answers a stream that failed before it began, using the
// same status mapping as any other operation.
func writeStreamStatus(w http.ResponseWriter, err error) {
	status := statusForError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// sseStream writes the event-stream framing.
type sseStream struct {
	writer     http.ResponseWriter
	controller *http.ResponseController
	started    bool
}

// begin sends the headers once, on the first event.
func (s *sseStream) begin() {
	if s.started {
		return
	}
	s.started = true
	header := s.writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	// Proxies that buffer would defeat the point of streaming at all.
	header.Set("X-Accel-Buffering", "no")
	s.writer.WriteHeader(http.StatusOK)
	// A stream lives far longer than any fixed response deadline.
	_ = s.controller.SetWriteDeadline(time.Time{})
}

func (s *sseStream) send(event entity.StreamEvent) error {
	s.begin()
	var frame strings.Builder
	if event.Name != "" {
		frame.WriteString("event: " + event.Name + "\n")
	}
	if event.ID != "" {
		frame.WriteString("id: " + event.ID + "\n")
	}
	payload, err := entity.StreamText(event.Data)
	if err != nil {
		return err
	}
	// A multi-line payload is several data: lines; the client rejoins them.
	for line := range strings.SplitSeq(payload, "\n") {
		frame.WriteString("data: " + line + "\n")
	}
	frame.WriteString("\n")
	if _, err := io.WriteString(s.writer, frame.String()); err != nil {
		return err
	}
	return s.flush()
}

// sendError reports a failure that arrived once the stream was underway.
func (s *sseStream) sendError(err error) {
	_ = s.send(entity.StreamEvent{Name: "error", Data: map[string]string{"error": err.Error()}})
}

func (s *sseStream) flush() error { return s.controller.Flush() }
