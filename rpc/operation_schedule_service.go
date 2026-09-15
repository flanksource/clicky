package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/google/uuid"
)

const (
	OperationScheduleKind = "clicky-operation"
	operationLabel        = "clicky.operation"
	contextLabelPrefix    = "clicky.context."
)

var (
	ErrOperationScheduleNotFound     = errors.New("operation schedule not found")
	ErrOperationNotFound             = errors.New("operation not found")
	ErrOperationNotSchedulable       = errors.New("operation is not schedulable")
	ErrOperationScheduleInvalid      = errors.New("invalid operation schedule")
	ErrOperationScheduleUnauthorized = errors.New("operation schedule is not authorized")
)

// OperationScheduleContextAdapter captures request identity as durable metadata
// and reconstructs the operation context when background execution begins.
type OperationScheduleContextAdapter interface {
	Capture(context.Context) (map[string]string, error)
	Rehydrate(context.Context, map[string]string) (context.Context, func(), error)
}

// OperationScheduleAuthorizer applies host policy to a schedulable operation.
type OperationScheduleAuthorizer func(context.Context, *RPCOperation) error

type OperationScheduleServiceOptions struct {
	Store          task.ScheduleStore
	Executor       *CommandExecutor
	ContextAdapter OperationScheduleContextAdapter
	Authorize      OperationScheduleAuthorizer
	Now            func() time.Time
}

type OperationScheduleService struct {
	store          *operationScheduleStore
	executor       *CommandExecutor
	contextAdapter OperationScheduleContextAdapter
	authorize      OperationScheduleAuthorizer
	scheduler      *task.Scheduler
}

