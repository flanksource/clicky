// Tests route overrides on the CRUD verbs. WithPath covers actions; an entity
// whose list or delete lives at a URL the derivation would not produce needs
// the same thing, or it cannot be registered without moving its callers.
package entity

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// crudCommands generates an entity and returns its subcommands by name.
func crudCommands(t *testing.T, routes map[string]RouteOverride) map[string]*cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "app"}
	info := EntityInfo{
		Name:     "trace-profiles",
		Type:     reflect.TypeOf(struct{}{}),
		ListType: reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{
			{Verb: "list", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }},
			{Verb: "get", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }},
			{Verb: "update", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }},
			{Verb: "delete", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }},
		},
	}
	applyRouteOverrides(info.Operations, routes)
	generateEntityCLI(root, info)

	var group *cobra.Command
	for _, candidate := range root.Commands() {
		if candidate.Name() == "trace-profiles" {
			group = candidate
		}
	}
	require.NotNil(t, group)
	byName := map[string]*cobra.Command{}
	for _, sub := range group.Commands() {
		byName[sub.Name()] = sub
	}
	return byName
}

func TestListCanBePublishedAtADeclaredPath(t *testing.T) {
	commands := crudCommands(t, map[string]RouteOverride{
		"list": {Path: "/api/v1/trace-profiles/catalog"},
	})

	require.Contains(t, commands, "list")
	assert.Equal(t, "/api/v1/trace-profiles/catalog",
		commands["list"].Annotations[annotationClickyOperationPath])
}

func TestDeleteCanBePublishedAtADeclaredPathAndMethod(t *testing.T) {
	// The route predates the registration: a POST to .../{name}/delete rather
	// than the DELETE on .../{name} the derivation would produce.
	commands := crudCommands(t, map[string]RouteOverride{
		"delete": {Path: "/api/v1/trace-profiles/{name}/delete", Method: "POST"},
	})

	require.Contains(t, commands, "delete")
	assert.Equal(t, "/api/v1/trace-profiles/{name}/delete",
		commands["delete"].Annotations[annotationClickyOperationPath])
	assert.Equal(t, "POST", commands["delete"].Annotations[annotationClickyOperationMethod])
}

func TestUpdateCanBePublishedAtADeclaredMethod(t *testing.T) {
	commands := crudCommands(t, map[string]RouteOverride{
		"update": {Method: "POST"},
	})

	require.Contains(t, commands, "update")
	assert.Equal(t, "POST", commands["update"].Annotations[annotationClickyOperationMethod])
}

func TestOperationsWithoutAnOverrideAreUnchanged(t *testing.T) {
	commands := crudCommands(t, nil)

	for _, verb := range []string{"list", "get", "update", "delete"} {
		require.Contains(t, commands, verb)
		assert.Emptyf(t, commands[verb].Annotations[annotationClickyOperationPath],
			"%s must not acquire a path it did not declare", verb)
	}
}
