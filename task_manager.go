package clicky

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// TaskStatus represents the status of a task
type TaskStatus int

const (
	// StatusPending indicates the task is waiting to start
	StatusPending TaskStatus = iota
	// StatusRunning indicates the task is currently running
	StatusRunning
	// StatusSuccess indicates the task completed successfully
	StatusSuccess
	// StatusFailed indicates the task failed
	StatusFailed
	// StatusWarning indicates the task completed with warnings
	StatusWarning
	// StatusCancelled indicates the task was cancelled
	StatusCancelled
)

// Waitable represents something that can be waited on (Task or TaskGroup)
type Waitable interface {
	Name() string
	Status() TaskStatus
	WaitFor() *WaitResult
	Context() context.Context
	Cancel()
	Duration() time.Duration
	IsGroup() bool
}

// WaitResult contains unified result information
type WaitResult struct {
	Status       TaskStatus
	Duration     time.Duration
	Error        error
	TaskCount    int // Number of individual tasks (1 for Task, N for TaskGroup)
	SuccessCount int // Number of successful tasks
	FailureCount int // Number of failed tasks
	WarningCount int // Number of tasks with warnings
}

// LogEntry represents a log message from a task
type LogEntry struct {
	Level   string
	Message string
	Time    time.Time
}

// RetryConfig holds configuration for task retry behavior
type RetryConfig struct {
	MaxRetries      int
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	RetryableErrors []string // Error message patterns that should trigger retries
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		BaseDelay:       1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		RetryableErrors: []string{"timeout", "connection", "temporary", "rate limit", "429"},
	}
}

// TaskFunc is a generic task function that returns a typed result
type TaskFunc[T any] func(*Task) (T, error)

// TaskResult holds a typed result and error
type TaskResult[T any] struct {
	Result T
	Error  error
}

// Task represents a single task being tracked by the TaskManager
type Task struct {
	name        string
	modelName   string
	prompt      string
	status      TaskStatus
	progress    int
	maxValue    int
	startTime   time.Time
	endTime     time.Time
	manager     *TaskManager
	logs        []LogEntry
	cancel      context.CancelFunc
	ctx         context.Context
	timeout     time.Duration
	runFunc     func(*Task) error
	err         error
	mu          sync.Mutex
	retryConfig RetryConfig
	retryCount  int
	parent      *TaskGroup // Reference to parent group (nil if ungrouped)
	
	// Generic result storage
	result      interface{}
	resultType  reflect.Type
}

