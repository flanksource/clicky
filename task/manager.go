package task

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/commons/collections"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
	"golang.org/x/term"
)

// Manager manages and displays multiple tasks with progress bars
type Manager struct {
	tasks         []*Task
	groups        []*Group
	mu            sync.RWMutex
	width         int
	verbose       atomic.Bool
	maxConcurrent int
	semaphore     chan struct{}
	retryConfig   RetryConfig
	isInteractive bool
	renderer      *lipgloss.Renderer
	styles        styleSet

	gracefulTimeout time.Duration
	onInterrupt     func()      // optional cleanup callback
	noColor         atomic.Bool // Disable colored output
	noProgress      atomic.Bool // Disable progress display
	noRender        atomic.Bool // Disable all task rendering

	// Priority queue for task scheduling
	taskQueue     *collections.Queue[*Task]
	workers       []*worker
	shutdown      chan struct{}
	workersActive atomic.Int32

	// Task identity tracking for deduplication
	tasksByIdentity sync.Map // map[string]*Task

	// Render loop control
	stopRenderCh  chan struct{}
	renderDone    chan struct{}
	renderStopped sync.Once
	renderStarted sync.Once
	renderOwnsTTY bool

	// Terminal state
	originalTermState *term.State

	// Output buffering
	outputBuffer    []OutputEntry
	bufferMutex     sync.Mutex
	originalStdout  *os.File
	originalStderr  *os.File
	capturingOutput bool
	stdoutReader    *os.File
	stdoutWriter    *os.File
	stderrReader    *os.File
	stderrWriter    *os.File
}

var global *Manager

type styleSet struct {
	success  lipgloss.Style
	failed   lipgloss.Style
	warning  lipgloss.Style
	running  lipgloss.Style
	bar      lipgloss.Style
	info     lipgloss.Style
	error    lipgloss.Style
	canceled lipgloss.Style
	pending  lipgloss.Style
}

func init() {
	global = newManager()

	if isTestEnvironment() {
		SetNoProgress(true)
		SetNoColor(true)
	}
}

func isTestEnvironment() bool {
	if os.Getenv("GO_TEST") != "" {
		return true
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	testEnvVars := []string{
		"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "TRAVIS", "CIRCLECI",
		"DEPS_TEST", "NO_PROGRESS", "TEST_ENV",
	}
	for _, envVar := range testEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	return false
}

func newManager() *Manager {
	return newManagerWithConcurrency(0)
}

func newManagerWithConcurrency(maxConcurrent int) *Manager {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || width == 0 {
		width = 120
	}

	isInteractive := term.IsTerminal(int(os.Stderr.Fd()))

	// Save original terminal state for restoration on exit
	var originalTermState *term.State
	if isInteractive {
		if state, err := term.GetState(int(os.Stderr.Fd())); err == nil {
			originalTermState = state
		}
	}

	renderer := lipgloss.NewRenderer(os.Stderr)

	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}

	taskQueue, err := collections.NewQueue(collections.QueueOpts[*Task]{
		Comparator: func(a, b *Task) int {
			if a.priority != b.priority {
				if a.priority < b.priority {
					return -1
				} else if a.priority > b.priority {
					return 1
				}
				return 0
			}
			if !a.enqueuedAt.Equal(b.enqueuedAt) {
				if a.enqueuedAt.Before(b.enqueuedAt) {
					return -1
				}
				return 1
			}
			return 0
		},
		Equals: func(a, b *Task) bool {
			if a.identity != "" && b.identity != "" {
				return a.identity == b.identity
			}
			return a == b
		},
		Dedupe: true,
		Metrics: collections.MetricsOpts[*Task]{
			Disable: true,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create task queue: %v", err))
	}

	taskLogger := logger.GetLogger("task")
	verbose := taskLogger.IsLevelEnabled(3) || os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != ""

	tm := &Manager{
		tasks:             make([]*Task, 0),
		groups:            make([]*Group, 0),
		width:             width,
		maxConcurrent:     maxConcurrent,
		retryConfig:       DefaultRetryConfig(),
		isInteractive:     isInteractive,
		renderer:          renderer,
		originalTermState: originalTermState,
		gracefulTimeout:   10 * time.Second,
		taskQueue:         taskQueue,
		workers:           make([]*worker, 0, maxConcurrent),
		shutdown:          make(chan struct{}),
		semaphore:         make(chan struct{}, maxConcurrent),
	}
	tm.verbose.Store(verbose)

	tm.styles.success = renderer.NewStyle().Foreground(lipgloss.Color("10"))
	tm.styles.failed = renderer.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.warning = renderer.NewStyle().Foreground(lipgloss.Color("11"))
	tm.styles.running = renderer.NewStyle().Foreground(lipgloss.Color("14"))
	tm.styles.bar = renderer.NewStyle().Foreground(lipgloss.Color("12"))
	tm.styles.info = renderer.NewStyle().Foreground(lipgloss.Color("8"))
	tm.styles.error = renderer.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.canceled = renderer.NewStyle().Foreground(lipgloss.Color("13"))
	tm.styles.pending = renderer.NewStyle().Foreground(lipgloss.Color("7"))

	shutdown.SetTerminalRestoreFunc(tm.cleanupTerminal)

	for i := 0; i < maxConcurrent; i++ {
		w := &worker{id: i, manager: tm}
		tm.workers = append(tm.workers, w)
		go w.run()
	}

	shutdown.AddHookWithPriority("TaskManager", shutdown.PriorityWorkers, func() {
		close(tm.shutdown)
		CancelAll()

		done := make(chan bool, 1)
		go func() {
			timeout := time.After(tm.gracefulTimeout)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeout:
					done <- false
					return
				case <-ticker.C:
					if tm.taskQueue.Empty() && tm.workersActive.Load() == 0 {
						done <- true
						return
					}
				}
			}
		}()

		select {
		case success := <-done:
			if success {
				fmt.Fprintf(os.Stderr, "All tasks completed gracefully\n")
			} else {
				fmt.Fprintf(os.Stderr, "Task shutdown timeout reached\n")
			}
		case <-time.After(tm.gracefulTimeout + time.Second):
			fmt.Fprintf(os.Stderr, "Task shutdown timeout exceeded\n")
		}

		tm.stopRender()
		tm.StopCapturingOutput()
	})

	return tm
}

