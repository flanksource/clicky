package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flanksource/clicky"
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
