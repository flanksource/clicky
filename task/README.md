# Nested task groups

Create groups with the active operation context. Attach nested groups to their owner with `WithParentGroup`; snapshots include `parentGroupId`, and parent status, results, cancellation, and completion include descendants.

```go
parent := task.StartGroup[bool]("Compare sources", task.WithGroupContext(ctx), task.WithConcurrency(1))
parent.Add("Open sources", func(ctx commonscontext.Context, owner *task.Task) (bool, error) {
    child := task.StartGroup[bool]("Read Git", task.WithParentGroup(parent.Group), task.WithGroupContext(ctx))
    child.Add("Read pinned commit", func(ctx commonscontext.Context, current *task.Task) (bool, error) {
        return readCommit(ctx)
    })
    _, err := child.GetResults()
    return err == nil, err
}, task.WithSlots(0), task.WithCancellationDrain())
result := parent.WaitFor()
```

Here `commonscontext` is `github.com/flanksource/commons/context`, `task` is `github.com/flanksource/clicky/task`, and `readCommit` is the operation being supervised. `WithSlots(0)` lets a supervising callback wait for nested workers when concurrency is one. Keep the callback alive until its nested work finishes: a completed callback's context is canceled. `WithCancellationDrain()` makes cancellation wait for callback cleanup before returning results.

Use `GroupFromContext(ctx)` to recover the active group in a shared provider. `StartStep` reports work performed inline without consuming another worker slot. Pass `WithContext(ctx)` when the step has a more specific request context, and finish it with the operation's error. A canceled context keeps the step canceled even when `Finish(nil)` is called.

A newly created child stays pending while its owner schedules work. If the owner determines that the child will receive no tasks, call `child.Cancel()` before waiting on the parent. Do this only after scheduling for that child has ended; status reads cannot distinguish an unused group from one awaiting its first task. Active children and children with scheduled tasks must retain their lifecycle.

```go
child := task.StartGroup[bool]("Read sources", task.WithParentGroup(parent.Group))
if len(sources) == 0 {
    child.Cancel() // Scheduling is complete and there is no work for this child.
} else {
    for _, source := range sources {
        source := source
        child.Add(source.Name, func(ctx commonscontext.Context, current *task.Task) (bool, error) {
            return readSource(ctx, source)
        })
    }
}
result := parent.WaitFor()
```

`WithGroupIdentity` is global. A live identity can be joined only by a caller with the same parent group; attempting to join it from another parent panics because the two owners cannot share a lifecycle.

Run lifecycle regressions with `go test -race ./task -run '^TestTask$' -ginkgo.focus='Nested group lifecycle'`; run inline-step regressions with `go test -race ./task -run '^TestTask$' -ginkgo.focus='Group steps'`.

Publish bounded measurements with `step.Task().SetProgress(completed, total)` and attach their unit and transfer metrics with `step.Task().SetProgressMetrics(task.ProgressMetrics{Unit: "objects", Bytes: bytesRead, BytesPerSecond: rate})`. A zero total means unknown progress; a known total displays zero completed work immediately. Keep the phase in `SetDescription`, and use `SetLabels` for filterable task identity. Snapshots carry independent copies of these labels and metrics for CLI, JSON and web consumers.
