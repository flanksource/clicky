package rpc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcBulkOpts struct {
	Status string `flag:"status"`
	Filter string `flag:"filter" help:"Act on every matching row instead of named ids"`
}

// A bulk action addresses its selection through the path: the ids ride
// comma-joined in the {id} segment, which is the route a selection toolbar
// dispatches to.
//
// TestEntityBulkActionPreservesLegacyRouteAndCatalog pins that shape for an
// ordinary action name. This covers the two things it cannot: that the ids in
// the segment actually reach the handler, and that an action whose name is also
// a CRUD verb keeps the same shape — `delete` aimed at forty rows is an action
// on a selection, not the entity's own delete, and collapsing it onto the flat
// route leaves those forty ids nowhere in the request.
func TestBulkActionIsAddressableByIDs(t *testing.T) {
	var ids []string

	record := func(selected []string, _ map[string]string) (any, error) {
		ids = append([]string{}, selected...)
		return selected, nil
	}
	byFilter := func(opts rpcBulkOpts, _ map[string]string) (any, error) {
		return map[string]string{"status": opts.Status}, nil
	}

	root := &cobra.Command{Use: "bulkapp", Short: "bulk app"}
	clicky.RegisterEntity(clicky.Entity[rpcFilterEntity, rpcBulkOpts, rpcFilterEntity]{
		Name: "rpc-bulk-entity",
		List: func(opts rpcBulkOpts) ([]rpcFilterEntity, error) {
			return []rpcFilterEntity{{ID: "1", Name: opts.Status}}, nil
		},
		BulkActions: []clicky.EntityBulkAction{
			clicky.BulkActionWithFilter("suspend", record, byFilter).WithShort("Suspend many"),
			clicky.BulkActionWithFilter("delete", record, byFilter).WithShort("Delete many"),
		},
	})
	clicky.GenerateCLI(root)

	server := NewSwaggerServer(&ServeConfig{
		Host: "localhost", Port: 8080,
		Executor: &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"},
	}, root, &OpenAPIConfig{})
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	routeFor := func(action string) (string, string) {
		t.Helper()
		for _, operation := range server.executor.service.Operations {
			meta := operation.Clicky
			if meta != nil && meta.Entity == "rpc-bulk-entity" && meta.ActionName == action {
				return operation.Method, operation.Path
			}
		}
		require.Failf(t, "bulk action not registered", "action %q", action)
		return "", ""
	}

	t.Run("every bulk action route carries the id segment", func(t *testing.T) {
		method, path := routeFor("suspend")
		assert.Equal(t, http.MethodPost, method)
		assert.Equal(t, "/api/v1/rpc-bulk-entity/{id}/suspend", path)

		_, deletePath := routeFor("delete")
		assert.Equal(t, "/api/v1/rpc-bulk-entity/{id}/delete", deletePath)
	})

	t.Run("comma-joined ids reach the handler", func(t *testing.T) {
		ids = nil
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(
			http.MethodPost, "/api/v1/rpc-bulk-entity/alpha,beta,gamma/suspend", nil))

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, ids)
	})
}
