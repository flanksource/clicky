// Tests the operation-listener seam for commands clicky generates from an opts
// struct, alongside the entity operations the seam already observed.
package entity

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

type probeOpts struct {
	Name string `flag:"name" help:"probe name"`
}

// collectEvents subscribes for the duration of the test and returns the slice
// the listener appends to. Subscriptions are process-wide, so every test must
// unsubscribe before returning or it leaks into the next one.
func collectEvents(t *testing.T) *[]OperationEvent {
	t.Helper()
	events := &[]OperationEvent{}
	unsubscribe := RegisterOperationListener(func(_ context.Context, event OperationEvent) {
		*events = append(*events, event)
	})
	t.Cleanup(unsubscribe)
	return events
}

// collectWithSurface also records the surface each event was delivered under.
func collectWithSurface(t *testing.T) (*[]OperationEvent, *[]string) {
	t.Helper()
	events := &[]OperationEvent{}
	surfaces := &[]string{}
	unsubscribe := RegisterOperationListener(func(ctx context.Context, event OperationEvent) {
		*events = append(*events, event)
		*surfaces = append(*surfaces, OperationSurfaceFromContext(ctx))
	})
	t.Cleanup(unsubscribe)
	return events, surfaces
}

// newProbeCommand builds "app widget build", the shape a host application gets
// from AddNamedCommandWithContext under a plain (non-entity) parent group.
func newProbeCommand(t *testing.T, fn func(ctx context.Context, opts probeOpts) (string, error)) (*cobra.Command, *cobra.Command) {
	t.Helper()
	root := &cobra.Command{Use: "app", SilenceUsage: true, SilenceErrors: true}
	group := &cobra.Command{Use: "widget"}
	root.AddCommand(group)
	cmd := AddNamedCommandWithContext("build", group, probeOpts{}, fn)
	return root, cmd
}

