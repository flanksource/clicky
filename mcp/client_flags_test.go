package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestToolFlagsAssembleTypedArguments(t *testing.T) {
	raw := json.RawMessage(`{
  "type":"object",
  "required":["displayName"],
  "properties":{
    "displayName":{"type":"string","description":"Name"},
    "count":{"type":"integer","default":2},
    "ratio":{"type":"number"},
    "enabled":{"type":"boolean"},
    "tags":{"type":"array"},
    "metadata":{"type":"object"},
    "mode":{"type":"string","enum":["fast","safe"]},
    "json":{"type":"string"}
  }
}`)
	cmd := &cobra.Command{Use: "tool", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	bindings, err := bindToolFlags(cmd, raw)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Flags().Lookup("display-name") == nil || cmd.Flags().Lookup("p-json") == nil {
		t.Fatal("expected kebab-cased and reserved-prefixed flags")
	}
	args := []string{
		"--display-name", "Ada", "--ratio", "1.5", "--enabled",
		"--tags", `["go","mcp"]`, "--metadata", `{"team":"cli"}`,
		"--mode", "safe", "--p-json", "raw",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	got, err := assembleArguments(cmd, bindings)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"displayName": "Ada", "count": int64(2), "ratio": 1.5, "enabled": true,
		"tags": []any{"go", "mcp"}, "metadata": map[string]any{"team": "cli"},
		"mode": "safe", "json": "raw",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestToolFlagsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		args   []string
		want   string
	}{
		{"enum", `{"properties":{"mode":{"type":"string","enum":["a","b"]}}}`, []string{"--mode", "c"}, "must be one of"},
		{"array JSON", `{"properties":{"items":{"type":"array"}}}`, []string{"--items", "{"}, "must be a JSON array"},
		{"object shape", `{"properties":{"value":{"type":"object"}}}`, []string{"--value", "[]"}, "must be a JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "tool"}
			bindings, err := bindToolFlags(cmd, json.RawMessage(tt.schema))
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatal(err)
			}
			_, err = assembleArguments(cmd, bindings)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestToolFlagsDetectNameCollision(t *testing.T) {
	cmd := &cobra.Command{Use: "tool"}
	_, err := bindToolFlags(cmd, json.RawMessage(`{"properties":{"fooBar":{"type":"string"},"foo-bar":{"type":"string"}}}`))
	if err == nil || !strings.Contains(err.Error(), "both map") {
		t.Fatalf("error = %v", err)
	}
}

func TestToolFlagsRequiredEnforcedByCobra(t *testing.T) {
	cmd := &cobra.Command{Use: "tool", SilenceErrors: true, SilenceUsage: true, RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	if _, err := bindToolFlags(cmd, json.RawMessage(`{"required":["name"],"properties":{"name":{"type":"string"}}}`)); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("error = %v", err)
	}
}

func TestToolFlagsDefaultSatisfiesRequiredProperty(t *testing.T) {
	var got map[string]any
	cmd := &cobra.Command{Use: "tool", SilenceErrors: true, SilenceUsage: true}
	bindings, err := bindToolFlags(cmd, json.RawMessage(`{"required":["count"],"properties":{"count":{"type":"integer","default":2}}}`))
	if err != nil {
		t.Fatal(err)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var err error
		got, err = assembleArguments(cmd, bindings)
		return err
	}
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("defaulted required property failed: %v", err)
	}
	if got["count"] != int64(2) {
		t.Fatalf("count = %#v", got["count"])
	}
}

func TestKebabFlagName(t *testing.T) {
	tests := map[string]string{"displayName": "display-name", "HTTPServerURL": "http-server-url", "snake_case": "snake-case"}
	for input, want := range tests {
		if got := kebabFlagName(input); got != want {
			t.Errorf("kebabFlagName(%q) = %q, want %q", input, got, want)
		}
	}
}
