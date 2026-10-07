package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("multiplexed events hub", func() {
	It("sends a hello frame naming the connection and the UI build", func() {
		server := newEventsTestServer(nil)
		client := openEvents(server.URL, DefaultPrefix)
		Expect(client.conn).To(MatchRegexp(`^[A-Za-z0-9-]+$`))
		Expect(client.build).To(Equal(testBuild))
	})

	It("keeps relaying sub events past the server's WriteTimeout", func() {
		const writeTimeout = 200 * time.Millisecond
		server := newHubTestServer(HubOptions{Build: testBuild}, writeTimeout, map[string]http.HandlerFunc{
			"GET /api/test/late": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				time.Sleep(3 * writeTimeout)
				fmt.Fprint(w, "data: late\n\n")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			},
		})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("late", "/api/test/late").StatusCode).To(Equal(http.StatusNoContent))

		Expect(client.nextFrame("late/").Data).To(Equal("late"))
	})

	It("forbids content sniffing on the events stream", func() {
		server := newEventsTestServer(nil)
		resp, err := http.Get(server.URL + DefaultPrefix + "?x=<script>")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(resp.Body.Close)
		Expect(resp.Header.Get("X-Content-Type-Options")).To(Equal("nosniff"))
	})

	It("relays a Snapshot stream route through the guarded root", func() {
		fixture := map[string]any{"web": map[string]any{"status": "running", "pid": float64(42)}}
		served := make(chan error, 1)
		server := newEventsTestServer(map[string]http.HandlerFunc{
			"GET /api/proc/status/stream": func(w http.ResponseWriter, r *http.Request) {
				served <- ServeSnapshot(w, r, SnapshotOptions{
					Load:     func(context.Context) (any, error) { return fixture, nil },
					Interval: time.Minute,
				})
			},
		})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("proc", "/api/proc/status/stream").StatusCode).To(Equal(http.StatusNoContent))

		want, err := json.Marshal(fixture)
		Expect(err).NotTo(HaveOccurred())
		frame := client.nextFrame("proc/")
		Expect(frame.Event).To(Equal("proc/message"))
		Expect(frame.Data).To(MatchJSON(want))
		Expect(client.unsubscribe("proc").StatusCode).To(Equal(http.StatusNoContent))
		Expect(<-served).To(Succeed(), "a stream ended by its client is not a failure")
	})

	It("prefixes the event name, keeps multi-line data and id, and drops comments and retry", func() {
		server := newEventsTestServer(map[string]http.HandlerFunc{
			"GET /api/test/multiline": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, ": keepalive comment\nretry: 1000\nid: 7\nevent: update\ndata: line one\ndata: line two\n\n")
				fmt.Fprint(w, "data: {\"plain\":true}\n\n")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			},
		})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("ml", "/api/test/multiline?x=1").StatusCode).To(Equal(http.StatusNoContent))

		named := client.nextFrame("ml/")
		Expect(named).To(Equal(sseFrame{
			Event: "ml/update",
			ID:    "7",
			Data:  "line one\nline two",
			Raw:   []string{"event: ml/update", "id: 7", "data: line one", "data: line two"},
		}))
		unnamed := client.nextFrame("ml/")
		Expect(unnamed.Event).To(Equal("ml/message"))
		Expect(unnamed.Data).To(Equal(`{"plain":true}`))
	})

	It("interleaves two concurrent subs without corrupting either one's frames", func() {
		const events = 300
		server := newEventsTestServer(map[string]http.HandlerFunc{"GET /api/test/burst": burstHandler})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("a", fmt.Sprintf("/api/test/burst?n=%d&tag=A", events)).StatusCode).To(Equal(http.StatusNoContent))
		Expect(client.subscribe("b", fmt.Sprintf("/api/test/burst?n=%d&tag=B", events)).StatusCode).To(Equal(http.StatusNoContent))

		got := client.collectUntilClosed("a", "b")
		for sub, tag := range map[string]string{"a": "A", "b": "B"} {
			frames := got[sub]
			Expect(frames).To(HaveLen(events + 1))
			for i, frame := range frames[:events] {
				Expect(frame.Event).To(Equal(sub + "/item"))
				Expect(frame.Data).To(Equal(fmt.Sprintf("%s-%d-a\n%s-%d-b", tag, i, tag, i)))
			}
			Expect(frames[events].Event).To(Equal(sub + "/__closed"))
			Expect(frames[events].Data).To(MatchJSON(`{"status":200}`))
		}
	})

	It("stops the sub's handler on DELETE and frees its id", func() {
		exited := make(chan struct{})
		server := newEventsTestServer(map[string]http.HandlerFunc{"GET /api/test/ticker": tickerHandler(exited)})
		client := openEvents(server.URL, DefaultPrefix)
		Expect(client.subscribe("t", "/api/test/ticker").StatusCode).To(Equal(http.StatusNoContent))
		Expect(client.nextFrame("t/").Event).To(Equal("t/message"))

		Expect(client.unsubscribe("t").StatusCode).To(Equal(http.StatusNoContent))

		Expect(exited).To(BeClosed(), "DELETE must return only after the handler stopped")
		Consistently(func() []string {
			var late []string
			for {
				select {
				case frame := <-client.frames:
					late = append(late, frame.Event)
				default:
					return late
				}
			}
		}, 150*time.Millisecond).ShouldNot(ContainElement("t/__closed"))
		Expect(client.unsubscribe("t").StatusCode).To(Equal(http.StatusNotFound))
	})

	It("rejects an unknown connection with 404", func() {
		server := newEventsTestServer(map[string]http.HandlerFunc{"GET /api/test/burst": burstHandler})
		ghost := &eventsClient{base: server.URL, prefix: DefaultPrefix, conn: "no-such-conn"}

		subscribed := ghost.subscribe("x", "/api/test/burst?n=1")
		Expect(subscribed.StatusCode).To(Equal(http.StatusNotFound))
		Expect(subscribed.Header.Get("Content-Type")).To(Equal("application/json"))
		Expect(readBody(subscribed)).To(ContainSubstring(`"error":`))
		Expect(ghost.unsubscribe("x").StatusCode).To(Equal(http.StatusNotFound))
	})

	DescribeTable("rejects a subscription with 400",
		func(id, path string) {
			server := newEventsTestServer(map[string]http.HandlerFunc{
				"GET /api/test/burst":      burstHandler,
				"POST /api/test/post-only": burstHandler,
			})
			client := openEvents(server.URL, DefaultPrefix)
			resp := client.subscribe(id, path)
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(readBody(resp)).To(ContainSubstring(`"error":`))
		},
		Entry("empty id", "", "/api/test/burst?n=1"),
		Entry("id with a space", "has space", "/api/test/burst?n=1"),
		Entry("id longer than 64", strings.Repeat("x", 65), "/api/test/burst?n=1"),
		Entry("the events stream itself", "e", "/api/events"),
		Entry("an events control route", "e", "/api/events/abc/subs"),
		Entry("a non-api path", "e", "/test/burst"),
		Entry("an unregistered api path (SPA catch-all)", "e", "/api/unregistered"),
		Entry("an absolute URL", "e", "http://example.com/api/test/burst?n=1"),
		Entry("an unclean path escaping into events", "e", "/api/test/../events"),
		Entry("a POST-only route", "e", "/api/test/post-only"),
	)

	It("rejects a second active sub with the same id with 409", func() {
		exited := make(chan struct{})
		server := newEventsTestServer(map[string]http.HandlerFunc{"GET /api/test/ticker": tickerHandler(exited)})
		client := openEvents(server.URL, DefaultPrefix)

		Expect(client.subscribe("dup", "/api/test/ticker").StatusCode).To(Equal(http.StatusNoContent))
		Expect(client.subscribe("dup", "/api/test/ticker").StatusCode).To(Equal(http.StatusConflict))
	})

	DescribeTable("emits __closed with the handler's status when a sub ends on its own",
		func(handler http.HandlerFunc, want string) {
			server := newEventsTestServer(map[string]http.HandlerFunc{"GET /api/test/ends": handler})
			client := openEvents(server.URL, DefaultPrefix)
			Expect(client.subscribe("end", "/api/test/ends").StatusCode).To(Equal(http.StatusNoContent))

			frames := client.collectUntilClosed("end")["end"]
			closed := frames[len(frames)-1]
			Expect(closed.Event).To(Equal("end/__closed"))
			Expect(closed.Data).To(MatchJSON(want))
		},
		Entry("a stream that returns", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: last\n\n")
		}, `{"status":200}`),
		Entry("a non-2xx response carries its body", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "backend down", http.StatusServiceUnavailable)
		}, `{"status":503,"error":"backend down\n"}`),
		Entry("a non-2xx body is cut to its first 500 bytes", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, strings.Repeat("e", 500)+strings.Repeat("TAIL", 100))
		}, fmt.Sprintf(`{"status":500,"error":%q}`, strings.Repeat("e", 500))),
		Entry("a 2xx response that is not an event stream", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true}`)
		}, `{"status":200,"error":"content type \"application/json\" is not text/event-stream: {\"ok\":true}"}`),
		Entry("a panicking handler", func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}, `{"status":500,"error":"handler panic: boom"}`),
	)

	It("cancels every sub and forgets the connection when the client disconnects", func() {
		exitedA, exitedB := make(chan struct{}), make(chan struct{})
		server := newEventsTestServer(map[string]http.HandlerFunc{
			"GET /api/test/ticker-a": tickerHandler(exitedA),
			"GET /api/test/ticker-b": tickerHandler(exitedB),
			"GET /api/test/burst":    burstHandler,
		})
		client := openEvents(server.URL, DefaultPrefix)
		Expect(client.subscribe("a", "/api/test/ticker-a").StatusCode).To(Equal(http.StatusNoContent))
		Expect(client.subscribe("b", "/api/test/ticker-b").StatusCode).To(Equal(http.StatusNoContent))
		client.nextFrame("a/")
		client.nextFrame("b/")

		client.cancel()

		Eventually(exitedA, 5*time.Second).Should(BeClosed())
		Eventually(exitedB, 5*time.Second).Should(BeClosed())
		Eventually(func() int {
			return client.subscribe("c", "/api/test/burst?n=1").StatusCode
		}, 5*time.Second).Should(Equal(http.StatusNotFound))
	})

	Context("with a custom prefix", func() {
		const prefix = "/api/stream"

		It("mounts the hub under the prefix and makes the default prefix an ordinary route", func() {
			server := newHubTestServer(HubOptions{Build: testBuild, Prefix: prefix}, 0, map[string]http.HandlerFunc{
				"GET /api/events": burstHandler,
			})
			client := openEvents(server.URL, prefix)

			Expect(client.subscribe("own", prefix).StatusCode).To(Equal(http.StatusBadRequest))
			Expect(client.subscribe("old", "/api/events?n=1&tag=X").StatusCode).To(Equal(http.StatusNoContent))
			frames := client.collectUntilClosed("old")["old"]
			Expect(frames[0].Data).To(Equal("X-0-a\nX-0-b"))
		})
	})

	DescribeTable("NewHub refuses invalid options",
		func(opts HubOptions, want string) {
			Expect(func() { NewHub(opts) }).To(PanicWith(ContainSubstring(want)))
		},
		Entry("a missing build id", HubOptions{}, "Build"),
		Entry("a relative prefix", HubOptions{Build: testBuild, Prefix: "api/events"}, "Prefix"),
		Entry("a prefix with a trailing slash", HubOptions{Build: testBuild, Prefix: "/api/events/"}, "Prefix"),
	)

	It("refuses to be registered twice", func() {
		hub := NewHub(HubOptions{Build: testBuild})
		mux := http.NewServeMux()
		hub.Register(mux, mux)
		Expect(func() { hub.Register(http.NewServeMux(), mux) }).To(PanicWith(ContainSubstring("already registered")))
	})
})
