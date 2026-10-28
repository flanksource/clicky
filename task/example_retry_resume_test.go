package task_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/flanksource/clicky/task"
)

func ExampleRetryMessage() {
	config := task.RetryConfig{OnError: func(ctx context.Context, request task.RetryRequest) (task.RetryDecision, error) {
		// A terminal, approval broker, or web host can await the operator here.
		return task.RetryDecision{Action: task.RetryResume, Message: "use the corrected configuration"}, nil
	}}
	err := task.Retry(context.Background(), config, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			return errors.New("invalid configuration")
		}
		fmt.Println(task.RetryMessage(ctx))
		return nil
	})
	fmt.Println(err)
	// Output:
	// use the corrected configuration
	// <nil>
}
