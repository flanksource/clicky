package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEntity struct {
	ID   string
	Name string
}

func (t testEntity) GetID() string   { return t.ID }
func (t testEntity) GetName() string { return t.Name }

type testListOpts struct{}

func TestEntitiesHandler_ReturnsRegisteredEntities(t *testing.T) {
	// Entity registry is a process-global; avoid colliding with other tests
	// by registering a uniquely-named entity and only asserting on it.
	const name = "rpc-entities-test"
	clicky.RegisterEntity(clicky.Entity[testEntity, testListOpts, testEntity]{
		Name: name,
		List: func(_ testListOpts) ([]testEntity, error) {
			return []testEntity{{ID: "1", Name: "one"}}, nil
		},
		Get: func(id string) (testEntity, error) {
			return testEntity{ID: id, Name: id}, nil
		},
	})

	server := NewSwaggerServer(
		&ServeConfig{Title: "t", Version: "v", SkipHealth: true},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "t", Version: "v"},
	)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var payload []EntityDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))

	var found *EntityDTO
	for i := range payload {
		if payload[i].Name == name {
			found = &payload[i]
			break
		}
	}
	require.NotNil(t, found, "registered entity %q should be exposed at /api/entities", name)

	verbs := map[string]bool{}
	for _, op := range found.Operations {
		verbs[op.Verb] = true
	}
	assert.True(t, verbs["list"], "list verb missing")
	assert.True(t, verbs["get"], "get verb missing")
}

func TestEntityAction_ExplicitGETMethodUsesNestedEntityPath(t *testing.T) {
	const name = "rpc-entities-action-get-test"
	clicky.NewEntity[testEntity, testListOpts, testEntity](name).
		List(func(_ testListOpts) ([]testEntity, error) {
			return []testEntity{{ID: "1", Name: "one"}}, nil
		}).
		WithAction(
			clicky.Action("records", func(id string, _ map[string]string) ([]testEntity, error) {
				return []testEntity{{ID: id, Name: "record"}}, nil
			}).WithShort("List records").WithMethod("GET"),
		).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)

	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	for _, op := range service.Operations {
		if op.Clicky != nil && op.Clicky.Entity == name && op.Clicky.ActionName == "records" {
			assert.Equal(t, "GET", op.Method)
			assert.Equal(t, "/api/v1/"+name+"/{id}/records", op.Path)
			return
		}
	}
	t.Fatalf("expected records action for entity %q", name)
}

type accountListOpts struct {
	Kind   string `flag:"kind" help:"Filter by kind"`
	Status string `flag:"status" help:"Filter by status"`
}

type pagedListOpts struct {
	Limit  int `flag:"limit" help:"Limit"`
	Offset int `flag:"offset" help:"Offset"`
}

func TestEntityPagedList_ResponseEnvelopeAndHeaders(t *testing.T) {
	const name = "rpc-entity-paged-list-test"
	clicky.NewEntity[testEntity, pagedListOpts, testEntity](name).
		ListPaged(func(opts pagedListOpts) (clicky.PagedResult[testEntity], error) {
			return clicky.NewPagedResult(
				[]testEntity{{ID: "5", Name: "five"}, {ID: "6", Name: "six"}},
				opts.Limit,
				opts.Offset,
				7,
			), nil
		}).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)
	server := NewSwaggerServer(
		&ServeConfig{
			Title:      "t",
			Version:    "v",
			SkipHealth: true,
			Executor: &ExecutorConfig{
				Enabled:    true,
				PathPrefix: "/api/v1",
			},
		},
		root,
		&OpenAPIConfig{Title: "t", Version: "v"},
	)
	mux := http.NewServeMux()
	server.RegisterExecutionRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+name+"?limit=2&offset=4", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "7", rr.Header().Get("X-Total-Count"))
	assert.Equal(t, "2", rr.Header().Get("X-Page-Limit"))
	assert.Equal(t, "4", rr.Header().Get("X-Page-Offset"))

	var payload struct {
		Data []map[string]any `json:"data"`
		Page struct {
			Limit  int   `json:"limit"`
			Offset int   `json:"offset"`
			Total  int64 `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 2)
	assert.Equal(t, "5", payload.Data[0]["_id"])
	assert.Equal(t, 2, payload.Page.Limit)
	assert.Equal(t, 4, payload.Page.Offset)
	assert.Equal(t, int64(7), payload.Page.Total)
}

func TestEntityPagedList_ClickyJSONUnwrapsToTable(t *testing.T) {
	const name = "rpc-entity-paged-list-clicky-json-test"
	clicky.NewEntity[testEntity, pagedListOpts, testEntity](name).
		ListPaged(func(opts pagedListOpts) (clicky.PagedResult[testEntity], error) {
			return clicky.NewPagedResult(
				[]testEntity{{ID: "5", Name: "five"}, {ID: "6", Name: "six"}},
				opts.Limit,
				opts.Offset,
				7,
			), nil
		}).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)
	server := NewSwaggerServer(
		&ServeConfig{
			Title:      "t",
			Version:    "v",
			SkipHealth: true,
			Executor: &ExecutorConfig{
				Enabled:    true,
				PathPrefix: "/api/v1",
			},
		},
		root,
		&OpenAPIConfig{Title: "t", Version: "v"},
	)
	mux := http.NewServeMux()
	server.RegisterExecutionRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+name+"?limit=2&offset=4", nil)
	req.Header.Set("Accept", "application/json+clicky")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	// Paging still travels via headers, never the clicky body.
	assert.Equal(t, "7", rr.Header().Get("X-Total-Count"))
	assert.Equal(t, "2", rr.Header().Get("X-Page-Limit"))
	assert.Equal(t, "4", rr.Header().Get("X-Page-Offset"))

	// The clicky document root is the table itself — not a {data, page} map —
	// so the React DataTable/picker renders rows and marks attached ones.
	var doc struct {
		Version int `json:"version"`
		Node    struct {
			Kind string            `json:"kind"`
			Rows []json.RawMessage `json:"rows"`
		} `json:"node"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &doc))
	assert.Equal(t, "table", doc.Node.Kind)
	assert.Len(t, doc.Node.Rows, 2)
}

