package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// blockingTargets is an operation whose every invocation reports the target it
// runs in and then holds until released, so a spec can observe which targets
// run side by side.
type blockingTargets struct {
	started chan string
	release chan struct{}
}

func newBlockingTargets() *blockingTargets {
	return &blockingTargets{started: make(chan string, 4), release: make(chan struct{})}
}

func (b *blockingTargets) operation(timeout time.Duration) RPCOperation {
	return RPCOperation{
		Name:   "scheduled",
		Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{Timeout: timeout}},
		ContextDataFunc: func(runCtx context.Context, _ map[string]string, _ []string) (any, error) {
			b.started <- runCtx.Value(environmentContextKey).(string)
			select {
			case <-b.release:
				return nil, nil
			case <-runCtx.Done():
				return nil, runCtx.Err()
			}
		},
	}
}

func newTargetScheduleService(
	store task.ScheduleStore,
	adapter *testOperationScheduleContextAdapter,
	operation RPCOperation,
) *OperationScheduleService {
	service, err := NewOperationScheduleService(OperationScheduleServiceOptions{
		Store: store, ContextAdapter: adapter,
		Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true}),
	})
	Expect(err).NotTo(HaveOccurred())
	return service
}

// runTargetTasks returns the run's per-target tasks keyed by target name.
func runTargetTasks(runID string) map[string]task.TaskSnapshot {
	tasks := map[string]task.TaskSnapshot{}
	for _, snapshot := range task.SnapshotByID(runID) {
		if snapshot.Type == "task" && snapshot.Name != OperationScheduleKind {
			tasks[snapshot.Name] = snapshot
		}
	}
	return tasks
}

func runStatus(runID string) func() string {
	return func() string {
		for _, snapshot := range task.SnapshotByID(runID) {
			if snapshot.Type == "group" {
				return snapshot.Status
			}
		}
		return ""
	}
}

func targetStatus(runID, target string) func() string {
	return func() string { return runTargetTasks(runID)[target].Status }
}

// expectOneTargetAtATime asserts that only one of dev and qa runs while the
// other waits pending, then lets both finish.
func expectOneTargetAtATime(runID string, targets *blockingTargets) {
	var first string
	Eventually(targets.started).Should(Receive(&first))
	Consistently(targets.started, 300*time.Millisecond).ShouldNot(Receive())
	other := map[string]string{"rehydrated:dev": "qa", "rehydrated:qa": "dev"}[first]
	Expect(other).NotTo(BeEmpty(), "unexpected first target %q", first)
	Expect(targetStatus(runID, other)()).To(Equal(string(task.StatusPending)))

	targets.release <- struct{}{}
	Eventually(targets.started).Should(Receive(Equal("rehydrated:" + other)))
	targets.release <- struct{}{}
	Eventually(runStatus(runID)).Should(Equal(string(task.StatusSuccess)))
}

func twoTargetPayload() json.RawMessage {
	payload, err := json.Marshal(operationSchedulePayload{
		OperationID: "scheduled", Targets: []string{"dev", "qa"}, Args: map[string]any{},
		Contexts: []map[string]string{{"environment": "dev"}, {"environment": "qa"}},
	})
	Expect(err).NotTo(HaveOccurred())
	return payload
}

