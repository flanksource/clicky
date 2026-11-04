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

Run lifecycle regressions with `go test -race ./task -run '^TestTask$' -ginkgo.focus='Nested group lifecycle'`; run inline-step regressions with `go test -race ./task -run '^TestTask$' -ginkgo.focus='Group steps'`.

Publish bounded measurements with `step.Task().SetProgress(completed, total)` and attach their unit and transfer metrics with `step.Task().SetProgressMetrics(task.ProgressMetrics{Unit: "objects", Bytes: bytesRead, BytesPerSecond: rate})`. A zero total means unknown progress; a known total displays zero completed work immediately. Keep the phase in `SetDescription`, and use `SetLabels` for filterable task identity. Snapshots carry independent copies of these labels and metrics for CLI, JSON and web consumers.
