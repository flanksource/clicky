package rpc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type refreshResult struct {
	ConnectionID string `json:"connectionId"`
	Provider     string `json:"provider"`
	Bound        int    `json:"bound"`
}

// A typed action that fails must answer with the error envelope. Before the
// fix the failed action's zero value was boxed into a non-nil `any`, so the
// executor's partial-data branch served `{"connectionId":"","provider":"", ...}`
// with the error only in the X-Error header — leaving web clients that read the
// body with nothing to show and no trace to correlate.
func TestEntityAction_FailureServesErrorNotZeroValue(t *testing.T) {
	const name = "rpc-action-error-wire-test"
	clicky.NewEntity[testEntity, testListOpts, testEntity](name).
		List(func(_ testListOpts) ([]testEntity, error) {
			return []testEntity{{ID: "1", Name: "one"}}, nil
		}).
		WithAction(clicky.Action("refresh", func(string, map[string]string) (refreshResult, error) {
			return refreshResult{}, errors.New("tenant name matches 2 companies")
		})).
		Register()

	root := &cobra.Command{Use: "testapp"}
	clicky.GenerateCLI(root)
	server := NewSwaggerServer(
		&ServeConfig{
			Title:                    "t",
			Version:                  "v",
			SkipHealth:               true,
			StructuredErrorResponses: true,
			Executor:                 &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"},
		},
		root,
		&OpenAPIConfig{Title: "t", Version: "v"},
	)
	mux := http.NewServeMux()
	server.RegisterExecutionRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/"+name+"/conn-1/refresh", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var body entity.ErrorResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "tenant name matches 2 companies", body.Message)
	assert.Equal(t, "internal_error", body.Code)
	assert.NotEmpty(t, body.Trace)
	assert.NotContains(t, rr.Body.String(), "connectionId", "the failed action's zero value must not be served as data")
}
