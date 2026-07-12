# Clicky Task Refactoring Patterns

## Contents

- [Current contracts](#current-contracts)
- [WaitGroup without results](#waitgroup-without-results)
- [Ordered results](#ordered-results)
- [Error behavior](#error-behavior)
- [Dependencies and nested work](#dependencies-and-nested-work)
- [Timeouts, retries, and cancellation](#timeouts-retries-and-cancellation)
- [Logging and renderer ownership](#logging-and-renderer-ownership)
- [Validation](#validation)

## Current contracts

The current implementation lives in `global_task_api.go` and `task/`.

```go
func clicky.StartTask[T any](
    name string,
    taskFunc task.TaskFunc[T],
    opts ...clicky.TaskOption,
) task.TypedTask[T]

func clicky.StartGroup[T any](
    name string,
    opts ...task.TaskGroupOption,
) task.TypedGroup[T]
```

`task.TaskFunc[T]` receives `flanksource/commons/context.Context` plus `*task.Task` and returns `(T, error)`. `task.TypedTask[T].GetResult()` waits and returns the typed value. `task.TypedGroup[T].GetResults()` returns a task-keyed map and stops on the first error.

`WaitFor()` returns a `*task.WaitResult` type with status, error, duration, and count fields. In the current group implementation, count fields are not populated, and a child error is returned before status and duration are set. Do not build caller behavior around those group fields without first changing and testing the implementation.

## WaitGroup without results

Before:

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
wg.Wait()
```

After:

```go
group := clicky.StartGroup[struct{}]("Processing items")
for _, item := range items {
    item := item
    group.Add(item.TaskName(), func(
        ctx flanksourceContext.Context,
        t *task.Task,
    ) (struct{}, error) {
        err := process(ctx, item)
        return struct{}{}, err
    })
}

if wait := group.WaitFor(); wait.Error != nil {
    return wait.Error
}
```

Use `struct{}` rather than `interface{}` when no value is returned; it states that result data is intentionally absent.

## Ordered results

A `WaitGroup` that writes into `results[index]` preserves input order. `GetResults()` does not, because it returns a map. Preserve handles in input order:

```go
handles := make([]task.TypedTask[Result], 0, len(items))
for _, item := range items {
    item := item
    handles = append(handles, group.Add(item.Name, func(
        ctx flanksourceContext.Context,
        t *task.Task,
    ) (Result, error) {
        return process(ctx, item)
    }))
}

results := make([]Result, 0, len(handles))
for _, handle := range handles {
    result, err := handle.GetResult()
    if err != nil {
        return nil, err
    }
    results = append(results, result)
}
```

Do not rebuild an ordered slice by ranging over the map returned from `GetResults()`.

## Error behavior

Record the original contract before refactoring:

- Fail fast: return the first task error through `GetResult`, `GetResults`, or `WaitFor`.
- Collect every error: wait for every retained handle, collect errors explicitly, and join them after all tasks finish.
- Return compact partial results: append successful handle results and return them with an aggregate error only when positional correspondence is not part of the contract.
- Return positional partial results: allocate the original length, assign each handle result at its input index, and aggregate every error after all handles complete. `TypedTask.GetResult()` can return a non-zero result together with an error; decide whether that partial value or the zero value belongs in a failed position.
- Best effort: log task failures, retain success/failure status, and return a policy-specific result.

Task tracking does not decide which policy is correct. Preserve the caller's existing contract.

Inside callbacks, log context and return the same error:

```go
if err != nil {
    t.Errorf("Fetching %s failed: %v", id, err)
    return Result{}, fmt.Errorf("fetching %s: %w", id, err)
}
```

## Dependencies and nested work

Use `clicky.WithDependencies(prerequisite.GetTask())` when a task must not start before another task completes. Keep the dependency graph acyclic and return prerequisite failures rather than silently running downstream work.

Use nested groups only when the hierarchy is meaningful to users or concurrency must be bounded separately. Avoid creating a group for every function call; one task should correspond to a useful operation/status boundary.

## Timeouts, retries, and cancellation

Use task options at creation:

```go
run := clicky.StartTask(
    "Fetching inventory",
    fetchInventory,
    clicky.WithTimeout(2*time.Minute),
    clicky.WithTaskTimeout(30*time.Second),
    clicky.WithRetryConfig(task.DefaultRetryConfig()),
)
```

Confirm the distinction between the task's overall timeout and its per-attempt timeout in `task/options.go`. Only retry operations that are idempotent or carry an idempotency key.

Pass the callback context to I/O calls. If a downstream API accepts only `context.Context`, use the callback value directly where its interface is accepted; do not replace it with a background context.

## Logging and renderer ownership

Within a callback, use `t.Debugf`, `t.Infof`, `t.Warnf`, and `t.Errorf`. These attach messages to the task and cooperate with the renderer. Outside callbacks, use Clicky's global logging path rather than direct standard output while tasks are active.

Avoid duplicate reporting: returning an error records task failure, so log only when the message adds operation-specific context.

## Validation

1. Run the focused package tests for the changed callers.
2. Run those tests with `-race` when shared state, cancellation, or completion ordering changed.
3. Assert maximum concurrency when introducing `WithConcurrency`.
4. Assert input ordering when replacing indexed result writes.
5. Assert failure, cancellation, timeout, and retry exhaustion paths.
6. Verify task names are stable, descriptive actions and do not contain secrets.
7. Exercise the CLI's final wait path so task output is flushed before process exit.
