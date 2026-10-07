package sse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	rpchttp "github.com/flanksource/clicky/rpc/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testBuild = "test-build"

// sseFrame is one event read off the multiplexed events stream. Raw holds
// every line of the frame verbatim so specs can assert what was NOT forwarded.
type sseFrame struct {
	Event string
	ID    string
	Data  string
	Raw   []string
}

// eventsClient is one open events connection: the conn id from the hello frame
// and the frames that followed it, in arrival order.
type eventsClient struct {
	base   string
	prefix string
	conn   string
	build  string
	frames chan sseFrame
	cancel context.CancelFunc
}

// newHubTestServer serves a mux shaped like a real dashboard: a "/" catch-all
// (the SPA route), the test stream handlers, and the hub routes dispatching
// through the guarded, TimingMiddleware-wrapped root. A non-zero writeTimeout
// becomes the server's http.Server.WriteTimeout.
func newHubTestServer(opts HubOptions, writeTimeout time.Duration, routes map[string]http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<html>spa</html>") })
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, handler)
	}
	hub := NewHub(opts)
	root := hub.Guard(rpchttp.TimingMiddleware(mux))
	hub.Register(mux, root)
	server := httptest.NewUnstartedServer(root)
	server.Config.WriteTimeout = writeTimeout
	server.Start()
	DeferCleanup(server.Close)
	return server
}

func newEventsTestServer(routes map[string]http.HandlerFunc) *httptest.Server {
	return newHubTestServer(HubOptions{Build: testBuild}, 0, routes)
}

// openEvents opens GET {prefix} and returns once the hello frame arrived.
func openEvents(base, prefix string) *eventsClient {
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+prefix, nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))

	client := &eventsClient{base: base, prefix: prefix, frames: make(chan sseFrame, 4096), cancel: cancel}
	go readTestFrames(resp.Body, client.frames)
	DeferCleanup(cancel)

	var hello sseFrame
	Eventually(client.frames, 5*time.Second).Should(Receive(&hello))
	Expect(hello.Event).To(Equal("__hello"))
	var payload struct {
		Conn  string `json:"conn"`
		Build string `json:"build"`
	}
	Expect(json.Unmarshal([]byte(hello.Data), &payload)).To(Succeed())
	Expect(payload.Conn).NotTo(BeEmpty())
	Expect(payload.Build).NotTo(BeEmpty())
	client.conn, client.build = payload.Conn, payload.Build
	return client
}

// readTestFrames is an independent SSE reader for the main stream (not the
// production parser, so a parser bug cannot hide behind its own oracle).
func readTestFrames(body io.ReadCloser, out chan<- sseFrame) {
	defer body.Close()
	defer close(out)
	reader := bufio.NewReaderSize(body, 1<<20)
	var frame sseFrame
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			if len(frame.Raw) > 0 && !strings.HasPrefix(frame.Raw[0], ":") {
				frame.Data = strings.Join(data, "\n")
				out <- frame
			}
			frame, data = sseFrame{}, nil
			continue
		}
		frame.Raw = append(frame.Raw, line)
		field, value, _ := strings.Cut(line, ": ")
		switch field {
		case "event":
			frame.Event = value
		case "id":
			frame.ID = value
		case "data":
			data = append(data, value)
		}
	}
}

func (c *eventsClient) subscribe(id, path string) *http.Response {
	body, err := json.Marshal(map[string]string{"id": id, "path": path})
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.Post(c.base+c.prefix+"/"+c.conn+"/subs", "application/json", bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(resp.Body.Close)
	return resp
}

func (c *eventsClient) unsubscribe(id string) *http.Response {
	req, err := http.NewRequest(http.MethodDelete, c.base+c.prefix+"/"+c.conn+"/subs/"+id, nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(resp.Body.Close)
	return resp
}

// nextFrame waits for the next frame whose event name starts with prefix,
// discarding frames of other subs.
func (c *eventsClient) nextFrame(prefix string) sseFrame {
	deadline := time.After(5 * time.Second)
	for {
		select {
		case frame, ok := <-c.frames:
			Expect(ok).To(BeTrue(), "events stream closed while waiting for %s", prefix)
			if strings.HasPrefix(frame.Event, prefix) {
				return frame
			}
		case <-deadline:
			Fail("timed out waiting for a frame for " + prefix)
		}
	}
}

// collectUntilClosed gathers every frame of the given subs until each emitted
// its __closed frame, returning them grouped per sub in arrival order.
func (c *eventsClient) collectUntilClosed(subs ...string) map[string][]sseFrame {
	got := map[string][]sseFrame{}
	open := map[string]bool{}
	for _, sub := range subs {
		open[sub] = true
	}
	deadline := time.After(10 * time.Second)
	for len(open) > 0 {
		select {
		case frame, ok := <-c.frames:
			Expect(ok).To(BeTrue(), "events stream closed with subs still open: %v", open)
			sub, name, _ := strings.Cut(frame.Event, "/")
			got[sub] = append(got[sub], frame)
			if name == "__closed" {
				delete(open, sub)
			}
		case <-deadline:
			Fail(fmt.Sprintf("timed out; subs still open: %v", open))
		}
	}
	return got
}

// tickerHandler streams a data frame every 5ms until its request context is
// cancelled, then closes exited so specs can prove the handler stopped.
func tickerHandler(exited chan<- struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer close(exited)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; ; i++ {
			fmt.Fprintf(w, "data: tick-%d\n\n", i)
			flusher.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
}

// burstHandler writes ?n= events whose data spans two lines tagged with ?tag=,
// then returns so the sub ends on its own.
func burstHandler(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	flusher := w.(http.Flusher)
	tag := r.URL.Query().Get("tag")
	for i := 0; i < n; i++ {
		fmt.Fprintf(w, "event: item\ndata: %s-%d-a\ndata: %s-%d-b\n\n", tag, i, tag, i)
		flusher.Flush()
	}
}

func readBody(resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	return string(body)
}
