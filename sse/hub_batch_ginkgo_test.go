package sse

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// flushRecorder is a ResponseWriter that remembers what had reached the client
// at its last flush, and how many flushes there were.
type flushRecorder struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	flushed string
	flushes int
}

func (w *flushRecorder) Header() http.Header { return w.header }
func (w *flushRecorder) WriteHeader(int)     {}

func (w *flushRecorder) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(p)
}

func (w *flushRecorder) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushes++
	w.flushed = w.body.String()
}

func (w *flushRecorder) visible() (string, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushed, w.flushes
}

var _ = Describe("hub sub batching", func() {
	It("flushes a burst of frames written at once a single time, after the last frame", func() {
		recorder := &flushRecorder{header: http.Header{}}
		ctx, cancel := context.WithCancel(context.Background())
		conn := &hubConn{id: "conn", ctx: ctx, cancel: cancel, w: recorder, flusher: recorder, subs: map[string]*hubSub{}}
		DeferCleanup(func() { cancel(); conn.wg.Wait() })
		burst := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: one\ndata: 1\n\nevent: two\ndata: 2\n\nevent: three\ndata: 3\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		})

		_, err := conn.startSub("s", httptest.NewRequest(http.MethodGet, "/api/test/burst", nil), burst)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() string { flushed, _ := recorder.visible(); return flushed }).Should(ContainSubstring("event: s/three"))
		flushed, flushes := recorder.visible()
		Expect(flushed).To(Equal("event: s/one\ndata: 1\n\nevent: s/two\ndata: 2\n\nevent: s/three\ndata: 3\n\n"))
		Expect(flushes).To(Equal(1))
	})
})