// TaskManager manages and displays multiple tasks with progress bars
type TaskManager struct {
	tasks         []*Task
	groups        []*TaskGroup
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
	styles        struct {
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
	
	// Signal management
	signalChan       chan os.Signal
	signalRegistered bool
	gracefulTimeout  time.Duration
	onInterrupt      func() // optional cleanup callback
	signalMu         sync.Mutex
	shutdownOnce     sync.Once
}

// TaskGroup represents a group of tasks that can be managed collectively
type TaskGroup struct {
	name      string
	items     []Waitable // Can contain Tasks or nested TaskGroups
	status    TaskStatus
	startTime time.Time
	endTime   time.Time
	manager   *TaskManager
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

// TaskOption configures task creation
type TaskOption func(*Task)

// WithTimeout sets a timeout for the task
func WithTimeout(d time.Duration) TaskOption {
	return func(t *Task) {
		t.timeout = d
	}
}

// WithFunc sets the function to run for the task
func WithFunc(fn func(*Task) error) TaskOption {
	return func(t *Task) {
		t.runFunc = fn
	}
}

// WithModel sets the model name for the task
func WithModel(modelName string) TaskOption {
	return func(t *Task) {
		t.modelName = modelName
	}
}

// WithPrompt sets the prompt for the task
func WithPrompt(prompt string) TaskOption {
	return func(t *Task) {
		t.prompt = prompt
	}
}

// WithRetryConfig sets custom retry configuration for the task
func WithRetryConfig(config RetryConfig) TaskOption {
	return func(t *Task) {
		t.retryConfig = config
	}
}

// NewTaskManager creates a new TaskManager instance
func NewTaskManager() *TaskManager {
	return NewTaskManagerWithConcurrency(0) // 0 means unlimited
}

// NewTaskManagerWithConcurrency creates a new TaskManager with concurrency limit
func NewTaskManagerWithConcurrency(maxConcurrent int) *TaskManager {
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

	tm := &TaskManager{
		tasks:           make([]*Task, 0),
		groups:          make([]*TaskGroup, 0),
		stopRender:      make(chan bool, 1),
		width:           width,
		verbose:         os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != "",
		maxConcurrent:   maxConcurrent,
		retryConfig:     DefaultRetryConfig(),
		isInteractive:   isInteractive,
		renderer:        renderer,
		gracefulTimeout: 10 * time.Second, // Default 10 second graceful shutdown
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

	// Register signal handling by default
	tm.registerSignalHandling()
	
	// Only start interactive rendering if stderr is a terminal
	if tm.isInteractive {
		go tm.render()
	}
	return tm
}

// SetVerbose enables or disables verbose logging
func (tm *TaskManager) SetVerbose(verbose bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.verbose = verbose
}

// SetMaxConcurrent sets the maximum number of concurrent tasks
func (tm *TaskManager) SetMaxConcurrent(max int) {
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
func (tm *TaskManager) SetRetryConfig(config RetryConfig) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.retryConfig = config
}

// SetGracefulTimeout sets the timeout for graceful shutdown
func (tm *TaskManager) SetGracefulTimeout(timeout time.Duration) {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	tm.gracefulTimeout = timeout
}

// SetInterruptHandler sets a custom callback to be called on interrupt
// This is useful for cleanup operations that need to happen before task cancellation
func (tm *TaskManager) SetInterruptHandler(fn func()) {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	tm.onInterrupt = fn
}

// DisableSignalHandling disables automatic signal handling
// Call this if you want to handle signals manually in your application
func (tm *TaskManager) DisableSignalHandling() {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	
	if tm.signalRegistered && tm.signalChan != nil {
		signal.Stop(tm.signalChan)
		close(tm.signalChan)
		tm.signalRegistered = false
	}
}

// Start creates and starts tracking a new task with optional timeout
func (tm *TaskManager) Start(name string, opts ...TaskOption) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	task := &Task{
		name:        name,
		status:      StatusPending,
		progress:    0,
		maxValue:    100,
		startTime:   time.Now(),
		manager:     tm,
		logs:        make([]LogEntry, 0),
		cancel:      cancel,
		ctx:         ctx,
		retryConfig: tm.retryConfig,
		retryCount:  0,
	}

	for _, opt := range opts {
		opt(task)
	}

	// Set up timeout if specified
	if task.timeout > 0 {
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, task.timeout)
		task.ctx = timeoutCtx
		oldCancel := task.cancel
		task.cancel = func() {
			timeoutCancel()
			oldCancel()
		}
	}

	tm.tasks = append(tm.tasks, task)
	tm.wg.Add(1)

	// Start the task execution
	go tm.runTask(task)

	return task
}

// StartWithResult creates and starts tracking a new task with typed result handling
func (tm *TaskManager) StartWithResult(name string, taskFunc func(*Task) (interface{}, error), opts ...TaskOption) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	task := &Task{
		name:        name,
		status:      StatusPending,
		progress:    0,
		maxValue:    100,
		startTime:   time.Now(),
		manager:     tm,
		logs:        make([]LogEntry, 0),
		cancel:      cancel,
		ctx:         ctx,
		retryConfig: tm.retryConfig,
		retryCount:  0,
	}

	// Wrap the result function in a regular func(*Task) error
	task.runFunc = func(t *Task) error {
		result, err := taskFunc(t)
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

	for _, opt := range opts {
		opt(task)
	}

	// Set up timeout if specified
	if task.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, task.timeout)
		task.ctx = ctx
		task.cancel = cancel
	}

	tm.tasks = append(tm.tasks, task)
	tm.wg.Add(1)

	// Start the task execution
	go tm.runTask(task)

	return task
}

