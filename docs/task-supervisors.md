# Supervisor tasks

Tasks reserve one global concurrency slot and one slot in their group by default. Pass `task.WithSlots(n)` to reserve a different number in both pools. Negative values panic; reservations larger than a finite global or group capacity fail with an error. Lowering global capacity affects subsequent admissions; running callbacks finish with their original reservation.

Use `task.WithSlots(0)` for a supervisor which creates and waits for child tasks. Supervisors start independently of occupied worker slots, remain visible, and block foreground completion waits. They retain normal dependency, cancellation, timeout, and error handling. Work performed by the children still consumes slots. Use ordinary tasks for network requests, commands, scans, and writes.

This runnable example completes with both global and group concurrency set to one:

```go
package main

import (
    "fmt"

    "github.com/flanksource/clicky"
    "github.com/flanksource/clicky/task"
    fc "github.com/flanksource/commons/context"
)

func main() {
    clicky.SetGlobalMaxConcurrency(1)
    group := clicky.StartGroup[int]("Example", task.WithConcurrency(1))
    supervisor := group.Add("Supervisor", func(ctx fc.Context, _ *task.Task) (int, error) {
        child := group.Add("Child", func(fc.Context, *task.Task) (int, error) {
            return 7, nil
        }, task.WithContext(ctx), task.WithCancellationDrain())
        return child.GetResult()
    }, task.WithSlots(0), task.WithCancellationDrain(), task.WithRetryConfig(task.RetryConfig{MaxRetries: 0}))
    value, err := supervisor.GetResult()
    clicky.WaitForGlobalCompletion()
    if err != nil {
        panic(err)
    }
    fmt.Println(value)
}
```

When a supervisor owns multiple tasks, retain and consume every handle even after one fails. `TypedGroup.GetResults()` returns on the first error, so it cannot serve as a drain before verification or staging. Bind children to the supervisor context and opt into cancellation draining when later work must wait for their callbacks to return. Disable retries on the supervisor to avoid creating the same children again.
