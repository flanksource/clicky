package sse

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

// streamEvent is one dispatched server-sent event: its name ("" means the
// default "message"), its id, and its data lines in order.
type streamEvent struct {
	name string
	id   string
	data []string
}

// readStream parses an event stream incrementally, calling emit for every
// event as soon as its terminating blank line arrives. It follows the WHATWG
// dispatch rules: comment lines and retry: are dropped, unknown fields are
// ignored, an event without data is not dispatched, and a trailing event cut
// off by the end of the stream is discarded. bufio.Reader grows to fit any
// line, so a large frame (a full listing) is never truncated. It returns nil
// at the end of the stream, or the first read or emit error.
func readStream(r io.Reader, emit func(streamEvent) error) error {
	reader := bufio.NewReaderSize(r, 64<<10)
	var ev streamEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if len(ev.data) > 0 {
				if err := emit(ev); err != nil {
					return err
				}
			}
			ev = streamEvent{}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			ev.name = value
		case "data":
			ev.data = append(ev.data, value)
		case "id":
			ev.id = value
		}
	}
}

// encodeSubEvent renders ev as one frame of the multiplexed stream, its name
// prefixed with the sub id so the client can route it.
func encodeSubEvent(subID string, ev streamEvent) []byte {
	name := ev.name
	if name == "" {
		name = "message"
	}
	var frame bytes.Buffer
	frame.WriteString("event: " + subID + "/" + name + "\n")
	if ev.id != "" {
		frame.WriteString("id: " + ev.id + "\n")
	}
	for _, line := range ev.data {
		frame.WriteString("data: " + line + "\n")
	}
	frame.WriteString("\n")
	return frame.Bytes()
}
