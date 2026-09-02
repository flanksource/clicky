package rpc

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bulkRouteCompatibilityEntity struct {
	ID   string
	Name string
}

func (entity bulkRouteCompatibilityEntity) GetID() string   { return entity.ID }
func (entity bulkRouteCompatibilityEntity) GetName() string { return entity.Name }

type bulkRouteCompatibilityListOptions struct{}

func TestEntityBulkActionPreservesLegacyRouteAndCatalog(t *testing.T) {
	const entityName = "bulk-route-compatibility"
	clicky.NewEntity[bulkRouteCompatibilityEntity, bulkRouteCompatibilityListOptions, bulkRouteCompatibilityEntity](entityName).
		List(func(bulkRouteCompatibilityListOptions) ([]bulkRouteCompatibilityEntity, error) {
			return nil, nil
		}).
		WithBulkAction(clicky.BulkAction(
			"archive",
			func(ids []string, _ map[string]string) (any, error) { return ids, nil },
		).WithShort("Archive selected")).
		Register()

	root := &cobra.Command{Use: "compatibility-test"}
	clicky.GenerateCLI(root)
	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	var bulkOperation *RPCOperation
	for index := range service.Operations {
		operation := &service.Operations[index]
		assert.NotContains(t, operation.Path, "/_bulk/")
		if operation.Clicky != nil && operation.Clicky.Entity == entityName && operation.Clicky.ActionName == "archive" {
			bulkOperation = operation
		}
	}
	require.NotNil(t, bulkOperation)
	assert.Equal(t, http.MethodPost, bulkOperation.Method)
	assert.Equal(t, "/api/v1/"+entityName+"/{id}/archive", bulkOperation.Path)
	assert.Contains(t, bulkOperation.Parameters, RPCParameter{
		Name: "id", Type: "string", Description: "Positional argument from command", Required: true, In: "path",
	})

	snapshot := EntitySnapshot()
	var catalogEntity *EntityDTO
	for index := range snapshot {
		if snapshot[index].Name == entityName {
			catalogEntity = &snapshot[index]
			break
		}
	}
	require.NotNil(t, catalogEntity)
	require.Len(t, catalogEntity.BulkActions, 1)
	catalogJSON, err := json.Marshal(catalogEntity.BulkActions[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"archive","short":"Archive selected"}`, string(catalogJSON))
	assert.NotContains(t, strings.ToLower(string(catalogJSON)), "collection_path")
	assert.NotContains(t, string(catalogJSON), "_bulk")
}
