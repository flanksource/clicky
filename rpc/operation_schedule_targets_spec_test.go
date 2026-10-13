package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("generated operation schedules target lifecycle", func() {
	It("migrates a persisted singleton context and saves the explicit target payload", func(ctx SpecContext) {
		store := newOperationScheduleStore()
		store.schedules["weekly"] = task.Schedule{Name: "weekly", Title: "Weekly", Kind: OperationScheduleKind, Cron: "0 1 * * *", Timeout: time.Minute, Labels: labelsWithContext("scheduled", map[string]string{"environment": "dev"}), Payload: json.RawMessage(`{"operationId":"scheduled","args":{}}`)}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{Store: store, ContextAdapter: &testOperationScheduleContextAdapter{}, Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}}}}, &ExecutorConfig{Enabled: true})})
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Load(ctx)).To(Succeed())
		payload, err := decodeOperationSchedulePayload(store.schedules["weekly"].Payload)
		Expect(err).NotTo(HaveOccurred())
		Expect(payload.Targets).To(Equal([]string{"dev"}))
		Expect(payload.Contexts).To(Equal([]map[string]string{{"environment": "dev"}}))
		Expect(store.schedules["weekly"].Timeout).To(BeZero())
	})

	It("gives each target its own timeout and releases timed out leases", func(ctx SpecContext) {
		adapter := &testOperationScheduleContextAdapter{}
		visited := make(chan string, 2)
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{Timeout: 20 * time.Millisecond}}, ContextDataFunc: func(targetCtx context.Context, _ map[string]string, _ []string) (any, error) {
			visited <- targetCtx.Value(environmentContextKey).(string)
			<-targetCtx.Done()
			return nil, targetCtx.Err()
		}}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{Store: newOperationScheduleStore(), ContextAdapter: adapter, Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true})})
		Expect(err).NotTo(HaveOccurred())
		payload, err := json.Marshal(operationSchedulePayload{OperationID: "scheduled", Targets: []string{"dev", "qa"}, Contexts: []map[string]string{{"environment": "dev"}, {"environment": "qa"}}})
		Expect(err).NotTo(HaveOccurred())
		group := task.StartGroup[any]("target timeouts")
		defer group.Cancel()
		err = service.runScheduledOperation(flanksourceContext.NewContext(ctx), task.Schedule{Payload: payload}, group.Group)
		Expect(err).To(MatchError(ContainSubstring(`target "dev"`)))
		Expect(err).To(MatchError(ContainSubstring(`target "qa"`)))
		Expect(visited).To(Receive(Equal("rehydrated:dev")))
		Expect(visited).To(Receive(Equal("rehydrated:qa")))
		Expect(adapter.cleanups.Load()).To(BeEquivalentTo(2))
	})

	It("stops pending targets when the run is cancelled", func(ctx SpecContext) {
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		adapter := &testOperationScheduleContextAdapter{}
		visited := []string{}
		operation := RPCOperation{Name: "scheduled", Clicky: &ClickyOperationMeta{Schedule: &entity.OperationScheduleMeta{}}, ContextDataFunc: func(targetCtx context.Context, _ map[string]string, _ []string) (any, error) {
			visited = append(visited, targetCtx.Value(environmentContextKey).(string))
			cancel()
			return nil, targetCtx.Err()
		}}
		service, err := NewOperationScheduleService(OperationScheduleServiceOptions{Store: newOperationScheduleStore(), ContextAdapter: adapter, Executor: NewCommandExecutor(&RPCService{Operations: []RPCOperation{operation}}, &ExecutorConfig{Enabled: true})})
		Expect(err).NotTo(HaveOccurred())
		payload, err := json.Marshal(operationSchedulePayload{OperationID: "scheduled", Targets: []string{"dev", "qa"}, Contexts: []map[string]string{{"environment": "dev"}, {"environment": "qa"}}})
		Expect(err).NotTo(HaveOccurred())
		group := task.StartGroup[any]("target cancellation")
		defer group.Cancel()
		Expect(service.runScheduledOperation(flanksourceContext.NewContext(runCtx), task.Schedule{Payload: payload}, group.Group)).To(HaveOccurred())
		Expect(visited).To(Equal([]string{"rehydrated:dev"}))
		Expect(adapter.cleanups.Load()).To(BeEquivalentTo(1))
	})
})
