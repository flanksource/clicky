package entity

import (
	"testing"

	"github.com/spf13/cobra"
)

// toolGroupOf annotates an entity-root command and one operation command, then
// returns the operation's resolved ToolGroup. override is the per-action group
// passed to annotateEntityOperationCommand ("" means inherit).
func toolGroupOf(t *testing.T, entityGroup, override string) string {
	t.Helper()
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{Name: "invoice", ToolGroup: entityGroup})

	opCmd := &cobra.Command{Use: "list"}
	annotateEntityOperationCommand(opCmd, entityCmd, "list", "", "collection", "", "", false, false, false, MCPToolHints{Group: override})

	meta := GetCommandOpenAPIMeta(opCmd)
	if meta == nil {
		t.Fatalf("GetCommandOpenAPIMeta returned nil for operation command")
	}
	return meta.ToolGroup
}

func TestToolGroupAnnotationFlow(t *testing.T) {
	tests := []struct {
		name        string
		entityGroup string
		override    string
		want        string
	}{
		{name: "inherited from entity", entityGroup: "billing", override: "", want: "billing"},
		{name: "per-action override", entityGroup: "billing", override: "reporting", want: "reporting"},
		{name: "empty override keeps inherited", entityGroup: "billing", override: "", want: "billing"},
		{name: "no group anywhere", entityGroup: "", override: "", want: ""},
		{name: "override with no entity group", entityGroup: "", override: "ops", want: "ops"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolGroupOf(t, tt.entityGroup, tt.override); got != tt.want {
				t.Errorf("ToolGroup = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolGroupEntityLevelAnnotation(t *testing.T) {
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{Name: "invoice", ToolGroup: "billing"})
	meta := GetCommandOpenAPIMeta(entityCmd)
	if meta == nil || meta.ToolGroup != "billing" {
		t.Fatalf("entity ToolGroup = %+v, want billing", meta)
	}
}

func TestToolGroupOnlyCommandMetadata(t *testing.T) {
	cmd := &cobra.Command{
		Use: "sync",
		Annotations: map[string]string{
			annotationClickyToolGroup: "Admin Write",
		},
	}
	meta := GetCommandOpenAPIMeta(cmd)
	if meta == nil {
		t.Fatal("GetCommandOpenAPIMeta returned nil for a command with only a tool group")
	}
	if meta.Entity != "" {
		t.Fatalf("Entity = %q, want empty", meta.Entity)
	}
	if meta.ToolGroup != "Admin Write" {
		t.Fatalf("ToolGroup = %q, want Admin Write", meta.ToolGroup)
	}
}

type toolGroupOpts struct{}

func TestToolGroupBuilderAndAction(t *testing.T) {
	e := NewEntity[sampleTableEntity, toolGroupOpts, sampleTableEntity]("invoice").
		ToolGroup("billing").
		Build()
	if e.ToolGroup != "billing" {
		t.Errorf("Entity.ToolGroup = %q, want billing", e.ToolGroup)
	}
	if e.ToolHints.Group != "billing" {
		t.Errorf("Entity.ToolHints.Group = %q, want billing", e.ToolHints.Group)
	}

	action := Action("recalculate", func(id string, _ map[string]string) (sampleTableEntity, error) {
		return sampleTableEntity{ID: id}, nil
	}).WithToolGroup("ops")
	if got := action.actionInfo().ToolGroup; got != "ops" {
		t.Errorf("ActionInfo.ToolGroup = %q, want ops", got)
	}
	if got := action.actionInfo().ToolHints.Group; got != "ops" {
		t.Errorf("ActionInfo.ToolHints.Group = %q, want ops", got)
	}
}

func TestToolHintsAnnotationFlow(t *testing.T) {
	readOnly := true
	destructive := false
	idempotent := true
	openWorld := false
	strict := true
	cmd := &cobra.Command{Use: "sync"}
	AnnotateTool(cmd, MCPToolHints{
		Title:             "Sync invoices",
		Icon:              "refresh-cw",
		Group:             "billing",
		Parent:            "invoice",
		ReadOnlyHint:      &readOnly,
		DestructiveHint:   &destructive,
		IdempotentHint:    &idempotent,
		OpenWorldHint:     &openWorld,
		DefaultPermission: ToolPermissionAsk,
		Strict:            &strict,
	})

	meta := GetCommandOpenAPIMeta(cmd)
	if meta == nil {
		t.Fatal("GetCommandOpenAPIMeta returned nil for a command with tool hints")
	}
	hints := meta.ToolHints
	if hints.Title != "Sync invoices" || hints.Icon != "refresh-cw" || hints.Group != "billing" || hints.Parent != "invoice" {
		t.Fatalf("unexpected string hints: %+v", hints)
	}
	if hints.ReadOnlyHint == nil || !*hints.ReadOnlyHint {
		t.Fatalf("ReadOnlyHint = %v, want true", hints.ReadOnlyHint)
	}
	if hints.DestructiveHint == nil || *hints.DestructiveHint {
		t.Fatalf("DestructiveHint = %v, want false", hints.DestructiveHint)
	}
	if hints.IdempotentHint == nil || !*hints.IdempotentHint {
		t.Fatalf("IdempotentHint = %v, want true", hints.IdempotentHint)
	}
	if hints.OpenWorldHint == nil || *hints.OpenWorldHint {
		t.Fatalf("OpenWorldHint = %v, want false", hints.OpenWorldHint)
	}
	if hints.DefaultPermission != ToolPermissionAsk {
		t.Fatalf("DefaultPermission = %q, want ask", hints.DefaultPermission)
	}
	if hints.Strict == nil || !*hints.Strict {
		t.Fatalf("Strict = %v, want true", hints.Strict)
	}
}

func TestToolHintsBuilderAndAction(t *testing.T) {
	strict := true
	e := NewEntity[sampleTableEntity, toolGroupOpts, sampleTableEntity]("invoice").
		ToolHints(MCPToolHints{Group: "billing", Parent: "accounting", DefaultPermission: ToolPermissionAuto, Strict: &strict}).
		Build()
	if e.ToolGroup != "billing" || e.ToolHints.Parent != "accounting" || e.ToolHints.DefaultPermission != ToolPermissionAuto {
		t.Fatalf("entity hints = %+v, group=%q", e.ToolHints, e.ToolGroup)
	}

	action := Action("recalculate", func(id string, _ map[string]string) (sampleTableEntity, error) {
		return sampleTableEntity{ID: id}, nil
	}).WithToolHints(MCPToolHints{Group: "ops", DefaultPermission: ToolPermissionOn})
	info := action.actionInfo()
	if info.ToolGroup != "ops" || info.ToolHints.Group != "ops" || info.ToolHints.DefaultPermission != ToolPermissionOn {
		t.Fatalf("action hints = %+v, group=%q", info.ToolHints, info.ToolGroup)
	}
}
