package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcFilterEntity struct {
	ID   string
	Name string
}

func (e rpcFilterEntity) GetID() string   { return e.ID }
func (e rpcFilterEntity) GetName() string { return e.Name }

type rpcFilterOpts struct {
	Owner  string `flag:"owner"`
	Status string `flag:"status"`
}

type rpcOwnerFilter struct{}

func (rpcOwnerFilter) Key() string   { return "owner" }
func (rpcOwnerFilter) Label() string { return "Owner" }

func (rpcOwnerFilter) Lookup(opts *rpcFilterOpts) (map[string]api.Textable, error) {
	if opts.Owner == "" {
		return nil, nil
	}
	opts.Owner = "team/" + opts.Owner
	return map[string]api.Textable{
		opts.Owner: api.Text{Content: strings.TrimPrefix(opts.Owner, "team/")},
	}, nil
}

func (rpcOwnerFilter) Options(opts rpcFilterOpts) map[string]api.Textable {
	return map[string]api.Textable{
		"team/platform": api.Text{Content: "Platform"},
	}
}

type rpcStatusFilter struct{}

func (rpcStatusFilter) Key() string   { return "status" }
func (rpcStatusFilter) Label() string { return "Status" }

func (rpcStatusFilter) Lookup(opts *rpcFilterOpts) (map[string]api.Textable, error) {
	if opts.Status == "" {
		return nil, nil
	}

	opts.Status = "status:" + opts.Status
	return map[string]api.Textable{
		opts.Status: api.Text{Content: "Healthy", Style: "font-semibold"},
	}, nil
}

func (rpcStatusFilter) Options(opts rpcFilterOpts) map[string]api.Textable {
	if opts.Owner == "team/platform" {
		return map[string]api.Textable{
			"status:healthy": api.Text{Content: "Healthy", Style: "font-semibold"},
		}
	}
	return map[string]api.Textable{
		"status:healthy":  api.Text{Content: "Healthy", Style: "font-semibold"},
		"status:degraded": api.Text{Content: "Degraded"},
	}
}

func TestSwaggerServer_FilterLookupRoutes(t *testing.T) {
	root := &cobra.Command{Use: "testapp", Short: "test app"}

	bulkExecutions := 0
	clicky.RegisterEntity(clicky.Entity[rpcFilterEntity, rpcFilterOpts, rpcFilterEntity]{
		Name:    "rpc-filter-entity",
		Filters: []clicky.Filter[rpcFilterOpts]{rpcOwnerFilter{}, rpcStatusFilter{}},
		List: func(opts rpcFilterOpts) ([]rpcFilterEntity, error) {
			return []rpcFilterEntity{{ID: "1", Name: opts.Status}}, nil
		},
		BulkActions: []clicky.EntityBulkAction{
			clicky.BulkActionWithFilter(
				"bulk-suspend",
				func(ids []string, flags map[string]string) (any, error) {
					bulkExecutions++
					return ids, nil
				},
				func(opts rpcFilterOpts, flags map[string]string) (any, error) {
					bulkExecutions++
					return map[string]string{"status": opts.Status}, nil
				},
			).WithShort("Suspend matching entities"),
		},
	})
	clicky.GenerateCLI(root)

	server := NewSwaggerServer(&ServeConfig{
		Host: "localhost",
		Port: 8080,
		Executor: &ExecutorConfig{
			Enabled:    true,
			SkipPreRun: true,
			PathPrefix: "/api/v1",
		},
	}, root, &OpenAPIConfig{})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	t.Run("GET list lookup returns clicky metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rpc-filter-entity?owner=platform&status=healthy&__lookup=filters", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json+clicky", w.Header().Get("Content-Type"))

		var payload map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &payload)
		require.NoError(t, err)

		assert.Contains(t, w.Body.String(), `"status:healthy"`)
		assert.NotContains(t, w.Body.String(), `"status:degraded"`)
	})

	t.Run("HEAD returns metadata headers with no body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/api/v1/rpc-filter-entity?owner=platform&status=healthy", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json+clicky", w.Header().Get("Content-Type"))
		assert.Empty(t, w.Body.String())
	})

	t.Run("GET companion lookup on bulk route does not execute action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rpc-filter-entity/123/bulk-suspend?owner=platform&status=healthy&__lookup=filters", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 0, bulkExecutions)
		assert.Contains(t, w.Body.String(), `"status:healthy"`)
	})

	t.Run("GET companion route without lookup does not execute action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rpc-filter-entity/123/bulk-suspend?owner=platform&status=healthy", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, 0, bulkExecutions)
	})
}
