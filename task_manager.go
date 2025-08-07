package clicky

import (
	"context"
	"fmt"
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
	// StatusRunning indicates the task is currently running
	StatusRunning TaskStatus = iota
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

// Task represents a single task being tracked by the TaskManager
type Task struct {
	name       string
	status     TaskStatus
	progress   int
	maxValue   int
	startTime  time.Time
	endTime    time.Time
	manager    *TaskManager
	logs       []LogEntry
	cancel     context.CancelFunc
	ctx        context.Context
	timeout    time.Duration
	mu         sync.Mutex
}

// TaskManager manages and displays multiple tasks with progress bars
type TaskManager struct {
	tasks      []*Task
	mu         sync.RWMutex
	wg         sync.WaitGroup
	stopRender chan bool
	width      int
	verbose    bool
	styles     struct {
		success lipgloss.Style
		failed  lipgloss.Style
		warning lipgloss.Style
		running lipgloss.Style
		bar     lipgloss.Style
		info    lipgloss.Style
		error   lipgloss.Style
		cancelled lipgloss.Style
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

// NewTaskManager creates a new TaskManager instance
func NewTaskManager() *TaskManager {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	if width == 0 {
		width = 80
	}

	tm := &TaskManager{
		tasks:      make([]*Task, 0),
		stopRender: make(chan bool, 1),
		width:      width,
		verbose:    os.Getenv("VERBOSE") != "" || os.Getenv("DEBUG") != "",
	}

	tm.styles.success = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	tm.styles.failed = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.warning = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	tm.styles.running = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	tm.styles.bar = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	tm.styles.info = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	tm.styles.error = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	tm.styles.cancelled = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	go tm.render()
	return tm
}

// SetVerbose enables or disables verbose logging
func (tm *TaskManager) SetVerbose(verbose bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.verbose = verbose
}

// Start creates and starts tracking a new task with optional timeout
func (tm *TaskManager) Start(name string, opts ...TaskOption) *Task {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	
	task := &Task{
		name:      name,
		status:    StatusRunning,
		progress:  0,
		maxValue:  100,
		startTime: time.Now(),
		manager:   tm,
		logs:      make([]LogEntry, 0),
		cancel:    cancel,
		ctx:       ctx,
	}

	for _, opt := range opts {
		opt(task)
	}

	// Set up timeout if specified
	if task.timeout > 0 {
		go func() {
			timer := time.NewTimer(task.timeout)
			defer timer.Stop()
			
			select {
			case <-timer.C:
				task.mu.Lock()
				if task.status == StatusRunning {
					task.status = StatusFailed
					task.endTime = time.Now()
					task.logs = append(task.logs, LogEntry{
						Level:   "error",
						Message: fmt.Sprintf("Task timed out after %v", task.timeout),
						Time:    time.Now(),
					})
					task.mu.Unlock()
					task.manager.wg.Done()
				} else {
					task.mu.Unlock()
				}
			case <-ctx.Done():
				return
			}
		}()
	}

	tm.tasks = append(tm.tasks, task)
	tm.wg.Add(1)
	return task
}

// Context returns the task's context for cancellation
func (t *Task) Context() context.Context {
	return t.ctx
}

// Cancel cancels the task
func (t *Task) Cancel() {
	t.mu.Lock()
	if t.status == StatusRunning {
		t.status = StatusCancelled
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		t.manager.wg.Done()
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
	if t.status == StatusRunning {
		t.status = StatusSuccess
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		t.manager.wg.Done()
	} else {
		t.mu.Unlock()
	}
}

// Failed marks the task as failed
func (t *Task) Failed() {
	t.mu.Lock()
	if t.status == StatusRunning {
		t.status = StatusFailed
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		t.manager.wg.Done()
	} else {
		t.mu.Unlock()
	}
}

// Warning marks the task as completed with warnings
func (t *Task) Warning() {
	t.mu.Lock()
	if t.status == StatusRunning {
		t.status = StatusWarning
		t.endTime = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		t.manager.wg.Done()
	} else {
		t.mu.Unlock()
	}
}

// Fatal marks the task as failed and exits the program immediately
func (t *Task) Fatal(err error) {
	t.mu.Lock()
	t.status = StatusFailed
	t.endTime = time.Now()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()
	t.manager.wg.Done()

	t.manager.mu.Lock()
	t.manager.stopRender <- true
	t.manager.mu.Unlock()

	fmt.Fprintf(os.Stderr, "\n✗ Fatal: %s: %v\n", t.name, err)
	os.Exit(1)
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

			fmt.Print("\033[H\033[J")
			fmt.Print(output)
			fmt.Printf("\033[%dA", lines)
		}
	}
}

func (tm *TaskManager) buildOutput(tasks []*Task, verbose bool) string {
	var completed []string
	var running []string

	for _, task := range tasks {
		task.mu.Lock()
		name := task.name
		status := task.status
		progress := task.progress
		maxValue := task.maxValue
		duration := task.getDuration()
		logs := make([]LogEntry, len(task.logs))
		copy(logs, task.logs)
		task.mu.Unlock()

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
		case StatusSuccess:
			completed = append(completed, tm.styles.success.Render(fmt.Sprintf("✓ %s (%s)", name, duration)))
			completed = append(completed, taskLogs...)
		case StatusFailed:
			completed = append(completed, tm.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", name, duration)))
			completed = append(completed, taskLogs...)
		case StatusWarning:
			completed = append(completed, tm.styles.warning.Render(fmt.Sprintf("⚠ %s (%s)", name, duration)))
			completed = append(completed, taskLogs...)
		case StatusCancelled:
			completed = append(completed, tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (%s)", name, duration)))
			completed = append(completed, taskLogs...)
		case StatusRunning:
			bar := tm.renderProgressBar(name, progress, maxValue)
			running = append(running, bar)
			running = append(running, taskLogs...)
		}
	}

	var output strings.Builder
	for _, line := range completed {
		output.WriteString(line)
		output.WriteString("\n")
	}
	for _, line := range running {
		output.WriteString(line)
		output.WriteString("\n")
	}

	return output.String()
}

func (tm *TaskManager) renderProgressBar(name string, value, maxValue int) string {
	if maxValue == 0 {
		maxValue = 100
	}

	percentage := float64(value) / float64(maxValue)
	if percentage > 1 {
		percentage = 1
	}

	barWidth := 30
	filled := int(percentage * float64(barWidth))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return tm.styles.running.Render(fmt.Sprintf("⟳ %-20s ", name)) +
		tm.styles.bar.Render(bar) +
		tm.styles.running.Render(fmt.Sprintf(" %3d%%", int(percentage*100)))
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

	fmt.Print("\033[H\033[J")

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

		// Print task status
		switch task.status {
		case StatusRunning:
			// Should not happen in Wait, but handle gracefully
			fmt.Println(tm.styles.running.Render(fmt.Sprintf("⟳ %s (incomplete)", task.name)))
		case StatusSuccess:
			fmt.Println(tm.styles.success.Render(fmt.Sprintf("✓ %s (%s)", task.name, duration)))
		case StatusFailed:
			fmt.Println(tm.styles.failed.Render(fmt.Sprintf("✗ %s (%s)", task.name, duration)))
			failed++
		case StatusWarning:
			fmt.Println(tm.styles.warning.Render(fmt.Sprintf("⚠ %s (%s)", task.name, duration)))
			warning++
		case StatusCancelled:
			fmt.Println(tm.styles.cancelled.Render(fmt.Sprintf("⊘ %s (%s)", task.name, duration)))
			cancelled++
		}

		// Print logs for this task
		for _, log := range logs {
			if log.Level == "info" && !verbose {
				continue
			}
			
			switch log.Level {
			case "info":
				fmt.Println(tm.styles.info.Render(fmt.Sprintf("  ℹ %s", log.Message)))
			case "error":
				fmt.Println(tm.styles.error.Render(fmt.Sprintf("  ✗ %s", log.Message)))
			case "warning":
				fmt.Println(tm.styles.warning.Render(fmt.Sprintf("  ⚠ %s", log.Message)))
			}
		}
		
		task.mu.Unlock()
	}

	fmt.Printf("\n")
	switch {
	case failed > 0:
		fmt.Println(tm.styles.failed.Render(fmt.Sprintf("Total: %.1fs (with %d failures)", totalDuration.Seconds(), failed)))
		return 1
	case cancelled > 0:
		fmt.Println(tm.styles.cancelled.Render(fmt.Sprintf("Total: %.1fs (with %d cancelled)", totalDuration.Seconds(), cancelled)))
		return 1
	case warning > 0:
		fmt.Println(tm.styles.warning.Render(fmt.Sprintf("Total: %.1fs (with %d warnings)", totalDuration.Seconds(), warning)))
		return 0
	default:
		fmt.Println(tm.styles.success.Render(fmt.Sprintf("Total: %.1fs", totalDuration.Seconds())))
		return 0
	}
}