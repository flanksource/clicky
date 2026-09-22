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

// A mutation published under a method defined as safe can be triggered by
// anything that follows links — a prefetcher, a crawler, a browser restoring
// tabs. The override must refuse rather than register it.
func TestSafeMethodsAreRefusedForMutations(t *testing.T) {
	for _, verb := range []string{"create", "update", "delete"} {
		for _, method := range []string{"GET", "get", "HEAD", "OPTIONS"} {
			t.Run(verb+"/"+method, func(t *testing.T) {
				operations := []EntityOperation{{Verb: verb}}
				assert.Panicsf(t, func() {
					applyRouteOverrides(operations, map[string]RouteOverride{verb: {Method: method}})
				}, "%s must not be publishable as %s", verb, method)
			})
		}
	}
}

func TestSafeMethodsRemainAllowedForReads(t *testing.T) {
	for _, verb := range []string{"list", "get"} {
		operations := []EntityOperation{{Verb: verb}}
		assert.NotPanicsf(t, func() {
			applyRouteOverrides(operations, map[string]RouteOverride{verb: {Method: "GET"}})
		}, "%s reads nothing, so GET is correct for it", verb)
		assert.Equal(t, "GET", operations[0].Method)
	}
}

func TestMutationsKeepTheirUnsafeMethodOverrides(t *testing.T) {
	operations := []EntityOperation{{Verb: "delete"}}
	applyRouteOverrides(operations, map[string]RouteOverride{"delete": {Method: "POST"}})
	assert.Equal(t, "POST", operations[0].Method, "a mutation may move between unsafe methods")
}

// An admin sub-entity is registered separately from its parent, so an override
// declared on it has to be applied there too. Missing it is silent: the
// operation keeps the derived route and nothing reports that the declaration
// was ignored.
func TestAdminOperationsHonourTheirOwnOverrides(t *testing.T) {
	resetEntityRegistry(t)

	RegisterEntity(Entity[samplePlainEntity, struct{}, any]{
		Name: "widget",
		List: func(struct{}) ([]samplePlainEntity, error) { return nil, nil },
		Admin: &Entity[samplePlainEntity, struct{}, any]{
			Name:   "widget",
			List:   func(struct{}) ([]samplePlainEntity, error) { return nil, nil },
			Routes: map[string]RouteOverride{"list": {Path: "/api/v1/widget/admin/catalog"}},
		},
	})

	var admin *EntityInfo
	for i, info := range GetEntities() {
		if info.IsAdmin {
			admin = &GetEntities()[i]
		}
	}
	if admin == nil {
		t.Fatal("the admin entity did not register")
	}
	for _, op := range admin.Operations {
		if op.Verb == "list" {
			assert.Equal(t, "/api/v1/widget/admin/catalog", op.RoutePath,
				"the admin entity's own override must reach its operations")
			return
		}
	}
	t.Fatal("the admin entity has no list operation")
}
