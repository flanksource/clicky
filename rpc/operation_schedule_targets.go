package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/flanksource/clicky/task"
)

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
