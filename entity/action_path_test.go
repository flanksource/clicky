// Tests that a declared route path survives from the builder to the generated
// command, which is the only thing that makes it reach the served route.
package entity

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestWithPathReachesTheActionInfo(t *testing.T) {
	spec := Action("cancel", func(string, map[string]string) (string, error) { return "", nil }).
		WithPath("/api/v1/cycle-runs/{id}/levels/{level}/cancel")

	assert.Equal(t, "/api/v1/cycle-runs/{id}/levels/{level}/cancel", spec.actionInfo().RoutePath)
}

func TestAnActionWithoutAPathDeclaresNone(t *testing.T) {
	spec := Action("cancel", func(string, map[string]string) (string, error) { return "", nil })

	assert.Empty(t, spec.actionInfo().RoutePath, "an action must not acquire a path it did not declare")
}

func TestDeclaredPathReachesTheGeneratedCommand(t *testing.T) {
	parent := &cobra.Command{Use: "cycle-run"}
	generateIDCommand(parent, idCommand{
		Verb:  "cancel",
		Short: "Cancel a level",
		Op: EntityOperation{
			Verb:      "cancel",
			Method:    "POST",
			RoutePath: "/api/v1/cycle-runs/{id}/levels/{level}/cancel",
			DataFunc:  func(map[string]string, []string) (any, error) { return nil, nil },
		},
		MetaVerb:   "action",
		Scope:      "entity",
		ActionName: "cancel",
		IDParam:    "id",
	})

	var child *cobra.Command
	for _, candidate := range parent.Commands() {
		if candidate.Name() == "cancel" {
			child = candidate
		}
	}
	if child == nil {
		t.Fatal("the action command was not generated")
	}
	assert.Equal(t, "/api/v1/cycle-runs/{id}/levels/{level}/cancel",
		child.Annotations[annotationClickyOperationPath])
}
