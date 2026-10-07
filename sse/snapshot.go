package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/flanksource/clicky/entity"
)

// SnapshotOptions describes a value Snapshot streams.
type SnapshotOptions struct {
	// Load produces the current value. It is called once immediately, then on
	// every Interval tick and every Wake. Required.
	Load func(ctx context.Context) (any, error)
	// Interval is how often Load is polled. Required (> 0).
	Interval time.Duration
	// Wake, when non-nil, triggers an immediate Load (e.g. a Notifier
	// subscription). Closing it is an error: Snapshot cannot tell a closed wake
	// from a spinning one.
	Wake <-chan struct{}
	// Exclude lists dotted JSON paths left out when deciding whether the value
	// changed — fields that churn without meaning anything to a reader, such as
	// a fetch timestamp or a sampled counter. A "*" segment matches every map
	// key or array element: "rateLimit", "*.processes.*.openFiles". They are
	// excluded only from the comparison: a frame sent because something else
	// changed carries their current values.
	Exclude []string
	// Event names the frames. Empty sends unnamed ("message") events.
	Event string
}

// Snapshot streams opts.Load's value: it loads immediately and then on every
// tick or wake, sends the value as JSON when it differs from the last one sent,
// and calls ping otherwise so idle connections stay alive and a departed client
// is noticed.
//
// send and ping are separate because entity.StreamSend has no comment frame: a
// plain handler passes a Writer's Send and Ping (or uses ServeSnapshot), while
// an rpc StreamFunc passes its send and a ping of its own choosing — a func
// returning nil opts out of keepalives explicitly.
//
// A Load (or encoding) failure is sent as an "error" event {"error": msg} and
// returned, ending the stream: a reader learns why it stopped. Snapshot returns
// ctx.Err() once ctx ends and a send or ping error as soon as one occurs. It
// panics on options it cannot run.
func Snapshot(ctx context.Context, send entity.StreamSend, ping func() error, opts SnapshotOptions) error {
	excludes := opts.validate(send, ping)
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	var last []byte
	sent := false
	for {
		payload, compare, err := loadSnapshot(ctx, opts.Load, excludes)
		switch {
		case err != nil && ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			return reportSnapshotError(send, err)
		case sent && bytes.Equal(compare, last):
			err = ping()
		default:
			err = send(entity.StreamEvent{Name: opts.Event, Data: payload})
			last, sent = compare, true
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case _, ok := <-opts.Wake:
			if !ok {
				return errors.New("sse: snapshot wake channel closed")
			}
		}
	}
}

// ServeSnapshot streams opts as the response to r through a Writer. It returns
// nil when the client went away and Snapshot's error otherwise (a load error
// has already been sent to the client as an "error" event by then).
func ServeSnapshot(w http.ResponseWriter, r *http.Request, opts SnapshotOptions) error {
	writer := NewWriter(w)
	err := Snapshot(r.Context(), writer.Send, writer.Ping, opts)
	if r.Context().Err() != nil {
		return nil
	}
	return err
}

func (opts SnapshotOptions) validate(send entity.StreamSend, ping func() error) [][]string {
	switch {
	case opts.Load == nil:
		panic("sse: SnapshotOptions.Load is required")
	case opts.Interval <= 0:
		panic(fmt.Sprintf("sse: SnapshotOptions.Interval must be positive, got %s", opts.Interval))
	case send == nil:
		panic("sse: Snapshot needs a send func")
	case ping == nil:
		panic("sse: Snapshot needs a ping func; pass one returning nil to send no keepalives")
	}
	return parseExcludes(opts.Exclude)
}

// loadSnapshot returns the value's JSON and the form it is compared by.
func loadSnapshot(ctx context.Context, load func(context.Context) (any, error), excludes [][]string) ([]byte, []byte, error) {
	value, err := load(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load snapshot: %w", err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, nil, fmt.Errorf("encode snapshot: %w", err)
	}
	compare, err := comparisonForm(payload, excludes)
	if err != nil {
		return nil, nil, err
	}
	return payload, compare, nil
}

func reportSnapshotError(send entity.StreamSend, err error) error {
	if sendErr := send(entity.StreamEvent{Name: "error", Data: map[string]string{"error": err.Error()}}); sendErr != nil {
		return errors.Join(err, fmt.Errorf("report snapshot error: %w", sendErr))
	}
	return err
}