// TestEntityRoot_RunnableListShortcut_OmitsGlobalFlags pins the default
// behaviour that the bare entity command (`testapp <entity>`) is runnable as
// its own list, exposed as GET /api/v1/<entity>, and that the promotion copies
// ONLY the ListOpts filter flags onto the root — never the inherited global
// persistent flags (format/no-color/loglevel/config/...). A regression here
// reintroduces the API-Explorer "wall of global-flag chips instead of a table"
// bug.
func TestEntityRoot_RunnableListShortcut_OmitsGlobalFlags(t *testing.T) {
	const name = "rpc-entity-list-shortcut-test"
	clicky.NewEntity[testEntity, accountListOpts, testEntity](name).
		List(func(_ accountListOpts) ([]testEntity, error) {
			return []testEntity{{ID: "1", Name: "one"}}, nil
		}).
		Register()

	root := &cobra.Command{Use: "testapp"}
	// Reproduce the real CLIs: global format/logging/task flags plus a couple of
	// app globals live on the root's persistent flag set, inherited by every
	// subcommand.
	clicky.BindAllFlags(root.PersistentFlags(), "format", "tasks")
	root.PersistentFlags().String("config", "", "Config file path")
	root.PersistentFlags().String("entity", "", "Select entity")
	clicky.GenerateCLI(root)

	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	var rootOp *RPCOperation
	for i := range service.Operations {
		op := &service.Operations[i]
		if op.Clicky == nil || op.Clicky.Entity != name {
			continue
		}
		// The runnable entity-root carries entity annotations and is exposed as
		// the canonical list operation.
		if op.Path == "/api/v1/"+name {
			rootOp = op
		}
		assert.NotEqual(t, "/api/v1/"+name+"/list", op.Path,
			"the redundant `list` endpoint must be skipped once the root is runnable")
	}

	require.NotNil(t, rootOp, "entity root must be exposed as GET /api/v1/%s", name)
	assert.Equal(t, "GET", rootOp.Method)
	require.NotNil(t, rootOp.Clicky)
	assert.Equal(t, "list", rootOp.Clicky.Verb)
	assert.Equal(t, "collection", rootOp.Clicky.Scope)

	paramNames := map[string]bool{}
	for _, p := range rootOp.Parameters {
		paramNames[p.Name] = true
	}

	assert.True(t, paramNames["kind"], "ListOpts filter `kind` must be a query param")
	assert.True(t, paramNames["status"], "ListOpts filter `status` must be a query param")

	for _, leaked := range []string{
		"format", "no-color", "json", "yaml", "csv", "html", "pdf", "markdown",
		"pretty", "tree", "table", "filter", "dump-schema", "loglevel",
		"log-level", "json-logs", "no-progress", "config", "entity",
	} {
		assert.False(t, paramNames[leaked],
			"global flag %q must NOT leak into the entity list operation params", leaked)
	}
}

func TestEntitiesHandler_CORS(t *testing.T) {
	server := NewSwaggerServer(
		&ServeConfig{Title: "t", Version: "v", SkipHealth: true},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "t", Version: "v"},
	)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/entities", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}
