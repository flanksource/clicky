package clicky

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
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
}

// TaskManager manages and displays multiple tasks with progress bars
type TaskManager struct {
	tasks         []*Task
	mu            sync.RWMutex
	wg            sync.WaitGroup
	stopRender    chan bool
	width         int
	verbose       bool
	maxConcurrent int
	semaphore     chan struct{}
	retryConfig   RetryConfig
	isInteractive bool
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
	
	tm := &TaskManager{
		tasks:         make([]*Task, 0),
		stopRender:    make(chan bool, 1),
		width:         width,
		verbose:       os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != "",
		maxConcurrent: maxConcurrent,
		retryConfig:   DefaultRetryConfig(),
		isInteractive: isInteractive,
	}

	if maxConcurrent > 0 {
		tm.semaphore = make(chan struct{}, maxConcurrent)
	}

	tm.styles.success = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	tm.styles.failed = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.warning = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	tm.styles.running = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	tm.styles.bar = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	tm.styles.info = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	tm.styles.error = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.cancelled = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	tm.styles.pending = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

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

// CancelAll cancels all running tasks
func (tm *TaskManager) CancelAll() {
	tm.mu.RLock()
	tasks := make([]*Task, len(tm.tasks))
	copy(tasks, tm.tasks)
	tm.mu.RUnlock()

	for _, task := range tasks {
		task.Cancel()
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
		if t.cancel != nil {
			t.cancel()
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
		if t.cancel != nil {
			t.cancel()
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
		t.logs = append(t.logs, LogEntry{
			Level:   "error",
			Message: err.Error(),
			Time:    time.Now(),
		})
		if t.cancel != nil {
			t.cancel()
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
		if t.cancel != nil {
			t.cancel()
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
			verbose := tm.verbose
			tm.mu.RUnlock()

			if len(tasks) == 0 {
				continue
			}

			output := tm.buildOutput(tasks, verbose)
			lines := strings.Count(output, "\n")

			fmt.Fprint(os.Stderr, "\033[H\033[J")
			fmt.Fprint(os.Stderr, output)
			fmt.Fprintf(os.Stderr, "\033[%dA", lines)
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
			task.mu.Lock()
			retryCount := task.retryCount
			task.mu.Unlock()

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

// Wait waits for all tasks to complete and returns the appropriate exit code
func (tm *TaskManager) Wait() int {
	tm.wg.Wait()
	tm.stopRender <- true

	tm.mu.RLock()
	tasks := tm.tasks
	verbose := tm.verbose
	tm.mu.RUnlock()

	fmt.Fprint(os.Stderr, "\033[H\033[J")

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
