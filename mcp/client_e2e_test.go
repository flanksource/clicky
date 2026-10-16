package mcp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestClientCommandsStdioAndOfflineHelp(t *testing.T) {
	isolateConfigHome(t)
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	serverDir := filepath.Join(workingDir, "testdata", "stdioserver")

	out, err := executeMCPCommand("add", "demo", "--", "go", "run", serverDir)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, `Added MCP server "demo"`) {
		t.Fatalf("add output: %s", out)
	}
	out, err = executeMCPCommand("list", "demo", "--tools")
	if err != nil || !strings.Contains(out, "echo") || !strings.Contains(out, "add") {
		t.Fatalf("list: %v\n%s", err, out)
	}

	registry := NewServerRegistry("testapp")
	if err := registry.Add("demo", ServerConfig{Type: "stdio", Command: "/server/is/offline"}); err != nil {
		t.Fatal(err)
	}
	out, err = executeMCPCommand("run", "demo", "--help")
	if err != nil || !strings.Contains(out, "Usage:\n  testapp mcp run demo [command]") || !strings.Contains(out, "Available Commands:") {
		t.Fatalf("server help: %v\n%s", err, out)
	}
	if strings.Contains(out, "Tools cached for") || strings.Contains(out, "completion") {
		t.Fatalf("server help did not use the MCP tool command tree:\n%s", out)
	}
	out, err = executeMCPCommand("run", "demo", "echo", "--help")
	if err != nil || !strings.Contains(out, "Usage:\n  testapp mcp run demo echo [flags]") || !strings.Contains(out, "--message") {
		t.Fatalf("offline help: %v\n%s", err, out)
	}
	out, err = executeMCPCommand("run", "demo", "help", "echo")
	if err != nil || !strings.Contains(out, "Usage:\n  testapp mcp run demo echo [flags]") {
		t.Fatalf("Cobra help command: %v\n%s", err, out)
	}
	if err := registry.Add("demo", ServerConfig{Type: "stdio", Command: "go", Args: []string{"run", serverDir}}); err != nil {
		t.Fatal(err)
	}
	out, err = executeMCPCommand("run", "demo", "echo", "--message", "hello")
	if err != nil || !strings.Contains(out, "hello") {
		t.Fatalf("run: %v\n%s", err, out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, runErr := executeMCPCommandContext(ctx, "run", "demo", "wait", "--seconds", "10")
		done <- runErr
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case runErr := <-done:
		if runErr == nil {
			t.Fatal("cancelled run unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cobra context cancellation did not stop the MCP invocation")
	}
	out, err = executeMCPCommand("remove", "demo")
	if err != nil || !strings.Contains(out, "Removed") {
		t.Fatalf("remove: %v\n%s", err, out)
	}
}

func executeMCPCommand(args ...string) (string, error) {
	return executeMCPCommandContext(context.Background(), args...)
}

func executeMCPCommandContext(ctx context.Context, args ...string) (string, error) {
	return executeMCPCommandWithClientOptions(ctx, ClientOptions{}, args...)
}

func executeMCPCommandWithClientOptions(ctx context.Context, options ClientOptions, args ...string) (string, error) {
	root := &cobra.Command{Use: "testapp", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewCommandWithClientOptions(nil, options))
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(append([]string{"mcp"}, args...))
	err := root.ExecuteContext(ctx)
	return output.String(), err
}
