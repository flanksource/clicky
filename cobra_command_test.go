package clicky

import (
	"os"
	"testing"
	"time"

	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
)

func TestAddCommand_BasicTypes(t *testing.T) {
	type TestOpts struct {
		Name   string `flag:"name" help:"User name" short:"n" required:"true"`
		Age    int    `flag:"age" help:"User age" default:"25"`
		Active bool   `flag:"active" help:"Is active" default:"true"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "user",
		Short: "User command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	// Test with flags
	rootCmd.SetArgs([]string{"user", "--name", "John", "--age", "30", "--active=false"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedOpts.Name != "John" {
		t.Errorf("Expected Name=John, got %s", capturedOpts.Name)
	}
	if capturedOpts.Age != 30 {
		t.Errorf("Expected Age=30, got %d", capturedOpts.Age)
	}
	if capturedOpts.Active {
		t.Errorf("Expected Active=false, got true")
	}
}

func TestAddCommand_ShortFlags(t *testing.T) {
	type TestOpts struct {
		Role  string `flag:"role" help:"User role" short:"r"`
		Limit int    `flag:"limit" help:"Result limit" short:"l" default:"50"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	// Test with short flags
	rootCmd.SetArgs([]string{"list", "-r", "admin", "-l", "100"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedOpts.Role != "admin" {
		t.Errorf("Expected Role=admin, got %s", capturedOpts.Role)
	}
	if capturedOpts.Limit != 100 {
		t.Errorf("Expected Limit=100, got %d", capturedOpts.Limit)
	}
}

func TestAddCommand_Slices(t *testing.T) {
	type TestOpts struct {
		Tags  []string `flag:"tags" help:"Filter tags"`
		Ports []int    `flag:"ports" help:"Port numbers"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Filter command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"filter", "--tags", "prod,staging", "--ports", "8080,9090"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(capturedOpts.Tags) != 2 || capturedOpts.Tags[0] != "prod" || capturedOpts.Tags[1] != "staging" {
		t.Errorf("Expected Tags=[prod,staging], got %v", capturedOpts.Tags)
	}
	if len(capturedOpts.Ports) != 2 || capturedOpts.Ports[0] != 8080 || capturedOpts.Ports[1] != 9090 {
		t.Errorf("Expected Ports=[8080,9090], got %v", capturedOpts.Ports)
	}
}

func TestAddCommand_Duration(t *testing.T) {
	type TestOpts struct {
		MaxAge duration.Duration `flag:"max-age" help:"Maximum age" default:"30d"`
		Delay  duration.Duration `flag:"delay" help:"Delay duration" default:"5m"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"cleanup", "--max-age", "7d", "--delay", "10s"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expectedMaxAge, _ := duration.ParseDuration("7d")
	expectedDelay, _ := duration.ParseDuration("10s")

	if capturedOpts.MaxAge.String() != expectedMaxAge.String() {
		t.Errorf("Expected MaxAge=7d, got %s", capturedOpts.MaxAge)
	}
	if capturedOpts.Delay.String() != expectedDelay.String() {
		t.Errorf("Expected Delay=10s, got %s", capturedOpts.Delay)
	}
}

func TestAddCommand_Time(t *testing.T) {
	type TestOpts struct {
		Since time.Time `flag:"since" help:"Since date" default:"now-7d"`
		Until time.Time `flag:"until" help:"Until date"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"query", "--since", "now-1d", "--until", "now"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Just verify it's set (time parsing is tested by naturaldate library)
	if capturedOpts.Since.IsZero() {
		t.Errorf("Expected Since to be set")
	}
	if capturedOpts.Until.IsZero() {
		t.Errorf("Expected Until to be set")
	}
}

func TestAddCommand_FileLoading(t *testing.T) {
	// Create test file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "test content from file"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	type TestOpts struct {
		Data string `flag:"data" help:"Data content"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"load", "--data", "@" + tmpFile.Name()})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedOpts.Data != content {
		t.Errorf("Expected Data=%s, got %s", content, capturedOpts.Data)
	}
}

func TestAddCommand_SliceFileLoading(t *testing.T) {
	// Create test file with lines
	tmpFile, err := os.CreateTemp("", "test-lines-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	lines := "line1\nline2\n# comment\n\nline3\n"
	if _, err := tmpFile.WriteString(lines); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	type TestOpts struct {
		Items []string `flag:"items" help:"List of items"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"import", "--items", "@" + tmpFile.Name()})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := []string{"line1", "line2", "line3"}
	if len(capturedOpts.Items) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(capturedOpts.Items))
	}

	for i, exp := range expected {
		if capturedOpts.Items[i] != exp {
			t.Errorf("Expected Items[%d]=%s, got %s", i, exp, capturedOpts.Items[i])
		}
	}
}

func TestAddCommand_StdinString(t *testing.T) {
	// Mock stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r

	stdinContent := "data from stdin"
	go func() {
		w.WriteString(stdinContent)
		w.Close()
	}()

	type TestOpts struct {
		Content string `flag:"content" help:"Content" stdin:"true"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "process",
		Short: "Process command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"process"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedOpts.Content != stdinContent {
		t.Errorf("Expected Content=%s, got %s", stdinContent, capturedOpts.Content)
	}
}

func TestAddCommand_StdinSlice(t *testing.T) {
	// Mock stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r

	stdinContent := "item1\nitem2\n# comment\nitem3\n"
	go func() {
		w.WriteString(stdinContent)
		w.Close()
	}()

	type TestOpts struct {
		Items []string `flag:"items" help:"Items list" stdin:"true"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"batch"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := []string{"item1", "item2", "item3"}
	if len(capturedOpts.Items) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(capturedOpts.Items))
	}

	for i, exp := range expected {
		if capturedOpts.Items[i] != exp {
			t.Errorf("Expected Items[%d]=%s, got %s", i, exp, capturedOpts.Items[i])
		}
	}
}

func TestAddCommand_DefaultValues(t *testing.T) {
	type TestOpts struct {
		Name  string `flag:"name" help:"Name" default:"default-name"`
		Count int    `flag:"count" help:"Count" default:"42"`
		Flag  bool   `flag:"flag" help:"Flag" default:"true"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	var capturedOpts TestOpts

	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Test defaults",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		capturedOpts = opts
		return opts, nil
	})

	rootCmd.AddCommand(cmd)

	// Execute without any flags
	rootCmd.SetArgs([]string{"defaults"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedOpts.Name != "default-name" {
		t.Errorf("Expected Name=default-name, got %s", capturedOpts.Name)
	}
	if capturedOpts.Count != 42 {
		t.Errorf("Expected Count=42, got %d", capturedOpts.Count)
	}
	if !capturedOpts.Flag {
		t.Errorf("Expected Flag=true, got false")
	}
}

func TestAddCommand_ErrorHandling(t *testing.T) {
	type TestOpts struct {
		Name string `flag:"name" help:"Name"`
	}

	rootCmd := &cobra.Command{Use: "test"}

	cmd := &cobra.Command{
		Use:   "error",
		Short: "Error command",
	}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		return nil, os.ErrNotExist
	})

	rootCmd.AddCommand(cmd)

	rootCmd.SetArgs([]string{"error", "--name", "test"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAddCommand_PanicOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-struct type")
		}
	}()

	rootCmd := &cobra.Command{Use: "test"}
	cmd := &cobra.Command{Use: "bad"}

	// Should panic because string is not a struct
	AddCommand(cmd, "not a struct", func(opts string) (any, error) {
		return nil, nil
	})

	rootCmd.AddCommand(cmd)
}

func TestAddCommand_PanicOnMultipleStdin(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for multiple stdin fields")
		}
	}()

	type TestOpts struct {
		Field1 string `flag:"field1" stdin:"true"`
		Field2 string `flag:"field2" stdin:"true"`
	}

	rootCmd := &cobra.Command{Use: "test"}
	cmd := &cobra.Command{Use: "bad"}

	AddCommand(cmd, TestOpts{}, func(opts TestOpts) (any, error) {
		return nil, nil
	})

	rootCmd.AddCommand(cmd)
}
