package mcp

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

func TestClientSessionCloseConcurrent(t *testing.T) {
	closeErr := errors.New("close error")
	var calls atomic.Int32
	session := &ClientSession{close: func() error {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return closeErr
	}}

	const goroutines = 32
	results := make(chan error, goroutines)
	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- session.Close()
		}()
	}
	wait.Wait()
	close(results)

	for err := range results {
		if !errors.Is(err, closeErr) {
			t.Fatalf("Close() error = %v, want %v", err, closeErr)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("close calls = %d, want 1", got)
	}
}

func TestDialStdioSessionOutlivesStartupTimeout(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "stdio-server")
	build := exec.Command("go", "build", "-o", binary, "./testdata/stdioserver")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build stdio server: %v\n%s", err, output)
	}

	const startupTimeout = 500 * time.Millisecond
	session, err := Dial(context.Background(), "demo", ServerConfig{
		Type: "stdio", Command: binary, Timeout: startupTimeout.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	time.Sleep(startupTimeout + 100*time.Millisecond)
	callCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := session.Caller.ListTools(callCtx, mcpsdk.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() after startup timeout: %v", err)
	}
	if len(result.Tools) != 3 {
		t.Fatalf("ListTools() returned %d tools, want 3", len(result.Tools))
	}
}
