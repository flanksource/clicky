package entity

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/formatters"
)

// PagedFunc supplies one page or one export of rows.
//
// It is preferred over ContextDataFunc when set, and the division is the point:
// the operation answers only what the rows are and what is true about them —
// how many exist, whether more can be asked for, where to resume — while clicky
// owns the response. Content negotiation, every header below, streaming and the
// download name are therefore identical across every operation that opts in,
// rather than re-derived per command.
type PagedFunc func(ctx context.Context, req PageRequest, flags map[string]string) (PageResponse, error)

// Export scopes. A request either asks for one page or for everything up to the
// operation's ceiling; there is no third answer, so anything else is refused
// rather than quietly treated as a page.
const (
	ScopePage = "page"
	ScopeAll  = "all"
)

// Export modes describe how the rows were produced, which is orthogonal to the
// scope that was asked for: a scope=page request against an operation that has
// to run its whole pipeline before any row is correct reports ModeBuffered.
const (
	ModePage      = "page"
	ModeBuffered  = "buffered"
	ModeStreaming = "streaming"
)

// DefaultExportFormat is what a caller that expressed no preference gets.
const DefaultExportFormat = "json"

// These are the library's own fallbacks, applied only when a caller states no
// PageLimits. An application with a view on how much its backends can serve is
// expected to pass its own; nothing here is claimed to match any consumer's.
const (
	// DefaultPageSize is the page an operation serves when the caller names none.
	DefaultPageSize = 100
	// DefaultMaxPageSize is the largest single page a caller may ask for. Past
	// this, the answer is scope=all rather than a bigger page.
	DefaultMaxPageSize = 1000
)

// PageLimits bound what one request may ask for. A zero value resolves to the
// defaults, so a caller with no opinion does not have to state one.
type PageLimits struct {
	DefaultPageSize int
	MaxPageSize     int
}

func (l PageLimits) resolve() PageLimits {
	if l.DefaultPageSize <= 0 {
		l.DefaultPageSize = DefaultPageSize
	}
	if l.MaxPageSize <= 0 {
		l.MaxPageSize = DefaultMaxPageSize
	}
	if l.DefaultPageSize > l.MaxPageSize {
		l.DefaultPageSize = l.MaxPageSize
	}
	return l
}

// PageRequest is one export: which rows, in what shape, and how far.
type PageRequest struct {
	// Scope is ScopePage or ScopeAll.
	Scope string
	// Format is the resolved export format, after ?format and Accept have been
	// reconciled — the operation never has to look at either again.
	Format string
	// Cursor is an opaque position handed back by a previous page.
	Cursor string
	// Filename overrides the download name. Empty means derive one.
	Filename string

	Limit  int
	Offset int

	// Download asks for a Content-Disposition even when no filename was named.
	Download bool

	// SkipTotal waives the exact total. A backend asked for the size of the whole
	// result has to produce the whole result first, which is the difference
	// between an export that streams and one that hangs until the last row is
	// found — so an all-rows export declines to ask.
	SkipTotal bool
}

// Total is a result-set size together with how much the backend was willing to
// promise about it.
type Total struct {
	Value int64
	Exact bool
}

// Relation reports how X-Total-Count should be read.
//
// A nil Total is the third answer and the reason this is a pointer method: a
// backend that states no total at all is different from one that reports zero,
// and without a relation those two are the same absent header.
func (t *Total) Relation() string {
	switch {
	case t == nil:
		return "unknown"
	case t.Exact:
		return "eq"
	default:
		return "gte"
	}
}

// PageResponse is everything the transport reports about the rows it is about
// to write. Every field is resolved before the first byte of the body, because
// a header cannot be corrected once the body has begun.
//
// It is unrelated to PagedResult[T], the buffered JSON envelope: this one is
// what clicky turns into an HTTP response, and its rows are still unread.
type PageResponse struct {
	Rows formatters.RowIterator

	// Mode is ModePage, ModeBuffered or ModeStreaming.
	Mode string

	// Total is nil when the backend states no size for the whole result.
	Total   *Total
	HasMore bool
	// Next is the opaque position resuming after this page, when the provider
	// issued one.
	Next string

	// Ceiling stops an all-rows export. Zero means the rows are already bounded,
	// and therefore that nothing is left to discover by reading to the end.
	Ceiling int

	// MaxRows is the ceiling this export was bounded by, reported so a caller
	// sees the same number the server enforced.
	MaxRows int

	// Truncated reports a cut this response already knows about — the operation's
	// own size, or a cap the backend applied. A ceiling is not known to have
	// bitten until the rows have been read, so that case is reported as a trailer.
	Truncated bool

	// Pageable reports whether the operation can serve a position past its first
	// page. A response that cannot must not report one as available.
	Pageable bool
}

