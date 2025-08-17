package task

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/commons/collections"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"golang.org/x/term"
)

// Manager manages and displays multiple tasks with progress bars
type Manager struct {
	tasks         []*Task
	groups        []*Group
	mu            sync.RWMutex
	wg            sync.WaitGroup
	stopRender    chan bool
	width         int
	verbose       bool
	maxConcurrent int
	semaphore     chan struct{}
	retryConfig   RetryConfig
	isInteractive bool
	renderer      *lipgloss.Renderer
	styles        styleSet

	// Signal management
	signalChan       chan os.Signal
	signalRegistered bool
	gracefulTimeout  time.Duration
	onInterrupt      func() // optional cleanup callback
	signalMu         sync.Mutex
	shutdownOnce     sync.Once
	noColor          bool // Disable colored output
	noProgress       bool // Disable progress display

	// Priority queue for task scheduling
	taskQueue     *collections.Queue[*Task]
	workers       []*worker
	shutdown      chan struct{}
	workersActive atomic.Int32
}

var Global *Manager

type styleSet struct {
	success   lipgloss.Style
	failed    lipgloss.Style
	warning   lipgloss.Style
	running   lipgloss.Style
	bar       lipgloss.Style
	info      lipgloss.Style
	error     lipgloss.Style
	cancelled lipgloss.Style
	pending   lipgloss.Style
}

// NewManager creates a new TaskManager instance
func NewManager() *Manager {
	return NewManagerWithConcurrency(0) // 0 means unlimited
}

// NewManagerWithConcurrency creates a new TaskManager with concurrency limit
func NewManagerWithConcurrency(maxConcurrent int) *Manager {
	// Check stderr for terminal size since we output there
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		width = 80
	}
	if width == 0 {
		width = 80
	}

	// Check if stderr is a terminal (for interactive mode)
	isInteractive := term.IsTerminal(int(os.Stderr.Fd()))

	// Create a renderer that outputs to stderr for proper color detection
	renderer := lipgloss.NewRenderer(os.Stderr)

	// Default to single worker if not specified
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	// Create priority queue for tasks
	taskQueue, err := collections.NewQueue(collections.QueueOpts[*Task]{
		Comparator: func(a, b *Task) int {
			// Compare by priority first (lower priority value = higher priority)
			if a.priority != b.priority {
				if a.priority < b.priority {
					return -1
				} else if a.priority > b.priority {
					return 1
				}
				return 0
			}
			// Then by enqueue time (earlier = higher priority)
			if !a.enqueuedAt.Equal(b.enqueuedAt) {
				if a.enqueuedAt.Before(b.enqueuedAt) {
					return -1
				}
				return 1
			}
			return 0
		},
		Dedupe: false,
		Metrics: collections.MetricsOpts[*Task]{
			Disable: true,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create task queue: %v", err))
	}

	// Check if task logger is at debug level for verbosity
	taskLogger := logger.GetLogger("task")
	verbose := taskLogger.IsLevelEnabled(3) || os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != ""

	// On debug level, progress updates should print to stderr
	noProgress := false
	if verbose && !isInteractive {
		noProgress = true
	}

	tm := &Manager{
		tasks:           make([]*Task, 0),
		groups:          make([]*Group, 0),
		stopRender:      make(chan bool, 1),
		width:           width,
		verbose:         verbose,
		noProgress:      noProgress,
		maxConcurrent:   maxConcurrent,
		retryConfig:     DefaultRetryConfig(),
		isInteractive:   isInteractive,
		renderer:        renderer,
		gracefulTimeout: 10 * time.Second, // Default 10 second graceful shutdown
		taskQueue:       taskQueue,
		workers:         make([]*worker, 0, maxConcurrent),
		shutdown:        make(chan struct{}),
	}

	if maxConcurrent > 0 {
		tm.semaphore = make(chan struct{}, maxConcurrent)
	}

	// Use the stderr renderer for creating styles
	tm.styles.success = renderer.NewStyle().Foreground(lipgloss.Color("10"))
	tm.styles.failed = renderer.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.warning = renderer.NewStyle().Foreground(lipgloss.Color("11"))
	tm.styles.running = renderer.NewStyle().Foreground(lipgloss.Color("14"))
	tm.styles.bar = renderer.NewStyle().Foreground(lipgloss.Color("12"))
	tm.styles.info = renderer.NewStyle().Foreground(lipgloss.Color("8"))
	tm.styles.error = renderer.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.cancelled = renderer.NewStyle().Foreground(lipgloss.Color("13"))
	tm.styles.pending = renderer.NewStyle().Foreground(lipgloss.Color("7"))

	// Start worker goroutines
	for i := 0; i < maxConcurrent; i++ {
		w := &worker{
			id:      i,
			manager: tm,
		}
		tm.workers = append(tm.workers, w)
		go w.run()
	}

	// Register signal handling by default
	tm.registerSignalHandling()

	// Only start interactive rendering if stderr is a terminal
	if tm.isInteractive {
		go tm.render()
	}
	return tm
}

// SetVerbose enables or disables verbose logging
func (tm *Manager) SetVerbose(verbose bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.verbose = verbose
}

// SetNoColor enables or disables colored output
func (tm *Manager) SetNoColor(noColor bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.noColor = noColor
}

// SetNoProgress enables or disables progress display
func (tm *Manager) SetNoProgress(noProgress bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.noProgress = noProgress
}

// SetMaxConcurrent sets the maximum number of concurrent tasks
func (tm *Manager) SetMaxConcurrent(max int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.maxConcurrent == max {
		return
	}

	tm.maxConcurrent = max
	if max > 0 {
		// Create new semaphore with new size
		newSem := make(chan struct{}, max)
		// Transfer existing permits if any
		if tm.semaphore != nil {
			close(tm.semaphore)
		}
		tm.semaphore = newSem
	} else {
		// Unlimited concurrency
		if tm.semaphore != nil {
			close(tm.semaphore)
			tm.semaphore = nil
		}
	}
}

// SetRetryConfig sets the default retry configuration for new tasks
func (tm *Manager) SetRetryConfig(config RetryConfig) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.retryConfig = config
}

