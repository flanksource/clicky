// Server-sent events for operations that stream, so a subscription reaches
// clients through the generated surface rather than a hand-written route.
package rpc

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/sse"
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
	stream := sse.NewWriter(w)

	err := op.StreamFunc(r.Context(), flags, args, stream.Send)
	switch {
	case err == nil, errors.Is(err, r.Context().Err()) && r.Context().Err() != nil:
		// Done, or the client hung up. Neither is a failure.
	case !stream.Started():
		writeStreamStatus(w, err)
		return
	default:
		// A failure to report the failure means the client is gone too.
		_ = stream.Send(entity.StreamEvent{Name: "error", Data: map[string]string{"error": err.Error()}})
	}
	if stream.Started() {
		_ = stream.Flush()
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
