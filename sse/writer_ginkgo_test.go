package sse

import (
	"net/http"
	"net/http/httptest"

	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Writer", func() {
	It("writes nothing, not even headers, until the first frame", func() {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		Expect(writer.Started()).To(BeFalse())
		Expect(recorder.Header()).To(BeEmpty())
		Expect(recorder.Body.String()).To(BeEmpty())
	})

	It("frames a named event with an id and multi-line data as one data line per line", func() {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		Expect(writer.Send(entity.StreamEvent{Name: "update", ID: "7", Data: "line one\nline two"})).To(Succeed())

		Expect(writer.Started()).To(BeTrue())
		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Header()).To(Equal(http.Header{
			"Content-Type":      {"text/event-stream"},
			"Cache-Control":     {"no-cache"},
			"Connection":        {"keep-alive"},
			"X-Accel-Buffering": {"no"},
		}))
		Expect(recorder.Body.String()).To(Equal("event: update\nid: 7\ndata: line one\ndata: line two\n\n"))
	})

	It("encodes structured data as JSON and bytes verbatim", func() {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		Expect(writer.Send(entity.StreamEvent{Data: map[string]int{"done": 3}})).To(Succeed())
		Expect(writer.Send(entity.StreamEvent{Data: []byte(`{"raw":true}`)})).To(Succeed())

		Expect(recorder.Body.String()).To(Equal("data: {\"done\":3}\n\ndata: {\"raw\":true}\n\n"))
	})

	It("starts the stream with a comment frame on Ping", func() {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		Expect(writer.Ping()).To(Succeed())

		Expect(writer.Started()).To(BeTrue())
		Expect(recorder.Header().Get("Content-Type")).To(Equal("text/event-stream"))
		Expect(recorder.Body.String()).To(Equal(": ping\n\n"))
		Expect(recorder.Flushed).To(BeTrue())
	})

	It("reports a payload that cannot be encoded without writing a partial frame", func() {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		Expect(writer.Send(entity.StreamEvent{Name: "bad", Data: make(chan int)})).To(MatchError(ContainSubstring("encode stream event")))

		Expect(recorder.Body.String()).To(BeEmpty())
	})
})
