package entity

import (
	"testing"

	"github.com/spf13/cobra"
)

// permissionOf annotates an entity-root command and one operation command, then
// returns the operation's resolved DefaultPermission. entityPerm is the
// entity-level default ("" means none); override is the per-action permission
// passed to annotateEntityOperationCommand ("" means inherit).
func permissionOf(t *testing.T, verb string, entityPerm, override ToolPermission) ToolPermission {
	t.Helper()
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{
		Name:      "invoice",
		ToolHints: MCPToolHints{DefaultPermission: entityPerm},
	})

	opCmd := &cobra.Command{Use: verb}
	annotateEntityOperationCommand(opCmd, entityCmd, verb, "", "collection", "", "", false, false, false, MCPToolHints{DefaultPermission: override})

	meta := GetCommandOpenAPIMeta(opCmd)
	if meta == nil {
		t.Fatalf("GetCommandOpenAPIMeta returned nil for operation command")
	}
	return meta.ToolHints.DefaultPermission
}

func TestToolPermissionVerbDefaults(t *testing.T) {
	tests := []struct {
		verb string
		want ToolPermission
	}{
		{verb: "list", want: ToolPermissionOn},
		{verb: "get", want: ToolPermissionOn},
		{verb: "create", want: ToolPermissionAsk},
		{verb: "update", want: ToolPermissionAsk},
		{verb: "delete", want: ToolPermissionAsk},
		{verb: "action", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			if got := permissionOf(t, tt.verb, "", ""); got != tt.want {
				t.Errorf("DefaultPermission for %q = %q, want %q", tt.verb, got, tt.want)
			}
		})
	}
}

func TestToolPermissionEntityOverridesVerbDefault(t *testing.T) {
	if got := permissionOf(t, "list", ToolPermissionAsk, ""); got != ToolPermissionAsk {
		t.Errorf("entity-level permission = %q, want ask", got)
	}
	if got := permissionOf(t, "delete", ToolPermissionOn, ""); got != ToolPermissionOn {
		t.Errorf("entity-level permission = %q, want on", got)
	}
}

func TestToolPermissionActionOverridesEntityAndVerb(t *testing.T) {
	if got := permissionOf(t, "delete", ToolPermissionOn, ToolPermissionAsk); got != ToolPermissionAsk {
		t.Errorf("per-action permission = %q, want ask", got)
	}
	if got := permissionOf(t, "action", ToolPermissionAsk, ToolPermissionOn); got != ToolPermissionOn {
		t.Errorf("per-action permission = %q, want on", got)
	}
}

func TestToolPermissionBuilderAndAction(t *testing.T) {
	e := NewEntity[sampleTableEntity, toolGroupOpts, sampleTableEntity]("invoice").
		ToolPermission(ToolPermissionAsk).
		Build()
	if e.ToolHints.DefaultPermission != ToolPermissionAsk {
		t.Errorf("Entity.ToolHints.DefaultPermission = %q, want ask", e.ToolHints.DefaultPermission)
	}

	action := Action("recalculate", func(id string, _ map[string]string) (sampleTableEntity, error) {
		return sampleTableEntity{ID: id}, nil
	}).WithToolPermission(ToolPermissionOn)
	if got := action.actionInfo().ToolHints.DefaultPermission; got != ToolPermissionOn {
		t.Errorf("ActionInfo.ToolHints.DefaultPermission = %q, want on", got)
	}
}

// TestToolPermissionPromotedEntityRoot verifies the promoted entity-root command
// (bare `invoice` running list) carries the list operation's permission.
func TestToolPermissionPromotedEntityRoot(t *testing.T) {
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{Name: "invoice"})
	listCmd := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
	annotateEntityOperationCommand(listCmd, entityCmd, "list", "", "collection", "", "", false, false, false, MCPToolHints{})
	entityCmd.AddCommand(listCmd)

	promoteListToEntityRoot(entityCmd)

	meta := GetCommandOpenAPIMeta(entityCmd)
	if meta == nil || meta.ToolHints.DefaultPermission != ToolPermissionOn {
		t.Fatalf("promoted root DefaultPermission = %+v, want on", meta)
	}
}
