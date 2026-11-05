package task

// WithOnProgress observes progress changes after the task releases its lock.
// The callback may read snapshots and must be safe for concurrent calls.
func WithOnProgress(changed func()) Option {
	return func(t *Task) { t.onProgress = changed }
}

func (t *Task) notifyProgress() {
	if t.onProgress != nil {
		t.onProgress()
	}
}
