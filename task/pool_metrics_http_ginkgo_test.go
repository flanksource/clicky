package task_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/clicky/route"
	"github.com/flanksource/clicky/task"
)

var _ = Describe("pool metrics over the task metrics routes", func() {
	It("serves sampled worker-pool gauges once the task API is registered", func() {
		mux := http.NewServeMux()
		task.RegisterHandlers(route.NewRouter(mux), "/api")
		id := task.MetricID(task.PoolMetricRunID, task.MetricWorkersLimit)

		points := func() []metrics.Point {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/tasks/metrics/"+id+"?since=1m", nil))
			Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
			var body struct {
				ID     string          `json:"id"`
				Points []metrics.Point `json:"points"`
			}
			Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
			Expect(body.ID).To(Equal(id))
			return body.Points
		}
		Eventually(points, 3*time.Second, 100*time.Millisecond).ShouldNot(BeEmpty())
		Expect(points()[0].Value).To(BeNumerically(">", 0), "a worker pool always has a positive limit")
	})
})
