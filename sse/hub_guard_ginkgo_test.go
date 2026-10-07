package sse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	eventStreamAccept = "text/event-stream"
	browserFetchMode  = "cors"
)

// browserStreamRequest is what a native EventSource sends: a GET with an
// event-stream Accept and the Sec-Fetch-* metadata every browser attaches.
func browserStreamRequest(ctx context.Context, method, url string) *http.Request {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Accept", eventStreamAccept)
	req.Header.Set("Sec-Fetch-Mode", browserFetchMode)
	return req
}

func browserStreamHeader() http.Header {
	return http.Header{"Accept": {eventStreamAccept}, "Sec-Fetch-Mode": {browserFetchMode}}
}

// doHeaders sends req and returns its response once the headers arrived,
// cancelling the request afterwards so an open stream does not outlive the spec.
func doHeaders(req *http.Request) *http.Response {
	ctx, cancel := context.WithCancel(req.Context())
	DeferCleanup(cancel)
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(resp.Body.Close)
	return resp
}

// newGuardTestServer serves a stream route, a JSON route and a POST launch
// route behind the guard, the shape of a dashboard's API.
func newGuardTestServer() *httptest.Server {
	stream := func(w http.ResponseWriter, r *http.Request) { tickerHandler(make(chan struct{}))(w, r) }
	return newEventsTestServer(map[string]http.HandlerFunc{
		"GET /api/proc/status/stream": stream,
		"GET /api/v1/tasks/stream":    stream,
		"GET /api/proc/status": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true}`)
		},
		"POST /api/todos/run": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "decode launch request", http.StatusBadRequest)
		},
	})
}

var _ = Describe("direct browser stream guard", func() {
	DescribeTable("refuses a native EventSource GET on a hub-servable stream route with 410",
		func(path string) {
			server := newGuardTestServer()

			resp := doHeaders(browserStreamRequest(context.Background(), http.MethodGet, server.URL+path))

			Expect(resp.StatusCode).To(Equal(http.StatusGone))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix("text/plain"))
			Expect(resp.Header.Get("Cache-Control")).To(Equal("no-store"))
			Expect(readBody(resp)).To(Equal("stream routes are served through " + DefaultPrefix + "; reload the page\n"))
		},
		Entry("a proc status stream", "/api/proc/status/stream"),
		Entry("a task stream", "/api/v1/tasks/stream"),
	)

	It("serves the refused route through a hub sub opened with browser fetch metadata", func() {
		server := newGuardTestServer()
		client := openEvents(server.URL, DefaultPrefix)

		body := strings.NewReader(`{"id":"proc","path":"/api/proc/status/stream"}`)
		subscribe, err := http.NewRequest(http.MethodPost, server.URL+DefaultPrefix+"/"+client.conn+"/subs", body)
		Expect(err).NotTo(HaveOccurred())
		subscribe.Header.Set("Content-Type", "application/json")
		subscribe.Header.Set("Sec-Fetch-Mode", browserFetchMode)
		Expect(doHeaders(subscribe).StatusCode).To(Equal(http.StatusNoContent))

		frame := client.nextFrame("proc/")
		Expect(frame.Event).To(Equal("proc/message"))
		Expect(frame.Data).To(Equal("tick-0"))
	})

	DescribeTable("serves direct GETs that are not browser stream loads",
		func(path string, header http.Header, wantContentType string) {
			server := newGuardTestServer()
			req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header = header

			resp := doHeaders(req)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix(wantContentType))
		},
		Entry("a CLI stream client without Sec-Fetch-Mode",
			"/api/proc/status/stream", http.Header{"Accept": {eventStreamAccept}}, eventStreamAccept),
		Entry("a CLI client on a task stream",
			"/api/v1/tasks/stream", http.Header{"Accept": {eventStreamAccept}}, eventStreamAccept),
		Entry("a browser JSON fetch of a non-stream route",
			"/api/proc/status", http.Header{"Accept": {"application/json"}, "Sec-Fetch-Mode": {browserFetchMode}}, "application/json"),
		Entry("the multiplexed events stream itself",
			DefaultPrefix, http.Header{"Accept": {eventStreamAccept}, "Sec-Fetch-Mode": {browserFetchMode}}, eventStreamAccept),
	)

	It("leaves a browser POST streaming launch route to its handler", func() {
		server := newGuardTestServer()
		req := browserStreamRequest(context.Background(), http.MethodPost, server.URL+"/api/todos/run")
		req.Body = io.NopCloser(strings.NewReader("not json"))

		resp := doHeaders(req)

		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest), "the launch handler's own decode error, not the guard")
		Expect(readBody(resp)).To(Equal("decode launch request\n"))
	})

	It("marks the request the hub dispatches for a sub", func() {
		server := newEventsTestServer(map[string]http.HandlerFunc{
			"GET /api/test/marker": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", eventStreamAccept)
				fmt.Fprintf(w, "data: %t\n\n", isHubSub(r.Context()))
			},
		})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("m", "/api/test/marker").StatusCode).To(Equal(http.StatusNoContent))

		Expect(client.nextFrame("m/").Data).To(Equal("true"))
	})

	DescribeTable("decides per request whether the guard refuses it",
		func(method string, header http.Header, path string, hubSub bool, wantRefused bool) {
			served := false
			guard := NewHub(HubOptions{Build: testBuild}).Guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				served = true
				w.WriteHeader(http.StatusOK)
			}))
			ctx := context.Background()
			if hubSub {
				ctx = withHubSub(ctx)
			}
			req := httptest.NewRequestWithContext(ctx, method, path, nil)
			req.Header = header
			rec := httptest.NewRecorder()

			guard.ServeHTTP(rec, req)

			Expect(served).To(Equal(!wantRefused))
			if wantRefused {
				Expect(rec.Code).To(Equal(http.StatusGone))
			}
		},
		Entry("a browser EventSource GET is refused",
			http.MethodGet, browserStreamHeader(), "/api/prs/stream", false, true),
		Entry("an Accept list naming event-stream among others is refused",
			http.MethodGet, http.Header{"Accept": {"application/json, text/event-stream"}, "Sec-Fetch-Mode": {browserFetchMode}}, "/api/prs/stream", false, true),
		Entry("a hub sub carrying every browser header is served (context marker, not a header)",
			http.MethodGet, browserStreamHeader(), "/api/prs/stream", true, false),
		Entry("a browser POST with an event-stream Accept is served",
			http.MethodPost, browserStreamHeader(), "/api/todos/run", false, false),
		Entry("a browser EventSource GET outside /api is served (embedded page bundle)",
			http.MethodGet, browserStreamHeader(), "/results/acme/widgets/7/api/tests/stream", false, false),
		Entry("the events stream itself is served",
			http.MethodGet, browserStreamHeader(), DefaultPrefix, false, false),
	)
})