// StartGroup creates and starts tracking a new task group
func (tm *TaskManager) StartGroup(name string) *TaskGroup {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	group := &TaskGroup{
		name:      name,
		items:     make([]Waitable, 0),
		status:    StatusPending,
		startTime: time.Now(),
		manager:   tm,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Add to groups list for tracking
	tm.groups = append(tm.groups, group)

	return group
}

// StartTaskInGroup creates and starts a task within an existing group
func (tm *TaskManager) StartTaskInGroup(group *TaskGroup, name string, opts ...TaskOption) *Task {
	task := tm.Start(name, opts...)
	group.Add(task)
	return task
}

// runTask executes a task respecting concurrency limits
func (tm *TaskManager) runTask(task *Task) {
	defer tm.wg.Done()

	// Acquire semaphore if concurrency is limited
	if tm.semaphore != nil {
		select {
		case tm.semaphore <- struct{}{}:
			defer func() { <-tm.semaphore }()
		case <-task.ctx.Done():
			task.mu.Lock()
			if task.status == StatusPending {
				task.status = StatusCancelled
				task.endTime = time.Now()
			}
			task.mu.Unlock()
			return
		}
	}

	// Mark as running
	task.mu.Lock()
	if task.status == StatusPending {
		task.status = StatusRunning
		task.startTime = time.Now()
		// Print start message in non-interactive mode
		if !tm.isInteractive {
			displayName := tm.formatTaskName(task.name, task.modelName, task.prompt)
			fmt.Fprintln(os.Stderr, tm.styles.running.Render(fmt.Sprintf("⟳ Starting %s", displayName)))
		}
	}
	task.mu.Unlock()

	// Run the task function if provided with retry logic
	if task.runFunc != nil {
		tm.runTaskWithRetry(task)
	}
}

// runTaskWithRetry executes a task with retry logic using exponential backoff and jitter
func (tm *TaskManager) runTaskWithRetry(task *Task) {
	for {
		// Monitor context for cancellation/timeout
		done := make(chan error, 1)
		go func() {
			done <- task.runFunc(task)
		}()

		var err error
		select {
		case err = <-done:
			// Task completed (either successfully or with error)
		case <-task.ctx.Done():
			// Task was cancelled or timed out
			task.mu.Lock()
			if task.status == StatusRunning {
				if task.ctx.Err() == context.DeadlineExceeded {
					err = fmt.Errorf("task timed out after %v", task.timeout)
				} else {
					task.status = StatusCancelled
					task.endTime = time.Now()
					task.mu.Unlock()
					return
				}
			}
			task.mu.Unlock()
		}

		task.mu.Lock()
		if task.status != StatusRunning {
			task.mu.Unlock()
			return
		}

		if err != nil {
			// Check if error is retryable
			shouldRetry := tm.shouldRetryError(err, task.retryConfig)

			if shouldRetry && task.retryCount < task.retryConfig.MaxRetries {
				task.retryCount++

				// Log retry attempt
				task.logs = append(task.logs, LogEntry{
					Level:   "warning",
					Message: fmt.Sprintf("Attempt %d failed, retrying: %v", task.retryCount, err),
					Time:    time.Now(),
				})
				task.mu.Unlock()

				// Calculate delay with exponential backoff and jitter
				delay := tm.calculateBackoffDelay(task.retryCount, task.retryConfig)

				// Wait for delay or cancellation
				select {
				case <-time.After(delay):
					// Continue to retry
					continue
				case <-task.ctx.Done():
					// Task was cancelled during backoff
					task.mu.Lock()
					task.status = StatusCancelled
					task.endTime = time.Now()
					task.mu.Unlock()
					return
				}
			} else {
				// No more retries or error is not retryable
				task.err = err
				task.status = StatusFailed
				task.endTime = time.Now()
				task.logs = append(task.logs, LogEntry{
					Level:   "error",
					Message: err.Error(),
					Time:    time.Now(),
				})
				task.mu.Unlock()
				return
			}
		} else {
			// Task succeeded
			task.status = StatusSuccess
			task.endTime = time.Now()
			task.mu.Unlock()
			return
		}
	}
}

// shouldRetryError checks if an error should trigger a retry
func (tm *TaskManager) shouldRetryError(err error, config RetryConfig) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	for _, pattern := range config.RetryableErrors {
		if strings.Contains(errMsg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// calculateBackoffDelay calculates the delay for the next retry with exponential backoff and jitter
func (tm *TaskManager) calculateBackoffDelay(retryCount int, config RetryConfig) time.Duration {
	// Calculate exponential backoff
	delay := float64(config.BaseDelay) * math.Pow(config.BackoffFactor, float64(retryCount-1))

	// Apply maximum delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	jitter := delay * config.JitterFactor * (rand.Float64() - 0.5) * 2
	finalDelay := delay + jitter

	// Ensure delay is never negative
	if finalDelay < 0 {
		finalDelay = float64(config.BaseDelay)
	}

	return time.Duration(finalDelay)
}

// Run starts all tasks and waits for completion
func (tm *TaskManager) Run() error {
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

// Context returns the task's context for cancellation
func (t *Task) Context() context.Context {
	return t.ctx
}

// Cancel cancels the task
func (t *Task) Cancel() {
	t.mu.Lock()
	if t.status == StatusPending || t.status == StatusRunning {
		t.status = StatusCancelled
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		// Don't call wg.Done() here - let runTask handle it
	} else {
		t.mu.Unlock()
	}
}

// CancelAll cancels all running tasks and groups
func (tm *TaskManager) CancelAll() {
	tm.mu.RLock()
	tasks := make([]*Task, len(tm.tasks))
	copy(tasks, tm.tasks)
	groups := make([]*TaskGroup, len(tm.groups))
	copy(groups, tm.groups)
	tm.mu.RUnlock()

	// Cancel all tasks
	for _, task := range tasks {
		task.Cancel()
	}
	
	// Cancel all groups
	for _, group := range groups {
		group.Cancel()
	}
}

// ClearTasks removes all completed tasks from the task list
// This is useful for long-running processes that need to reset the task list
// Note: This does NOT cancel running tasks - use CancelAll() for that
func (tm *TaskManager) ClearTasks() {
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

// registerSignalHandling sets up signal handling for graceful shutdown
func (tm *TaskManager) registerSignalHandling() {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()
	
	if tm.signalRegistered {
		return // Already registered
	}
	
	tm.signalChan = make(chan os.Signal, 2) // Buffer for 2 signals (graceful + hard)
	signal.Notify(tm.signalChan, os.Interrupt, syscall.SIGTERM)
	tm.signalRegistered = true
	
	// Start signal handler goroutine
	go tm.handleSignals()
}

// handleSignals processes incoming signals for graceful and hard shutdown
func (tm *TaskManager) handleSignals() {
	firstSignal := true
	
	for sig := range tm.signalChan {
		if firstSignal {
			// First signal: initiate graceful shutdown
			firstSignal = false
			go tm.gracefulShutdown(sig)
			
			// Set up a timer for the second signal (hard exit)
			go func() {
				select {
				case <-time.After(tm.gracefulTimeout):
					// Timeout reached, proceed with hard exit anyway
					tm.hardExit("timeout")
				case <-tm.signalChan:
					// Second signal received, immediate hard exit
					tm.hardExit("forced")
				}
			}()
		} else {
			// Additional signals after first: immediate hard exit
			tm.hardExit("multiple signals")
			return
		}
	}
}

// gracefulShutdown initiates graceful shutdown process
func (tm *TaskManager) gracefulShutdown(sig os.Signal) {
	tm.shutdownOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\n🛑 Received %v - initiating graceful shutdown...\n", sig)
		fmt.Fprintf(os.Stderr, "   Press Ctrl+C again to force immediate exit\n\n")
		
		// Call user-defined interrupt handler if provided
		if tm.onInterrupt != nil {
			tm.onInterrupt()
		}
		
		// Cancel all running tasks
		tm.CancelAll()
		
		// Wait for tasks to complete with timeout
		done := make(chan bool, 1)
		go func() {
			tm.wg.Wait()
			done <- true
		}()
		
		select {
		case <-done:
			// All tasks completed gracefully
			fmt.Fprintf(os.Stderr, "✅ All tasks completed gracefully\n")
			tm.displayFinalSummary()
			os.Exit(0)
			
		case <-time.After(tm.gracefulTimeout):
			// Timeout reached
			fmt.Fprintf(os.Stderr, "⏰ Graceful shutdown timeout reached\n")
			tm.displayFinalSummary()
			os.Exit(1)
		}
	})
}

// hardExit performs immediate forced exit
func (tm *TaskManager) hardExit(reason string) {
	fmt.Fprintf(os.Stderr, "\n💥 Force exit (%s) - terminating immediately\n", reason)
	
	// Cancel all tasks immediately (best effort)
	tm.CancelAll()
	
	// Brief summary without waiting
	tm.displayBriefSummary()
	
	os.Exit(130) // Standard exit code for interrupted process
}

// displayFinalSummary shows complete task summary during graceful shutdown
func (tm *TaskManager) displayFinalSummary() {
	tm.mu.RLock()
	tasks := tm.tasks
	tm.mu.RUnlock()
	
	if len(tasks) == 0 {
		return
	}
	
	fmt.Fprintf(os.Stderr, "\n=== Final Task Summary ===\n")
	
	var completed, failed, cancelled int
	for _, task := range tasks {
		task.mu.Lock()
		status := task.status
		displayName := tm.formatTaskName(task.name, task.modelName, task.prompt)
		duration := task.getDuration()
		task.mu.Unlock()
		
		switch status {
		case StatusSuccess:
			fmt.Fprintln(os.Stderr, tm.styles.success.Render(fmt.Sprintf("✓ %s (%s)", displayName, duration)))
			completed++
		case StatusFailed:
			fmt.Fprintln(os.Stderr, tm.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", displayName, duration)))
			failed++
		case StatusCancelled:
			fmt.Fprintln(os.Stderr, tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (%s)", displayName, duration)))
			cancelled++
		case StatusRunning, StatusPending:
			fmt.Fprintln(os.Stderr, tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (interrupted)", displayName)))
			cancelled++
		}
	}
	
	fmt.Fprintf(os.Stderr, "\n📊 Summary: %d completed, %d failed, %d cancelled\n", completed, failed, cancelled)
}

// displayBriefSummary shows minimal summary during hard exit
func (tm *TaskManager) displayBriefSummary() {
	tm.mu.RLock()
	taskCount := len(tm.tasks)
	tm.mu.RUnlock()
	
	if taskCount > 0 {
		fmt.Fprintf(os.Stderr, "📊 Interrupted: %d tasks terminated\n", taskCount)
	}
}

// Infof logs an info message (only shown in verbose mode)
func (t *Task) Infof(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "info",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// Errorf logs an error message
func (t *Task) Errorf(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "error",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// Warnf logs a warning message
func (t *Task) Warnf(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logs = append(t.logs, LogEntry{
		Level:   "warning",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// SetStatus updates the task's display name/status message
func (t *Task) SetStatus(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.name = message
}

// SetProgress updates the task's progress
func (t *Task) SetProgress(value, max int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress = value
	t.maxValue = max
}

// Success marks the task as successfully completed
func (t *Task) Success() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusRunning {
		t.status = StatusSuccess
		t.endTime = time.Now()
		// Calculate duration while we have the lock
		duration := t.getDuration()
		displayName := t.manager.formatTaskName(t.name, t.modelName, t.prompt)

		if t.cancel != nil {
			t.cancel()
		}
		// Print completion in non-interactive mode
		if !t.manager.isInteractive {
			fmt.Fprintln(os.Stderr, t.manager.styles.success.Render(fmt.Sprintf("✓ %s (%s)", displayName, duration)))
		}
	}
}

// Failed marks the task as failed
func (t *Task) Failed() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusRunning {
		t.status = StatusFailed
		t.endTime = time.Now()
		// Calculate duration while we have the lock
		duration := t.getDuration()
		displayName := t.manager.formatTaskName(t.name, t.modelName, t.prompt)

		if t.cancel != nil {
			t.cancel()
		}
		// Print failure in non-interactive mode
		if !t.manager.isInteractive {
			fmt.Fprintln(os.Stderr, t.manager.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", displayName, duration)))
		}
	}
}

// FailedWithError marks the task as failed with an error
func (t *Task) FailedWithError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusRunning {
		t.status = StatusFailed
		t.err = err
		t.endTime = time.Now()
		// Calculate duration while we have the lock
		duration := t.getDuration()
		displayName := t.manager.formatTaskName(t.name, t.modelName, t.prompt)

		t.logs = append(t.logs, LogEntry{
			Level:   "error",
			Message: err.Error(),
			Time:    time.Now(),
		})
		if t.cancel != nil {
			t.cancel()
		}
		// Print failure with error in non-interactive mode
		if !t.manager.isInteractive {
			fmt.Fprintln(os.Stderr, t.manager.styles.failed.Render(fmt.Sprintf("✗ %s (%s): %v", displayName, duration, err)))
		}
	}
}

// Warning marks the task as completed with warnings
func (t *Task) Warning() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusRunning {
		t.status = StatusWarning
		t.endTime = time.Now()
		// Calculate duration while we have the lock
		duration := t.getDuration()
		displayName := t.manager.formatTaskName(t.name, t.modelName, t.prompt)

		if t.cancel != nil {
			t.cancel()
		}
		// Print warning in non-interactive mode
		if !t.manager.isInteractive {
			fmt.Fprintln(os.Stderr, t.manager.styles.warning.Render(fmt.Sprintf("⚠ %s (%s)", displayName, duration)))
		}
	}
}

// Fatal marks the task as failed and exits the program immediately
func (t *Task) Fatal(err error) {
	t.mu.Lock()
	t.status = StatusFailed
	t.err = err
	t.endTime = time.Now()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	t.manager.mu.Lock()
	t.manager.stopRender <- true
	t.manager.mu.Unlock()

	fmt.Fprintf(os.Stderr, "\n✗ Fatal: %s: %v\n", t.name, err)
	os.Exit(1)
}

// Error returns the task's error if any
func (t *Task) Error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// Status returns the current task status
func (t *Task) Status() TaskStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// Waitable interface implementation for Task

// Name returns the task name
func (t *Task) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.name
}

// WaitFor waits for this specific task to complete and returns the result
func (t *Task) WaitFor() *WaitResult {
	// Wait for task completion
	for t.Status() == StatusPending || t.Status() == StatusRunning {
		select {
		case <-t.ctx.Done():
			// Task was cancelled
			break
		case <-time.After(10 * time.Millisecond):
			// Continue polling
		}
	}
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	result := &WaitResult{
		Status:    t.status,
		Duration:  t.Duration(),
		Error:     t.err,
		TaskCount: 1, // Single task
	}
	
	// Count based on status
	switch t.status {
	case StatusSuccess:
		result.SuccessCount = 1
	case StatusFailed:
		result.FailureCount = 1
	case StatusWarning:
		result.WarningCount = 1
	}
	
	return result
}

// GetResult returns the stored result and error
func (t *Task) GetResult() (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result, t.err
}

// GetTypedResult retrieves the result with type assertion
func (t *Task) GetTypedResult(target interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.err != nil {
		return t.err
	}
	
	if t.result == nil {
		return nil
	}
	
	// Use reflection to set the target value
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}
	
	resultValue := reflect.ValueOf(t.result)
	targetElement := targetValue.Elem()
	
	if !resultValue.Type().AssignableTo(targetElement.Type()) {
		return fmt.Errorf("result type %T cannot be assigned to target type %T", t.result, target)
	}
	
	targetElement.Set(resultValue)
	return nil
}

// Duration returns the task duration
func (t *Task) Duration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.status == StatusPending {
		return 0
	}
	
	endTime := t.endTime
	if t.status == StatusRunning {
		endTime = time.Now()
	}
	
	return endTime.Sub(t.startTime)
}

// IsGroup returns false for Task
func (t *Task) IsGroup() bool {
	return false
}

// TaskGroup methods

// Add adds a Waitable item (Task or TaskGroup) to this group
func (g *TaskGroup) Add(item Waitable) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.items = append(g.items, item)
	
	// Set parent reference if it's a Task
	if task, ok := item.(*Task); ok {
		task.parent = g
	}
	
	// Update start time if this is the first item or it started earlier
	if task, ok := item.(*Task); ok {
		if g.startTime.IsZero() || task.startTime.Before(g.startTime) {
			g.startTime = task.startTime
		}
	}
}

// AddWithResult creates a new task with result callback and adds it to the group  
func (g *TaskGroup) AddWithResult(name string, taskFunc func(*Task) (interface{}, error), opts ...TaskOption) *Task {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Create the task using the group's manager
	task := g.manager.StartWithResult(name, taskFunc, opts...)
	
	// Add to the group's items
	g.items = append(g.items, task)
	
	// Set parent reference
	task.parent = g
	
	// Update start time if this is the first item or it started earlier
	if g.startTime.IsZero() || task.startTime.Before(g.startTime) {
		g.startTime = task.startTime
	}
	
	return task
}

// GetResults waits for all tasks in the group and returns typed results
func (g *TaskGroup) GetResults() map[*Task]interface{} {
	g.mu.RLock()
	items := make([]Waitable, len(g.items))
	copy(items, g.items)
	g.mu.RUnlock()
	
	results := make(map[*Task]interface{})
	for _, item := range items {
		if task, ok := item.(*Task); ok {
			// Wait for the task to complete
			task.WaitFor()
			results[task] = task.result
		}
	}
	
	return results
}

// Waitable interface implementation for TaskGroup

// Name returns the group name
func (g *TaskGroup) Name() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.name
}

// Status returns the aggregate status of all items in the group
func (g *TaskGroup) Status() TaskStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.calculateStatus()
}

// calculateStatus determines group status based on child statuses
func (g *TaskGroup) calculateStatus() TaskStatus {
	if len(g.items) == 0 {
		return StatusPending
	}
	
	hasRunning := false
	hasWarning := false
	hasFailed := false
	allCompleted := true
	
	for _, item := range g.items {
		status := item.Status()
		switch status {
		case StatusRunning:
			hasRunning = true
			allCompleted = false
		case StatusPending:
			allCompleted = false
		case StatusFailed:
			hasFailed = true
		case StatusWarning:
			hasWarning = true
		case StatusCancelled:
			hasFailed = true
		}
	}
	
	if hasRunning {
		return StatusRunning
	}
	if !allCompleted {
		return StatusPending
	}
	if hasFailed {
		return StatusFailed
	}
	if hasWarning {
		return StatusWarning
	}
	return StatusSuccess
}

// WaitFor waits for all child items to complete and returns aggregate results
func (g *TaskGroup) WaitFor() *WaitResult {
	result := &WaitResult{}
	
	// Wait for all child items
	g.mu.RLock()
	items := make([]Waitable, len(g.items))
	copy(items, g.items)
	g.mu.RUnlock()
	
	for _, item := range items {
		childResult := item.WaitFor()
		result.TaskCount += childResult.TaskCount
		result.SuccessCount += childResult.SuccessCount
		result.FailureCount += childResult.FailureCount
		result.WarningCount += childResult.WarningCount
		
		// Keep the first error encountered
		if result.Error == nil && childResult.Error != nil {
			result.Error = childResult.Error
		}
	}
	
	result.Status = g.Status()
	result.Duration = g.Duration()
	
	return result
}

// Context returns the group's context for cancellation
func (g *TaskGroup) Context() context.Context {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.ctx
}

// Cancel cancels all items in the group
func (g *TaskGroup) Cancel() {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if g.cancel != nil {
		g.cancel()
	}
	
	// Cancel all child items
	for _, item := range g.items {
		item.Cancel()
	}
}

// Duration returns the total duration from first start to last completion
func (g *TaskGroup) Duration() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if g.startTime.IsZero() {
		return 0
	}
	
	// Find the latest end time among all items
	var latestEnd time.Time
	allCompleted := true
	
	for _, item := range g.items {
		status := item.Status()
		if status == StatusPending || status == StatusRunning {
			allCompleted = false
			break
		}
		
		itemDuration := item.Duration()
		if itemDuration > 0 {
			// Calculate item end time
			if task, ok := item.(*Task); ok {
				task.mu.Lock()
				if !task.endTime.IsZero() && task.endTime.After(latestEnd) {
					latestEnd = task.endTime
				}
				task.mu.Unlock()
			}
		}
	}
	
	if !allCompleted {
		return time.Since(g.startTime)
	}
	
	if latestEnd.IsZero() {
		return time.Since(g.startTime)
	}
	
	return latestEnd.Sub(g.startTime)
}

// IsGroup returns true for TaskGroup
func (g *TaskGroup) IsGroup() bool {
	return true
}

func (tm *TaskManager) render() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopRender:
			return
		case <-ticker.C:
			tm.mu.RLock()
			tasks := make([]*Task, len(tm.tasks))
			copy(tasks, tm.tasks)
			groups := make([]*TaskGroup, len(tm.groups))
			copy(groups, tm.groups)
			verbose := tm.verbose
			isInteractive := tm.isInteractive
			tm.mu.RUnlock()

			if len(tasks) == 0 {
				continue
			}

			// Only use ANSI escape codes if we're in interactive mode
			if isInteractive {
				output := tm.buildOutput(tasks, verbose)
				lines := strings.Count(output, "\n")

				fmt.Fprint(os.Stderr, "\033[H\033[J")
				fmt.Fprint(os.Stderr, output)
				fmt.Fprintf(os.Stderr, "\033[%dA", lines)
			}
		}
	}
}

