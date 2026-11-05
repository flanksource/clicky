package task_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskFromContext", func() {
	It("returns nil when the context has no task", func() {
		Expect(task.TaskFromContext(context.Background())).To(BeNil())
		group := task.StartGroup[bool]("context without a task")
		Expect(task.TaskFromContext(group.Context())).To(BeNil())
	})

	DescribeTable("exposes the owning task while preserving request values and group identity",
		func(grouped, timed bool) {
			type requestKey struct{}
			request := context.WithValue(context.Background(), requestKey{}, "request")
			opts := []task.Option{task.WithContext(request)}
			var group *task.Group
			if grouped {
				group = task.StartGroup[bool]("task context group", task.WithGroupContext(request)).Group
				opts = append(opts, task.WithGroup(group), task.WithContext(request))
			}
			if timed {
				opts = append(opts, task.WithTimeout(time.Minute))
			}
			run := task.StartTask("task context callback", func(ctx commonscontext.Context, current *task.Task) ([]any, error) {
				derived, cancel := context.WithCancel(ctx)
				defer cancel()
				return []any{
					task.TaskFromContext(ctx), task.TaskFromContext(derived),
					task.TaskFromContext(current.Context()), task.TaskFromContext(current.FlanksourceContext()),
					task.GroupFromContext(ctx), ctx.Value(requestKey{}),
				}, nil
			}, opts...)
			Expect(run.GetResult()).To(Equal([]any{run.Task, run.Task, run.Task, run.Task, group, "request"}))
			Expect(task.TaskFromContext(request)).To(BeNil())
		},
		Entry("standalone", false, false),
		Entry("standalone with a timeout", false, true),
		Entry("grouped", true, false),
		Entry("grouped with a timeout", true, true),
	)

	It("shadows a parent's task in a child context without changing the parent", func() {
		parent := task.StartTask("context parent", func(ctx commonscontext.Context, current *task.Task) ([]*task.Task, error) {
			child := task.StartTask("context child", func(childCtx commonscontext.Context, _ *task.Task) (*task.Task, error) {
				return task.TaskFromContext(childCtx), nil
			}, task.WithContext(ctx))
			found, err := child.GetResult()
			if err != nil {
				return nil, err
			}
			return []*task.Task{found, child.Task, task.TaskFromContext(ctx), current}, nil
		}, task.WithSlots(0))
		found, err := parent.GetResult()
		Expect(err).NotTo(HaveOccurred())
		Expect(found[0]).To(BeIdenticalTo(found[1]))
		Expect(found[2:]).To(Equal([]*task.Task{parent.Task, parent.Task}))
	})

	It("keeps concurrently running task contexts independent", func() {
		type observation struct {
			ctx   context.Context
			owner *task.Task
		}
		contexts := make(chan observation, 2)
		release := make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		defer unblock()
		group := task.StartGroup[bool]("concurrent task contexts", task.WithConcurrency(2))
		for _, name := range []string{"first context", "second context"} {
			group.Add(name, func(ctx commonscontext.Context, current *task.Task) (bool, error) {
				contexts <- observation{ctx: ctx, owner: current}
				<-release
				return true, nil
			})
		}
		var first, second observation
		Eventually(contexts).Should(Receive(&first))
		Eventually(contexts).Should(Receive(&second))
		Expect(task.TaskFromContext(first.ctx)).To(BeIdenticalTo(first.owner))
		Expect(task.TaskFromContext(second.ctx)).To(BeIdenticalTo(second.owner))
		Expect(first.owner).NotTo(BeIdenticalTo(second.owner))
		unblock()
		Expect(group.WaitFor().Error).NotTo(HaveOccurred())
	})

	It("binds inline steps, planned steps, and managed runs to their own task", func() {
		group := task.StartGroup[bool]("manual task contexts")
		step := group.StartStep("inline")
		defer step.Finish(nil)
		planned := group.PlanStep("planned", task.WithContext(step.Task().Context()))
		defer planned.Finish(nil)
		run := task.StartManagedRun("managed", task.WithGroupContext(step.Task().Context()))
		defer run.Finish(task.StatusSuccess, nil)
		for _, current := range []*task.Task{step.Task(), planned.Task(), run.Task()} {
			Expect(task.TaskFromContext(current.Context())).To(BeIdenticalTo(current))
			Expect(task.TaskFromContext(current.FlanksourceContext())).To(BeIdenticalTo(current))
		}
		Expect(planned.Start()).To(Succeed())
		Expect(task.TaskFromContext(planned.Task().Context())).To(BeIdenticalTo(planned.Task()))
		Expect(task.TaskFromContext(step.Task().Context())).To(BeIdenticalTo(step.Task()))
	})

	It("preserves task identity through retry decisions and resumed attempts", func() {
		attempts := 0
		var found []*task.Task
		config := task.RetryConfig{OnError: func(ctx context.Context, _ task.RetryRequest) (task.RetryDecision, error) {
			found = append(found, task.TaskFromContext(ctx))
			return task.RetryDecision{Action: task.RetryResume, Message: "resumed"}, nil
		}}
		run := task.StartTask("retry task context", func(ctx commonscontext.Context, _ *task.Task) (string, error) {
			attempts++
			found = append(found, task.TaskFromContext(ctx))
			if attempts == 1 {
				return "", errors.New("retry requested")
			}
			return task.RetryMessage(ctx), nil
		}, task.WithRetryConfig(config))
		Expect(run.GetResult()).To(Equal("resumed"))
		Expect(found).To(Equal([]*task.Task{run.Task, run.Task, run.Task}))
	})

	It("preserves owner cancellation and its cause", func() {
		request, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)
		started := make(chan struct{})
		run := task.StartTask("canceled task context", func(ctx commonscontext.Context, _ *task.Task) ([]any, error) {
			close(started)
			<-ctx.Done()
			return []any{task.TaskFromContext(ctx), context.Cause(ctx)}, ctx.Err()
		}, task.WithContext(request), task.WithCancellationDrain())
		Eventually(started).Should(BeClosed())
		cause := errors.New("owner stopped")
		cancel(cause)
		found, err := run.GetResult()
		Expect(err).To(MatchError(context.Canceled))
		Expect(found).To(Equal([]any{run.Task, cause}))
	})
})
