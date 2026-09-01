package rpc

// Paged operations supply rows instead of a formatted body and leave the shared
// transport responsible for negotiation, headers, streaming, and downloads.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/formatters"
)

// pagedFlushEvery is how often a streaming export pushes its buffer at the
// client. Flushing every row costs a syscall — and, behind Compress, a gzip sync
// marker that measurably worsens the ratio — for a row nobody sees any sooner;
// flushing never turns a stream back into the wait it exists to avoid. At a few
// hundred bytes a row this is tens of kilobytes per flush: enough work per
// syscall to be worth making, little enough that a slow backend still shows
// progress.
const pagedFlushEvery = 256

// streamErrorTrailer carries the reason an export stopped early. It is a trailer
// because the failure it reports happens after the status line is already gone.
const streamErrorTrailer = "X-Stream-Error"

// handlePagedCommand answers a request whose operation supplies rows rather than
// a formatted body. The operation says only what the rows are and what is true
// about them; everything below — negotiation, headers, the stream, the download
// name — is the same for every operation that opts in.
func (s *SwaggerServer) handlePagedCommand(w http.ResponseWriter, r *http.Request, op *RPCOperation, paged entity.PagedFunc, name string) {
	// Before the first thing that can fail: an error a browser is not allowed to
	// read is worse than the error it describes.
	entity.SetCORSHeaders(w)

	pageReq, err := entity.ParsePageRequest(r, entity.PageLimits{})
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	flags, err := s.requestFlags(r, op)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	res, err := paged(r.Context(), pageReq, flags)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if res.Rows == nil {
		s.writeError(w, r, entity.NewStatusError(http.StatusInternalServerError, "invalid_response",
			"the operation returned no rows to read"))
		return
	}

	rows := peekRows(res.Rows)
	// Every path from here closes, including the panic below: the iterator holds
	// a cursor or a connection that nothing else will release.
	defer rows.Close() //nolint:errcheck
	if err := rows.PeekErr(); err != nil {
		s.writeError(w, r, entity.NewStatusErrorf(http.StatusInternalServerError, "query_failed", "%v", err))
		return
	}
	if err := streamableExport(pageReq.Format, res.Ceiling); err != nil {
		s.writeError(w, r, err)
		return
	}

	entity.SetExportHeaders(w, name, pageReq, res)

	if strings.EqualFold(r.Method, http.MethodHead) {
		// Every header is resolved and a HEAD asks for nothing else, so the walk
		// is not driven just to throw its bytes away — that would charge the
		// caller the whole query for a body it did not request. WriteHeader is
		// called explicitly rather than left to net/http, which would write the
		// status past any wrapper and answer without the Content-Encoding this
		// request's GET would carry.
		w.WriteHeader(http.StatusOK)
		return
	}

	// X-Stream-Error rides along only where a trailer section is already being
	// paid for. Declaring one of its own would cost every page its Content-Length
	// and force chunked encoding — the exact price DeclaresTruncatedTrailer
	// exists to avoid — to carry a reason that, on a real abort, Go never gets to
	// write anyway: the connection is dropped first. The abort is the signal; the
	// trailer is a note for the in-process callers that can still read one.
	if entity.DeclaresTruncatedTrailer(pageReq, res) {
		w.Header().Add("Trailer", streamErrorTrailer)
	}
	s.streamExport(w, r, rows, pageReq, res)
}

// streamExport writes the body and the answers that only reading it produces.
func (s *SwaggerServer) streamExport(w http.ResponseWriter, r *http.Request, rows *peekedRows, req entity.PageRequest, res entity.PageResponse) {
	trailers := entity.DeclaresTruncatedTrailer(req, res)
	bounded := &boundedRows{RowIterator: rows, limit: res.Ceiling}
	var source formatters.RowIterator = rows
	if res.Ceiling > 0 {
		source = bounded
	}

	_, err := formatters.WriteTableStream(r.Context(), w, source, formatters.StreamOptions{
		Format:     req.Format,
		MaxRows:    int64(res.Ceiling),
		CSVBOM:     req.Download,
		FlushEvery: pagedFlushEvery,
	})
	if err != nil {
		// The status line and the headers are gone and the body cannot be
		// recalled, so the only remaining way to say the export is incomplete is
		// to end the transfer wrong. curl -f and every browser surface an aborted
		// transfer; a short body under a 200 they read as the whole answer.
		// Compress re-panics with this value after finishing its member, so the
		// connection still breaks when the response was compressed. The reason is
		// recorded only where it was declared; setting an undeclared trailer would
		// be dropped on the wire and misread everywhere else as one that was sent.
		if trailers {
			w.Header().Set(streamErrorTrailer, s.clientErrorMessage(err))
		}
		panic(http.ErrAbortHandler)
	}
	if trailers {
		w.Header().Set("X-Truncated", strconv.FormatBool(bounded.truncated))
	}
}

