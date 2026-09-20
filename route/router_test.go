// Tests the Router: routes a host application registers itself are observed
// through the same listener seam as generated operations.
package route

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky/entity"
)

// captureEvents subscribes for the duration of the test and returns the events
// collected so far.
func captureEvents(t *testing.T) func() []entity.OperationEvent {
	t.Helper()
	var events []entity.OperationEvent
	unsubscribe := entity.RegisterOperationListener(func(_ context.Context, event entity.OperationEvent) {
		events = append(events, event)
	})
	t.Cleanup(unsubscribe)
	return func() []entity.OperationEvent { return events }
}

func TestRawRouteIsObservedOnce(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	router.RawFunc("POST /api/v1/monitors", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}, Meta{Entity: "monitors", Verb: "create"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/monitors", nil))

	require.Equal(t, http.StatusCreated, recorder.Code, "the handler's own response must reach the caller")
	events := collected()
	require.Len(t, events, 1, "one request registers exactly one event")
	assert.Equal(t, "monitors", events[0].Entity)
	assert.Equal(t, "create", events[0].Verb)
}

func TestRawRouteReportsDeclaredMetadataNotAGuess(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	// A path that no naming convention would decompose into this entity/verb:
	// the declaration at the registration site is the only source of truth.
	router.RawFunc("POST /api/v1/scheme-health/probe", func(w http.ResponseWriter, _ *http.Request) {},
		Meta{Entity: "scheme-health", Verb: "probe"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/scheme-health/probe", nil))

	events := collected()
	require.Len(t, events, 1)
	assert.Equal(t, "scheme-health", events[0].Entity)
	assert.Equal(t, "probe", events[0].Verb)
}

func TestRawRouteCarriesTheHTTPSurface(t *testing.T) {
	var surface string
	unsubscribe := entity.RegisterOperationListener(func(ctx context.Context, _ entity.OperationEvent) {
		surface = entity.OperationSurfaceFromContext(ctx)
	})
	defer unsubscribe()

	router := NewRouter(http.NewServeMux())
	router.RawFunc("GET /api/v1/workloads", func(w http.ResponseWriter, _ *http.Request) {}, Meta{Entity: "workloads", Verb: "list", ReadOnly: true})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/workloads", nil))

	assert.Equal(t, "http", surface, "a listener must be able to tell an HTTP call from a CLI one")
}

func TestRawRouteReportsTheHandlerStatusAsItsError(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	router.RawFunc("DELETE /api/v1/monitors/{id}", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "monitor not found", http.StatusNotFound)
	}, Meta{Entity: "monitors", Verb: "delete"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/v1/monitors/m-1", nil))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	events := collected()
	require.Len(t, events, 1)
	require.Error(t, events[0].Error, "a failed request must not be recorded as a success")
	assert.Contains(t, events[0].Error.Error(), "404")
}

func TestRawRouteReportsThePathValueAsTheTarget(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	router.RawFunc("DELETE /api/v1/monitors/{id}", func(w http.ResponseWriter, _ *http.Request) {},
		Meta{Entity: "monitors", Verb: "delete", IDParam: "id"})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/v1/monitors/m-42", nil))

	events := collected()
	require.Len(t, events, 1)
	assert.Equal(t, "m-42", events[0].TargetID, "the wildcard names which record the call acted on")
}

// flushRecorder records whether the handler's Flush reached the transport.
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *flushRecorder) Flush() { f.flushes++ }

func TestRawRouteKeepsStreamingHandlersStreaming(t *testing.T) {
	router := NewRouter(http.NewServeMux())
	router.RawFunc("GET /api/v1/trace-runs/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for range 3 {
			_, _ = w.Write([]byte("data: tick\n\n"))
			w.(http.Flusher).Flush()
		}
	}, Meta{Entity: "trace-runs", Verb: "events", ReadOnly: true})

	recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil))

	assert.Equal(t, 3, recorder.flushes, "an SSE handler must still push each event as it is written")
}

