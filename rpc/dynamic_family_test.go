package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
				return map[string]api.Textable{"prod": api.Text{}.Append("Production")}, 1, nil
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
		errorWriter:  entity.NewErrorWriter(entity.ErrorOptions{}),
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

	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"} {
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

	// A surface is matched to its operations through the operation's own
	// x-clicky meta; without it an OperationCatalog for the surface finds no
	// list operation and renders the empty endpoint list.
	operation := spec.Paths["/api/v1/profile/daily"]["get"]
	require.NotNil(t, operation.Clicky, "the instance's operation names its surface")
	// A catalog takes a surface's list operation to be its verb-list,
	// collection-scoped one; anything less renders as a bare endpoint link.
	assert.Equal(t, [3]string{"daily", "list", "collection"},
		[3]string{operation.Clicky.Surface, operation.Clicky.Verb, operation.Clicky.Scope})
}

// A consumer that hand-writes paths the command tree cannot describe (multipart
// uploads, settings routes) used to render its own static document to add them,
// and that document silently dropped every family instance. Extensions have to
// ride the served document, so the two coexist.
func TestDynamicFamily_SpecKeepsTheConsumersExtensions(t *testing.T) {
	const handWritten = "/api/v1/settings/logging"
	server := NewSwaggerServer(
		&ServeConfig{Version: "1.0.0", Executor: &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"}},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "Test API", Version: "1.0.0", Extensions: []func(*OpenAPISpec){
			func(spec *OpenAPISpec) {
				spec.Paths[handWritten] = OpenAPIPath{"get": OpenAPIOperation{OperationID: "loggingSettings"}}
			},
		}},
	)
	registerTestFamily(t, profileFamily("daily"))

	for _, format := range []struct {
		name  string
		serve func(http.ResponseWriter, *http.Request)
	}{
		{"exported handler", server.HandleOpenAPIJSON},
		{"cached rendering", func(w http.ResponseWriter, r *http.Request) {
			server.serveSpec(w, r, specFormatJSON, "application/json")
		}},
	} {
		rec := httptest.NewRecorder()
		format.serve(rec, httptest.NewRequest("GET", "/api/openapi.json", nil))

		var spec OpenAPISpec
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec), format.name)
		assert.Equal(t, []bool{true, true}, []bool{
			spec.Paths[handWritten] != nil,
			spec.Paths["/api/v1/profile/daily"] != nil,
		}, "%s: the extension path and the family instance both appear", format.name)
	}
}

type requestMarker struct{}

// A family can only describe an instance by its filters, but the store behind
// it may know the whole contract — declared params, paging, sort. A request
// extension describes what exists as the request is served, and the path it
// writes is the one the document keeps.
func TestDynamicFamily_RequestExtensionsDescribeInstancesAheadOfTheFamily(t *testing.T) {
	const path = "/api/v1/profile/daily"
	server := NewSwaggerServer(
		&ServeConfig{Version: "1.0.0", Executor: &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"}},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "Test API", Version: "1.0.0", RequestExtensions: []func(context.Context, *OpenAPISpec) error{
			func(ctx context.Context, spec *OpenAPISpec) error {
				request := ctx.Value(requestMarker{}).(string)
				spec.Paths[path] = OpenAPIPath{"get": OpenAPIOperation{
					Parameters: []OpenAPIParameter{{Name: "stream", In: "query"}, {Name: "limit", In: "query"}},
					Clicky:     &ClickyOperationMeta{Surface: "daily", Verb: "list", Scope: "collection"},
				}}
				if spec.Clicky == nil {
					spec.Clicky = &ClickySpecMeta{}
				}
				spec.Clicky.Surfaces = append(spec.Clicky.Surfaces, ClickySurface{Key: "daily", Entity: "daily"})
				if spec.Components == nil {
					spec.Components = &OpenAPIComponents{}
				}
				if spec.Components.ClickyFilters == nil {
					spec.Components.ClickyFilters = map[string]entity.FilterSpec{}
				}
				spec.Components.ClickyFilters["filter-"+request] = entity.FilterSpec{Name: "filter-" + request}
				return nil
			},
		}},
	)
	registerTestFamily(t, profileFamily("daily"))

	serve := func(request string) OpenAPISpec {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/openapi.json", nil)
		server.HandleOpenAPIJSON(rec, req.WithContext(context.WithValue(req.Context(), requestMarker{}, request)))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var spec OpenAPISpec
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))
		return spec
	}
	serve("first")
	spec := serve("second")

	var parameters []string
	for _, parameter := range spec.Paths[path]["get"].Parameters {
		parameters = append(parameters, parameter.Name)
	}
	var surfaces, filters []string
	for _, surface := range spec.Clicky.Surfaces {
		if surface.Key == "daily" {
			surfaces = append(surfaces, surface.Key)
		}
	}
	for name := range spec.Components.ClickyFilters {
		filters = append(filters, name)
	}
	assert.Equal(t, map[string][]string{
		"parameters": {"stream", "limit"},
		"surfaces":   {"daily"},
		"filters":    {"filter-second"},
	}, map[string][]string{"parameters": parameters, "surfaces": surfaces, "filters": filters},
		"the extension's description wins the path once, and one request's writes never reach the next")
}