// SetVerbose enables or disables verbose logging
func SetVerbose(verbose bool) {
	global.verbose.Store(verbose)
}

// SetNoColor enables or disables colored output
func SetNoColor(noColor bool) {
	global.noColor.Store(noColor)
}

// SetNoProgress enables or disables progress display
func SetNoProgress(noProgress bool) {
	global.noProgress.Store(noProgress)
}

// SetNoRender enables or disables all task rendering, including final summaries.
func SetNoRender(noRender bool) {
	global.noRender.Store(noRender)
}

// IsNoRender reports whether task rendering is currently disabled.
func IsNoRender() bool {
	return global.noRender.Load()
}

// SetMaxConcurrent sets the maximum number of concurrent tasks
func SetMaxConcurrent(max int) {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.maxConcurrent == max {
		return
	}

	global.maxConcurrent = max
	if max > 0 {
		newSem := make(chan struct{}, max)
		if global.semaphore != nil {
			close(global.semaphore)
		}
		global.semaphore = newSem
	} else {
		if global.semaphore != nil {
			close(global.semaphore)
			global.semaphore = nil
		}
	}
}

// SetRetryConfig sets the default retry configuration for new tasks
func SetRetryConfig(config RetryConfig) {
	global.retryConfig = config
}

// SetGracefulTimeout sets the timeout for graceful shutdown
func SetGracefulTimeout(timeout time.Duration) {
	global.gracefulTimeout = timeout
}

// SetInterruptHandler sets a custom callback to be called on interrupt
func SetInterruptHandler(fn func()) {
	global.onInterrupt = fn
}

func (tm *Manager) newTask(name string, opts ...Option) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	flanksourceCtx := flanksourceContext.NewContext(ctx)
	flanksourceCtx.Logger = logger.GetSlogLogger().Named(fmt.Sprintf("task.%s", name))

	task := &Task{
		name:           name,
		id:             uuid.NewString(),
		status:         StatusPending,
		progress:       0,
		maxValue:       100,
		startTime:      time.Now(),
		manager:        tm,
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

	if len(task.dependencies) == 0 {
		task.priority = 0
	} else {
		task.priority = 1
	}

	task.enqueuedAt = time.Now()
	return task
}

func (tm *Manager) enqueue(task *Task) *Task {
	if !tm.noRender.Load() {
		tm.renderStarted.Do(func() {
			if !tm.noProgress.Load() {
				tm.startRenderLoop()
			}
		})
	}

	if task.identity != "" {
		if existing, ok := tm.tasksByIdentity.Load(task.identity); ok {
			return existing.(*Task)
		}
		tm.tasksByIdentity.Store(task.identity, task)
	}

	task.enqueuedAt = time.Now()

	tm.mu.Lock()
	tm.tasks = append(tm.tasks, task)
	tm.mu.Unlock()

	tm.taskQueue.Enqueue(task)
	return task
}

// Start creates and starts tracking a new task
func (tm *Manager) Start(name string, opts ...Option) *Task {
	return tm.enqueue(tm.newTask(name, opts...))
}

func StartTask[T any](name string, taskFunc func(flanksourceContext.Context, *Task) (T, error), opts ...Option) TypedTask[T] {
	wrappedFunc := func(ctx flanksourceContext.Context, t *Task) (interface{}, error) {
		result, err := taskFunc(ctx, t)
		return result, err
	}
	t := global.StartWithResult(name, wrappedFunc, opts...)
	return TypedTask[T]{t}
}

// StartWithResult creates and starts tracking a new task with typed result handling
func (tm *Manager) StartWithResult(name string, taskFunc func(flanksourceContext.Context, *Task) (interface{}, error), opts ...Option) *Task {
	task := tm.newTask(name, opts...)

	task.runFunc = func(ctx flanksourceContext.Context, t *Task) error {
		result, err := taskFunc(ctx, t)

		t.mu.Lock()
		t.result = result
		if result != nil {
			t.resultType = reflect.TypeOf(result)
		}
		t.mu.Unlock()

		if err != nil {
			t.err = err
			return err
		}
		return nil
	}

	return tm.enqueue(task)
}

// StartGroup creates and starts tracking a new task group
func StartGroup[T any](name string, opts ...TaskGroupOption) TypedGroup[T] {
	ctx, cancel := context.WithCancel(context.Background())
	group := &Group{
		name:    name,
		Items:   make([]Taskable, 0),
		manager: global,
		ctx:     ctx,
		cancel:  cancel,
	}

	global.groups = append(global.groups, group)

	for _, opt := range opts {
		opt(group)
	}

	if group.concurrency > 0 {
		group.sem = semaphore.NewWeighted(int64(group.concurrency))
	}

	return TypedGroup[T]{group}
}
