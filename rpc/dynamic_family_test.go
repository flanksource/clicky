package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// profileSpec is the shape a consumer whose entities are database rows resolves:
// a name that only exists because something created it a moment ago.
func profileSpec(name string) entity.DynamicEntitySpec {
	return entity.DynamicEntitySpec{
		Name:     name,
		Title:    "Profile " + name,
		Icon:     "database",
		ListType: reflect.TypeOf(struct{}{}),
		ItemType: reflect.TypeOf(struct {
			Profile string `json:"profile"`
		}{}),
		List: func(_ context.Context, flags map[string]string, _ []string) (any, error) {
			return []map[string]any{{"profile": name, "env": flags["env"]}}, nil
		},
		Filters: []entity.DynamicFilter{{
			Key:   "env",
			Label: "Environment",
			Options: func(_ context.Context, _ map[string]string, _ string, _ int) (map[string]api.Textable, int, error) {
				return map[string]api.Textable{"prod": api.Text{Content: "Production"}}, 1, nil
			},
		}},
	}
}

// profileFamily resolves only the names it was given, so an unknown one is the
// 404 the contract requires rather than an empty spec.
func profileFamily(names ...string) entity.DynamicEntityFamily {
	known := make(map[string]bool, len(names))
	for _, name := range names {
		known[name] = true
	}
	return entity.DynamicEntityFamily{
		Name:   "profile",
		Parent: "reporting",
		Resolve: func(_ context.Context, name string) (entity.DynamicEntitySpec, error) {
			if !known[name] {
				return entity.DynamicEntitySpec{}, entity.UnknownDynamicEntity("profile", name)
			}
			return profileSpec(name), nil
		},
		List: func(_ context.Context) ([]entity.DynamicEntitySpec, error) {
			specs := make([]entity.DynamicEntitySpec, 0, len(names))
			for _, name := range names {
				specs = append(specs, profileSpec(name))
			}
			return specs, nil
		},
	}
}

// familyServer has no operation registered for the family's path: an instance
// that did not exist at startup could not have one.
func familyServer() *SwaggerServer {
	service := &RPCService{Name: "api", Operations: []RPCOperation{{
		Name: "config list", Path: "/api/v1/config", Method: "GET",
		DataFunc: func(map[string]string, []string) (any, error) { return []map[string]any{}, nil },
	}}}
	return &SwaggerServer{
		config:       &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		converterCfg: &Config{PathPrefix: "/api/v1"},
		executor:     NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
	}
}

func TestDynamicFamily_ServesAnInstanceThatHasNoRegisteredOperation(t *testing.T) {
	registerTestFamily(t, profileFamily("daily"))
	server := familyServer()

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/profile/daily?env=prod", nil))

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var rows []map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&rows))
	assert.Equal(t, []map[string]any{{"profile": "daily", "env": "prod"}}, rows,
		"the filter value reaches the instance even though no declared parameter carries it")
}

func TestDynamicFamily_UnknownNameIsANotFoundInTheSharedErrorShape(t *testing.T) {
	registerTestFamily(t, profileFamily("daily"))
	server := familyServer()

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/profile/missing", nil))

	res := rec.Result()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	var body entity.StatusError
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "not_found", body.Code)
	assert.Equal(t, `no profile named "missing"`, body.Message)
	assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
}

