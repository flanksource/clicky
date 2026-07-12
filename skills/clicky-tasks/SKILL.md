---
name: clicky-tasks
description: Refactor Go work and concurrency to Clicky's task APIs, including StartTask, StartGroup, typed results, dependencies, retries, timeouts, cancellation, and task-safe logging. Use when replacing goroutines, sync.WaitGroup, ad hoc output, or manual result collection with Clicky task execution.
---

# Clicky Tasks

Replace ad hoc goroutine orchestration with Clicky's tracked task and group APIs while preserving concurrency, result ordering, cancellation, and error semantics.

## Workflow

1. Inspect the complete concurrency boundary: goroutine creation, `WaitGroup`, channels, shared result writes, cancellation, error aggregation, and logging.
2. Decide whether the work is one task or a typed group:
   - Use `clicky.StartTask[T]` for one tracked operation.
   - Use `clicky.StartGroup[T]` when multiple related operations should run concurrently.
3. Preserve observable behavior explicitly. Record whether callers require input order, all errors, partial results, bounded concurrency, or fail-fast behavior.
4. Convert output inside task callbacks to task logging. Return errors instead of calling `Fatalf`, panicking, or exiting.
5. Wait at the owning boundary with `GetResult`, `GetResults`, `WaitFor`, or `clicky.WaitForGlobalCompletion`, depending on whether typed values or only final status are needed.
6. Run focused tests under the race detector when shared state or cancellation behavior changed.

## Imports and callback contract

Prefer the top-level Clicky entrypoints. Import the task package for task types and options:

```go
import (
    "github.com/flanksource/clicky"
    "github.com/flanksource/clicky/task"
    flanksourceContext "github.com/flanksource/commons/context"
)
```

Task callbacks receive `flanksource/commons/context.Context`, not the standard-library `context.Context` type:

```go
run := clicky.StartTask("Loading configuration", func(
    ctx flanksourceContext.Context,
    t *task.Task,
) (Config, error) {
    t.Debugf("Reading %s", path)
    config, err := loadConfig(ctx, path)
    if err != nil {
        t.Errorf("Loading configuration failed: %v", err)
        return Config{}, err
    }
    return config, nil
})

config, err := run.GetResult()
```

Supported task logging methods are `Debugf`, `Infof`, `Warnf`, and `Errorf`. Do not invent `Tracef`. Use Clicky's global logging helpers outside a task callback when the interactive renderer may own the terminal.

## Typed groups

```go
group := clicky.StartGroup[Result](
    "Processing files",
    task.WithConcurrency(4),
)

handles := make([]task.TypedTask[Result], 0, len(files))
for _, file := range files {
    file := file
    handles = append(handles, group.Add(
        fmt.Sprintf("Processing %s", file),
        func(ctx flanksourceContext.Context, t *task.Task) (Result, error) {
            t.Infof("Processing %s", file)
            return processFile(ctx, file)
        },
    ))
}

if wait := group.WaitFor(); wait.Error != nil {
    return nil, wait.Error
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

Capture the loop value even on Go versions that provide per-iteration range variables; it keeps the task closure's intent explicit across supported modules.

`TypedGroup.GetResults()` returns `map[task.TypedTask[T]]T`. Map iteration is unordered, and the method returns on the first task error. Retain task handles in input order when result order matters, and preserve a custom aggregation strategy when callers need every error or partial results.

## Options and completion

- Use `task.WithConcurrency` when a group must bound parallelism.
- Use `clicky.WithTimeout`, `clicky.WithTaskTimeout`, `clicky.WithDependencies`, and `clicky.WithRetryConfig` for individual tasks.
- Use `task.DefaultRetryConfig()` as a starting point only after confirming retry safety and idempotency.
- Use the callback context for downstream calls and cancellation checks.
- Use `GetResult()` when the typed value matters; it waits for completion.
- Use `WaitFor()` only when completion and summary status matter. The current group implementation does not populate per-status count fields and returns an error before setting status or duration when a child fails.
- Use `clicky.WaitForGlobalCompletion()` at the CLI ownership boundary when the process exit code should reflect all global tasks.

## Guardrails

- Do not mechanically replace a `WaitGroup` until result ordering and error behavior are understood.
- Do not collapse an indexed partial-result contract into a compact success-only slice; keep a fixed-length result slice keyed by handle position.
- Do not mix a task group with a second `WaitGroup` for the same work.
- Do not write concurrently to an existing slice or map just because tasks now provide tracking; return typed values or keep synchronization explicit.
- Do not call `fmt.Print`, `log.Printf`, or `os.Stdout` while the live renderer owns the terminal.
- Do not convert `Fatalf` to `t.Errorf` without also returning the error.
- Do not add retries to non-idempotent operations without an explicit safety contract.
- Do not discard cancellation by replacing the callback context with `context.Background()`.

## Detailed reference

Read [references/refactoring-patterns.md](references/refactoring-patterns.md) before converting ordered results, channels, nested groups, dependencies, timeouts, retries, or multi-error flows.
