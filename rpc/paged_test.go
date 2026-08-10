package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func numberedRows(count int) []map[string]any {
	rows := make([]map[string]any, count)
	for index := range rows {
		rows[index] = map[string]any{"n": fmt.Sprintf("row-%d", index)}
	}
	return rows
}

func drain(rows *peekedRows) []map[string]any {
	out := []map[string]any{}
	for rows.Next() {
		out = append(out, rows.Row())
	}
	return out
}

// A peeked row that is dropped is a row the caller paid for and never saw, and a
// peeked row replayed twice is a row that never existed. Both are silent, so the
// wrapper is exercised on its own rather than only through an export.
func TestPeekedRows_ReplaysEveryRowExactlyOnce(t *testing.T) {
	for _, count := range []int{0, 1, 5} {
		source := &staticRows{rows: numberedRows(count)}
		rows := peekRows(source)

		require.NoError(t, rows.PeekErr())
		assert.Equal(t, source.rows, drain(rows), "%d rows must arrive in order, none lost or repeated", count)
		assert.NoError(t, rows.Err())
	}
}

func TestPeekedRows_ReportsFirstReadFailureBeforeAnyRow(t *testing.T) {
	rows := peekRows(&staticRows{rows: numberedRows(3), failAt: 1})

	require.EqualError(t, rows.PeekErr(), "backend failed at row 0")
	assert.Empty(t, drain(rows), "a failed first read yields no rows")
}

// A failure past the first row is deliberately NOT a PeekErr: by then the status
// is committed and the only honest answer is an aborted transfer.
func TestPeekedRows_LaterFailureIsNotAPeekError(t *testing.T) {
	rows := peekRows(&staticRows{rows: numberedRows(4), failAt: 3})

	require.NoError(t, rows.PeekErr())
	assert.Len(t, drain(rows), 2)
	assert.EqualError(t, rows.Err(), "backend failed at row 2")
}

// WriteTableStream closes the iterator it is handed and the handler closes it on
// every path that never reaches WriteTableStream, so a double close is normal.
func TestPeekedRows_CloseIsIdempotent(t *testing.T) {
	source := &staticRows{rows: numberedRows(2)}
	rows := peekRows(source)

	require.NoError(t, rows.Close())
	require.NoError(t, rows.Close())
	assert.Equal(t, 1, source.closes)
}

func TestBoundedRows_ReportsWhetherTheCeilingCutAnything(t *testing.T) {
	tests := []struct {
		name          string
		rows          int
		limit         int
		wantDelivered int
		wantTruncated bool
	}{
		{name: "under the ceiling", rows: 2, limit: 5, wantDelivered: 2},
		{name: "exactly on the ceiling", rows: 3, limit: 3, wantDelivered: 3},
		{name: "past the ceiling", rows: 6, limit: 3, wantDelivered: 3, wantTruncated: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bounded := &boundedRows{RowIterator: peekRows(&staticRows{rows: numberedRows(test.rows)}), limit: test.limit}
			delivered := 0
			for bounded.Next() {
				delivered++
			}
			assert.Equal(t, test.wantDelivered, delivered)
			assert.Equal(t, test.wantTruncated, bounded.truncated)
		})
	}
}

// pagedServer serves one paged operation at /api/v1/config.
func pagedServer(paged entity.PagedFunc) *SwaggerServer {
	op := RPCOperation{
		Name:      "config list",
		Path:      "/api/v1/config",
		Method:    "GET",
		PagedFunc: paged,
		Clicky:    &entity.ClickyOperationMeta{Entity: "config"},
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{op}}
	return &SwaggerServer{
		config:       &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		converterCfg: &Config{PathPrefix: "/api/v1"},
		executor:     NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
	}
}

func staticPaged(rows *staticRows, res entity.PageResponse) entity.PagedFunc {
	return func(_ context.Context, _ entity.PageRequest, _ map[string]string) (entity.PageResponse, error) {
		res.Rows = rows
		return res, nil
	}
}

func TestHandlePaged_StreamsWithExportHeaders(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(3)}
	server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming, Pageable: true}))

	req := httptest.NewRequest("GET", "/api/v1/config?format=csv&_download", nil)
	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, req)

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/csv; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, entity.ModeStreaming, res.Header.Get("X-Export-Mode"))
	assert.Equal(t, "unknown", res.Header.Get("X-Total-Relation"))
	assert.Contains(t, res.Header.Get("Content-Disposition"), `filename="config.csv"`)
	assert.Contains(t, res.Header.Get("Access-Control-Expose-Headers"), "X-Export-Mode")
	assert.Equal(t, "\xEF\xBB\xBFN\nrow-0\nrow-1\nrow-2\n", rec.Body.String(), "every row, once, behind the download BOM")
	assert.Equal(t, 1, source.closes)
}

