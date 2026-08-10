package rpc

import (
	"fmt"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The paging and dynamic-family work grafted two new branches onto
// handleExecuteCommand, which every clicky consumer's executor routes run
// through. Neither branch may be reachable by an operation that opted into
// neither feature, so the responses below are recorded verbatim: status, every
// header, and the exact bytes. They were captured from the handler before the
// branches existed, so a diff here is a regression in the shared path rather
// than a restatement of whatever the code now does.

// dumpResponse renders a response as a stable, diffable transcript. Header order
// is not part of the contract, so it is sorted; everything else is byte-exact.
func dumpResponse(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	res := rec.Result()
	defer res.Body.Close() //nolint:errcheck

	names := make([]string, 0, len(res.Header))
	for name := range res.Header {
		names = append(names, name)
	}
	sort.Strings(names)

	var out strings.Builder
	fmt.Fprintf(&out, "%d\n", res.StatusCode)
	for _, name := range names {
		fmt.Fprintf(&out, "%s: %s\n", name, strings.Join(res.Header[name], ", "))
	}
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	fmt.Fprintf(&out, "\n%s", body)
	return out.String()
}

// legacyServer builds the executor shape a plain consumer has: one DataFunc
// list operation, one operation that also answers a filter lookup, and nothing
// paged or family-backed anywhere.
func legacyServer() *SwaggerServer {
	list := RPCOperation{
		Name:   "config list",
		Path:   "/api/v1/config",
		Method: "GET",
		Parameters: []RPCParameter{
			{Name: "env", Type: "string", In: "query", Description: "Environment"},
		},
		DataFunc: func(flags map[string]string, _ []string) (any, error) {
			return []map[string]any{{"name": "dev", "env": flags["env"]}}, nil
		},
	}
	lookup := RPCOperation{
		Name:   "widget list",
		Path:   "/api/v1/widget",
		Method: "GET",
		DataFunc: func(_ map[string]string, _ []string) (any, error) {
			return []map[string]any{{"name": "left"}}, nil
		},
		LookupFunc: func(_ map[string]string, _ []string) (any, error) {
			return map[string]any{"filters": map[string]any{
				"env": map[string]any{"label": "Environment"},
			}}, nil
		},
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{list, lookup}}
	return &SwaggerServer{
		config:       &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		converterCfg: &Config{PathPrefix: "/api/v1"},
		executor:     NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
	}
}

func TestHandleExecuteCommand_LegacyResponsesAreUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		accept string
		want   string
	}{
		{
			name:   "json list",
			method: "GET",
			target: "/api/v1/config?env=prod",
			accept: "application/json",
			want: "200\n" +
				"Access-Control-Allow-Headers: Content-Type, Accept\n" +
				"Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS\n" +
				"Access-Control-Allow-Origin: *\n" +
				"Content-Type: application/json\n" +
				"X-Cli-Command: \n" +
				"X-Execution-Success: true\n" +
				"X-Exit-Code: 0\n" +
				"\n[\n  {\n    \"env\": \"prod\",\n    \"name\": \"dev\"\n  }\n]",
		},
		{
			name:   "preflight",
			method: "OPTIONS",
			target: "/api/v1/config",
			want: "200\n" +
				"Access-Control-Allow-Headers: Content-Type, Accept\n" +
				"Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS\n" +
				"Access-Control-Allow-Origin: *\n" +
				"\n",
		},
		{
			name:   "unknown path",
			method: "GET",
			target: "/api/v1/nothing",
			want: "404\n" +
				"Access-Control-Allow-Headers: Content-Type, Accept\n" +
				"Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS\n" +
				"Access-Control-Allow-Origin: *\n" +
				"Content-Type: text/plain; charset=utf-8\n" +
				"X-Content-Type-Options: nosniff\n" +
				"\nNo operation found for GET /api/v1/nothing\n",
		},
		{
			name:   "filter lookup",
			method: "GET",
			target: "/api/v1/widget?__lookup=filters",
			want: "200\n" +
				"Access-Control-Allow-Headers: Content-Type, Accept\n" +
				"Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS\n" +
				"Access-Control-Allow-Origin: *\n" +
				"Content-Type: application/json+clicky\n" +
				"\n{\"filters\":{\"env\":{\"label\":\"Environment\"}}}\n",
		},
		{
			name:   "head is a lookup",
			method: "HEAD",
			target: "/api/v1/widget",
			want: "200\n" +
				"Access-Control-Allow-Headers: Content-Type, Accept\n" +
				"Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS\n" +
				"Access-Control-Allow-Origin: *\n" +
				"Content-Type: application/json+clicky\n" +
				"\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := legacyServer()
			req := httptest.NewRequest(test.method, test.target, nil)
			if test.accept != "" {
				req.Header.Set("Accept", test.accept)
			}
			rec := httptest.NewRecorder()

			server.handleExecuteCommand(rec, req)

			assert.Equal(t, test.want, dumpResponse(t, rec))
		})
	}
}

// registerTestFamily keeps the process-wide family registry as it found it, so
// one test's family cannot leak into another's routes or OpenAPI document.
func registerTestFamily(t *testing.T, family entity.DynamicEntityFamily) {
	t.Helper()
	entity.RegisterDynamicEntityFamily(family)
	t.Cleanup(func() { entity.UnregisterDynamicEntityFamily(family.Name) })
}

// staticRows is a RowIterator over a fixed slice, optionally failing at a
// chosen position, and recording that it was closed.
type staticRows struct {
	columns  []api.ColumnDef
	rows     []map[string]any
	failAt   int
	index    int
	err      error
	closes   int
	closeErr error
}

func (s *staticRows) Columns() []api.ColumnDef { return s.columns }

func (s *staticRows) Next() bool {
	if s.err != nil {
		return false
	}
	if s.failAt > 0 && s.index == s.failAt-1 {
		s.err = fmt.Errorf("backend failed at row %d", s.index)
		return false
	}
	if s.index >= len(s.rows) {
		return false
	}
	s.index++
	return true
}

func (s *staticRows) Row() map[string]any { return s.rows[s.index-1] }
func (s *staticRows) Err() error          { return s.err }

func (s *staticRows) Close() error {
	s.closes++
	return s.closeErr
}
