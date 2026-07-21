package task

import (
	"context"
	"fmt"
)

// ControlAction is a lifecycle command exposed by a task or run.
type ControlAction string

const (
	ControlStart   ControlAction = "start"
	ControlStop    ControlAction = "stop"
	ControlRestart ControlAction = "restart"
)

// TaskController performs the currently-supported lifecycle actions for a run
// or task. Actions must reflect live state and may change between snapshots.
type TaskController interface {
	Actions() []ControlAction
	Control(context.Context, ControlAction) error
}

// StdinController is implemented by task controllers that accept live stdin.
// Stdin is intentionally never included in snapshots or persisted history.
type StdinController interface {
	WriteStdin([]byte) error
}

func controllerActions(controller TaskController) []ControlAction {
	if controller == nil {
		return nil
	}
	return controller.Actions()
}

func actionAllowed(actions []ControlAction, action ControlAction) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

func groupByID(id string) *Group {
	if global == nil {
		return nil
	}
	global.mu.RLock()
	defer global.mu.RUnlock()
	for _, group := range global.groups {
		if group.ID() == id {
			return group
		}
	}
	return nil
}

// ControlRun performs one advertised lifecycle action for a live run.
func ControlRun(ctx context.Context, id string, action ControlAction) error {
	group := groupByID(id)
	if group == nil {
		return fmt.Errorf("run %q not found", id)
	}
	group.mu.RLock()
	controller := group.controller
	group.mu.RUnlock()
	if controller == nil || !actionAllowed(controller.Actions(), action) {
		return fmt.Errorf("run %q does not support %q", id, action)
	}
	return controller.Control(ctx, action)
}

// WriteTaskStdin writes bytes to a live task that advertises stdin support.
func WriteTaskStdin(runID, taskID string, data []byte) error {
	group := groupByID(runID)
	if group == nil {
		return fmt.Errorf("run %q not found", runID)
	}
	for _, item := range group.GetTasks() {
		t := item.GetTask()
		if t.ID() != taskID {
			continue
		}
		t.mu.Lock()
		controller := t.controller
		t.mu.Unlock()
		stdin, ok := controller.(StdinController)
		if !ok {
			return fmt.Errorf("task %q does not support stdin", taskID)
		}
		return stdin.WriteStdin(data)
	}
	return fmt.Errorf("task %q not found in run %q", taskID, runID)
}
