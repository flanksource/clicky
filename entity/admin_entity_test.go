package entity

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

type adminCtxKey struct{}

// adminEntityInfo returns the registered IsAdmin sub-entity with the given name.
func adminEntityInfo(t *testing.T, name string) EntityInfo {
	t.Helper()
	for _, info := range GetEntities() {
		if info.IsAdmin && info.Name == name {
			return info
		}
	}
	t.Fatalf("no admin entity registered under %q", name)
	return EntityInfo{}
}

func operationByVerb(t *testing.T, info EntityInfo, verb string) EntityOperation {
	t.Helper()
	for _, op := range info.Operations {
		if op.Verb == verb {
			return op
		}
	}
	t.Fatalf("admin entity %q has no %q operation (has %d operations)", info.Name, verb, len(info.Operations))
	return EntityOperation{}
}

// TestAdminEntityRegistersContextOperations covers the context-aware variants on
// an Admin sub-entity. Request-scoped callers (an HTTP server resolving a
// per-request DB handle off the context) can only declare GetWithContext /
// ListWithContext; when RegisterEntity ignored those the admin group registered
// zero operations, so both the CLI subcommand and its generated HTTP route
// silently disappeared.
func TestAdminEntityRegistersContextOperations(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	const marker = "request-scoped"
	RegisterEntity(Entity[nestedTestEntity, nestedTestOpts, nestedTestEntity]{
		Name: "activity",
		List: func(_ nestedTestOpts) ([]nestedTestEntity, error) { return nil, nil },
		Admin: &Entity[nestedTestEntity, nestedTestOpts, nestedTestEntity]{
			ListWithContext: func(ctx context.Context, _ nestedTestOpts) ([]nestedTestEntity, error) {
				return []nestedTestEntity{{ID: "l1", Name: ctx.Value(adminCtxKey{}).(string)}}, nil
			},
			GetWithContext: func(ctx context.Context, id string) (nestedTestEntity, error) {
				return nestedTestEntity{ID: id, Name: ctx.Value(adminCtxKey{}).(string)}, nil
			},
		},
	})

	info := adminEntityInfo(t, "activity")
	ctx := context.WithValue(context.Background(), adminCtxKey{}, marker)

	get := operationByVerb(t, info, "get")
	if get.ContextDataFunc == nil {
		t.Fatal("admin get operation has no ContextDataFunc")
	}
	got, err := get.ContextDataFunc(ctx, map[string]string{}, []string{"a-guid"})
	if err != nil {
		t.Fatalf("admin get: %v", err)
	}
	item, ok := got.(nestedTestEntity)
	if !ok {
		t.Fatalf("admin get returned %T, want nestedTestEntity", got)
	}
	if item.ID != "a-guid" || item.Name != marker {
		t.Errorf("admin get returned %+v, want the positional id and the context value %q", item, marker)
	}

	list := operationByVerb(t, info, "list")
	if list.ContextDataFunc == nil {
		t.Fatal("admin list operation has no ContextDataFunc")
	}
	if _, err = list.ContextDataFunc(ctx, map[string]string{}, nil); err != nil {
		t.Fatalf("admin list: %v", err)
	}

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)
	for _, path := range [][]string{{"admin", "activity", "get"}, {"admin", "activity", "list"}} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Errorf("expected %v subcommand, got %v err=%v", path, cmd, err)
		}
	}
}

// TestAdminEntityGetWithFlagsAndContext asserts the flags+context get variant
// also survives registration, and that the flag map reaches the handler.
func TestAdminEntityGetWithFlagsAndContext(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[nestedTestEntity, nestedTestOpts, nestedTestEntity]{
		Name: "activity",
		List: func(_ nestedTestOpts) ([]nestedTestEntity, error) { return nil, nil },
		Admin: &Entity[nestedTestEntity, nestedTestOpts, nestedTestEntity]{
			GetWithFlagsAndContext: func(_ context.Context, id string, flags map[string]string) (nestedTestEntity, error) {
				return nestedTestEntity{ID: id, Name: flags["trace"]}, nil
			},
		},
	})

	get := operationByVerb(t, adminEntityInfo(t, "activity"), "get")
	if get.ContextDataFunc == nil {
		t.Fatal("admin get operation has no ContextDataFunc")
	}
	got, err := get.ContextDataFunc(context.Background(), map[string]string{"id": "b-guid", "trace": "on"}, nil)
	if err != nil {
		t.Fatalf("admin get: %v", err)
	}
	if item := got.(nestedTestEntity); item.ID != "b-guid" || item.Name != "on" {
		t.Errorf("admin get returned %+v, want id b-guid and trace flag on", item)
	}
}