// SetGracefulTimeout sets the timeout for graceful shutdown
func (tm *Manager) SetGracefulTimeout(timeout time.Duration) {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	tm.gracefulTimeout = timeout
}

// SetInterruptHandler sets a custom callback to be called on interrupt
func (tm *Manager) SetInterruptHandler(fn func()) {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	tm.onInterrupt = fn
}

func (tm *Manager) newTask(name string, opts ...Option) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	flanksourceCtx := flanksourceContext.NewContext(ctx)
	flanksourceCtx.Logger = logger.GetSlogLogger().Named(fmt.Sprintf("task.%s", name))

	task := &Task{
		name:           name,
		status:         StatusPending,
		progress:       0,
		maxValue:       100,
		startTime:      time.Now(),
		manager:        tm,
		logs:           make([]LogEntry, 0),
		cancel:         cancel,
		ctx:            flanksourceCtx,
		flanksourceCtx: flanksourceCtx,
		retryConfig:    tm.retryConfig,
		retryCount:     0,
		doneChan:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(task)
	}

	// Set up timeout if specified
	if task.timeout > 0 {
		timeoutCtx, timeoutCancel := flanksourceCtx.WithTimeout(task.timeout)
		task.ctx = timeoutCtx
		task.flanksourceCtx = timeoutCtx

		oldCancel := task.cancel
		task.cancel = func() {
			timeoutCancel()
			oldCancel()
		}
	}

	// Calculate priority based on dependencies
	if len(task.dependencies) == 0 {
		task.priority = 0 // No dependencies = highest priority
	} else {
		task.priority = 1 // Has dependencies = lower priority
	}

	task.enqueuedAt = time.Now()

	return task
}

func (tm *Manager) enqueue(task *Task) *Task {
	task.enqueuedAt = time.Now()
	tm.tasks = append(tm.tasks, task)
	tm.taskQueue.Enqueue(task)
	return task
}

// Start creates and starts tracking a new task
func (tm *Manager) Start(name string, opts ...Option) *Task {
	return tm.enqueue(tm.newTask(name, opts...))
}

func StartTask[T any](name string, taskFunc func(flanksourceContext.Context, *Task) (T, error), opts ...Option) TypedTask[T] {

	// Wrap the typed function to work with the existing interface{} system
	wrappedFunc := func(ctx flanksourceContext.Context, t *Task) (interface{}, error) {
		result, err := taskFunc(ctx, t)
		return result, err
	}
	t := Global.StartWithResult(name, wrappedFunc, opts...)
	return TypedTask[T]{t}

}

