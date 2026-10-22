package task

import (
	"context"
	"fmt"
	"time"
)

// Retry runs run with the retry policy of config, for work that is not a Task.
// attempt is 1-based. A non-retryable error is returned as-is; once MaxRetries
// retries are exhausted the last error is returned wrapped, so errors.Is still
// matches it. Cancelling ctx while waiting returns context.Cause(ctx).
func Retry(ctx context.Context, config RetryConfig, run func(ctx context.Context, attempt int) error) error {
	for attempt := 1; ; attempt++ {
		err := run(ctx, attempt)
		if err == nil || !shouldRetryError(err, config) {
			return err
		}
		if attempt > config.MaxRetries {
			return fmt.Errorf("failed after %d attempts: %w", attempt, err)
		}
		timer := time.NewTimer(config.Delay(attempt))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry cancelled after attempt %d (%v): %w", attempt, err, context.Cause(ctx))
		}
	}
}