func (tm *TaskManager) buildOutput(tasks []*Task, verbose bool) string {
	var pendingTasks []*Task
	var running []string
	var completed []string
	totalTasks := len(tasks)
	completedCount := 0
	runningCount := 0

	for _, task := range tasks {
		task.mu.Lock()
		name := task.name
		modelName := task.modelName
		prompt := task.prompt
		status := task.status
		progress := task.progress
		maxValue := task.maxValue
		duration := task.getDuration()
		logs := make([]LogEntry, len(task.logs))
		copy(logs, task.logs)
		task.mu.Unlock()

		// Format display name with model and prompt
		displayName := tm.formatTaskName(name, modelName, prompt)

		// Build log lines for this task
		var taskLogs []string
		for _, log := range logs {
			if log.Level == "info" && !verbose {
				continue // Skip info logs if not in verbose mode
			}

			var logLine string
			switch log.Level {
			case "info":
				logLine = tm.styles.info.Render(fmt.Sprintf("  ℹ %s", log.Message))
			case "error":
				logLine = tm.styles.error.Render(fmt.Sprintf("  ✗ %s", log.Message))
			case "warning":
				logLine = tm.styles.warning.Render(fmt.Sprintf("  ⚠ %s", log.Message))
			}
			taskLogs = append(taskLogs, logLine)
		}

		switch status {
		case StatusPending:
			pendingTasks = append(pendingTasks, task)
		case StatusSuccess:
			completedCount++
			completed = append(completed, tm.styles.success.Render(fmt.Sprintf("✓ %s (%s)", displayName, duration)))
			completed = append(completed, taskLogs...)
		case StatusFailed:
			completedCount++
			completed = append(completed, tm.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", displayName, duration)))
			completed = append(completed, taskLogs...)
		case StatusWarning:
			completedCount++
			completed = append(completed, tm.styles.warning.Render(fmt.Sprintf("⚠ %s (%s)", displayName, duration)))
			completed = append(completed, taskLogs...)
		case StatusCancelled:
			completedCount++
			completed = append(completed, tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (%s)", displayName, duration)))
			completed = append(completed, taskLogs...)
		case StatusRunning:
			runningCount++
			// Use width-aware formatting for running tasks to prevent wrapping
			runningName := tm.formatTaskNameWithWidth(name, modelName, prompt, tm.width)

			// Add retry info if task has been retried
			retryCount := task.retryCount

			if retryCount > 0 {
				runningName = fmt.Sprintf("%s (retry %d/%d)", runningName, retryCount, task.retryConfig.MaxRetries)
			}

			bar := tm.renderProgressBar(runningName, progress, maxValue, duration)
			running = append(running, bar)
			running = append(running, taskLogs...)
		}
	}

	var output strings.Builder

	// Show completed tasks first
	for _, line := range completed {
		output.WriteString(line)
		output.WriteString("\n")
	}

	// Then running tasks
	for _, line := range running {
		output.WriteString(line)
		output.WriteString("\n")
	}

	// Finally pending tasks - group them if more than 3
	pendingCount := len(pendingTasks)
	if pendingCount > 3 {
		// Show a single meta task for all pending tasks
		processedCount := completedCount + runningCount
		metaTask := fmt.Sprintf("⏳ Processing %d of %d tasks (%d pending)",
			processedCount, totalTasks, pendingCount)
		output.WriteString(tm.styles.pending.Render(metaTask))
		output.WriteString("\n")

		// Show first 2 pending tasks as a preview
		for i := 0; i < 2 && i < pendingCount; i++ {
			task := pendingTasks[i]
			task.mu.Lock()
			displayName := tm.formatTaskName(task.name, task.modelName, task.prompt)
			task.mu.Unlock()
			output.WriteString(tm.styles.info.Render(fmt.Sprintf("  • %s", displayName)))
			output.WriteString("\n")
		}

		// Show ellipsis if there are more
		if pendingCount > 2 {
			output.WriteString(tm.styles.info.Render(fmt.Sprintf("  • ... and %d more", pendingCount-2)))
			output.WriteString("\n")
		}
	} else {
		// Show all pending tasks individually when 3 or fewer
		for _, task := range pendingTasks {
			task.mu.Lock()
			displayName := tm.formatTaskName(task.name, task.modelName, task.prompt)
			task.mu.Unlock()
			output.WriteString(tm.styles.pending.Render(fmt.Sprintf("⏳ %s (pending)", displayName)))
			output.WriteString("\n")
		}
	}

	return output.String()
}

func (tm *TaskManager) formatTaskName(name, modelName, prompt string) string {
	return tm.formatTaskNameWithWidth(name, modelName, prompt, 0)
}

func (tm *TaskManager) formatTaskNameWithWidth(name, modelName, prompt string, maxWidth int) string {
	var parts []string

	if modelName != "" {
		parts = append(parts, modelName)
	}

	if name != "" {
		parts = append(parts, name)
	}

	// Calculate available space for prompt if maxWidth is specified
	currentLen := len(strings.Join(parts, " "))
	if currentLen > 0 {
		currentLen += 1 // for space
	}

	if prompt != "" {
		maxPromptLen := 50
		if maxWidth > 0 {
			// Reserve space for quotes, spinner/progress bar (35 chars), and duration (12 chars)
			reservedSpace := 2 + 35 + 12 + 5 // quotes + progress + duration + padding
			availableForPrompt := maxWidth - currentLen - reservedSpace
			if availableForPrompt > 10 && availableForPrompt < maxPromptLen {
				maxPromptLen = availableForPrompt
			}
		}

		truncatedPrompt := prompt
		if len(prompt) > maxPromptLen {
			truncatedPrompt = prompt[:maxPromptLen-3] + "..."
		}
		parts = append(parts, fmt.Sprintf("\"%s\"", truncatedPrompt))
	}

	return strings.Join(parts, " ")
}

func (tm *TaskManager) renderProgressBar(name string, value, maxValue int, duration string) string {
	barWidth := 30

	// If maxValue is 0 or unknown, show infinite spinner
	if maxValue == 0 {
		// Create a simple spinner animation
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinnerIndex := (int(time.Now().UnixNano()/1e8) % len(spinner))
		spinnerChar := spinner[spinnerIndex]

		// Show spinner with dots animation
		dots := strings.Repeat("•", (int(time.Now().UnixNano()/1e9)%4)+1) + strings.Repeat(" ", 3-(int(time.Now().UnixNano()/1e9)%4))
		bar := spinnerChar + " " + dots + strings.Repeat("░", barWidth-6)

		return tm.styles.running.Render(fmt.Sprintf("⟳ %s ", name)) +
			tm.styles.bar.Render(bar) +
			tm.styles.running.Render(fmt.Sprintf(" (%s)", duration))
	}

	// Regular progress bar
	percentage := float64(value) / float64(maxValue)
	if percentage > 1 {
		percentage = 1
	}

	filled := int(percentage * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return tm.styles.running.Render(fmt.Sprintf("⟳ %s ", name)) +
		tm.styles.bar.Render(bar) +
		tm.styles.running.Render(fmt.Sprintf(" %3d%% (%s)", int(percentage*100), duration))
}

func (t *Task) getDuration() string {
	// Note: This should be called with mutex already locked
	var end time.Time
	if t.endTime.IsZero() {
		end = time.Now()
	} else {
		end = t.endTime
	}

	duration := end.Sub(t.startTime)
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", duration.Seconds())
}

// WaitSilent waits for all tasks to complete without displaying results
// Returns the exit code (0 for success, non-zero for failure)
func (tm *TaskManager) WaitSilent() int {
	tm.wg.Wait()
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
func (tm *TaskManager) Wait() int {
	tm.wg.Wait()
	tm.stopRender <- true

	tm.mu.RLock()
	tasks := tm.tasks
	verbose := tm.verbose
	isInteractive := tm.isInteractive
	tm.mu.RUnlock()

	// Only clear screen if in interactive mode
	if isInteractive {
		fmt.Fprint(os.Stderr, "\033[H\033[J")
	}

	var failed, warning, cancelled int
	var totalDuration time.Duration

	for _, task := range tasks {
		task.mu.Lock()
		duration := task.getDuration()
		if !task.endTime.IsZero() {
			totalDuration += task.endTime.Sub(task.startTime)
		}
		logs := make([]LogEntry, len(task.logs))
		copy(logs, task.logs)

		// Format display name with model and prompt for final output
		displayName := tm.formatTaskName(task.name, task.modelName, task.prompt)

		// Print task status
		switch task.status {
		case StatusPending:
			fmt.Fprintln(os.Stderr, tm.styles.pending.Render(fmt.Sprintf("⏳ %s (not started)", displayName)))
		case StatusRunning:
			// Should not happen in Wait, but handle gracefully
			fmt.Fprintln(os.Stderr, tm.styles.running.Render(fmt.Sprintf("⟳ %s (incomplete)", displayName)))
		case StatusSuccess:
			fmt.Fprintln(os.Stderr, tm.styles.success.Render(fmt.Sprintf("✓ %s (%s)", displayName, duration)))
		case StatusFailed:
			fmt.Fprintln(os.Stderr, tm.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", displayName, duration)))
			failed++
		case StatusWarning:
			fmt.Fprintln(os.Stderr, tm.styles.warning.Render(fmt.Sprintf("⚠ %s (%s)", displayName, duration)))
			warning++
		case StatusCancelled:
			fmt.Fprintln(os.Stderr, tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (%s)", displayName, duration)))
			cancelled++
		}

		// Print logs for this task
		for _, log := range logs {
			if log.Level == "info" && !verbose {
				continue
			}

			switch log.Level {
			case "info":
				fmt.Fprintln(os.Stderr, tm.styles.info.Render(fmt.Sprintf("  ℹ %s", log.Message)))
			case "error":
				fmt.Fprintln(os.Stderr, tm.styles.error.Render(fmt.Sprintf("  ✗ %s", log.Message)))
			case "warning":
				fmt.Fprintln(os.Stderr, tm.styles.warning.Render(fmt.Sprintf("  ⚠ %s", log.Message)))
			}
		}

		task.mu.Unlock()
	}

	fmt.Fprintf(os.Stderr, "\n")
	switch {
	case failed > 0:
		fmt.Fprintln(os.Stderr, tm.styles.failed.Render(fmt.Sprintf("Total: %.1fs (with %d failures)", totalDuration.Seconds(), failed)))
		return 1
	case cancelled > 0:
		fmt.Fprintln(os.Stderr, tm.styles.cancelled.Render(fmt.Sprintf("Total: %.1fs (with %d cancelled)", totalDuration.Seconds(), cancelled)))
		return 1
	case warning > 0:
		fmt.Fprintln(os.Stderr, tm.styles.warning.Render(fmt.Sprintf("Total: %.1fs (with %d warnings)", totalDuration.Seconds(), warning)))
		return 0
	default:
		fmt.Fprintln(os.Stderr, tm.styles.success.Render(fmt.Sprintf("Total: %.1fs", totalDuration.Seconds())))
		return 0
	}
}
