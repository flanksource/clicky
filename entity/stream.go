// Streaming operations: an operation that produces events over time rather than
// one answer, so a subscription is a registered operation like any other.
package entity

import (
	"context"
	"encoding/json"
	"fmt"
)

// StreamEvent is one thing that happened. Data is encoded as JSON unless it is
// already a string or a byte slice, which are sent verbatim — a log line is
// already a line and must not arrive quoted.
type StreamEvent struct {
	// Name groups events a client can subscribe to separately. Empty sends an
	// unnamed event, which every client receives.
	Name string
	// ID lets a client resume where it left off. Empty omits it.
	ID string
	// Data is the event itself.
	Data any
}

// StreamSend delivers one event. It returns an error when the client has gone
// or the transport failed; a StreamFunc must stop and return that error rather
// than continuing to produce events nobody receives.
type StreamSend func(StreamEvent) error

// StreamFunc produces events until it is done, the context ends, or sending
// fails. Returning a nil error closes the stream normally; returning
// ctx.Err() after the caller hung up is normal too and is not reported as a
// failure. Any other error is reported to the client — as a status if nothing
// has been sent yet, and as an error event once the stream is underway.
type StreamFunc func(ctx context.Context, flags map[string]string, args []string, send StreamSend) error

// StreamText renders an event's data as the text a transport sends. Strings and
// byte slices are already text and are used as they are — a log line must not
// arrive quoted; everything else is JSON.
func StreamText(data any) (string, error) {
	switch typed := data.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("encode stream event: %w", err)
	}
	return string(encoded), nil
}
