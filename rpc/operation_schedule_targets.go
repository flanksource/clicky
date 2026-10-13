package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
)

// targetConcurrency resolves how many of a run's targets run at once. 0 means
// one at a time, which is how every run behaved before the field existed.
func targetConcurrency(requested, targets int) (int, error) {
	if requested == 0 {
		return 1, nil
	}
	if requested < 0 || requested > targets {
		return 0, fmt.Errorf("%w: concurrency %d must be between 1 and the %d selected targets",
			ErrOperationScheduleInvalid, requested, targets)
	}
	return requested, nil
}

// runScheduledOperation adds one task per target to the run group and returns
// without waiting on them: the group's concurrency gates the targets, and at a
// concurrency of 1 this runner holds the only slot until it returns. The
// scheduler already waits on the whole group.
//
// Targets are never retried. A maintenance operation that failed part-way is
// not safe to re-run behind the operator's back, whatever its error says.
func (s *OperationScheduleService) runScheduledOperation(
	ctx flanksourceContext.Context,
	schedule task.Schedule,
	group *task.Group,
) error {
	operation, payload, err := s.validateStoredSchedule(schedule)
	if err != nil {
		return err
	}
	run := task.TypedGroup[any]{Group: group}
	identity := context.WithoutCancel(ctx)
	for i, target := range payload.Targets {
		metadata := payload.Contexts[i]
		run.Add(target, func(targetCtx flanksourceContext.Context, t *task.Task) (any, error) {
			return s.runTarget(targetCtx, t, operation.Clicky.Schedule.Timeout, payload, metadata)
		}, task.WithContext(identity), task.WithRetryConfig(task.RetryConfig{MaxRetries: 0}))
	}
	return nil
}

// runTarget executes the operation in one target's rehydrated context. The
// timeout starts here, when the target gets its slot, rather than when it was
// queued behind the others; and it fails the target instead of cancelling it,
// because a target that ran out of time did not succeed.
func (s *OperationScheduleService) runTarget(
	ctx context.Context,
	t *task.Task,
	timeout time.Duration,
	payload operationSchedulePayload,
	metadata map[string]string,
) (any, error) {
	targetCtx, cancel := context.WithCancel(ctx)
	if timeout > 0 {
		cancel()
		targetCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	rehydrated, cleanup, err := s.rehydrate(flanksourceContext.NewContext(targetCtx, flanksourceContext.WithLogger(t)), metadata)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	result, _, err := s.executor.ExecuteOperation(rehydrated, payload.OperationID, payload.Args)
	if err == nil {
		err = targetCtx.Err()
	}
	if ctx.Err() == nil && errors.Is(targetCtx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("timed out after %s: %w", timeout, err)
	}
	return result, err
}

func (s *OperationScheduleService) captureTargets(ctx context.Context, targets []string, operation *RPCOperation) (operationSchedulePayload, error) {
	selection := operationSchedulePayload{Targets: append([]string(nil), targets...)}
	if len(targets) == 0 {
		return selection, fmt.Errorf("%w: at least one target is required", ErrOperationScheduleInvalid)
	}
	seen := map[string]bool{}
	for _, target := range targets {
		if strings.TrimSpace(target) == "" || target != strings.TrimSpace(target) || seen[target] {
			return selection, fmt.Errorf("%w: target %q is empty, untrimmed or duplicated", ErrOperationScheduleInvalid, target)
		}
		seen[target] = true
		metadata, err := s.contextAdapter.Capture(ctx, target)
		if err != nil {
			return selection, fmt.Errorf("%w: capture target %q: %v", ErrOperationScheduleInvalid, target, err)
		}
		captured, err := s.contextAdapter.Target(metadata)
		if err != nil || captured != target {
			return selection, fmt.Errorf("%w: target %q capture is inconsistent: %v", ErrOperationScheduleInvalid, target, err)
		}
		if err := s.authorizeMetadata(ctx, operation, metadata); err != nil {
			return selection, err
		}
		selection.Contexts = append(selection.Contexts, maps.Clone(metadata))
	}
	return selection, nil
}

func (s *OperationScheduleService) rehydrate(ctx context.Context, metadata map[string]string) (context.Context, func(), error) {
	bound, cleanup, err := s.contextAdapter.Rehydrate(ctx, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("rehydrate operation context: %w", err)
	}
	if bound == nil || cleanup == nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, fmt.Errorf("rehydrate operation context returned an incomplete context lease")
	}
	return bound, cleanup, nil
}

func (s *OperationScheduleService) authorizeMetadata(ctx context.Context, operation *RPCOperation, metadata map[string]string) error {
	if s.authorize == nil {
		return nil
	}
	bound, cleanup, err := s.rehydrate(ctx, metadata)
	if err != nil {
		return err
	}
	defer cleanup()
	return s.authorizeOperation(bound, operation)
}

func (s *OperationScheduleService) authorizeTargets(ctx context.Context, operation *RPCOperation, schedule task.Schedule) error {
	_, payload, err := s.validateStoredSchedule(schedule)
	if err != nil {
		return err
	}
	for _, metadata := range payload.Contexts {
		if err := s.authorizeMetadata(ctx, operation, metadata); err != nil {
			return err
		}
	}
	return nil
}

func (s *OperationScheduleService) matchesContext(metadata map[string]string, schedule task.Schedule) bool {
	if len(metadata) == 0 {
		return true
	}
	payload, err := decodeOperationSchedulePayload(schedule.Payload)
	if err != nil {
		return false
	}
	for _, target := range payload.Contexts {
		if maps.Equal(metadata, target) {
			return true
		}
	}
	return false
}

func (s *OperationScheduleService) migrateSchedule(schedule *task.Schedule) error {
	payload, err := decodeOperationSchedulePayload(schedule.Payload)
	if err != nil || len(payload.Targets) > 0 {
		return err
	}
	metadata := contextMetadata(schedule.Labels)
	target, err := s.contextAdapter.Target(metadata)
	if err != nil {
		return err
	}
	payload.Targets = []string{target}
	payload.Contexts = []map[string]string{metadata}
	schedule.Payload, err = json.Marshal(payload)
	if err != nil {
		return err
	}
	schedule.Timeout = 0
	schedule.Labels = labelsWithContext(payload.OperationID, nil)
	return nil
}