type OperationScheduleInput struct {
	Name        string         `json:"name"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
	Cron        string         `json:"cron"`
	Timezone    string         `json:"timezone,omitempty"`
	Enabled     bool           `json:"enabled"`
}

type OperationRunInput struct {
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
}

type OperationSchedule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
	Cron        string         `json:"cron"`
	Timezone    string         `json:"timezone,omitempty"`
	Enabled     bool           `json:"enabled"`
	LastRun     *time.Time     `json:"lastRun,omitempty"`
	NextRun     *time.Time     `json:"nextRun,omitempty"`
}

type OperationScheduleRun struct {
	RunID string `json:"runId"`
}

type operationSchedulePayload struct {
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
}

func NewOperationScheduleService(options OperationScheduleServiceOptions) (*OperationScheduleService, error) {
	if options.Store == nil {
		return nil, fmt.Errorf("operation schedule store is required")
	}
	if options.Executor == nil {
		return nil, fmt.Errorf("operation schedule executor is required")
	}
	if options.ContextAdapter == nil {
		return nil, fmt.Errorf("operation schedule context adapter is required")
	}
	store := &operationScheduleStore{ScheduleStore: options.Store}
	service := &OperationScheduleService{
		store: store, executor: options.Executor, contextAdapter: options.ContextAdapter,
		authorize: options.Authorize,
	}
	service.scheduler = task.NewScheduler(task.SchedulerOptions{Store: store, Now: options.Now})
	service.scheduler.RegisterRunner(OperationScheduleKind, service.runScheduledOperation)
	return service, nil
}

func (s *OperationScheduleService) Load(ctx context.Context) error {
	schedules, err := s.store.ListSchedules(ctx)
	if err != nil {
		return fmt.Errorf("list operation schedules: %w", err)
	}
	for _, schedule := range schedules {
		if _, _, err := s.validateStoredSchedule(schedule); err != nil {
			return fmt.Errorf("load operation schedule %q: %w", schedule.Name, err)
		}
		if err := s.scheduler.Add(ctx, schedule); err != nil {
			return fmt.Errorf("load operation schedule %q: %w", schedule.Name, err)
		}
	}
	return nil
}

func (s *OperationScheduleService) Start(ctx flanksourceContext.Context) {
	s.scheduler.Start(ctx)
}

func (s *OperationScheduleService) Stop() {
	s.scheduler.Stop()
}

func (s *OperationScheduleService) List(ctx context.Context) ([]OperationSchedule, error) {
	metadata, err := s.capture(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]OperationSchedule, 0)
	for _, schedule := range s.scheduler.Schedules() {
		if !maps.Equal(metadata, contextMetadata(schedule.Labels)) {
			continue
		}
		item, err := operationScheduleFromTask(schedule)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *OperationScheduleService) Create(ctx context.Context, input OperationScheduleInput) (OperationSchedule, error) {
	return s.save(ctx, uuid.NewString(), input)
}

func (s *OperationScheduleService) Update(
	ctx context.Context,
	id string,
	input OperationScheduleInput,
) (OperationSchedule, error) {
	if _, err := s.scheduleForContext(ctx, id); err != nil {
		return OperationSchedule{}, err
	}
	return s.save(ctx, id, input)
}

func (s *OperationScheduleService) Delete(ctx context.Context, id string) error {
	schedule, err := s.scheduleForContext(ctx, id)
	if err != nil {
		return err
	}
	operation, _, err := s.validateStoredSchedule(schedule)
	if err != nil {
		return err
	}
	if err := s.authorizeOperation(ctx, operation); err != nil {
		return err
	}
	return s.scheduler.Delete(ctx, id)
}

func (s *OperationScheduleService) Run(ctx context.Context, id string) (OperationScheduleRun, error) {
	schedule, err := s.scheduleForContext(ctx, id)
	if err != nil {
		return OperationScheduleRun{}, err
	}
	operation, _, err := s.validateStoredSchedule(schedule)
	if err != nil {
		return OperationScheduleRun{}, err
	}
	if err := s.authorizeOperation(ctx, operation); err != nil {
		return OperationScheduleRun{}, err
	}
	group, err := s.scheduler.Trigger(flanksourceContext.NewContext(ctx), id)
	if err != nil {
		return OperationScheduleRun{}, err
	}
	return OperationScheduleRun{RunID: group.ID()}, nil
}

func (s *OperationScheduleService) RunNow(ctx context.Context, input OperationRunInput) (OperationScheduleRun, error) {
	operation, err := s.schedulableOperation(input.OperationID, input.Args)
	if err != nil {
		return OperationScheduleRun{}, err
	}
	if err := s.authorizeOperation(ctx, operation); err != nil {
		return OperationScheduleRun{}, err
	}
	metadata, err := s.capture(ctx)
	if err != nil {
		return OperationScheduleRun{}, err
	}
	payload, err := json.Marshal(operationSchedulePayload{OperationID: input.OperationID, Args: nonNilArgs(input.Args)})
	if err != nil {
		return OperationScheduleRun{}, fmt.Errorf("encode operation arguments: %w", err)
	}
	group, err := s.scheduler.RunNow(flanksourceContext.NewContext(ctx), task.Schedule{
		Name: uuid.NewString(), Title: operationTitle(operation), Kind: OperationScheduleKind,
		Labels: labelsWithContext(operationID(operation), metadata), Payload: payload,
		Timeout: operation.Clicky.Schedule.Timeout,
	})
	if err != nil {
		return OperationScheduleRun{}, err
	}
	return OperationScheduleRun{RunID: group.ID()}, nil
}

func (s *OperationScheduleService) save(
	ctx context.Context,
	id string,
	input OperationScheduleInput,
) (OperationSchedule, error) {
	if strings.TrimSpace(input.Name) == "" {
		return OperationSchedule{}, fmt.Errorf("%w: schedule name is required", ErrOperationScheduleInvalid)
	}
	operation, err := s.schedulableOperation(input.OperationID, input.Args)
	if err != nil {
		return OperationSchedule{}, err
	}
	if err := s.authorizeOperation(ctx, operation); err != nil {
		return OperationSchedule{}, err
	}
	metadata, err := s.capture(ctx)
	if err != nil {
		return OperationSchedule{}, err
	}
	payload, err := json.Marshal(operationSchedulePayload{OperationID: input.OperationID, Args: nonNilArgs(input.Args)})
	if err != nil {
		return OperationSchedule{}, fmt.Errorf("encode operation arguments: %w", err)
	}
	schedule := task.Schedule{
		Name: id, Title: strings.TrimSpace(input.Name), Kind: OperationScheduleKind,
		Labels: labelsWithContext(input.OperationID, metadata), Payload: payload,
		Cron: input.Cron, Timezone: input.Timezone, Enabled: input.Enabled,
		Timeout: operation.Clicky.Schedule.Timeout, Overlap: task.OverlapSkip, CatchUp: task.CatchUpNone,
	}
	if err := s.scheduler.Add(ctx, schedule); err != nil {
		return OperationSchedule{}, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err)
	}
	for _, saved := range s.scheduler.Schedules() {
		if saved.Name == id {
			return operationScheduleFromTask(saved)
		}
	}
	return OperationSchedule{}, fmt.Errorf("operation schedule %q was not registered", id)
}

func (s *OperationScheduleService) scheduleForContext(ctx context.Context, id string) (task.Schedule, error) {
	metadata, err := s.capture(ctx)
	if err != nil {
		return task.Schedule{}, err
	}
	for _, schedule := range s.scheduler.Schedules() {
		if schedule.Name == id && maps.Equal(metadata, contextMetadata(schedule.Labels)) {
			return schedule, nil
		}
	}
	return task.Schedule{}, fmt.Errorf("%w: %s", ErrOperationScheduleNotFound, id)
}

func (s *OperationScheduleService) schedulableOperation(id string, args map[string]any) (*RPCOperation, error) {
	operation := s.executor.FindOperationByID(id)
	if operation == nil {
		return nil, fmt.Errorf("%w: %s", ErrOperationNotFound, id)
	}
	if operation.Clicky == nil || operation.Clicky.Schedule == nil {
		return nil, fmt.Errorf("%w: operation %s is not schedulable", ErrOperationNotSchedulable, id)
	}
	request, err := executionRequestFromValues(context.Background(), operation, nonNilArgs(args))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err)
	}
	if err := s.executor.ValidateParameters(request, operation); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOperationScheduleInvalid, err)
	}
	return operation, nil
}

func (s *OperationScheduleService) validateStoredSchedule(
	schedule task.Schedule,
) (*RPCOperation, operationSchedulePayload, error) {
	payload, err := decodeOperationSchedulePayload(schedule.Payload)
	if err != nil {
		return nil, operationSchedulePayload{}, err
	}
	operation, err := s.schedulableOperation(payload.OperationID, payload.Args)
	return operation, payload, err
}

func (s *OperationScheduleService) runScheduledOperation(
	ctx flanksourceContext.Context,
	schedule task.Schedule,
	_ *task.Group,
) error {
	_, payload, err := s.validateStoredSchedule(schedule)
	if err != nil {
		return err
	}
	rehydrated, cleanup, err := s.contextAdapter.Rehydrate(ctx, contextMetadata(schedule.Labels))
	if err != nil {
		return fmt.Errorf("rehydrate operation context: %w", err)
	}
	if rehydrated == nil || cleanup == nil {
		return fmt.Errorf("rehydrate operation context returned an incomplete context lease")
	}
	defer cleanup()
	_, _, err = s.executor.ExecuteOperation(rehydrated, payload.OperationID, payload.Args)
	return err
}

func (s *OperationScheduleService) capture(ctx context.Context) (map[string]string, error) {
	metadata, err := s.contextAdapter.Capture(ctx)
	if err != nil {
		return nil, fmt.Errorf("capture operation context: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("capture operation context returned nil metadata")
	}
	for key := range metadata {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("capture operation context returned an empty metadata key")
		}
	}
	return metadata, nil
}

func (s *OperationScheduleService) authorizeOperation(ctx context.Context, operation *RPCOperation) error {
	if s.authorize == nil {
		return nil
	}
	if err := s.authorize(ctx, operation); err != nil {
		return fmt.Errorf("%w: operation %s: %v", ErrOperationScheduleUnauthorized, operationID(operation), err)
	}
	return nil
}

func operationScheduleFromTask(schedule task.Schedule) (OperationSchedule, error) {
	payload, err := decodeOperationSchedulePayload(schedule.Payload)
	if err != nil {
		return OperationSchedule{}, fmt.Errorf("operation schedule %q: %w", schedule.Name, err)
	}
	return OperationSchedule{
		ID: schedule.Name, Name: schedule.Title, OperationID: payload.OperationID,
		Args: payload.Args, Cron: schedule.Cron, Timezone: schedule.Timezone, Enabled: schedule.Enabled,
		LastRun: schedule.LastRun, NextRun: schedule.NextRun,
	}, nil
}

func decodeOperationSchedulePayload(payload json.RawMessage) (operationSchedulePayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var decoded operationSchedulePayload
	if err := decoder.Decode(&decoded); err != nil {
		return operationSchedulePayload{}, fmt.Errorf("decode operation payload: %w", err)
	}
	if decoded.OperationID == "" {
		return operationSchedulePayload{}, fmt.Errorf("operation payload operationId is required")
	}
	decoded.Args = nonNilArgs(decoded.Args)
	return decoded, nil
}

func labelsWithContext(operationID string, metadata map[string]string) map[string]string {
	labels := make(map[string]string, len(metadata)+1)
	labels[operationLabel] = operationID
	for key, value := range metadata {
		labels[contextLabelPrefix+key] = value
	}
	return labels
}

func contextMetadata(labels map[string]string) map[string]string {
	metadata := map[string]string{}
	for key, value := range labels {
		if strings.HasPrefix(key, contextLabelPrefix) {
			metadata[strings.TrimPrefix(key, contextLabelPrefix)] = value
		}
	}
	return metadata
}

func nonNilArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

func operationTitle(operation *RPCOperation) string {
	if operation.Description != "" {
		return operation.Description
	}
	return operationID(operation)
}

type operationScheduleStore struct {
	task.ScheduleStore
}

func (s *operationScheduleStore) ListSchedules(ctx context.Context) ([]task.Schedule, error) {
	schedules, err := s.ScheduleStore.ListSchedules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]task.Schedule, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule.Kind == OperationScheduleKind {
			result = append(result, schedule)
		}
	}
	return result, nil
}