var _ = Describe("generated operation schedules target lifecycle", func() {
	It("migrates a persisted singleton context and saves the explicit target payload", func(ctx SpecContext) {
		store := newOperationScheduleStore()
		store.schedules["weekly"] = task.Schedule{Name: "weekly", Title: "Weekly", Kind: OperationScheduleKind, Cron: "0 1 * * *", Timeout: time.Minute, Labels: labelsWithContext("scheduled", map[string]string{"environment": "dev"}), Payload: json.RawMessage(`{"operationId":"scheduled","args":{}}`)}
		service := newTargetScheduleService(store, &testOperationScheduleContextAdapter{}, RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}})
		Expect(service.Load(ctx)).To(Succeed())
		payload, err := decodeOperationSchedulePayload(store.schedules["weekly"].Payload)
		Expect(err).NotTo(HaveOccurred())
		Expect(payload.Targets).To(Equal([]string{"dev"}))
		Expect(payload.Contexts).To(Equal([]map[string]string{{"environment": "dev"}}))
		Expect(store.schedules["weekly"].Timeout).To(BeZero())
	})

	It("runs a legacy multi-target schedule saved without a concurrency one target at a time", func(ctx SpecContext) {
		store := newOperationScheduleStore()
		store.schedules["legacy"] = task.Schedule{Name: "legacy", Title: "Legacy", Kind: OperationScheduleKind, Cron: "0 1 * * *", Labels: labelsWithContext("scheduled", nil), Payload: twoTargetPayload()}
		targets := newBlockingTargets()
		service := newTargetScheduleService(store, &testOperationScheduleContextAdapter{}, targets.operation(0))
		Expect(service.Load(ctx)).To(Succeed())
		devCtx := context.WithValue(ctx, environmentContextKey, "dev")

		listed, err := service.List(devCtx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listed).To(HaveLen(1))
		Expect(listed[0].Concurrency).To(Equal(1))

		run, err := service.Run(devCtx, "legacy")
		Expect(err).NotTo(HaveOccurred())
		expectOneTargetAtATime(run.RunID, targets)
	})

	It("runs one target at a time unless asked for more", func(ctx SpecContext) {
		targets := newBlockingTargets()
		service := newTargetScheduleService(newOperationScheduleStore(), &testOperationScheduleContextAdapter{}, targets.operation(0))
		run, err := service.RunNow(ctx, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}})
		Expect(err).NotTo(HaveOccurred())
		expectOneTargetAtATime(run.RunID, targets)
	})

	It("runs as many targets side by side as the concurrency allows", func(ctx SpecContext) {
		targets := newBlockingTargets()
		service := newTargetScheduleService(newOperationScheduleStore(), &testOperationScheduleContextAdapter{}, targets.operation(0))
		run, err := service.RunNow(ctx, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}, Concurrency: 2})
		Expect(err).NotTo(HaveOccurred())

		started := []string{}
		for range 2 {
			var target string
			Eventually(targets.started).Should(Receive(&target))
			started = append(started, target)
		}
		Expect(started).To(ConsistOf("rehydrated:dev", "rehydrated:qa"))
		Expect(targetStatus(run.RunID, "dev")()).To(Equal(string(task.StatusRunning)))
		Expect(targetStatus(run.RunID, "qa")()).To(Equal(string(task.StatusRunning)))
		close(targets.release)
		Eventually(runStatus(run.RunID)).Should(Equal(string(task.StatusSuccess)))
	})

	It("echoes the concurrency, defaulting to one, and rejects one outside 1..targets", func(ctx SpecContext) {
		service := newTargetScheduleService(newOperationScheduleStore(), &testOperationScheduleContextAdapter{}, newBlockingTargets().operation(0))
		devCtx := context.WithValue(ctx, environmentContextKey, "dev")
		input := OperationScheduleInput{Name: "Nightly", OperationID: "scheduled", Targets: []string{"dev", "qa"}, Cron: "0 1 * * *"}

		sequential, err := service.Create(devCtx, input)
		Expect(err).NotTo(HaveOccurred())
		input.Concurrency = 2
		parallel, err := service.Create(devCtx, input)
		Expect(err).NotTo(HaveOccurred())
		Expect([]int{sequential.Concurrency, parallel.Concurrency}).To(Equal([]int{1, 2}))

		listed, err := service.List(devCtx)
		Expect(err).NotTo(HaveOccurred())
		concurrency := map[string]int{}
		for _, schedule := range listed {
			concurrency[schedule.ID] = schedule.Concurrency
		}
		Expect(concurrency).To(Equal(map[string]int{sequential.ID: 1, parallel.ID: 2}))

		for _, invalid := range []int{-1, 3} {
			input.Concurrency = invalid
			_, err = service.Create(devCtx, input)
			Expect(err).To(MatchError(ErrOperationScheduleInvalid), "concurrency %d", invalid)
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("concurrency %d", invalid))))
			_, err = service.RunNow(devCtx, OperationRunInput{OperationID: "scheduled", Targets: input.Targets, Concurrency: invalid})
			Expect(err).To(MatchError(ErrOperationScheduleInvalid), "concurrency %d", invalid)
		}
	})

	It("fails a target that outlives its timeout and releases its lease", func(ctx SpecContext) {
		adapter := &testOperationScheduleContextAdapter{}
		targets := newBlockingTargets()
		service := newTargetScheduleService(newOperationScheduleStore(), adapter, targets.operation(50*time.Millisecond))
		run, err := service.RunNow(ctx, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}, Concurrency: 2})
		Expect(err).NotTo(HaveOccurred())

		Eventually(runStatus(run.RunID)).Should(Equal(string(task.StatusFailed)))
		for _, target := range []string{"dev", "qa"} {
			Expect(runTargetTasks(run.RunID)[target]).To(And(
				HaveField("Status", string(task.StatusFailed)),
				HaveField("Error", ContainSubstring("timed out after 50ms")),
			), target)
		}
		Eventually(adapter.cleanups.Load).Should(BeEquivalentTo(2))
	})

	It("starts a target's timeout when the target starts, not while it waits for a slot", func(ctx SpecContext) {
		const work, timeout = 250 * time.Millisecond, 400 * time.Millisecond
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{Timeout: timeout}}, ContextDataFunc: func(runCtx context.Context, _ map[string]string, _ []string) (any, error) {
			select {
			case <-time.After(work):
				return nil, nil
			case <-runCtx.Done():
				return nil, runCtx.Err()
			}
		}}
		service := newTargetScheduleService(newOperationScheduleStore(), &testOperationScheduleContextAdapter{}, operation)
		run, err := service.RunNow(ctx, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}})
		Expect(err).NotTo(HaveOccurred())
		Eventually(runStatus(run.RunID), 2*time.Second).Should(Equal(string(task.StatusSuccess)))
	})

	It("fails the run when one target fails, without retrying it, while the other target still runs", func(ctx SpecContext) {
		var devCalls atomic.Int64
		release := make(chan struct{})
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}, ContextDataFunc: func(runCtx context.Context, _ map[string]string, _ []string) (any, error) {
			if runCtx.Value(environmentContextKey) == "rehydrated:dev" {
				devCalls.Add(1)
				return nil, fmt.Errorf("connection reset by peer")
			}
			<-release
			return nil, nil
		}}
		service := newTargetScheduleService(newOperationScheduleStore(), &testOperationScheduleContextAdapter{}, operation)
		run, err := service.RunNow(ctx, OperationRunInput{OperationID: "scheduled", Targets: []string{"dev", "qa"}, Concurrency: 2})
		Expect(err).NotTo(HaveOccurred())

		Eventually(targetStatus(run.RunID, "dev")).Should(Equal(string(task.StatusFailed)))
		Eventually(targetStatus(run.RunID, "qa")).Should(Equal(string(task.StatusRunning)))
		close(release)
		Eventually(runStatus(run.RunID)).Should(Equal(string(task.StatusFailed)))
		Expect(targetStatus(run.RunID, "qa")()).To(Equal(string(task.StatusSuccess)))
		Expect(runTargetTasks(run.RunID)["dev"].Error).To(ContainSubstring("connection reset by peer"))
		Consistently(devCalls.Load, 300*time.Millisecond).Should(BeEquivalentTo(1))
	})

	It("stops pending targets when the run is cancelled", func(ctx SpecContext) {
		adapter := &testOperationScheduleContextAdapter{}
		group := task.StartGroup[any]("target cancellation", task.WithConcurrency(1))
		defer group.Cancel()
		var mu sync.Mutex
		visited := []string{}
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}, ContextDataFunc: func(targetCtx context.Context, _ map[string]string, _ []string) (any, error) {
			mu.Lock()
			visited = append(visited, targetCtx.Value(environmentContextKey).(string))
			mu.Unlock()
			group.Cancel()
			return nil, targetCtx.Err()
		}}
		service := newTargetScheduleService(newOperationScheduleStore(), adapter, operation)
		Expect(service.runScheduledOperation(flanksourceContext.NewContext(ctx), task.Schedule{Payload: twoTargetPayload()}, group.Group)).To(Succeed())

		Eventually(group.Status).Should(Equal(task.StatusCancelled))
		Consistently(func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(visited)
		}, 200*time.Millisecond).Should(Equal(1))
		Expect(adapter.cleanups.Load()).To(BeEquivalentTo(1))
	})
})