// C1: a query that dies on its first read must be a status, not a short body
// under a 200 that reads as "no results".
func TestHandlePaged_FirstRowFailureIsAStatus(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(3), failAt: 1}
	server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming}))

	req := httptest.NewRequest("GET", "/api/v1/config?format=csv", nil)
	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, req)

	res := rec.Result()
	require.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Empty(t, res.Header.Get("Content-Disposition"), "no export header is committed before the first row is known")

	var body entity.StatusError
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "query_failed", body.Code)
	// H2: the failing path still releases the cursor.
	assert.Equal(t, 1, source.closes)
	// An error body a browser cannot read is worse than the error it describes.
	assert.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
}

// C1: past the first row the status is spent, so the transfer is broken instead.
// A plain page declares no trailer section, so the abort is the whole signal.
func TestHandlePaged_MidStreamFailureAbortsTheTransfer(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(6), failAt: 3}
	server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming}))

	req := httptest.NewRequest("GET", "/api/v1/config?format=csv", nil)
	rec := httptest.NewRecorder()

	defer func() {
		assert.Equal(t, http.ErrAbortHandler, recover(), "an aborted transfer is the only failure a client detects here")
		assert.Empty(t, rec.Header().Values("Trailer"), "a page does not buy a trailer section to report this in")
		assert.Empty(t, rec.Header().Get(streamErrorTrailer), "an undeclared trailer is dropped, so setting one only misleads")
		assert.Equal(t, 1, source.closes, "the panic path still closes")
	}()
	server.handleExecuteCommand(rec, req)
	t.Fatal("expected the handler to abort")
}

// Where a trailer section is already declared it costs nothing to say why the
// stream stopped, so the reason rides along with X-Truncated.
func TestHandlePaged_MidStreamFailureRidesADeclaredTrailer(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(9), failAt: 3}
	server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming, Ceiling: 5, MaxRows: 5}))

	req := httptest.NewRequest("GET", "/api/v1/config?format=csv&scope=all", nil)
	rec := httptest.NewRecorder()

	defer func() {
		assert.Equal(t, http.ErrAbortHandler, recover())
		assert.Contains(t, rec.Header().Values("Trailer"), streamErrorTrailer,
			"a trailer nobody declared is a trailer nobody reads")
		assert.Equal(t, "backend failed at row 2", rec.Header().Get(streamErrorTrailer))
		assert.Equal(t, 1, source.closes)
	}()
	server.handleExecuteCommand(rec, req)
	t.Fatal("expected the handler to abort")
}

// A page must keep the Content-Length an unconditional trailer declaration would
// have cost it. Only a real server computes one, so this goes over a socket.
func TestHandlePaged_PageKeepsItsContentLength(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(3)}
	server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModePage, Pageable: true}))

	backend := httptest.NewServer(http.HandlerFunc(server.handleExecuteCommand))
	defer backend.Close()

	res, err := backend.Client().Get(backend.URL + "/api/v1/config?format=csv")
	require.NoError(t, err)
	defer res.Body.Close() //nolint:errcheck

	assert.Empty(t, res.Header.Values("Trailer"), "a page declares no trailer")
	assert.Empty(t, res.TransferEncoding, "and so is not forced into chunked encoding")
	assert.Equal(t, int64(len("N\nrow-0\nrow-1\nrow-2\n")), res.ContentLength,
		"the length is knowable and has to be stated")
}

// M3: a HEAD resolves every header and stops. Driving the walk to throw its
// bytes away would charge the caller the whole query for a body it did not ask
// for.
func TestHandlePaged_HeadResolvesHeadersWithoutWalking(t *testing.T) {
	source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(50)}
	server := pagedServer(staticPaged(source, entity.PageResponse{
		Mode: entity.ModeStreaming, Total: &entity.Total{Value: 50, Exact: true},
	}))

	req := httptest.NewRequest("HEAD", "/api/v1/config?format=csv&scope=all", nil)
	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, req)

	res := rec.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/csv; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, "50", res.Header.Get("X-Total-Count"))
	assert.Equal(t, "eq", res.Header.Get("X-Total-Relation"))
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, 1, source.index, "only the peek is read; the other 49 rows are never fetched")
	assert.Equal(t, 1, source.closes)
}

