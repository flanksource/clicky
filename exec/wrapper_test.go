package exec

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test 1: Basic wrapper execution
func TestWrapperBasicExecution(t *testing.T) {
	echo := Process{
		Cmd: "echo",
	}.AsWrapper()

	result, err := echo("hello", "world")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", result.ExitCode)
	}

	output := strings.TrimSpace(result.Stdout)
	expected := "hello world"
	if output != expected {
		t.Errorf("Expected output %q, got: %q", expected, output)
	}

	if result.Command == "" {
		t.Error("Expected Command field to be populated")
	}
}

// Test 2: Wrapper with functional options (timeout)
func TestWrapperWithTimeout(t *testing.T) {
	sleep := Process{
		Cmd: "sleep",
	}.AsWrapper()

	start := time.Now()
	result, err := sleep("10", WithTimeout(1*time.Second))
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	if duration >= 10*time.Second {
		t.Errorf("Expected timeout after ~1s, took %v", duration)
	}

	if result == nil {
		t.Fatal("Expected result to be populated even with timeout")
	}
}

// Test 3: Concurrent wrapper calls
func TestWrapperConcurrency(t *testing.T) {
	date := Process{
		Cmd:  "date",
		Args: []string{"+%s.%N"},
	}.AsWrapper()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*ExecResult, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, err := date()
			results[idx] = result
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	outputs := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		if errors[i] != nil {
			t.Errorf("Goroutine %d failed: %v", i, errors[i])
			continue
		}

		if results[i] == nil {
			t.Errorf("Goroutine %d got nil result", i)
			continue
		}

		if results[i].ExitCode != 0 {
			t.Errorf("Goroutine %d got non-zero exit code: %d", i, results[i].ExitCode)
		}

		output := strings.TrimSpace(results[i].Stdout)
		outputs[output] = true
	}

	if len(outputs) < 2 {
		t.Errorf("Expected multiple different timestamps, got only %d unique values", len(outputs))
	}
}

// Test 4: Non-zero exit handling
func TestWrapperErrorHandling(t *testing.T) {
	sh := Process{
		Cmd:  "sh",
		Args: []string{"-c"},
	}.AsWrapper()

	t.Run("without WithErrOnNonZero", func(t *testing.T) {
		result, err := sh("exit 42")
		if err != nil {
			t.Errorf("Expected no error without WithErrOnNonZero, got: %v", err)
		}

		if result == nil {
			t.Fatal("Expected result to be populated")
		}

		if result.ExitCode != 42 {
			t.Errorf("Expected exit code 42, got: %d", result.ExitCode)
		}
	})

	t.Run("with WithErrOnNonZero", func(t *testing.T) {
		result, err := sh("exit 42", WithErrOnNonZero())
		if err == nil {
			t.Fatal("Expected error with WithErrOnNonZero, got nil")
		}

		if !strings.Contains(err.Error(), "exit code 42") {
			t.Errorf("Expected error to mention exit code, got: %v", err)
		}

		if result == nil {
			t.Fatal("Expected result to be populated even with error")
		}

		if result.ExitCode != 42 {
			t.Errorf("Expected exit code 42 in result, got: %d", result.ExitCode)
		}
	})
}

// Test 5: Template configuration preservation
func TestWrapperTemplatePreservation(t *testing.T) {
	template := Process{
		Cmd:     "printenv",
		Env:     map[string]string{"TEST_VAR": "original"},
		Timeout: 5 * time.Second,
	}

	printenv := template.AsWrapper()

	result1, err := printenv("TEST_VAR")
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	if !strings.Contains(result1.Stdout, "original") {
		t.Errorf("Expected TEST_VAR=original in output, got: %s", result1.Stdout)
	}

	result2, err := printenv("TEST_VAR", WithEnv("TEST_VAR", "modified"))
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if !strings.Contains(result2.Stdout, "modified") {
		t.Errorf("Expected TEST_VAR=modified in output, got: %s", result2.Stdout)
	}

	result3, err := printenv("TEST_VAR")
	if err != nil {
		t.Fatalf("Third call failed: %v", err)
	}

	if !strings.Contains(result3.Stdout, "original") {
		t.Errorf("Expected template to be unchanged, TEST_VAR should be 'original', got: %s", result3.Stdout)
	}
}

// Test 6: WithContext option
func TestWrapperWithContext(t *testing.T) {
	wrapper := Process{
		Cmd: "sleep",
	}.AsWrapper()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := wrapper("10", WithContext(ctx))
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected context timeout error, got nil")
	}

	if duration >= 10*time.Second {
		t.Errorf("Expected context timeout after ~1s, took %v", duration)
	}
}

// Test 7: WithDir option
func TestWrapperWithDir(t *testing.T) {
	wrapper := Process{
		Cmd: "pwd",
	}.AsWrapper()

	result, err := wrapper(WithDir("/tmp"))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	output := strings.TrimSpace(result.Stdout)
	if output != "/tmp" && output != "/private/tmp" {
		t.Errorf("Expected working directory to be /tmp, got: %s", output)
	}
}

// Test 8: Multiple arguments with base command
func TestWrapperWithBaseArgs(t *testing.T) {
	wrapper := Process{
		Cmd:  "echo",
		Args: []string{"base"},
	}.AsWrapper()

	result, err := wrapper("additional", "args")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	output := strings.TrimSpace(result.Stdout)
	expected := "base additional args"
	if output != expected {
		t.Errorf("Expected output %q, got: %q", expected, output)
	}
}

// Test 9: Invalid argument type
func TestWrapperInvalidArgumentType(t *testing.T) {
	wrapper := Process{
		Cmd: "echo",
	}.AsWrapper()

	_, err := wrapper(123)
	if err == nil {
		t.Fatal("Expected error for invalid argument type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid argument type") {
		t.Errorf("Expected 'invalid argument type' error, got: %v", err)
	}
}

// Test 10: ExecResult metadata
func TestWrapperResultMetadata(t *testing.T) {
	wrapper := Process{
		Cmd: "echo",
	}.AsWrapper()

	result, err := wrapper("test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Duration == 0 {
		t.Error("Expected Duration to be set")
	}

	if result.PID == 0 {
		t.Error("Expected PID to be set")
	}

	if result.Command == "" {
		t.Error("Expected Command to be set")
	}

	if !strings.Contains(result.Command, "echo") {
		t.Errorf("Expected Command to contain 'echo', got: %s", result.Command)
	}
}
