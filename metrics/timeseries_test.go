package metrics_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/metrics"
)

// base is a fixed reference time so specs never depend on wall-clock now.
var base = time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

func pointsAt(start time.Time, step time.Duration, values ...float64) []metrics.Point {
	out := make([]metrics.Point, len(values))
	for i, v := range values {
		out[i] = metrics.Point{At: start.Add(time.Duration(i) * step), Value: v}
	}
	return out
}

var _ = Describe("Member codec", func() {
	It("round-trips a point losslessly", func() {
		p := metrics.Point{At: base, Value: 42.5}
		decoded, err := metrics.ParseMember(metrics.EncodeMember(p))
		Expect(err).NotTo(HaveOccurred())
		Expect(decoded.At.UnixMilli()).To(Equal(p.At.UnixMilli()))
		Expect(decoded.Value).To(Equal(p.Value))
	})

	It("rejects a malformed member", func() {
		_, err := metrics.ParseMember("not-a-member")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("In-memory Timeseries", func() {
	It("returns only points inside the queried range, ascending", func() {
		ts := metrics.NewMemory(metrics.MemoryConfig{Retention: time.Hour, MaxPoints: 100})
		for _, p := range pointsAt(base, time.Minute, 1, 2, 3, 4, 5) {
			Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: p.At, Value: p.Value})).To(Succeed())
		}

		got, err := ts.Query(metrics.QueryRequest{
			ID:    "cpu",
			Since: base.Add(time.Minute),
			Until: base.Add(3 * time.Minute),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(pointsAt(base.Add(time.Minute), time.Minute, 2, 3, 4)))
	})

	It("drops points older than the retention window on record", func() {
		ts := metrics.NewMemory(metrics.MemoryConfig{Retention: 10 * time.Minute, MaxPoints: 100})
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base.Add(-time.Hour), Value: 1})).To(Succeed())
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base, Value: 2})).To(Succeed())

		got, err := ts.Query(metrics.QueryRequest{ID: "cpu"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]metrics.Point{{At: base, Value: 2}}))
	})

	It("caps retained points at MaxPoints, keeping the newest", func() {
		ts := metrics.NewMemory(metrics.MemoryConfig{Retention: time.Hour, MaxPoints: 3})
		for _, p := range pointsAt(base, time.Second, 1, 2, 3, 4, 5) {
			Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: p.At, Value: p.Value})).To(Succeed())
		}
		got, err := ts.Query(metrics.QueryRequest{ID: "cpu"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(pointsAt(base.Add(2*time.Second), time.Second, 3, 4, 5)))
	})
})

var _ = Describe("HTTP handler", func() {
	newServer := func() (*httptest.Server, metrics.Timeseries) {
		ts := metrics.NewMemory(metrics.MemoryConfig{Retention: time.Hour, MaxPoints: 100})
		mux := http.NewServeMux()
		metrics.RegisterRoutes(mux, ts, "/api/v1")
		return httptest.NewServer(mux), ts
	}

	It("serves recorded points as a JSON envelope", func() {
		srv, ts := newServer()
		defer srv.Close()
		now := time.Now()
		// Record in the recent past so all points fall inside the default
		// [now-1h, now] query window.
		for i, v := range []float64{10, 20, 30} {
			Expect(ts.Record(metrics.RecordRequest{
				ID:    "sqlserver.cpu",
				At:    now.Add(-time.Duration(3-i) * time.Second),
				Value: v,
			})).To(Succeed())
		}

		resp, err := http.Get(srv.URL + "/api/v1/metrics/sqlserver.cpu?since=1h")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var body struct {
			ID     string          `json:"id"`
			Points []metrics.Point `json:"points"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		Expect(body.ID).To(Equal("sqlserver.cpu"))
		Expect(body.Points).To(HaveLen(3))
		Expect(body.Points[2].Value).To(Equal(30.0))
	})

	It("rejects an unparseable since bound", func() {
		srv, _ := newServer()
		defer srv.Close()
		resp, err := http.Get(srv.URL + "/api/v1/metrics/cpu?since=nonsense")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})
})