// L1: the HEAD and the GET have to agree about how the body would have been
// encoded, which they only do if both reach the compressor's WriteHeader.
func TestHandlePaged_HeadAndGetAgreeOnContentEncoding(t *testing.T) {
	encodings := map[string]string{}
	for _, method := range []string{"GET", "HEAD"} {
		source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(3)}
		server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming}))

		req := httptest.NewRequest(method, "/api/v1/config?format=csv", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		Compress(http.HandlerFunc(server.handleExecuteCommand)).ServeHTTP(rec, req)

		encodings[method] = rec.Result().Header.Get("Content-Encoding")
	}

	require.Equal(t, "gzip", encodings["GET"], "a csv export is worth compressing")
	assert.Equal(t, encodings["GET"], encodings["HEAD"])
}

func TestHandlePaged_TruncationIsReportedAsTheDeclaredTrailer(t *testing.T) {
	tests := []struct {
		name string
		rows int
		want string
	}{
		{name: "ceiling not reached", rows: 2, want: "false"},
		{name: "ceiling bit", rows: 9, want: "true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(test.rows)}
			server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming, Ceiling: 3, MaxRows: 3}))

			req := httptest.NewRequest("GET", "/api/v1/config?format=csv&scope=all", nil)
			rec := httptest.NewRecorder()
			server.handleExecuteCommand(rec, req)

			res := rec.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
			assert.Contains(t, res.Header.Values("Trailer"), "X-Truncated")
			assert.Equal(t, test.want, rec.Header().Get("X-Truncated"))
			assert.Equal(t, min(test.rows, 3), strings.Count(rec.Body.String(), "\n")-1, "the ceiling bounds the body")
		})
	}
}

func TestHandlePaged_RefusesWhatItCannotStream(t *testing.T) {
	tests := []struct {
		name   string
		target string
		accept string
		status int
		code   string
	}{
		{name: "refused representation", target: "/api/v1/config", accept: "application/json;q=0", status: http.StatusNotAcceptable, code: "not_acceptable"},
		{name: "unstreamable format", target: "/api/v1/config?format=clicky-json", status: http.StatusNotAcceptable, code: "not_acceptable"},
		{name: "unbounded pdf", target: "/api/v1/config?format=pdf", status: http.StatusInternalServerError, code: "unbounded_pdf"},
		{name: "invalid scope", target: "/api/v1/config?scope=some", status: http.StatusBadRequest, code: "invalid_scope"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &staticRows{columns: []api.ColumnDef{{Name: "n"}}, rows: numberedRows(3)}
			server := pagedServer(staticPaged(source, entity.PageResponse{Mode: entity.ModeStreaming}))

			req := httptest.NewRequest("GET", test.target, nil)
			if test.accept != "" {
				req.Header.Set("Accept", test.accept)
			}
			rec := httptest.NewRecorder()
			server.handleExecuteCommand(rec, req)

			res := rec.Result()
			require.Equal(t, test.status, res.StatusCode)
			var body entity.StatusError
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			assert.Equal(t, test.code, body.Code)
		})
	}
}

// The provider's own failure is a status too, and it must not leak the iterator
// it never returned.
func TestHandlePaged_ProviderErrorIsWrittenInTheSharedErrorShape(t *testing.T) {
	server := pagedServer(func(_ context.Context, _ entity.PageRequest, _ map[string]string) (entity.PageResponse, error) {
		return entity.PageResponse{}, entity.NewStatusError(http.StatusForbidden, "no_access", "this connection is not yours")
	})

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/config", nil))

	res := rec.Result()
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	var body entity.StatusError
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "no_access", body.Code)
}

// A paged operation still answers an explicit lookup through the ordinary lookup
// path — only the bare HEAD is reinterpreted.
func TestHandlePaged_ExplicitLookupStillReachesTheLookup(t *testing.T) {
	op := RPCOperation{
		Name:   "config list",
		Path:   "/api/v1/config",
		Method: "GET",
		PagedFunc: func(_ context.Context, _ entity.PageRequest, _ map[string]string) (entity.PageResponse, error) {
			return entity.PageResponse{Rows: &staticRows{}}, nil
		},
		LookupFunc: func(_ map[string]string, _ []string) (any, error) {
			return map[string]any{"filters": map[string]any{}}, nil
		},
	}
	service := &RPCService{Name: "api", Operations: []RPCOperation{op}}
	server := &SwaggerServer{
		config:       &ServeConfig{Executor: &ExecutorConfig{Enabled: true}},
		converterCfg: &Config{PathPrefix: "/api/v1"},
		executor:     NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"}),
	}

	rec := httptest.NewRecorder()
	server.handleExecuteCommand(rec, httptest.NewRequest("GET", "/api/v1/config?__lookup=filters", nil))

	assert.Equal(t, "application/json+clicky", rec.Result().Header.Get("Content-Type"))
}
