package entity

import (
	"testing"

	"github.com/spf13/cobra"
)

// operationHints returns the tool hints an entity operation command carries when
// neither the entity nor the action declares any.
func operationHints(t *testing.T, verb string) MCPToolHints {
	t.Helper()
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{Name: "invoice"})

	opCmd := &cobra.Command{Use: verb}
	annotateEntityOperationCommand(opCmd, entityCmd, verb, "", "collection", "", "", false, false, false, MCPToolHints{})

	meta := GetCommandOpenAPIMeta(opCmd)
	if meta == nil {
		t.Fatalf("GetCommandOpenAPIMeta returned nil for %q operation command", verb)
	}
	return meta.ToolHints
}

// Clicky states what an operation IS, never what it is allowed to do. Neither
// the permission slot nor the safety hints are stamped from a verb: an
// unannotated command reaches its consumer saying nothing, so the consumer's own
// policy is the only thing that decides.
//
// Deriving safety here is not merely redundant with mcp.EffectiveToolHints — it
// silently overrides it, because that inference only fills a hint still unset. A
// verb table here that disagreed with the one there would win without anything
// reporting the conflict, which is how `update` briefly stopped being
// destructive.
func TestEntityVerbStampsNoAuthorityOrSafety(t *testing.T) {
	for _, verb := range []string{"list", "get", "create", "update", "delete", "action"} {
		t.Run(verb, func(t *testing.T) {
			hints := operationHints(t, verb)
			if hints.DefaultPermission != "" {
				t.Errorf("DefaultPermission = %q, want empty", hints.DefaultPermission)
			}
			if hints.ReadOnlyHint != nil {
				t.Errorf("ReadOnlyHint = %v, want nil", *hints.ReadOnlyHint)
			}
			if hints.DestructiveHint != nil {
				t.Errorf("DestructiveHint = %v, want nil", *hints.DestructiveHint)
			}
			if hints.IdempotentHint != nil {
				t.Errorf("IdempotentHint = %v, want nil", *hints.IdempotentHint)
			}
		})
	}
}

// A hint an author declared is still carried: leaving inference out must not
// leave the declared path unwired.
func TestEntityDeclaredHintsSurvive(t *testing.T) {
	readOnly := true
	entityCmd := &cobra.Command{Use: "invoice"}
	annotateEntityCommand(entityCmd, EntityInfo{Name: "invoice"})

	opCmd := &cobra.Command{Use: "update"}
	annotateEntityOperationCommand(opCmd, entityCmd, "update", "", "entity", "", "", false, false, false,
		MCPToolHints{ReadOnlyHint: &readOnly, DefaultPermission: ToolPermissionOn})

	meta := GetCommandOpenAPIMeta(opCmd)
	if meta == nil {
		t.Fatal("GetCommandOpenAPIMeta returned nil")
	}
	if meta.ToolHints.ReadOnlyHint == nil || !*meta.ToolHints.ReadOnlyHint {
		t.Errorf("ReadOnlyHint = %v, want true", meta.ToolHints.ReadOnlyHint)
	}
	if meta.ToolHints.DefaultPermission != ToolPermissionOn {
		t.Errorf("DefaultPermission = %q, want %q", meta.ToolHints.DefaultPermission, ToolPermissionOn)
	}
}
