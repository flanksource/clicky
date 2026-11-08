package task

import (
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("snapshot log cursors", func() {
	It("keeps retained entries stable across levels, repeated reads, and eviction", func() {
		task := &Task{bufferedLogger: logger.NewBufferedLogger(1)}
		task.loggerOnce.Do(func() {})
		task.Infof("first")
		task.Warnf("warning")
		first := SnapshotTask(task, nil)
		Expect(first.Logs).To(HaveLen(2))
		Expect(first.Logs[0].Seq).To(BeNumerically("<", first.Logs[1].Seq))
		Expect(SnapshotTask(task, nil).Logs).To(Equal(first.Logs))
		task.Infof("replacement")
		second := SnapshotTask(task, nil)
		Expect(second.Logs).To(HaveLen(2))
		Expect(second.Logs[0]).To(Equal(first.Logs[1]))
		Expect(second.Logs[1].Seq).To(BeNumerically(">", first.Logs[1].Seq))
		task.ClearLogs()
		cleared := SnapshotTask(task, nil)
		Expect(cleared.Logs).To(BeEmpty())
		Expect(cleared.LogsClearedThrough).To(BeNumerically(">=", second.Logs[1].Seq))
		task.Infof("after clear")
		Expect(SnapshotTask(task, nil).Logs[0].Seq).To(BeNumerically(">", cleared.LogsClearedThrough))
	})

	It("distinguishes duplicate entries with identical timestamps", func() {
		entry := logger.BufferedLogEntry{Time: time.Now(), Level: logger.Info, Message: "duplicate"}
		var cursor snapshotLogCursor
		first := cursor.update([]logger.BufferedLogEntry{entry, entry})
		Expect(first).To(HaveLen(2))
		Expect(first[0].Seq).To(BeNumerically("<", first[1].Seq))
		Expect(cursor.update([]logger.BufferedLogEntry{entry, entry})).To(Equal(first))
	})

	It("keeps sequences ascending when timestamp ties reorder levels", func() {
		info := logger.BufferedLogEntry{Time: time.Now(), Level: logger.Info, Message: "info"}
		warning := info
		warning.Level, warning.Message = logger.Warn, "warning"
		var cursor snapshotLogCursor
		first := cursor.update([]logger.BufferedLogEntry{info, warning})
		Expect(cursor.update([]logger.BufferedLogEntry{warning, info})).To(Equal(first))
	})

	It("serializes concurrent log reads and clears", func() {
		task := &Task{}
		var workers sync.WaitGroup
		for worker := range 3 {
			workers.Go(func() {
				for range 20 {
					switch worker {
					case 0:
						task.Infof("concurrent entry")
					case 1:
						task.ClearLogs()
					case 2:
						SnapshotTask(task, nil)
					}
				}
			})
		}
		workers.Wait()
		task.ClearLogs()
		boundary := SnapshotTask(task, nil).LogsClearedThrough
		task.Infof("after concurrent clear")
		logs := SnapshotTask(task, nil).Logs
		Expect(logs).To(HaveLen(1))
		Expect(logs[0].Seq).To(BeNumerically(">", boundary))
	})
})
