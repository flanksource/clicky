package shutdown

import (
	"container/heap"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/flanksource/commons/logger"
	"golang.org/x/term"
)

const (
	PriorityIngress  = 0
	PriorityDefault  = 100
	PriorityWorkers  = 200
	PriorityDatabase = 300
	PriorityCritical = 400
)

type Hook struct {
	label    string
	priority int
	fn       func()
	index    int // for heap interface
}

type HookHeap []*Hook

func (h HookHeap) Len() int           { return len(h) }
func (h HookHeap) Less(i, j int) bool { return h[i].priority < h[j].priority }
func (h HookHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *HookHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*Hook)
	item.index = n
	*h = append(*h, item)
}

func (h *HookHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*h = old[0 : n-1]
	return item
}

var (
	hooks               HookHeap
	hooksMux            sync.Mutex
	once                sync.Once
	terminalRestoreFunc func()
	terminalRestoreMu   sync.Mutex
	terminalWriter      io.Writer = os.Stderr
)

// SetTerminalRestoreFunc registers a callback that performs full terminal
// state restoration (e.g. term.Restore with saved state). Called automatically
// during Shutdown, force-exit, and RecoverAndShutdown.
func SetTerminalRestoreFunc(fn func()) {
	terminalRestoreMu.Lock()
	defer terminalRestoreMu.Unlock()
	terminalRestoreFunc = fn
}

// SetTerminalWriter points the terminal reset at the writer that owns the
// terminal, so the sequence serializes with everything else clicky prints
// there. It must be a writer that reaches the terminal synchronously: the reset
// is terminal control, not log output, and a writer that defers or drops it
// leaves the cursor hidden after exit. Defaults to os.Stderr — the writer this
// package can reach without depending on the renderer that sits above it.
func SetTerminalWriter(w io.Writer) {
	terminalRestoreMu.Lock()
	defer terminalRestoreMu.Unlock()
	if w == nil {
		w = os.Stderr
	}
	terminalWriter = w
}

// restoreTerminal ensures terminal is in a clean state before exit.
// Uses raw ANSI escapes as a safety net, then calls the registered
// restore callback for full term.Restore if available.
//
// The escapes are only meaningful to a terminal; writing them to a redirected
// stderr appends junk bytes to whatever the user captured.
//
// Deliberately no "\x1b[?1049l": nothing here ever enters the alternate screen,
// and asking a terminal to leave a buffer it was never in is not a no-op
// everywhere — terminals that honour it restore the primary buffer and discard
// the output the command just finished printing. A consumer that does use the
// alternate screen restores it through SetTerminalRestoreFunc, which knows it
// entered.
func restoreTerminal() {
	terminalRestoreMu.Lock()
	writer := terminalWriter
	fn := terminalRestoreFunc
	terminalRestoreMu.Unlock()

	if term.IsTerminal(int(os.Stderr.Fd())) {
		fmt.Fprint(writer, "\x1b[?25h\x1b[0m")
	}
	if fn != nil {
		fn()
	}
}

// RecoverAndShutdown is intended to be deferred in main(). On panic it
// runs all shutdown hooks and restores terminal state, then re-panics
// so the stack trace is preserved.
func RecoverAndShutdown() {
	if r := recover(); r != nil {
		Shutdown()
		restoreTerminal()
		panic(r)
	}
}

// AddHook registers a shutdown hook with default priority
func AddHook(label string, fn func()) {
	AddHookWithPriority(label, PriorityDefault, fn)
}

// AddHookWithPriority registers a shutdown hook with specific priority
func AddHookWithPriority(label string, priority int, fn func()) {
	hooksMux.Lock()
	defer hooksMux.Unlock()

	hook := &Hook{
		label:    label,
		priority: priority,
		fn:       fn,
	}
	heap.Push(&hooks, hook)
}

// Shutdown executes all registered hooks in priority order
func Shutdown() {
	logger.Debugf("Starting graceful shutdown with %d hooks", len(hooks))
	hooksMux.Lock()
	defer hooksMux.Unlock()

	// Always restore terminal state, even if no hooks are registered.
	defer restoreTerminal()

	if len(hooks) == 0 {
		return
	}

	logger.Debugf("Executing %d shutdown hooks", len(hooks))

	// Execute hooks in priority order (lowest priority first)
	for hooks.Len() > 0 {
		hook := heap.Pop(&hooks).(*Hook)
		logger.Debugf("Executing shutdown hook: %s (priority=%d)", hook.label, hook.priority)

		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Panic in shutdown hook %s: %v", hook.label, r)
				}
			}()
			hook.fn()
		}()
	}

	// The deferred restoreTerminal above already covers every return path.
	logger.Debugf("All shutdown hooks executed")
}

// WaitForSignal waits for interrupt signals and triggers shutdown
func WaitForSignal() {
	once.Do(func() {
		sigChan := make(chan os.Signal, 100)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "\n🛑 Received %s - initiating graceful shutdown..., %d hooks\n", sig, len(hooks))
		fmt.Fprintf(os.Stderr, "   Press Ctrl+C again to force immediate exit\n\n")

		// Set up force exit on second signal
		go func() {
			select {
			case <-sigChan:
				fmt.Fprintf(os.Stderr, "\n⚠️  Force exit\n")
				restoreTerminal()
				os.Exit(1)
			case <-time.After(30 * time.Second):
				return
			}
		}()

		Shutdown()
		os.Exit(0)
	})
}

// RunAndWait runs the provided function and then waits for shutdown signal
func RunAndWait(fn func() error) error {
	if err := fn(); err != nil {
		return err
	}
	WaitForSignal()
	return nil
}
