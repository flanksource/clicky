package valkey_test

import (
	"time"

	"github.com/alicebob/miniredis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	valkeygo "github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/clicky/valkey"
)

var base = time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

func newClient() (valkeygo.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	Expect(err).NotTo(HaveOccurred())
	client, err := valkeygo.NewClient(valkeygo.ClientOption{
		InitAddress:  []string{mr.Addr()},
		DisableCache: true,
	})
	Expect(err).NotTo(HaveOccurred())
	return client, mr
}

var _ = Describe("Valkey Timeseries", func() {
	var (
		client valkeygo.Client
		mr     *miniredis.Miniredis
		ts     metrics.Timeseries
	)

	BeforeEach(func() {
		client, mr = newClient()
		ts = valkey.New(client, valkey.Config{KeyPrefix: "oipa:", Retention: time.Hour})
	})

	AfterEach(func() {
		client.Close()
		mr.Close()
	})

	It("round-trips recorded points within a query range, ascending", func() {
		for i, v := range []float64{1, 2, 3, 4, 5} {
			Expect(ts.Record(metrics.RecordRequest{
				ID:    "cpu",
				At:    base.Add(time.Duration(i) * time.Minute),
				Value: v,
			})).To(Succeed())
		}

		got, err := ts.Query(metrics.QueryRequest{
			ID:    "cpu",
			Since: base.Add(time.Minute),
			Until: base.Add(3 * time.Minute),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]metrics.Point{
			{At: base.Add(time.Minute), Value: 2},
			{At: base.Add(2 * time.Minute), Value: 3},
			{At: base.Add(3 * time.Minute), Value: 4},
		}))
	})

	It("trims points older than the retention window on record", func() {
		ts = valkey.New(client, valkey.Config{KeyPrefix: "oipa:", Retention: 10 * time.Minute})
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base.Add(-time.Hour), Value: 1})).To(Succeed())
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base, Value: 2})).To(Succeed())

		got, err := ts.Query(metrics.QueryRequest{ID: "cpu"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]metrics.Point{{At: base, Value: 2}}))
	})

	It("sets an expiry on the metric key", func() {
		Expect(ts.Record(metrics.RecordRequest{ID: "cpu", At: base, Value: 1})).To(Succeed())
		Expect(mr.TTL("oipa:metric:cpu")).To(Equal(time.Hour))
	})

	It("returns an empty slice for an unknown metric", func() {
		got, err := ts.Query(metrics.QueryRequest{ID: "missing"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeEmpty())
	})
})