// exportHeader is one paging fact a response carries, described once.
//
// Three consumers read this list and none of them may disagree: the
// Access-Control-Expose-Headers value (a header a browser cannot read is a
// header that does not exist), the OpenAPI response documentation (a contract
// only the first-party UI knows is a contract no generated client can honour),
// and the setters below.
type exportHeader struct {
	name        string
	description string
	kind        string
}

var exportHeaderSpecs = []exportHeader{
	{"Content-Disposition", "Attachment filename; present when ?_download or ?filename is given", "string"},
	{"X-Export-Mode", "How the rows were produced: page, buffered or streaming. Orthogonal to scope — a scope=page request against an operation that cannot stream reports buffered", "string"},
	{"X-Page-Limit", "Rows this page was limited to. Present when scope=page", "integer"},
	{"X-Page-Offset", "Rows skipped before this page. Present when scope=page and the operation can be paged", "integer"},
	{"X-Total-Count", "Size of the whole result set. Absent when the backend does not report one — read X-Total-Relation to tell that from zero", "integer"},
	{"X-Total-Relation", "How to read X-Total-Count: eq (exact), gte (lower bound), or unknown (the backend states no total)", "string"},
	{"X-Has-More", "Whether a further page can be requested. False for an operation with no total order, which serves its first page and refuses every page after it", "boolean"},
	{"X-Next-Cursor", "Opaque position resuming after this page. Present when scope=page and the provider issued one", "string"},
	{"X-Truncated", "The rows were cut short. Present when scope=all and the cut is known before the body; otherwise sent as the declared trailer", "boolean"},
	{"X-Max-Rows", "Ceiling this export was bounded by. Present when scope=all; a PDF reports its own lower ceiling", "integer"},
}

// ExportHeaderNames are the header names, for Access-Control-Expose-Headers.
func ExportHeaderNames() []string {
	names := make([]string, 0, len(exportHeaderSpecs))
	for _, spec := range exportHeaderSpecs {
		names = append(names, spec.name)
	}
	return names
}

// ExportHeaderDoc documents one response header for the OpenAPI spec. It is
// deliberately not the rpc package's own header type: rpc imports entity, so
// the description travels as plain data and rpc adapts it.
type ExportHeaderDoc struct {
	Description string
	Type        string
}

// ExportResponseHeaders documents the same headers for the OpenAPI response.
func ExportResponseHeaders() map[string]ExportHeaderDoc {
	headers := make(map[string]ExportHeaderDoc, len(exportHeaderSpecs))
	for _, spec := range exportHeaderSpecs {
		headers[spec.name] = ExportHeaderDoc{Description: spec.description, Type: spec.kind}
	}
	return headers
}

// SetCORSHeaders permits a cross-origin caller to read this response, whatever
// it turns out to be.
//
// It is deliberately independent of the response: none of it depends on the
// rows, so it is set before the first thing that can fail. Setting it only on
// success is what makes an error body unreadable in a browser — and an error
// nobody can read is worse than the one it describes.
func SetCORSHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Expose-Headers", strings.Join(ExportHeaderNames(), ", "))
}

// DeclaresTruncatedTrailer reports whether this response has to declare
// Trailer: X-Truncated, and so whether it is a response that ends with trailers
// at all.
//
// Only a ceiling that might still bite needs one. A buffered export has none to
// hit, and one already known to have been cut has its answer in the headers —
// declaring a trailer for either costs the response its Content-Length, forces
// chunked encoding, and promises an answer that never comes.
//
// A trailer is a poor last resort: browsers do not expose them, so this reaches
// CLI and library callers only. It is kept for the one case that is genuinely
// unknowable up front — a stream whose backend states no total — rather than as
// the primary channel.
//
// It is exported because the transport has to know the same thing to send the
// trailer it declares, and to know whether anything else may ride along in the
// trailer section it has already paid for. Two copies of this condition is the
// worst outcome available: a declared trailer that never arrives leaves a client
// waiting, and an undeclared one is dropped on the floor.
func DeclaresTruncatedTrailer(req PageRequest, res PageResponse) bool {
	return req.Scope != ScopePage && res.Ceiling > 0 && !res.Truncated
}