func TestDynamicFamily_LookupComesFromTheResolvedSpec(t *testing.T) {
	registerTestFamily(t, profileFamily("daily"))
	server := familyServer()

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/profile/daily?__lookup=filters", nil))

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "application/json+clicky", res.Header.Get("Content-Type"))

	var body struct {
		Filters map[string]struct {
			Label   string                    `json:"label"`
			Options map[string]map[string]any `json:"options"`
		} `json:"filters"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	require.Contains(t, body.Filters, "env")
	assert.Equal(t, "Environment", body.Filters["env"].Label)
	assert.Contains(t, body.Filters["env"].Options, "prod")
}

func TestDynamicFamily_PagedInstanceStreamsThroughTheExportContract(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(2)}
	family := profileFamily("daily")
	family.Paged = func(_ context.Context, spec entity.DynamicEntitySpec, _ entity.PageRequest, _ map[string]string) (entity.PageResponse, error) {
		require.Equal(t, "daily", spec.Name)
		return entity.PageResponse{Rows: source, Mode: entity.ModeStreaming}, nil
	}
	registerTestFamily(t, family)
	server := familyServer()

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/profile/daily?format=csv&_download", nil))

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/csv; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Contains(t, res.Header.Get("Content-Disposition"), `filename="daily.csv"`,
		"the download is named after the instance, not the family")
	assert.Equal(t, "\xEF\xBB\xBFN\nrow-0\nrow-1\n", rec.Body.String())
	assert.Equal(t, 1, source.closes)
}

// One pattern set serves every instance the family will ever have: registering
// per instance is impossible, because the instances do not exist yet.
func TestDynamicFamily_RegistersOneRouteForTheWholeFamily(t *testing.T) {
	registerTestFamily(t, profileFamily("daily", "weekly"))
	server := familyServer()

	mux := http.NewServeMux()
	server.registerExecutionRoutes(mux)

	for _, method := range []string{"GET", "POST", "OPTIONS", "HEAD"} {
		_, pattern := mux.Handler(httptest.NewRequest(method, "/api/v1/profile/daily", nil))
		assert.Equal(t, method+" /api/v1/profile/{name}", wildcardPattern(method, pattern),
			"%s must resolve through the family's single pattern", method)
	}

	for _, name := range []string{"daily", "weekly"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/profile/"+name, nil))
		assert.Equal(t, http.StatusOK, rec.Result().StatusCode, "%s resolves through the same route", name)
	}
}

// A HEAD is served by the GET pattern under Go 1.22's ServeMux, so it reports
// the GET pattern rather than one of its own.
func wildcardPattern(method, pattern string) string {
	if method == "HEAD" && pattern == "GET /api/v1/profile/{name}" {
		return "HEAD /api/v1/profile/{name}"
	}
	return pattern
}

func TestDynamicFamily_SpecDescribesTheInstancesThatExistNow(t *testing.T) {
	server := NewSwaggerServer(
		&ServeConfig{Version: "1.0.0", Executor: &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"}},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "Test API", Version: "1.0.0"},
	)

	rec := httptest.NewRecorder()
	server.serveSpec(rec, httptest.NewRequest("GET", "/api/openapi.json", nil), specFormatJSON, "application/json")
	require.NotContains(t, rec.Body.String(), "/api/v1/profile/daily", "nothing is registered yet")

	registerTestFamily(t, profileFamily("daily"))

	rec = httptest.NewRecorder()
	server.serveSpec(rec, httptest.NewRequest("GET", "/api/openapi.json", nil), specFormatJSON, "application/json")

	var spec OpenAPISpec
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))
	require.Contains(t, spec.Paths, "/api/v1/profile/daily",
		"an entity created after startup has to appear without the cache being invalidated")
	assert.Contains(t, spec.Paths["/api/v1/profile/daily"], "get")

	require.NotNil(t, spec.Clicky)
	var found *ClickySurface
	for index := range spec.Clicky.Surfaces {
		if spec.Clicky.Surfaces[index].Entity == "daily" {
			found = &spec.Clicky.Surfaces[index]
		}
	}
	require.NotNil(t, found, "the instance carries a UI surface")
	assert.Equal(t, "daily", found.Key)
	assert.Equal(t, "Profile daily", found.Title)
	assert.Equal(t, "reporting", found.Parent)
	assert.Equal(t, "database", found.Icon)
}

// A family must not swallow a path that is not one of its instances.
func TestDynamicFamily_LeavesUnrelatedPathsAlone(t *testing.T) {
	registerTestFamily(t, profileFamily("daily"))
	server := familyServer()

	for _, target := range []string{"/api/v1/config", "/api/v1/profile", "/api/v1/other/daily", "/api/v1/profile/daily/extra"} {
		_, _, matched := server.matchDynamicFamily(target)
		assert.False(t, matched, "%s is not a family instance", target)
	}
}
