package task

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
)

type snapshotLogCursor struct {
	mu      sync.Mutex
	last    uint64
	cleared uint64
	entries map[snapshotLogKey][]uint64
}

type snapshotLogKey struct {
	time    time.Time
	level   logger.LogLevel
	message string
}

// update assigns stream-local sequences to newly observed entries. Retention
// can evict entries independently per level, so an index is not a stable cursor.
// Keep only retained identities and sort by sequence for SSE's binary search.
func (c *snapshotLogCursor) update(entries []logger.BufferedLogEntry) []LogEntry {
	retained := make(map[snapshotLogKey][]uint64, len(entries))
	var logs []LogEntry
	for _, entry := range entries {
		key := snapshotLogKey{time: entry.Time, level: entry.Level, message: entry.Message}
		var seq uint64
		if previous := c.entries[key]; len(previous) > 0 {
			seq, c.entries[key] = previous[0], previous[1:]
		} else {
			c.last++
			seq = c.last
		}
		retained[key] = append(retained[key], seq)
		logs = append(logs, LogEntry{Seq: seq, Level: entry.Level.String(), Message: entry.Message, Time: entry.Time.UTC().Format(time.RFC3339Nano)})
	}
	c.entries = retained
	slices.SortFunc(logs, func(a, b LogEntry) int { return cmp.Compare(a.Seq, b.Seq) })
	return logs
}

// snapshotLogs serializes snapshot cursors with explicit clears without changing
// the Commons logger's retention or logging behavior.
func (t *Task) snapshotLogs(snapshot *TaskSnapshot) {
	t.logCursor.mu.Lock()
	defer t.logCursor.mu.Unlock()
	snapshot.Logs = t.logCursor.update(t.getBufferedLogger().GetLogs())
	snapshot.LogsClearedThrough = t.logCursor.cleared
	if len(snapshot.Logs) > 0 {
		snapshot.Message = snapshot.Logs[len(snapshot.Logs)-1].Message
	}
}
