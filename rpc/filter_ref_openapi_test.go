package rpc

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
)

type refUser struct {
	ID   string
	Name string
}

func (u refUser) GetID() string   { return u.ID }
func (u refUser) GetName() string { return u.Name }

type refTaskOpts struct {
	Owner string `flag:"owner"`
}

type refIncidentOpts struct {
	Assignee string `flag:"assignee"`
}

func findParam(t *testing.T, op OpenAPIOperation, name string) OpenAPIParameter {
	t.Helper()
	for _, p := range op.Parameters {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("parameter %q not found on operation %s", name, op.OperationID)
	return OpenAPIParameter{}
}

// TestOpenAPIEmitsNamedFilterRefAndLookup asserts a named filter reused by two
// entities is emitted once into components.x-clicky-filters and referenced by
// each entity's filter parameter via x-clicky-lookup.$ref, with the lookup URL
// pointing at that entity's own list path.
func TestOpenAPIEmitsNamedFilterRefAndLookup(t *testing.T) {
	entity.RegisterFilter(entity.NamedFilter{
		Name:   "ref-users",
		Label:  "Owner",
		Source: entity.StaticOptions(map[string]api.Textable{"u1": api.Text{Content: "Alice"}}),
	})

	clicky.NewEntity[refUser, refTaskOpts, refUser]("ref-tasks").
		Filters(entity.Use[refTaskOpts]("ref-users").As("owner")).
		List(func(refTaskOpts) ([]refUser, error) { return nil, nil }).
		Register()

	clicky.NewEntity[refUser, refIncidentOpts, refUser]("ref-incidents").
		Filters(entity.Use[refIncidentOpts]("ref-users").As("assignee")).
		List(func(refIncidentOpts) ([]refUser, error) { return nil, nil }).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)

	// The reusable definition is emitted once into the components bucket.
	require.NotNil(t, spec.Components)
	require.Contains(t, spec.Components.ClickyFilters, "ref-users")
	def := spec.Components.ClickyFilters["ref-users"]
	assert.Equal(t, "Owner", def.Label)
	assert.Equal(t, entity.SourceStatic, def.Source.Kind)
	assert.Equal(t, "Alice", def.Source.Options["u1"])

	// ref-tasks binds the filter to "owner" and points the lookup URL at its own path.
	ownerParam := findParam(t, spec.Paths["/api/v1/ref-tasks"]["get"], "owner")
	require.NotNil(t, ownerParam.Lookup, "owner filter param should carry x-clicky-lookup")
	assert.Equal(t, "#/components/x-clicky-filters/ref-users", ownerParam.Lookup.Ref)
	assert.Equal(t, "/api/v1/ref-tasks", ownerParam.Lookup.URL)
	assert.Equal(t, "owner", ownerParam.Lookup.Filter)
	assert.Equal(t, "__lookup_q", ownerParam.Lookup.SearchParam)

	// ref-incidents binds the SAME filter to a different key, with its own URL.
	assigneeParam := findParam(t, spec.Paths["/api/v1/ref-incidents"]["get"], "assignee")
	require.NotNil(t, assigneeParam.Lookup)
	assert.Equal(t, "#/components/x-clicky-filters/ref-users", assigneeParam.Lookup.Ref,
		"both entities reference the one shared definition")
	assert.Equal(t, "/api/v1/ref-incidents", assigneeParam.Lookup.URL)
	assert.Equal(t, "assignee", assigneeParam.Lookup.Filter)
}
