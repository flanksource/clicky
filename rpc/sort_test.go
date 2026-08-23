package rpc

import (
	"testing"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcSortableEntity struct {
	ID         string    `json:"id"`
	Name       string    `json:"name" sort:"name"`
	UpdatedGMT time.Time `json:"updatedGMT" pretty:"label=Updated" sort:"updated"`
}

func (e rpcSortableEntity) GetID() string   { return e.ID }
func (e rpcSortableEntity) GetName() string { return e.Name }
func (rpcSortableEntity) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		api.Column("name").Build(),
		api.Column("updatedGMT").Label("Updated").Build(),
		api.Column("id").Build(),
	}
}
func (e rpcSortableEntity) Row() map[string]any {
	return map[string]any{"name": e.Name, "updatedGMT": e.UpdatedGMT, "id": e.ID}
}

type rpcSortableOptions struct {
	clicky.SortOptions
}

func TestSortableEntityParametersExposeEnumsDefaultsAndRoles(t *testing.T) {
	const entityName = "rpc-sortable-parameters-test"
	clicky.NewEntity[rpcSortableEntity, rpcSortableOptions, rpcSortableEntity](entityName).
		Sort(clicky.SortSpec{Default: clicky.SortOptions{Key: "updated", Direction: clicky.SortDirectionDesc}}).
		List(func(rpcSortableOptions) ([]rpcSortableEntity, error) { return nil, nil }).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)
	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	var operation *RPCOperation
	for i := range service.Operations {
		candidate := &service.Operations[i]
		if candidate.Clicky != nil && candidate.Clicky.Entity == entityName && candidate.Clicky.Verb == "list" {
			operation = candidate
			break
		}
	}
	require.NotNil(t, operation)

	parameters := map[string]RPCParameter{}
	for _, parameter := range operation.Parameters {
		parameters[parameter.Name] = parameter
	}
	require.Contains(t, parameters, "sort")
	require.Contains(t, parameters, "order")
	assert.Equal(t, []string{"name", "updated"}, parameters["sort"].Enum)
	assert.Equal(t, "updated", parameters["sort"].Default)
	assert.Equal(t, []string{"asc", "desc"}, parameters["order"].Enum)
	assert.Equal(t, "desc", parameters["order"].Default)
	assert.Equal(t, "sort", ParamRole(*operation, parameters["sort"]))
	assert.Equal(t, "order", ParamRole(*operation, parameters["order"]))

	spec, err := NewOpenAPIGenerator(nil).GenerateFromCobra(root)
	require.NoError(t, err)
	openAPIOperation := spec.Paths["/api/v1/"+entityName]["get"]
	openAPIParameters := map[string]OpenAPIParameter{}
	for _, parameter := range openAPIOperation.Parameters {
		openAPIParameters[parameter.Name] = parameter
	}
	require.Contains(t, openAPIParameters, "sort")
	require.Contains(t, openAPIParameters, "order")
	assert.Equal(t, []interface{}{"name", "updated"}, openAPIParameters["sort"].Schema.Enum)
	assert.Equal(t, "sort", openAPIParameters["sort"].Clicky.Role)
	assert.Equal(t, []interface{}{"asc", "desc"}, openAPIParameters["order"].Schema.Enum)
	assert.Equal(t, "order", openAPIParameters["order"].Clicky.Role)
}