// SetExportHeaders writes every paging fact this response carries. It must be
// called before the first byte of the body.
func SetExportHeaders(w http.ResponseWriter, name string, req PageRequest, res PageResponse) {
	header := w.Header()
	SetCORSHeaders(w)
	header.Set("Content-Type", ExportContentType(req.Format))
	header.Set("X-Export-Mode", res.Mode)

	if req.Scope == ScopePage {
		header.Set("X-Page-Limit", strconv.Itoa(req.Limit))
		// An operation with no total order serves its first page and refuses every
		// page after it. Reporting rows behind that page would be true and
		// useless: the only request it invites is one this server answers 400.
		// Offset is withheld for the same reason — it names a position that
		// cannot be asked for.
		if res.Pageable {
			header.Set("X-Page-Offset", strconv.Itoa(req.Offset))
			header.Set("X-Has-More", strconv.FormatBool(res.HasMore))
			if res.Next != "" {
				header.Set("X-Next-Cursor", res.Next)
			}
		} else {
			header.Set("X-Has-More", "false")
		}
	} else {
		header.Set("X-Max-Rows", strconv.Itoa(res.MaxRows))
		if DeclaresTruncatedTrailer(req, res) {
			header.Set("Trailer", "X-Truncated")
		}
	}

	// A total the backend could not state exactly is a lower bound, and a caller
	// rendering it as a count would be reporting a number nobody promised. No
	// total at all is a third answer: without it, a missing X-Total-Count reads
	// the same as a zero one, so the relation is always stated.
	header.Set("X-Total-Relation", res.Total.Relation())
	if res.Total != nil {
		header.Set("X-Total-Count", strconv.FormatInt(res.Total.Value, 10))
	}
	if res.Truncated {
		header.Set("X-Truncated", "true")
	}

	if req.Download || req.Filename != "" {
		filename := req.Filename
		if filename == "" {
			filename = name + ExportExtension(req.Format)
		}
		header.Set("Content-Disposition", ContentDisposition(filename))
	}
}

// ParsePageRequest reads the transport half of a request: which rows, in what
// shape, and how far. Every failure is a StatusError so the caller writes one
// error shape rather than choosing between two.
func ParsePageRequest(r *http.Request, limits PageLimits) (PageRequest, error) {
	limits = limits.resolve()
	query := r.URL.Query()

	format, err := RequestedFormat(r)
	if err != nil {
		return PageRequest{}, err
	}
	request := PageRequest{
		Format:   format,
		Scope:    query.Get("scope"),
		Limit:    limits.DefaultPageSize,
		Cursor:   query.Get("cursor"),
		Filename: query.Get("filename"),
		Download: query.Has("_download"),
	}
	if request.Scope == "" {
		request.Scope = ScopePage
	}
	if request.Scope != ScopePage && request.Scope != ScopeAll {
		return request, NewStatusErrorf(http.StatusBadRequest, "invalid_scope", "invalid export scope %q", request.Scope)
	}
	if value := query.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit <= 0 || limit > limits.MaxPageSize {
			return request, NewStatusErrorf(http.StatusBadRequest, "invalid_limit",
				"limit must be between 1 and %d; export more with scope=all", limits.MaxPageSize)
		}
		request.Limit = limit
	}
	if value := query.Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return request, NewStatusError(http.StatusBadRequest, "invalid_offset", "offset must be zero or greater")
		}
		request.Offset = offset
	}
	if request.Scope == ScopeAll {
		// An export reads forward to the ceiling and never jumps, so a position
		// within the result is meaningless to it. It also waives the exact total:
		// see PageRequest.SkipTotal.
		request.Offset = 0
		request.Cursor = ""
		request.SkipTotal = true
	}
	if request.Cursor != "" && request.Offset != 0 {
		return request, NewStatusError(http.StatusBadRequest, "invalid_cursor",
			"a cursor already says where to resume, so it cannot be combined with an offset")
	}
	return request, nil
}
