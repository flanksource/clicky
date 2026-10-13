package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func (a *testOperationScheduleContextAdapter) Capture(ctx context.Context, target string) (map[string]string, error) {
	if target == "" {
		target = ctx.Value(environmentContextKey).(string)
		if target == "global" {
			return map[string]string{}, nil
		}
	}
	return map[string]string{"environment": target}, nil
}

func (a *testOperationScheduleContextAdapter) Target(metadata map[string]string) (string, error) {
	return metadata["environment"], nil
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
	It("continues after a target failure, releases every lease, and retains identity after request cancellation", func(ctx SpecContext) {
		started := make(chan struct{})
		release := make(chan struct{})
		invocations := make(chan string, 2)
		adapter := &testOperationScheduleContextAdapter{}
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{Timeout: time.Minute}}, ContextDataFunc: func(runCtx context.Context, _ map[string]string, _ []string) (any, error) {
			environment := runCtx.Value(environmentContextKey).(string)
			if environment == "rehydrated:dev" {
				close(started)
				<-release
			}
			invocations <- environment + ":" + runCtx.Value(scheduleContextKey("identity")).(string)
			if environment == "rehydrated:dev" {
				return nil, fmt.Errorf("dev failed")
			}
			return nil, runCtx.Err()
		}}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{Store: newOperationScheduleStore(), Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true}), ContextAdapter: adapter})
		Expect(err).NotTo(HaveOccurred())
		request, cancel := context.WithCancel(context.WithValue(ctx, scheduleContextKey("identity"), "operator"))
		defer cancel()
		_, err = service.RunNow(request, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}})
		Expect(err).NotTo(HaveOccurred())
		Eventually(started).Should(BeClosed())
		cancel()
		close(release)
		Eventually(invocations).Should(Receive(Equal("rehydrated:dev:operator")))
		Eventually(invocations).Should(Receive(Equal("rehydrated:qa:operator")))
		Eventually(adapter.cleanups.Load).Should(BeEquivalentTo(2))
	})
	It("rejects a forbidden target before saving any schedule", func(ctx SpecContext) {
		store := newOperationScheduleStore()
		adapter := &testOperationScheduleContextAdapter{}
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{Store: store, Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true}), ContextAdapter: adapter, Authorize: func(targetCtx context.Context, _ *RPCOperation) error {
			if targetCtx.Value(environmentContextKey) == "rehydrated:qa" {
				return fmt.Errorf("qa forbidden")
			}
			return nil
		}})
		Expect(err).NotTo(HaveOccurred())
		_, err = service.Create(ctx, OperationScheduleInput{Name: "Denied", OperationID: "scheduled", Targets: []string{"dev", "qa"}, Cron: "0 1 * * *"})
		Expect(err).To(MatchError(ContainSubstring("qa forbidden")))
		Expect(store.schedules).To(BeEmpty())
		Expect(adapter.cleanups.Load()).To(BeEquivalentTo(2))
	})
	It("persists one schedule with multiple targets and exposes it in each target context", func(ctx SpecContext) {
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{
			Store: newOperationScheduleStore(), ContextAdapter: &testOperationScheduleContextAdapter{},
			Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}}}}, &ExecutorConfig{Enabled: true}),
		})
		Expect(err).NotTo(HaveOccurred())
		created, err := service.Create(context.WithValue(ctx, environmentContextKey, "dev"), OperationScheduleInput{
			Name: "Multiple environments", OperationID: "scheduled", Targets: []string{"dev", "qa"}, Cron: "0 1 * * *", Enabled: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(created.Targets).To(Equal([]string{"dev", "qa"}))
		for _, name := range []string{"dev", "qa", "global"} {
			listed, err := service.List(context.WithValue(ctx, environmentContextKey, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(listed).To(HaveLen(1))
			Expect(listed[0].ID).To(Equal(created.ID))
		}
		listed, err := service.List(context.WithValue(ctx, environmentContextKey, "other"))
		Expect(err).NotTo(HaveOccurred())
		Expect(listed).To(BeEmpty())
	})
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
			OperationID: "database_compact", Targets: []string{"dev"}, Args: map[string]any{"batch-size": 250},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(run.RunID).NotTo(BeEmpty())
		Eventually(invocations).Should(Receive(Equal("rehydrated:dev:250")))
		Eventually(adapter.cleanups.Load).Should(BeEquivalentTo(1))

		created, err := service.Create(requestContext, OperationScheduleInput{
			Name: "Weekly compact", OperationID: "database_compact", Targets: []string{"dev"},
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
			Name: "Nightly", OperationID: "scheduled", Targets: []string{"dev"}, Args: map[string]any{},
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
		concurrency := HaveKeyWithValue("concurrency", HaveField("Type", "integer"))
		Expect(operationScheduleInputSchema().Properties).To(concurrency)
		Expect(operationRunInputSchema().Properties).To(concurrency)
		Expect(operationScheduleSchema().Properties).To(concurrency)
	})
})
