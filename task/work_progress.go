package task

// WorkProgress counts fixed units of work independently from implementation tasks.
// Completed excludes Cached; remaining units are queued until made Unstarted.
type WorkProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Cached    int `json:"cached"`
	Failed    int `json:"failed"`
	Running   int `json:"running"`
	Canceled  int `json:"canceled"`
	Unstarted int `json:"unstarted"`
}

// SetWorkProgress publishes a coherent set of counts. A run's total is immutable.
func (r *ManagedRun) SetWorkProgress(progress WorkProgress) {
	used := 0
	for _, count := range []int{progress.Completed, progress.Cached, progress.Failed, progress.Running, progress.Canceled, progress.Unstarted} {
		if count < 0 {
			panic("task: work counts must not be negative")
		}
		used += count
	}
	if progress.Total < 1 || used > progress.Total {
		panic("task: invalid work total")
	}
	r.group.mu.Lock()
	defer r.group.mu.Unlock()
	if r.task.completed.Load() {
		panic("task: cannot update finished work")
	}
	if r.group.work != nil && r.group.work.Total != progress.Total {
		panic("task: work total is immutable")
	}
	r.group.work = &progress
	r.task.SetProgress(progress.Completed+progress.Cached+progress.Failed, progress.Total)
}

func (g *Group) snapshotWork() *WorkProgress {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.work == nil {
		return nil
	}
	progress := *g.work
	return &progress
}

// Group attaches executable steps to this externally-owned run.
func (r *ManagedRun) Group() *Group { return r.group }