// hijackRecorder reports Hijack support without a real connection.
type hijackRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestRawRouteKeepsHijackReachable(t *testing.T) {
	router := NewRouter(http.NewServeMux())
	router.RawFunc("GET /arthas/proxy/{rest...}", func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		require.True(t, ok, "a reverse proxy must still be able to take over the connection")
		_, _, _ = hijacker.Hijack()
	}, Meta{Entity: "arthas", Verb: "proxy"})

	recorder := &hijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/arthas/proxy/anything/at/all", nil))

	assert.True(t, recorder.hijacked)
}

func TestUnobservedRawRouteAllocatesNothingExtra(t *testing.T) {
	router := NewRouter(http.NewServeMux())
	router.RawFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {}, Meta{Entity: "health", Verb: "get", ReadOnly: true})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	allocs := testing.AllocsPerRun(100, func() {
		router.ServeHTTP(httptest.NewRecorder(), request)
	})

	// The recorder itself allocates; the point is that with no subscriber the
	// Router adds no event construction on top of it.
	baseline := testing.AllocsPerRun(100, func() {
		httptest.NewRecorder()
	})
	assert.Less(t, allocs-baseline, float64(12), "an unobserved route must not pay for observation")
}

func TestRawRouteDeclaresItsEffectRatherThanLeavingItToTheVerb(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	// "events" is an SSE subscription and "probe" creates real records, but
	// neither verb reads that way. Only the declaration distinguishes them.
	router.RawFunc("GET /api/v1/trace-runs/events", func(http.ResponseWriter, *http.Request) {},
		Meta{Entity: "trace-runs", Verb: "events", ReadOnly: true})
	router.RawFunc("POST /api/v1/scheme-health/probe", func(http.ResponseWriter, *http.Request) {},
		Meta{Entity: "scheme-health", Verb: "probe"})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/trace-runs/events", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/scheme-health/probe", nil))

	events := collected()
	require.Len(t, events, 2)
	require.NotNil(t, events[0].ReadOnly, "a route always declares its effect")
	assert.True(t, *events[0].ReadOnly, "subscribing to a stream changes nothing")
	require.NotNil(t, events[1].ReadOnly)
	assert.False(t, *events[1].ReadOnly, "an undeclared route is treated as a mutation")
}

func TestMountedGeneratedRouteIsNotObservedTwice(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	// Generated routes report themselves at the entity layer. If the transport
	// reported them as well, every generated operation would appear twice.
	router.MountGenerated("POST /api/v1/policy", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/policy", nil))

	assert.Empty(t, collected(), "the transport must not re-report what the entity layer already reported")
}

func TestSelfRoutedMountDerivesTheVerbFromTheMethod(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	// One mount, many operations behind it: the handler decides, so the route
	// can only report the method until it is split into declared routes.
	router.RawFunc("/api/v1/arthas/sessions/", func(http.ResponseWriter, *http.Request) {},
		Meta{Entity: "arthas", SelfRouted: true})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/arthas/sessions/s-1/classes", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/v1/arthas/sessions/s-1", nil))

	events := collected()
	require.Len(t, events, 2)
	assert.Equal(t, "get", events[0].Verb)
	require.NotNil(t, events[0].ReadOnly)
	assert.True(t, *events[0].ReadOnly, "a safe method on a self-routed mount reads")
	assert.Equal(t, "delete", events[1].Verb)
	require.NotNil(t, events[1].ReadOnly)
	assert.False(t, *events[1].ReadOnly)
}

func TestDeclaredRouteIgnoresTheRequestMethod(t *testing.T) {
	collected := captureEvents(t)

	router := NewRouter(http.NewServeMux())
	// A POST that only reads stays a read, because the registration said so.
	router.RawFunc("POST /api/v1/test-runner/preview", func(http.ResponseWriter, *http.Request) {},
		Meta{Entity: "test-runner", Verb: "preview", ReadOnly: true})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/test-runner/preview", nil))

	events := collected()
	require.Len(t, events, 1)
	assert.Equal(t, "preview", events[0].Verb)
	require.NotNil(t, events[0].ReadOnly)
	assert.True(t, *events[0].ReadOnly, "the method must not override the declaration")
}