// streamableExport refuses, before any header is committed, the two shapes
// WriteTableStream cannot even start on: a format it does not write rows in, and
// a PDF with no ceiling to bound the document it has to buffer. Left to the
// stream both would surface as an aborted transfer, which is reserved for a
// failure that genuinely arrived too late to be a status.
func streamableExport(format string, ceiling int) error {
	switch format {
	case "json", "ndjson", "yaml", "csv", "markdown", "html", "excel":
		return nil
	case "pdf":
		if ceiling <= 0 {
			return entity.NewStatusError(http.StatusInternalServerError, "unbounded_pdf",
				"a pdf export has to state the ceiling it is bounded by")
		}
		return nil
	default:
		return entity.NewStatusErrorf(http.StatusNotAcceptable, "not_acceptable",
			"%s is not a representation this operation can stream as rows", format)
	}
}

// requestFlags reads the operation's declared parameters and then forwards every
// undeclared query parameter alongside them, exactly as handleLookupCommand
// does: an entity resolved at runtime carries filters the operation was
// generated without, and dropping them would silently ignore what the caller
// asked to filter on. The transport's own parameters travel with them — an
// operation that declares a parameter named "limit" means it.
func (s *SwaggerServer) requestFlags(r *http.Request, op *RPCOperation) (map[string]string, error) {
	req, err := s.executor.ExtractRequestFromHTTP(r, op)
	if err != nil {
		return nil, entity.NewStatusErrorf(http.StatusBadRequest, "invalid_parameters", "%v", err)
	}
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "__") || len(values) == 0 {
			continue
		}
		if _, declared := req.Flags[key]; !declared {
			req.Flags[key] = values[0]
		}
	}
	return req.Flags, nil
}

// exportName is the base a download filename is derived from.
func exportName(op *RPCOperation) string {
	if op.Clicky != nil && op.Clicky.Entity != "" {
		return op.Clicky.Entity
	}
	if op.Name != "" {
		return strings.ReplaceAll(op.Name, " ", "-")
	}
	return "export"
}

// pagedOperation resolves the paged operation a request addresses, or nil when
// it addresses none. A HEAD has no operation of its own — it asks for the
// headers the GET beside it would answer with — so it resolves against that one.
func (s *SwaggerServer) pagedOperation(r *http.Request, op *RPCOperation) *RPCOperation {
	if op == nil && strings.EqualFold(r.Method, http.MethodHead) {
		op = s.executor.FindOperation(http.MethodGet, r.URL.EscapedPath())
	}
	if op == nil || op.PagedFunc == nil {
		return nil
	}
	return op
}

// explicitLookup reports a request that asked for filter metadata by name.
// isLookupRequest also reads a bare HEAD as a lookup, which a paged operation
// answers with its export headers instead.
func explicitLookup(r *http.Request) bool {
	return r.URL.Query().Get("__lookup") == "filters"
}

// peekedRows reads the first row eagerly and replays it into the stream.
//
// The first row is the last moment a backend failure is still reportable as one.
// Past it the status is committed, so a query that dies on its first read
// otherwise produces a short, well-formed CSV under a 200 that no client can
// tell apart from an empty result. Pulling the row here moves that failure back
// in front of the headers; replaying it keeps the row the caller asked for,
// which is why this is a wrapper and not a discarded read.
//
// Close is idempotent because two owners call it: WriteTableStream closes the
// iterator it is handed, and the handler closes it on every path that never
// reaches WriteTableStream.
type peekedRows struct {
	formatters.RowIterator
	peeked    map[string]any
	pending   bool
	replaying bool
	peekErr   error
	closed    bool
	closeErr  error
}

func peekRows(rows formatters.RowIterator) *peekedRows {
	peeked := &peekedRows{RowIterator: rows}
	if rows.Next() {
		peeked.peeked, peeked.pending = rows.Row(), true
		return peeked
	}
	peeked.peekErr = rows.Err()
	return peeked
}

// PeekErr is what the first read failed with, if it failed. It is distinct from
// Err, which also reports a failure found later in the walk — by which time the
// status can no longer be changed.
func (p *peekedRows) PeekErr() error { return p.peekErr }

func (p *peekedRows) Next() bool {
	if p.pending {
		p.pending, p.replaying = false, true
		return true
	}
	p.replaying = false
	return p.RowIterator.Next()
}

func (p *peekedRows) Row() map[string]any {
	if p.replaying {
		return p.peeked
	}
	return p.RowIterator.Row()
}

func (p *peekedRows) Err() error {
	if p.peekErr != nil {
		return p.peekErr
	}
	return p.RowIterator.Err()
}

func (p *peekedRows) Close() error {
	if p.closed {
		return p.closeErr
	}
	p.closed, p.closeErr = true, p.RowIterator.Close()
	return p.closeErr
}

// boundedRows stops a walk at the export's ceiling and reports whether stopping
// cut anything off.
//
// The cut is discovered by asking for one row past the ceiling, which is the
// only way to tell a result that ended exactly on it from one that was
// truncated — and X-Truncated is a trailer precisely because that answer does
// not exist until the walk is over.
type boundedRows struct {
	formatters.RowIterator
	limit     int
	delivered int
	truncated bool
}

func (b *boundedRows) Next() bool {
	if b.delivered >= b.limit {
		b.truncated = b.RowIterator.Next()
		return false
	}
	if !b.RowIterator.Next() {
		return false
	}
	b.delivered++
	return true
}
