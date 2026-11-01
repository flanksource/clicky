package metrics_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/clicky/route"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type streamEvent struct {
	Name  string
	Frame metrics.StreamFrame
	Error string
}

// openStream subscribes to the metrics stream and returns a func reading the
// next named event (pings skipped) plus the response status.
func openStream(srv *httptest.Server, query url.Values) (int, func() streamEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	DeferCleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/metrics/stream?"+query.Encode(), nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(resp.Body.Close)
	scanner := bufio.NewScanner(resp.Body)
	return resp.StatusCode, func() streamEvent {
		var ev streamEvent
		var data strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data.WriteString(strings.TrimPrefix(line, "data: "))
			case line == "" && ev.Name != "":
				if ev.Name == "error" {
					var body map[string]string
					Expect(json.Unmarshal([]byte(data.String()), &body)).To(Succeed())
					ev.Error = body["error"]
				} else {
					Expect(json.Unmarshal([]byte(data.String()), &ev.Frame)).To(Succeed())
				}
				return ev
			}
		}
		Fail("stream ended before the next event: " + errString(scanner.Err()))
		return ev
	}
}

func errString(err error) string {
	if err == nil {
		return "EOF"
	}
	return err.Error()
}

func streamServer(src metrics.SeriesSource) *httptest.Server {
	mux := http.NewServeMux()
	metrics.RegisterStreamRoutes(route.NewRouter(mux), src, "/api/v1")
	srv := httptest.NewServer(mux)
	DeferCleanup(srv.Close)
	return srv
}

// funcSource answers each tick with the next result of its func.
type funcSource struct {
	calls atomic.Int32
	next  func(call int32, req metrics.SeriesRequest) ([]metrics.Series, error)
}

func (f *funcSource) QuerySeries(_ context.Context, req metrics.SeriesRequest) ([]metrics.Series, error) {
	return f.next(f.calls.Add(1), req)
}

var _ = Describe("Metrics stream", func() {
	ids := func(series []metrics.Series) []string {
		out := []string{}
		for _, s := range series {
			out = append(out, s.ID)
		}
		return out
	}

	It("answers every id of one subscription in a single snapshot", func() {
		ts := metrics.NewMemory(metrics.MemoryConfig{})
		now := time.Now()
		for _, id := range []string{"cpu", "rss", "run-1:openFiles"} {
			Expect(ts.Record(metrics.RecordRequest{ID: id, At: now.Add(-time.Second), Value: 1})).To(Succeed())
		}
		srv := streamServer(metrics.Sources{"": metrics.StoreSource(ts)})

		status, next := openStream(srv, url.Values{"id": {"cpu", "rss", "run-1:openFiles", "cpu"}, "since": {"5m"}})
		Expect(status).To(Equal(http.StatusOK))
		ev := next()
		Expect(ev.Name).To(Equal("snapshot"))
		Expect(ids(ev.Frame.Series)).To(Equal([]string{"cpu", "rss", "run-1:openFiles"}))
	})

	It("appends only the points recorded after the previous frame", func() {
		ts := metrics.NewMemory(metrics.MemoryConfig{})
		start := time.Now().Add(-time.Minute)
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: start, Value: 1})).To(Succeed())
		srv := streamServer(metrics.StoreSource(ts))

		_, next := openStream(srv, url.Values{"id": {"cpu"}, "since": {"5m"}, "interval": {"1s"}})
		Expect(next().Frame.Series[0].Points).To(HaveLen(1))
		later := start.Add(30 * time.Second)
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: later, Value: 2})).To(Succeed())

		ev := next()
		Expect(ev.Name).To(Equal("append"))
		Expect(ev.Frame.Series).To(HaveLen(1))
		Expect(ev.Frame.Series[0].Points).To(HaveLen(1))
		Expect(ev.Frame.Series[0].Points[0].Value).To(Equal(2.0))
	})

	It("keeps the requested id as Source when one id expands to several series", func() {
		src := &funcSource{next: func(_ int32, req metrics.SeriesRequest) ([]metrics.Series, error) {
			Expect(req.Step).To(BeNumerically(">=", time.Second))
			return []metrics.Series{
				{ID: "profile:pod-cpu#pod=a", Source: "profile:pod-cpu", Labels: map[string]string{"pod": "a"}, Points: []metrics.Point{}},
				{ID: "profile:pod-cpu#pod=b", Source: "profile:pod-cpu", Labels: map[string]string{"pod": "b"}, Points: []metrics.Point{}},
			}, nil
		}}
		srv := streamServer(metrics.Sources{"profile": src})

		_, next := openStream(srv, url.Values{"id": {"profile:pod-cpu"}})
		frame := next().Frame
		Expect(ids(frame.Series)).To(Equal([]string{"profile:pod-cpu#pod=a", "profile:pod-cpu#pod=b"}))
		Expect(frame.Series[1].Source).To(Equal("profile:pod-cpu"))
	})

	It("reports series that disappeared as removed", func() {
		src := &funcSource{next: func(call int32, _ metrics.SeriesRequest) ([]metrics.Series, error) {
			series := []metrics.Series{{ID: "a", Source: "a", Points: []metrics.Point{}}}
			if call == 1 {
				series = append(series, metrics.Series{ID: "b", Source: "b", Points: []metrics.Point{}})
			}
			return series, nil
		}}
		srv := streamServer(src)

		_, next := openStream(srv, url.Values{"id": {"a", "b"}, "interval": {"1s"}})
		Expect(next().Frame.Series).To(HaveLen(2))
		ev := next()
		Expect(ev.Name).To(Equal("append"))
		Expect(ev.Frame).To(Equal(metrics.StreamFrame{Series: []metrics.Series{}, Removed: []string{"b"}}))
	})

	It("sends a query failure after the stream started as an error event", func() {
		src := &funcSource{next: func(call int32, _ metrics.SeriesRequest) ([]metrics.Series, error) {
			if call > 1 {
				return nil, context.DeadlineExceeded
			}
			return []metrics.Series{{ID: "a", Source: "a", Points: []metrics.Point{}}}, nil
		}}
		srv := streamServer(src)

		_, next := openStream(srv, url.Values{"id": {"a"}, "interval": {"1s"}})
		Expect(next().Name).To(Equal("snapshot"))
		Expect(next().Error).To(ContainSubstring("deadline exceeded"))
	})

	DescribeTable("rejects a request before streaming",
		func(src metrics.SeriesSource, query url.Values) {
			srv := streamServer(src)
			resp, err := http.Get(srv.URL + "/api/v1/metrics/stream?" + query.Encode())
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		},
		Entry("no id", metrics.Sources{}, url.Values{}),
		Entry("an id no source owns", metrics.Sources{"profile": metrics.Sources{}}, url.Values{"id": {"cloudwatch:cpu"}}),
		Entry("an interval below one second", metrics.Sources{}, url.Values{"id": {"a"}, "interval": {"10ms"}}),
		Entry("a non-positive step", metrics.Sources{}, url.Values{"id": {"a"}, "step": {"0s"}}),
		Entry("since after until", metrics.Sources{}, url.Values{"id": {"a"}, "since": {"2026-06-02T12:00:00Z"}, "until": {"2026-06-01T12:00:00Z"}}),
	)
})
