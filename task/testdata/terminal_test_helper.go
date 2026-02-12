// terminal_test_helper is a minimal program used by terminal_test.go
// to verify terminal state restoration across different exit modes.
package main

import (
	"fmt"
	"os"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
)

func main() {
	mode := os.Getenv("EXIT_MODE")
	if mode == "" {
		fmt.Fprintln(os.Stderr, "EXIT_MODE not set")
		os.Exit(2)
	}

	task.SetNoProgress(false)
	task.SetNoColor(true)

	switch mode {
	case "normal":
		runNormalExit()
	case "panic":
		runPanicExit()
	case "sigint":
		runSigintExit()
	case "sigint_double":
		runSigintDoubleExit()
	default:
		fmt.Fprintf(os.Stderr, "unknown EXIT_MODE: %s\n", mode)
		os.Exit(2)
	}
}

func runNormalExit() {
	task.StartTask("test-task", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		t.SetDescription("working")
		time.Sleep(100 * time.Millisecond)
		return "done", nil
	})
	task.Wait()
}

func runPanicExit() {
	defer shutdown.RecoverAndShutdown()

	task.StartTask("setup-task", func(_ flanksourceContext.Context, t *task.Task) (string, error) {
		t.SetDescription("setup")
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	})
	task.Wait()

	panic("test panic from main")
}

func runSigintExit() {
	task.SetGracefulTimeout(5 * time.Second)

	task.StartTask("long-task", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		t.SetDescription("waiting for signal")
		select {
		case <-ctx.Done():
			return "cancelled", nil
		case <-time.After(30 * time.Second):
			return "done", nil
		}
	})

	fmt.Fprintln(os.Stderr, "READY")
	shutdown.WaitForSignal()
}

func runSigintDoubleExit() {
	task.SetGracefulTimeout(30 * time.Second)

	task.StartTask("long-task", func(ctx flanksourceContext.Context, t *task.Task) (string, error) {
		t.SetDescription("waiting for signal")
		select {
		case <-ctx.Done():
			return "cancelled", nil
		case <-time.After(60 * time.Second):
			return "done", nil
		}
	})

	fmt.Fprintln(os.Stderr, "READY")
	shutdown.WaitForSignal()
}
