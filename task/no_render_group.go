package task

// WithNoRender keeps the run's tasks off the terminal: the live frame, the
// plain per-tick deltas and the final summary all skip them, and they never
// start the render loop. It is the run-scoped form of SetNoRender, for
// background work a server hosts beside the command's own tasks (a chat run
// served over HTTP) that would otherwise read as a long-running task line.
// Everything else is unchanged: the tasks are scheduled, waited on, report
// status and results, and appear in run snapshots and the task-manager APIs.
func WithNoRender() TaskGroupOption {
	return func(group *Group) {
		group.noRender = true
	}
}

// rendered reports whether the task belongs on the terminal. A group's
// noRender is fixed before the group is published, so it is read unlocked.
func (t *Task) rendered() bool {
	return t.parent == nil || !t.parent.noRender
}

// renderedTasks keeps the tasks that belong on the terminal, in order.
func renderedTasks(tasks []*Task) []*Task {
	kept := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		if t.rendered() {
			kept = append(kept, t)
		}
	}
	return kept
}
