package task

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RetryAction string

// RetryResume can resume after MaxRetries; RetryAutomatic keeps the bounded policy.
const (
	RetryAutomatic RetryAction = "automatic"
	RetryResume    RetryAction = "retry"
	RetryFail      RetryAction = "fail"
)

type RetryRequest struct {
	Attempt          int    `json:"attempt"`
	Error            string `json:"error"`
	Automatic        bool   `json:"automatic"`
	RemainingRetries int    `json:"remainingRetries"`
	Cause            error  `json:"-"`
}

type RetryDecision struct {
	Action  RetryAction `json:"action"`
	Message string      `json:"message,omitempty"`
}

type retryMessageKey struct{}

// RetryMessage is the user's message for the resumed attempt. It is never
// logged or included in snapshots; the operation decides how to use it.
func RetryMessage(ctx context.Context) string {
	message, _ := ctx.Value(retryMessageKey{}).(string)
	return message
}

func retryAfterError(ctx context.Context, config RetryConfig, attempt int, cause error) (context.Context, bool, error) {
	if ctx.Err() != nil {
		return ctx, false, errors.Join(cause, context.Cause(ctx))
	}
	retryable := shouldRetryError(cause, config)
	automatic := retryable && attempt <= config.MaxRetries
	if config.OnError != nil {
		decision, err := config.OnError(ctx, RetryRequest{Attempt: attempt, Error: cause.Error(), Cause: cause, Automatic: automatic, RemainingRetries: max(0, config.MaxRetries-attempt+1)})
		if err != nil {
			return ctx, false, errors.Join(cause, fmt.Errorf("retry decision: %w", err))
		}
		if ctx.Err() != nil {
			return ctx, false, errors.Join(cause, context.Cause(ctx))
		}
		switch decision.Action {
		case RetryResume:
			return context.WithValue(ctx, retryMessageKey{}, decision.Message), true, nil
		case RetryFail:
			return ctx, false, cause
		case RetryAutomatic:
		default:
			return ctx, false, errors.Join(cause, fmt.Errorf("invalid retry decision %q", decision.Action))
		}
	}
	if !automatic {
		if retryable && attempt > config.MaxRetries {
			cause = fmt.Errorf("failed after %d attempts: %w", attempt, cause)
		}
		return ctx, false, cause
	}
	timer := time.NewTimer(config.Delay(attempt))
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx, true, nil
	case <-ctx.Done():
		return ctx, false, errors.Join(cause, context.Cause(ctx))
	}
}