func TestDynamicFamily_RequestExtensionFailureFailsTheDocument(t *testing.T) {
	server := NewSwaggerServer(
		&ServeConfig{Version: "1.0.0", Executor: &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"}},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "Test API", Version: "1.0.0", RequestExtensions: []func(context.Context, *OpenAPISpec) error{
			func(context.Context, *OpenAPISpec) error { return errors.New("profile store unavailable") },
		}},
	)

	rec := httptest.NewRecorder()
	server.HandleOpenAPIJSON(rec, httptest.NewRequest("GET", "/api/openapi.json", nil))

	assert.Equal(t, []any{http.StatusInternalServerError, true},
		[]any{rec.Code, strings.Contains(rec.Body.String(), "profile store unavailable")})
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

// The family matcher compares only the first segment, so a family named after a
// registered entity would capture that entity's item route. The registered
// operation has to win, or registering a family makes an existing entity
// unreachable.
func TestDynamicFamily_DoesNotCaptureARegisteredOperationsPath(t *testing.T) {
	family := profileFamily("daily")
	family.Name = "config"
	registerTestFamily(t, family)

	service := &RPCService{Name: "api", Operations: []RPCOperation{{
		Name: "config get", Path: "/api/v1/config/{id}", Method: "GET",
		DataFunc: func(flags map[string]string, args []string) (any, error) {
			return map[string]any{"served_by": "operation"}, nil
		},
	}}}
	server := &SwaggerServer{
		config:       &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		converterCfg: &Config{PathPrefix: "/api/v1"},
		executor:     NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
		errorWriter:  entity.NewErrorWriter(entity.ErrorOptions{}),
	}

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/config/daily", nil))

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, rec.Body.String(), "operation",
		"the registered operation serves its own path, not the colliding family")
	assert.NotContains(t, rec.Body.String(), "profile")

	spec := NewOpenAPIGenerator(nil).GenerateFromService(service)
	require.NoError(t, server.addFamilyPaths(context.Background(), spec, []entity.DynamicEntityFamily{family}))
	assert.NotContains(t, spec.Paths, "/api/v1/config/daily",
		"the generated family path must not shadow the registered operation served at runtime")
	assert.Equal(t, "config_get", spec.Paths["/api/v1/config/{id}"]["get"].OperationID,
		"the registered templated path that captures the instance keeps describing it")
}

// A family instance is read-only, so a POST must not quietly run its List and
// answer 200 with rows.
func TestDynamicFamily_RefusesAWriteMethod(t *testing.T) {
	registerTestFamily(t, profileFamily("daily"))
	server := familyServer()

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("POST", "/api/v1/profile/daily", nil))

	res := rec.Result()
	require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	assert.Equal(t, "GET, HEAD", res.Header.Get("Allow"))

	var body entity.StatusError
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "method_not_allowed", body.Code)
}

// A family whose store is unreachable must not remove every other path from the
// document: a broken family costs its own instances, nothing more.
func TestDynamicFamily_ListFailureLeavesTheRestOfTheDocument(t *testing.T) {
	family := profileFamily("daily")
	family.List = func(context.Context) ([]entity.DynamicEntitySpec, error) {
		return nil, errors.New("backing store is unreachable")
	}
	registerTestFamily(t, family)

	server := NewSwaggerServer(
		&ServeConfig{Version: "1.0.0", Executor: &ExecutorConfig{Enabled: true, PathPrefix: "/api/v1"}},
		createTestRootCommand(),
		&OpenAPIConfig{Title: "Test API", Version: "1.0.0"},
	)

	rec := httptest.NewRecorder()
	server.serveSpec(rec, httptest.NewRequest("GET", "/api/openapi.json", nil), specFormatJSON, "application/json")

	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
	var spec OpenAPISpec
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))
	assert.NotEmpty(t, spec.Paths, "the static operations are still described")
	assert.NotContains(t, spec.Paths, "/api/v1/profile/daily")
}