// StartWithResult creates and starts tracking a new task with typed result handling
func (tm *Manager) StartWithResult(name string, taskFunc func(flanksourceContext.Context, *Task) (interface{}, error), opts ...Option) *Task {

	task := tm.newTask(name, opts...)

	// Wrap the result function in a regular func(flanksourceContext.Context, *Task) error
	task.runFunc = func(ctx flanksourceContext.Context, t *Task) error {
		result, err := taskFunc(ctx, t)
		if err != nil {
			t.err = err
			return err
		}
		// Store the result
		t.mu.Lock()
		t.result = result
		if result != nil {
			t.resultType = reflect.TypeOf(result)
		}
		t.mu.Unlock()
		return nil
	}

	return tm.enqueue(task)
}

// StartGroup creates and starts tracking a new task group
func StartGroup[T any](name string) TypedGroup[T] {
	ctx, cancel := context.WithCancel(context.Background())
	group := &Group{
		name:    name,
		Items:   make([]Taskable, 0),
		manager: Global,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Add to groups list for tracking
	Global.groups = append(Global.groups, group)

	return TypedGroup[T]{group}
}

// Run starts all tasks and waits for completion
func (tm *Manager) Run() error {
	tm.Wait()

	// Check if any tasks failed
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, task := range tm.tasks {
		if task.err != nil {
			return fmt.Errorf("task %s failed: %w", task.name, task.err)
		}
	}
	return nil
}

// CancelAll cancels all running tasks and groups
func (tm *Manager) CancelAll() {

	// Cancel all tasks
	for _, task := range tm.tasks {
		task.Cancel()
	}

	// Cancel all groups
	for _, group := range tm.groups {
		group.Cancel()
	}
}

// ClearTasks removes all completed tasks from the task list
func (tm *Manager) ClearTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Only keep running or pending tasks
	var activeTasks []*Task
	for _, task := range tm.tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		if status == StatusPending || status == StatusRunning {
			activeTasks = append(activeTasks, task)
		}
	}

	tm.tasks = activeTasks
}

// WaitSilent waits for all tasks to complete without displaying results
func (tm *Manager) WaitSilent() int {
	// Wait for queue to be empty and all workers to be idle
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Check if queue is empty and no workers are active
		if tm.taskQueue.Empty() && tm.workersActive.Load() == 0 {
			// Also check all tasks are completed
			allComplete := true
			tm.mu.RLock()
			for _, task := range tm.tasks {
				if !task.completed.Load() {
					allComplete = false
					break
				}
			}
			tm.mu.RUnlock()

			if allComplete {
				break
			}
		}

		<-ticker.C
	}

	tm.stopRender <- true

	tm.mu.RLock()
	tasks := tm.tasks
	tm.mu.RUnlock()

	// Calculate exit code based on task status
	for _, task := range tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		switch status {
		case StatusFailed, StatusCancelled:
			return 1
		}
	}

	return 0
}

// Wait waits for all tasks to complete and returns the appropriate exit code
func (tm *Manager) Wait() int {
	// Wait for queue to be empty and all workers to be idle
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Check if queue is empty and no workers are active
		if tm.taskQueue.Empty() && tm.workersActive.Load() == 0 {
			// Also check all tasks are completed
			allComplete := true
			tm.mu.RLock()
			for _, task := range tm.tasks {
				if !task.completed.Load() {
					allComplete = false
					break
				}
			}
			tm.mu.RUnlock()

			if allComplete {
				break
			}
		}

		<-ticker.C
	}

	tm.stopRender <- true

	var failed, cancelled int

	for _, task := range tm.tasks {
		task.mu.Lock()
		status := task.status
		task.mu.Unlock()

		switch status {
		case StatusFailed:
			failed++
		case StatusCancelled:
			cancelled++
		}
	}

	if failed+cancelled > 0 {
		return 1
	}
	return 0
}

// Debug returns debug information about the task manager
func (tm *Manager) Debug() string {
	var result string
	result += fmt.Sprintf("Task Manager: {no-color=%v, no-progress=%v, workers=%v}\n", tm.noColor, tm.noProgress, tm.workersActive.Load())
	result += fmt.Sprintf("  Total Tasks: %d\n", len(tm.tasks))
	result += fmt.Sprintf("  Active Workers: %d\n", tm.workersActive.Load())
	result += "  Task Details:\n"
	for _, task := range tm.tasks {
		task.mu.Lock()
		result += fmt.Sprintf("    - %s: %v\n", task.name, task.status)
		task.mu.Unlock()
	}
	return result
}
