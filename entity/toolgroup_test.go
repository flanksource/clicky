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
	annotateEntityOperationCommand(opCmd, entityCmd, "list", "", "collection", "", "", false, false, false, override)

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

type toolGroupOpts struct{}

func TestToolGroupBuilderAndAction(t *testing.T) {
	e := NewEntity[sampleTableEntity, toolGroupOpts, sampleTableEntity]("invoice").
		ToolGroup("billing").
		Build()
	if e.ToolGroup != "billing" {
		t.Errorf("Entity.ToolGroup = %q, want billing", e.ToolGroup)
	}

	action := Action("recalculate", func(id string, _ map[string]string) (sampleTableEntity, error) {
		return sampleTableEntity{ID: id}, nil
	}).WithToolGroup("ops")
	if got := action.actionInfo().ToolGroup; got != "ops" {
		t.Errorf("ActionInfo.ToolGroup = %q, want ops", got)
	}
}
