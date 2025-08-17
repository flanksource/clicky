package task

import (
	"flag"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/spf13/pflag"
)

// Option configures task creation
type Option func(*Task)

type Health string

const (
	HealthOK      Health = "ok"
	HealthWarning Health = "warning"
	HealthError   Health = "error"
	HealthPending Health = "pending"
)

type HealthMixin interface {
	Health() Health
}

func (h Health) Style() string {
	switch h {
	case HealthOK:
		return "text-green-500"
	case HealthWarning:
		return "text-yellow-500"
	case HealthError:
		return "text-red-500"
	}
	return "text-gray-500"
}

// WithTimeout sets a timeout for the task
func WithTimeout(d time.Duration) Option {
	return func(t *Task) {
		t.timeout = d
	}
}

// WithTaskTimeout sets an individual task timeout applied at execution time
func WithTaskTimeout(d time.Duration) Option {
	return func(t *Task) {
		t.taskTimeout = d
	}
}

// WithDependencies sets tasks that must complete before this task can start
func WithDependencies(deps ...*Task) Option {
	return func(t *Task) {
		if t != nil {
			t.dependencies = append(t.dependencies, deps...)
		}
	}
}

// WithFunc sets the function to run for the task
func WithFunc(fn func(flanksourceContext.Context, *Task) error) Option {
	return func(t *Task) {
		t.runFunc = fn
	}
}

// WithModel sets the model name for the task
func WithModel(modelName string) Option {
	return func(t *Task) {
		t.modelName = modelName
	}
}

// WithPrompt sets the prompt for the task
func WithPrompt(prompt string) Option {
	return func(t *Task) {
		t.prompt = prompt
	}
}

// WithRetryConfig sets custom retry configuration for the task
func WithRetryConfig(config RetryConfig) Option {
	return func(t *Task) {
		t.retryConfig = config
	}
}

// WithPriority sets the priority for task scheduling (lower = higher priority)
func WithPriority(priority int) Option {
	return func(t *Task) {
		t.priority = priority
	}
}

// ManagerOptions contains configuration options for TaskManager
type ManagerOptions struct {
	NoColor         bool          // Disable colored output
	NoProgress      bool          // Disable progress display
	MaxConcurrent   int           // Maximum concurrent tasks (0 = unlimited)
	GracefulTimeout time.Duration // Timeout for graceful shutdown

	// Retry configuration
	MaxRetries int           // Maximum retry attempts
	RetryDelay time.Duration // Base delay between retries
}

// DefaultManagerOptions returns sensible defaults
func DefaultManagerOptions() *ManagerOptions {
	return &ManagerOptions{
		NoColor:         false,
		NoProgress:      false,
		MaxConcurrent:   1,
		GracefulTimeout: 10 * time.Second,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
	}
}

// Apply configures a TaskManager with these options
func (opts *ManagerOptions) Apply(tm *Manager) {
	tm.SetNoColor(opts.NoColor)
	tm.SetNoProgress(opts.NoProgress)
	tm.SetMaxConcurrent(opts.MaxConcurrent)
	tm.SetGracefulTimeout(opts.GracefulTimeout)

	if opts.MaxRetries > 0 {
		config := tm.retryConfig
		config.MaxRetries = opts.MaxRetries
		config.BaseDelay = opts.RetryDelay
		tm.SetRetryConfig(config)
	}
}

// BindManagerFlags adds TaskManager flags to standard flag set
func BindManagerFlags(flags *flag.FlagSet, options *ManagerOptions) {
	flags.BoolVar(&options.NoColor, "no-color", options.NoColor,
		"Disable colored output")
	flags.BoolVar(&options.NoProgress, "no-progress", options.NoProgress,
		"Disable progress display")
	flags.IntVar(&options.MaxConcurrent, "max-concurrent", options.MaxConcurrent,
		"Maximum concurrent tasks (0 = unlimited)")
	flags.DurationVar(&options.GracefulTimeout, "graceful-timeout", options.GracefulTimeout,
		"Timeout for graceful shutdown on interrupt")
	flags.IntVar(&options.MaxRetries, "max-retries", options.MaxRetries,
		"Maximum retry attempts for failed tasks")
	flags.DurationVar(&options.RetryDelay, "retry-delay", options.RetryDelay,
		"Base delay between retry attempts")
}

// BindManagerPFlags adds TaskManager flags to pflag set (for Cobra)
func BindManagerPFlags(flags *pflag.FlagSet, options *ManagerOptions) {
	flags.BoolVar(&options.NoColor, "no-color", options.NoColor,
		"Disable colored output")
	flags.BoolVar(&options.NoProgress, "no-progress", options.NoProgress,
		"Disable progress display")
	flags.IntVar(&options.MaxConcurrent, "max-concurrent", options.MaxConcurrent,
		"Maximum concurrent tasks (0 = unlimited)")
	flags.DurationVar(&options.GracefulTimeout, "graceful-timeout", options.GracefulTimeout,
		"Timeout for graceful shutdown on interrupt")
	flags.IntVar(&options.MaxRetries, "max-retries", options.MaxRetries,
		"Maximum retry attempts for failed tasks")
	flags.DurationVar(&options.RetryDelay, "retry-delay", options.RetryDelay,
		"Base delay between retry attempts")
}

// NewManagerWithOptions creates a TaskManager with the given options
func NewManagerWithOptions(options *ManagerOptions) *Manager {
	tm := NewManagerWithConcurrency(options.MaxConcurrent)
	options.Apply(tm)
	return tm
}
