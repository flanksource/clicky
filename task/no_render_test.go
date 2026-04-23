package task

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/text"
	flanksourceContext "github.com/flanksource/commons/context"
)

func TestNoRenderSuppressesFinalOutput(t *testing.T) {
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	t.Cleanup(func() {
		os.Stderr = originalStderr
		_ = w.Close()
		_ = r.Close()
	})
	os.Stderr = w

	var output bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, r)
		close(done)
	}()

	manager := newTestManager(1)
	manager.noRender.Store(true)

	task := manager.newTask("hidden")
	task.runFunc = func(flanksourceContext.Context, *Task) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}
	manager.enqueue(task)

	select {
	case <-task.doneChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task completion")
	}

	manager.stopRender()

	_ = w.Close()
	<-done

	if got := strings.TrimSpace(text.StripANSI(output.String())); got != "" {
		t.Fatalf("expected no rendered task output, got %q", got)
	}
}
