package task

import (
	"fmt"
	"maps"
	"math"

	"github.com/flanksource/commons/text"
)

// ProgressMetrics describes measured work independently from its phase label.
type ProgressMetrics struct {
	Unit           string  `json:"unit,omitempty"`
	Bytes          int64   `json:"bytes,omitempty"`
	BytesPerSecond float64 `json:"bytesPerSecond,omitempty"`
}

func (t *Task) SetProgressMetrics(metrics ProgressMetrics) {
	if metrics.Bytes < 0 || metrics.BytesPerSecond < 0 || math.IsNaN(metrics.BytesPerSecond) || math.IsInf(metrics.BytesPerSecond, 0) {
		panic("task: invalid progress metrics")
	}
	t.mu.Lock()
	t.progressMetrics = &metrics
	t.mu.Unlock()
	t.dirty.Store(true)
	t.notifyProgress()
}

func (m ProgressMetrics) String() string {
	result := m.Unit
	if m.Bytes > 0 {
		result += " · " + text.HumanizeBytes(m.Bytes)
	}
	if m.BytesPerSecond > 0 {
		result += fmt.Sprintf(" · %s/s", text.HumanizeBytes(int64(m.BytesPerSecond)))
	}
	return result
}

func (t *Task) SetLabels(labels map[string]string) {
	t.mu.Lock()
	t.labels = maps.Clone(labels)
	t.mu.Unlock()
	t.dirty.Store(true)
}

func (t *Task) snapshotProgress(snap *TaskSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	snap.Description, snap.Progress, snap.MaxValue = t.description, t.progress, t.maxValue
	snap.Labels = maps.Clone(t.labels)
	if t.progressMetrics != nil {
		metrics := *t.progressMetrics
		snap.ProgressMetrics = &metrics
	}
}