func TestOperationListenerObservesGeneratedCommandOnCLI(t *testing.T) {
	events, surfaces := collectWithSurface(t)
	root, _ := newProbeCommand(t, func(_ context.Context, opts probeOpts) (string, error) {
		return "built " + opts.Name, nil
	})

	root.SetArgs([]string{"widget", "build", "--name", "gadget"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	event := (*events)[0]
	if event.Entity != "widget" || event.Verb != "build" {
		t.Errorf("identity = %q/%q, want widget/build", event.Entity, event.Verb)
	}
	if event.Result != "built gadget" {
		t.Errorf("Result = %v, want %q", event.Result, "built gadget")
	}
	if event.Error != nil {
		t.Errorf("Error = %v, want nil", event.Error)
	}
	if event.Parameters["name"] != "gadget" {
		t.Errorf("Parameters = %v, want name=gadget", event.Parameters)
	}
	if (*surfaces)[0] != "cli" {
		t.Errorf("surface = %q, want cli", (*surfaces)[0])
	}
}

func TestOperationListenerObservesGeneratedCommandOnTransport(t *testing.T) {
	events, surfaces := collectWithSurface(t)
	_, cmd := newProbeCommand(t, func(_ context.Context, opts probeOpts) (string, error) {
		return "built " + opts.Name, nil
	})

	run := GetContextDataFunc(cmd)
	if run == nil {
		t.Fatal("generated command registered no ContextDataFunc")
	}
	ctx := ContextWithOperationSurface(context.Background(), "http")
	result, err := run(ctx, map[string]string{"name": "gadget"}, nil)
	if err != nil {
		t.Fatalf("ContextDataFunc: %v", err)
	}
	if result != "built gadget" {
		t.Errorf("result = %v, want %q", result, "built gadget")
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	if (*events)[0].Entity != "widget" || (*events)[0].Verb != "build" {
		t.Errorf("identity = %q/%q, want widget/build", (*events)[0].Entity, (*events)[0].Verb)
	}
	if (*surfaces)[0] != "http" {
		t.Errorf("surface = %q, want http", (*surfaces)[0])
	}
}

func TestOperationListenerReportsGeneratedCommandFailure(t *testing.T) {
	events := collectEvents(t)
	boom := errors.New("boom")
	root, _ := newProbeCommand(t, func(_ context.Context, _ probeOpts) (string, error) {
		return "", boom
	})

	root.SetArgs([]string{"widget", "build"})
	if err := root.Execute(); !errors.Is(err, boom) {
		t.Fatalf("Execute error = %v, want %v", err, boom)
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	if !errors.Is((*events)[0].Error, boom) {
		t.Errorf("event Error = %v, want %v", (*events)[0].Error, boom)
	}
}

func TestOperationListenerObservesLegacyGeneratedCommand(t *testing.T) {
	events := collectEvents(t)
	root := &cobra.Command{Use: "app", SilenceUsage: true, SilenceErrors: true}
	group := &cobra.Command{Use: "widget"}
	root.AddCommand(group)
	cmd := AddNamedCommand("build", group, probeOpts{}, func(opts probeOpts) (string, error) {
		return "built " + opts.Name, nil
	})

	run := GetDataFunc(cmd)
	if run == nil {
		t.Fatal("legacy generated command registered no DataFunc")
	}
	if _, err := run(map[string]string{"name": "gadget"}, nil); err != nil {
		t.Fatalf("DataFunc: %v", err)
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	if (*events)[0].Entity != "widget" || (*events)[0].Verb != "build" {
		t.Errorf("identity = %q/%q, want widget/build", (*events)[0].Entity, (*events)[0].Verb)
	}
}

func TestOperationListenerPrefersEntityAnnotation(t *testing.T) {
	events := collectEvents(t)
	root := &cobra.Command{Use: "app", SilenceUsage: true, SilenceErrors: true}
	group := &cobra.Command{Use: "widget"}
	root.AddCommand(group)
	cmd := AddNamedCommandWithContext("build", group, probeOpts{}, func(_ context.Context, _ probeOpts) (string, error) {
		return "ok", nil
	})
	setCommandAnnotation(cmd, annotationClickyEntityName, "sprocket")
	setCommandAnnotation(cmd, annotationClickyOperationAction, "assemble")

	if _, err := GetContextDataFunc(cmd)(context.Background(), nil, nil); err != nil {
		t.Fatalf("ContextDataFunc: %v", err)
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	if (*events)[0].Entity != "sprocket" || (*events)[0].Verb != "assemble" {
		t.Errorf("identity = %q/%q, want sprocket/assemble", (*events)[0].Entity, (*events)[0].Verb)
	}
}

// A top-level command has no group to name it, so it names itself rather than
// borrowing the application's own name.
func TestOperationListenerNamesTopLevelGeneratedCommand(t *testing.T) {
	events := collectEvents(t)
	root := &cobra.Command{Use: "app", SilenceUsage: true, SilenceErrors: true}
	cmd := AddNamedCommandWithContext("install", root, probeOpts{}, func(_ context.Context, _ probeOpts) (string, error) {
		return "ok", nil
	})

	if _, err := GetContextDataFunc(cmd)(context.Background(), nil, nil); err != nil {
		t.Fatalf("ContextDataFunc: %v", err)
	}

	if len(*events) != 1 {
		t.Fatalf("got %d events, want exactly 1", len(*events))
	}
	if (*events)[0].Entity != "install" || (*events)[0].Verb != "install" {
		t.Errorf("identity = %q/%q, want install/install", (*events)[0].Entity, (*events)[0].Verb)
	}
}

// With no subscription the generated command must behave exactly as before,
// including returning the operation's own result.
func TestGeneratedCommandUnobservedWithoutListeners(t *testing.T) {
	_, cmd := newProbeCommand(t, func(_ context.Context, opts probeOpts) (string, error) {
		return "built " + opts.Name, nil
	})
	result, err := GetContextDataFunc(cmd)(context.Background(), map[string]string{"name": "gadget"}, nil)
	if err != nil {
		t.Fatalf("ContextDataFunc: %v", err)
	}
	if result != "built gadget" {
		t.Errorf("result = %v, want %q", result, "built gadget")
	}
}
