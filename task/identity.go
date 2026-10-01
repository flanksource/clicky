package task

// claimIdentity registers task as the holder of its identity, or returns the
// task already holding it. A task without an identity always claims.
func (tm *Manager) claimIdentity(task *Task) *Task {
	if task.identity == "" {
		return task
	}
	existing, loaded := tm.tasksByIdentity.LoadOrStore(task.identity, task)
	if loaded {
		return existing.(*Task)
	}
	return task
}

// releaseIdentity frees the identity a retired task held, leaving it alone if
// another task has claimed it since.
func (tm *Manager) releaseIdentity(task *Task) {
	if task.identity != "" {
		tm.tasksByIdentity.CompareAndDelete(task.identity, task)
	}
}

// discardDuplicate releases a task that lost its identity to a running one: it
// was never enqueued, so nothing else will cancel its context.
func discardDuplicate(task *Task) {
	task.mu.Lock()
	cancel := task.cancel
	task.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// joinLiveGroup returns the live group holding group's identity, or registers
// group as its holder and returns nil. The caller holds tm.mu.
func (tm *Manager) joinLiveGroup(group *Group) *Group {
	if group.identity == "" {
		return nil
	}
	if existing := tm.groupsByIdentity[group.identity]; existing != nil && existing.live() {
		return existing
	}
	if tm.groupsByIdentity == nil {
		tm.groupsByIdentity = map[string]*Group{}
	}
	tm.groupsByIdentity[group.identity] = group
	return nil
}
