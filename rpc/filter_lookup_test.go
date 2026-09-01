package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
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
		// The bulk route is id-addressed, so its lookup companion is too: the
		// segment is present but says nothing, because a lookup asks what the
		// filters are rather than acting on a selection.
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

func TestLookupPreservesUndeclaredSiblingParameters(t *testing.T) {
	var captured map[string]string
	op := &RPCOperation{
		ContextLookupFunc: func(_ context.Context, flags map[string]string, _ []string) (any, error) {
			captured = flags
			return map[string]any{"filters": map[string]any{}}, nil
		},
	}
	server := &SwaggerServer{
		executor: NewCommandExecutor(&RPCService{}, &ExecutorConfig{}),
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/profile?region=prod&filter.service=api&__lookup=filters",
		nil,
	)
	response := httptest.NewRecorder()

	server.handleLookupCommand(response, request, op)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "prod", captured["region"])
	assert.Equal(t, "api", captured["filter.service"])
	assert.NotContains(t, captured, "__lookup")
}

func TestLookupPreservesBackendErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "classified", err: entity.NewStatusError(http.StatusServiceUnavailable, "lookup_unavailable", "retry later"), status: http.StatusServiceUnavailable, code: "lookup_unavailable"},
		{name: "unclassified", err: errors.New("backend unavailable"), status: http.StatusInternalServerError, code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &SwaggerServer{
				// A backend failure keeps its own status only on the structured
				// surface; the legacy one answers 400 for every lookup failure.
				config:      &ServeConfig{StructuredErrorResponses: true},
				executor:    NewCommandExecutor(&RPCService{}, &ExecutorConfig{}),
				errorWriter: entity.NewErrorWriter(entity.ErrorOptions{}),
			}
			op := &RPCOperation{ContextLookupFunc: func(context.Context, map[string]string, []string) (any, error) {
				return nil, test.err
			}}
			response := httptest.NewRecorder()

			server.handleLookupCommand(response, httptest.NewRequest(http.MethodGet, "/lookup", nil), op)

			require.Equal(t, test.status, response.Code)
			var body entity.ErrorResponse
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, test.code, body.Code)
		})
	}
}
