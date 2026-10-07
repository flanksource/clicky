package sse

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flanksource/clicky/entity"
)

// Writer writes the event-stream framing onto one response. Nothing is written
// — not even the headers — until the first Send or Ping, so a handler that
// fails before producing anything can still answer with a status.
//
// A Writer is not safe for concurrent use: one goroutine owns a response.
type Writer struct {
	writer     http.ResponseWriter
	controller *http.ResponseController
	started    bool
}

// NewWriter returns a Writer for w.
func NewWriter(w http.ResponseWriter) *Writer {
	return &Writer{writer: w, controller: http.NewResponseController(w)}
}

// Started reports whether the headers (and so the 200 status) have been sent.
func (s *Writer) Started() bool { return s.started }

// begin sends the headers once, on the first frame.
func (s *Writer) begin() error {
	if s.started {
		return nil
	}
	s.started = true
	header := s.writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	// Proxies that buffer would defeat the point of streaming at all.
	header.Set("X-Accel-Buffering", "no")
	s.writer.WriteHeader(http.StatusOK)
	// A stream lives far longer than any fixed response deadline. A writer
	// without deadlines (a recorder, the hub's sub writer) has none to lift.
	if err := s.controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("clear stream write deadline: %w", err)
	}
	return nil
}

// Send writes one event and flushes it. Data is rendered by entity.StreamText:
// strings and byte slices verbatim, anything else as JSON.
func (s *Writer) Send(event entity.StreamEvent) error {
	payload, err := entity.StreamText(event.Data)
	if err != nil {
		return err
	}
	var frame strings.Builder
	if event.Name != "" {
		frame.WriteString("event: " + event.Name + "\n")
	}
	if event.ID != "" {
		frame.WriteString("id: " + event.ID + "\n")
	}
	// A multi-line payload is several data: lines; the client rejoins them.
	for line := range strings.SplitSeq(payload, "\n") {
		frame.WriteString("data: " + line + "\n")
	}
	frame.WriteString("\n")
	return s.write(frame.String())
}

// Ping writes a comment frame. Clients ignore it; it keeps idle proxies from
// closing the connection and lets the server notice a client that has gone.
func (s *Writer) Ping() error { return s.write(": ping\n\n") }

func (s *Writer) write(frame string) error {
	if err := s.begin(); err != nil {
		return err
	}
	if _, err := io.WriteString(s.writer, frame); err != nil {
		return err
	}
	return s.Flush()
}

// Flush pushes everything written so far to the client.
func (s *Writer) Flush() error { return s.controller.Flush() }
