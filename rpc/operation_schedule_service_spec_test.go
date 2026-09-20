package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/route"
	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type scheduleContextKey string

const environmentContextKey scheduleContextKey = "environment"

type testOperationScheduleContextAdapter struct {
	cleanups atomic.Int64
}

func (a *testOperationScheduleContextAdapter) Capture(ctx context.Context) (map[string]string, error) {
	return map[string]string{"environment": ctx.Value(environmentContextKey).(string)}, nil
}

func (a *testOperationScheduleContextAdapter) Rehydrate(
	ctx context.Context,
	metadata map[string]string,
) (context.Context, func(), error) {
	rehydrated := context.WithValue(ctx, environmentContextKey, "rehydrated:"+metadata["environment"])
	return rehydrated, func() { a.cleanups.Add(1) }, nil
}

type testOperationScheduleStore struct {
	mu        sync.Mutex
	schedules map[string]task.Schedule
}

func newOperationScheduleStore() *testOperationScheduleStore {
	return &testOperationScheduleStore{schedules: map[string]task.Schedule{}}
}

func (s *testOperationScheduleStore) ListSchedules(context.Context) ([]task.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]task.Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		result = append(result, schedule)
	}
	return result, nil
}

func (s *testOperationScheduleStore) SaveSchedule(_ context.Context, schedule task.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schedules[schedule.Name] = schedule
	return nil
}

func (s *testOperationScheduleStore) DeleteSchedule(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, name)
	return nil
}

func (*testOperationScheduleStore) RecordFire(context.Context, string, task.Fire) error { return nil }

var _ = Describe("generated operation schedules", func() {
	It("captures context metadata and rehydrates it for manual and recurring execution", func(ctx SpecContext) {
		invocations := make(chan string, 2)
		operation := RPCOperation{
			Name: "database compact",
			Schema: entity.Schema{Type: "object", Properties: map[string]entity.Property{
				"batch-size": {Type: "integer"},
			}},
			Parameters: []RPCParameter{{Name: "batch-size", Type: "integer"}},
			ContextDataFunc: func(runCtx context.Context, flags map[string]string, _ []string) (any, error) {
				invocations <- runCtx.Value(environmentContextKey).(string) + ":" + flags["batch-size"]
				return nil, nil
			},
			Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{Timeout: 5 * time.Minute}},
		}
		executor := NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true})
		adapter := &testOperationScheduleContextAdapter{}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{
			Store: newOperationScheduleStore(), Executor: executor, ContextAdapter: adapter,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Load(ctx)).To(Succeed())
		requestContext := context.WithValue(ctx, environmentContextKey, "dev")

		run, err := service.RunNow(requestContext, OperationRunInput{
			OperationID: "database_compact", Args: map[string]any{"batch-size": 250},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(run.RunID).NotTo(BeEmpty())
		Eventually(invocations).Should(Receive(Equal("rehydrated:dev:250")))
		Eventually(adapter.cleanups.Load).Should(BeEquivalentTo(1))

		created, err := service.Create(requestContext, OperationScheduleInput{
			Name: "Weekly compact", OperationID: "database_compact",
			Args: map[string]any{"batch-size": 500}, Cron: "0 2 * * 0", Timezone: "UTC", Enabled: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(created.ID).NotTo(BeEmpty())
		Expect(created.Name).To(Equal("Weekly compact"))

		savedRun, err := service.Run(requestContext, created.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(savedRun.RunID).NotTo(BeEmpty())
		Eventually(invocations).Should(Receive(Equal("rehydrated:dev:500")))
		Eventually(adapter.cleanups.Load).Should(BeEquivalentTo(2))

		schedules, err := service.List(requestContext)
		Expect(err).NotTo(HaveOccurred())
		Expect(schedules).To(HaveLen(1))
		Expect(schedules[0].ID).To(Equal(created.ID))
		Expect(schedules[0].Name).To(Equal("Weekly compact"))
		Expect(schedules[0].OperationID).To(Equal("database_compact"))
		Expect(schedules[0].LastRun).NotTo(BeNil())
	})

	It("rejects unknown, unschedulable, and invalid operation arguments", func(ctx SpecContext) {
		executor := NewCommandExecutor(&RPCService{Operations: []RPCOperation{
			{Name: "scheduled", Parameters: []RPCParameter{{Name: "count", Type: "integer", Required: true}}, Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}},
			{Name: "ordinary", Clicky: &ClickyOperationMeta{}},
		}}, &ExecutorConfig{Enabled: true})
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{
			Store: newOperationScheduleStore(), Executor: executor,
			ContextAdapter: &testOperationScheduleContextAdapter{},
		})
		Expect(err).NotTo(HaveOccurred())
		requestContext := context.WithValue(ctx, environmentContextKey, "dev")

		_, err = service.RunNow(requestContext, OperationRunInput{OperationID: "missing"})
		Expect(err).To(MatchError(ContainSubstring("operation not found")))
		_, err = service.RunNow(requestContext, OperationRunInput{OperationID: "ordinary"})
		Expect(err).To(MatchError(ContainSubstring("not schedulable")))
		_, err = service.RunNow(requestContext, OperationRunInput{OperationID: "scheduled"})
		Expect(err).To(MatchError(ContainSubstring("required parameter count is missing")))
		_, err = service.RunNow(requestContext, OperationRunInput{OperationID: "scheduled", Args: map[string]any{"other": true}})
		Expect(err).To(MatchError(ContainSubstring("unknown argument")))
		_, err = service.RunNow(requestContext, OperationRunInput{OperationID: "scheduled", Args: map[string]any{"count": "many"}})
		Expect(err).To(MatchError(ContainSubstring("invalid type")))
	})

	It("publishes reusable schedule routes and OpenAPI", func(ctx SpecContext) {
		executor := NewCommandExecutor(&RPCService{Operations: []RPCOperation{{
			Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}},
		}}}, &ExecutorConfig{Enabled: true})
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{
			Store: newOperationScheduleStore(), Executor: executor,
			ContextAdapter: &testOperationScheduleContextAdapter{},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Load(ctx)).To(Succeed())
		mux := http.NewServeMux()
		service.RegisterRoutes(route.NewRouter(mux))

		body, err := json.Marshal(OperationScheduleInput{
			Name: "Nightly", OperationID: "scheduled", Args: map[string]any{},
			Cron: "0 1 * * *", Timezone: "UTC", Enabled: true,
		})
		Expect(err).NotTo(HaveOccurred())
		request := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), environmentContextKey, "dev"))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		Expect(response.Code).To(Equal(http.StatusCreated), response.Body.String())

		listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
		listRequest = listRequest.WithContext(context.WithValue(listRequest.Context(), environmentContextKey, "dev"))
		listResponse := httptest.NewRecorder()
		mux.ServeHTTP(listResponse, listRequest)
		Expect(listResponse.Code).To(Equal(http.StatusOK))
		Expect(listResponse.Body.String()).To(ContainSubstring(`"operationId":"scheduled"`))

		spec := &OpenAPISpec{Paths: map[string]OpenAPIPath{}}
		AddOperationScheduleOpenAPI(spec)
		Expect(spec.Paths).To(HaveKey("/api/v1/schedules"))
		Expect(spec.Paths).To(HaveKey("/api/v1/schedules/{id}/run"))
		Expect(spec.Paths).To(HaveKey("/api/v1/schedules/run-now"))
	})
})
