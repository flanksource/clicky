package task_test

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resumable retry", func() {
	It("exposes the cause and passes user input to a resumed attempt after automatic retries are exhausted", func() {
		cause := errors.New("the selected configuration is invalid")
		message := "The configuration has been corrected; use the new settings."
		var requests []task.RetryRequest
		config := task.RetryConfig{MaxRetries: 0, OnError: func(_ context.Context, request task.RetryRequest) (task.RetryDecision, error) {
			requests = append(requests, request)
			return task.RetryDecision{Action: task.RetryResume, Message: message}, nil
		}}
		calls := 0
		err := task.Retry(context.Background(), config, func(ctx context.Context, attempt int) error {
			calls++
			if attempt == 1 {
				return cause
			}
			Expect(task.RetryMessage(ctx)).To(Equal(message))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(2))
		Expect(requests).To(HaveLen(1))
		Expect(errors.Is(requests[0].Cause, cause)).To(BeTrue())
		Expect(requests[0].Error).To(Equal(cause.Error()))
		Expect(requests[0].Automatic).To(BeFalse())
	})
	It("uses the same response seam for tracked tasks", func() {
		var calls atomic.Int32
		config := task.RetryConfig{OnError: func(context.Context, task.RetryRequest) (task.RetryDecision, error) {
			return task.RetryDecision{Action: task.RetryResume, Message: "continue"}, nil
		}}
		handle := task.StartTask("Resumable operation", func(ctx commonscontext.Context, _ *task.Task) (string, error) {
			if calls.Add(1) == 1 {
				return "", errors.New("operator input required")
			}
			return task.RetryMessage(ctx), nil
		}, task.WithRetryConfig(config))
		result, err := handle.GetResult()
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("continue"))
		Expect(calls.Load()).To(Equal(int32(2)))
	})
	It("preserves the original cause when the operator declines to retry", func() {
		cause := errors.New("invalid project settings")
		config := task.RetryConfig{OnError: func(context.Context, task.RetryRequest) (task.RetryDecision, error) {
			return task.RetryDecision{Action: task.RetryFail}, nil
		}}
		Expect(task.Retry(context.Background(), config, func(context.Context, int) error { return cause })).To(MatchError(cause))
	})
	It("publishes the pending error and cancels a task waiting for an operator", func() {
		waiting := make(chan struct{})
		cause := errors.New("credentials need attention")
		config := task.RetryConfig{OnError: func(ctx context.Context, request task.RetryRequest) (task.RetryDecision, error) {
			close(waiting)
			<-ctx.Done()
			return task.RetryDecision{}, context.Cause(ctx)
		}}
		handle := task.StartTask("Waiting operation", func(commonscontext.Context, *task.Task) (bool, error) { return false, cause }, task.WithRetryConfig(config), task.WithCancellationDrain())
		Eventually(waiting).Should(BeClosed())
		snapshot := task.SnapshotTask(handle.GetTask(), nil)
		Expect(snapshot.Error).To(Equal(cause.Error()))
		Expect(snapshot.Retry).NotTo(BeNil())
		Expect(snapshot.Retry.Error).To(Equal(cause.Error()))
		handle.Cancel()
		_, err := handle.GetResult()
		Expect(err).To(HaveOccurred())
		Expect(handle.GetTask().Status()).To(Equal(task.StatusCancelled))
		Expect(task.SnapshotTask(handle.GetTask(), nil).Retry).To(BeNil())
	})
})
