package entity

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// headersFor runs the setter over one scenario and returns what it wrote.
func headersFor(name string, req PageRequest, res PageResponse) http.Header {
	w := httptest.NewRecorder()
	SetExportHeaders(w, name, req, res)
	return w.Header()
}

var _ = Describe("Total relation", func() {
	DescribeTable("distinguishes an exact count, a lower bound and no count at all",
		func(total *Total, expected string) {
			Expect(total.Relation()).To(Equal(expected))
		},
		Entry("a backend that states no total", (*Total)(nil), "unknown"),
		Entry("an exact count", &Total{Value: 42, Exact: true}, "eq"),
		Entry("a zero count is still exact", &Total{Value: 0, Exact: true}, "eq"),
		Entry("a lower bound", &Total{Value: 1000, Exact: false}, "gte"),
	)
})

var _ = Describe("Export headers", func() {
	It("sets every header it documents", func() {
		// The specs are the contract three consumers read; a documented header
		// nobody sets is the disagreement this guards against.
		page := headersFor("report", PageRequest{
			Scope: ScopePage, Format: "csv", Limit: 25, Offset: 50, Download: true,
		}, PageResponse{
			Mode: ModePage, Pageable: true, HasMore: true, Next: "cursor-1",
			Total: &Total{Value: 500, Exact: true},
		})
		all := headersFor("report", PageRequest{Scope: ScopeAll, Format: "csv"}, PageResponse{
			Mode: ModeStreaming, MaxRows: 10000, Truncated: true,
		})

		for _, spec := range exportHeaderSpecs {
			Expect(page.Get(spec.name)+all.Get(spec.name)).ToNot(BeEmpty(),
				"%s is documented but never set", spec.name)
		}
	})

	It("exposes exactly the documented headers to a cross-origin caller", func() {
		w := httptest.NewRecorder()
		SetCORSHeaders(w)

		Expect(w.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		exposed := strings.Split(w.Header().Get("Access-Control-Expose-Headers"), ", ")
		Expect(exposed).To(Equal(ExportHeaderNames()))
	})

	It("documents every header for OpenAPI with a description and a type", func() {
		docs := ExportResponseHeaders()

		Expect(docs).To(HaveLen(len(exportHeaderSpecs)))
		for _, spec := range exportHeaderSpecs {
			Expect(docs[spec.name]).To(Equal(ExportHeaderDoc{Description: spec.description, Type: spec.kind}))
		}
	})

	It("always states the total relation so a missing count is not a zero one", func() {
		header := headersFor("report", PageRequest{Scope: ScopePage, Format: "json"},
			PageResponse{Mode: ModePage, Pageable: true})

		Expect(header.Get("X-Total-Relation")).To(Equal("unknown"))
		Expect(header).ToNot(HaveKey("X-Total-Count"))
	})

	It("reports a stated zero as a count rather than an absence", func() {
		header := headersFor("report", PageRequest{Scope: ScopePage, Format: "json"},
			PageResponse{Mode: ModePage, Pageable: true, Total: &Total{Value: 0, Exact: true}})

		Expect(header.Get("X-Total-Count")).To(Equal("0"))
		Expect(header.Get("X-Total-Relation")).To(Equal("eq"))
	})

	It("withholds a position that cannot be asked for when the rows are not pageable", func() {
		header := headersFor("report", PageRequest{Scope: ScopePage, Format: "json", Offset: 100}, PageResponse{
			Mode: ModePage, Pageable: false, HasMore: true, Next: "cursor-1",
		})

		Expect(header).ToNot(HaveKey("X-Page-Offset"))
		Expect(header).ToNot(HaveKey("X-Next-Cursor"))
		Expect(header.Get("X-Has-More")).To(Equal("false"))
		Expect(header.Get("X-Page-Limit")).ToNot(BeEmpty())
	})

	It("reports the position and the next cursor when the rows are pageable", func() {
		header := headersFor("report", PageRequest{Scope: ScopePage, Format: "json", Limit: 25, Offset: 100}, PageResponse{
			Mode: ModePage, Pageable: true, HasMore: true, Next: "cursor-1",
		})

		Expect(header.Get("X-Page-Limit")).To(Equal("25"))
		Expect(header.Get("X-Page-Offset")).To(Equal("100"))
		Expect(header.Get("X-Has-More")).To(Equal("true"))
		Expect(header.Get("X-Next-Cursor")).To(Equal("cursor-1"))
	})

	DescribeTable("declares the truncation trailer only when a ceiling might still bite",
		func(req PageRequest, res PageResponse, expectTrailer bool) {
			header := headersFor("report", req, res)
			if expectTrailer {
				Expect(header.Get("Trailer")).To(Equal("X-Truncated"))
			} else {
				Expect(header).ToNot(HaveKey("Trailer"))
			}
		},
		Entry("a stream whose ceiling has not been reached yet",
			PageRequest{Scope: ScopeAll, Format: "csv"},
			PageResponse{Mode: ModeStreaming, Ceiling: 10000, MaxRows: 10000}, true),
		Entry("a cut already known before the body needs no trailer",
			PageRequest{Scope: ScopeAll, Format: "csv"},
			PageResponse{Mode: ModeStreaming, Ceiling: 10000, MaxRows: 10000, Truncated: true}, false),
		Entry("a buffered export has no ceiling left to hit",
			PageRequest{Scope: ScopeAll, Format: "csv"},
			PageResponse{Mode: ModeBuffered, MaxRows: 10000}, false),
		Entry("a page is bounded by its own limit",
			PageRequest{Scope: ScopePage, Format: "csv", Limit: 25},
			PageResponse{Mode: ModePage, Pageable: true, Ceiling: 10000}, false),
	)

	It("reports a known cut as a header rather than a promise of one", func() {
		header := headersFor("report", PageRequest{Scope: ScopeAll, Format: "csv"},
			PageResponse{Mode: ModeStreaming, Ceiling: 10000, MaxRows: 10000, Truncated: true})

		Expect(header.Get("X-Truncated")).To(Equal("true"))
		Expect(header.Get("X-Max-Rows")).To(Equal("10000"))
	})

	DescribeTable("attaches a download name only when one was asked for",
		func(req PageRequest, expected string) {
			Expect(headersFor("invoices", req, PageResponse{Mode: ModePage}).Get("Content-Disposition")).To(Equal(expected))
		},
		Entry("no download and no filename", PageRequest{Scope: ScopePage, Format: "csv"}, ""),
		Entry("_download derives the name from the operation",
			PageRequest{Scope: ScopePage, Format: "csv", Download: true},
			`attachment; filename="invoices.csv"; filename*=UTF-8''invoices.csv`),
		Entry("an explicit filename wins",
			PageRequest{Scope: ScopePage, Format: "csv", Filename: "q1.csv"},
			`attachment; filename="q1.csv"; filename*=UTF-8''q1.csv`),
	)
})

var _ = Describe("Content-Disposition encoding", func() {
	It("carries a non-ASCII name as an RFC 8187 ext-value with an ASCII fallback", func() {
		// 報告書 is E5A0B1 E5918A E69BB8 in UTF-8; the space is %20 and the dot is
		// an attr-char, so only the three ideographs and the space are encoded.
		Expect(ContentDisposition("報告書 2024.csv")).To(Equal(
			`attachment; filename="___ 2024.csv"; filename*=UTF-8''%E5%A0%B1%E5%91%8A%E6%9B%B8%202024.csv`))
	})

	It("strips a path so the name can only ever be a filename", func() {
		Expect(ContentDisposition(`../../etc/passwd`)).To(Equal(
			`attachment; filename="passwd"; filename*=UTF-8''passwd`))
	})

	DescribeTable("neutralises what would close the header parameter",
		func(filename, expected string) {
			Expect(SanitizeExportFilename(filename)).To(Equal(expected))
		},
		Entry("a quote", `rep"ort.csv`, "rep_ort.csv"),
		Entry("a semicolon", `rep;ort.csv`, "rep_ort.csv"),
		Entry("a control character", "rep\x00ort.csv", "rep_ort.csv"),
		Entry("a windows path", `C:\tmp\report.csv`, "report.csv"),
		Entry("nothing usable left", " . ", "export.json"),
	)

	It("substitutes a usable name when the fallback would be all placeholders", func() {
		Expect(ContentDisposition("報告書")).To(Equal(
			`attachment; filename="export.json"; filename*=UTF-8''%E5%A0%B1%E5%91%8A%E6%9B%B8`))
	})
})

var _ = Describe("Accept negotiation", func() {
	DescribeTable("picks the highest-weighted representation the caller will take",
		func(accept, expected string) {
			format, err := AcceptedFormat(accept)
			Expect(err).ToNot(HaveOccurred())
			Expect(format).To(Equal(expected))
		},
		Entry("no preference falls back to the default", "", DefaultExportFormat),
		Entry("an unrecognised type falls back to the default", "text/plain", DefaultExportFormat),
		Entry("a wildcard accepts anything, so the default is acceptable", "*/*", DefaultExportFormat),
		Entry("a named range overrides a wildcard refusal", "*/*;q=0, text/csv", "csv"),
		Entry("a browser's ranked wildcard does not outrank a type it named",
			"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "html"),
		Entry("weight beats position", "text/html;q=0.1, application/clicky+json;q=0.9", "clicky-json"),
		Entry("a tie keeps the order the caller wrote", "text/csv, text/markdown", "csv"),
		Entry("an unweighted entry is q=1", "text/csv;q=0.9, text/markdown", "markdown"),
		Entry("a refusal of one format still allows another", "application/json;q=0, text/csv", "csv"),
		Entry("refusing a format the default is not still yields the default", "text/csv;q=0", DefaultExportFormat),
	)

	DescribeTable("treats q=0 as a refusal rather than a low preference",
		func(accept string) {
			format, err := AcceptedFormat(accept)

			Expect(format).To(BeEmpty())
			statusErr, ok := err.(*StatusError)
			Expect(ok).To(BeTrue(), "expected a StatusError, got %T", err)
			Expect(statusErr.Status).To(Equal(http.StatusNotAcceptable))
			Expect(statusErr.Code).To(Equal("not_acceptable"))
		},
		Entry("the default refused outright", "application/json;q=0"),
		Entry("the default refused with nothing recognised alongside it", "application/json;q=0, text/plain"),
		Entry("the default refused while another format is also refused", "application/json;q=0, text/csv;q=0"),
		Entry("a wildcard refusal leaves nothing acceptable at all", "*/*;q=0"),
		Entry("a wildcard refusal alongside an unrecognised type", "*/*;q=0, text/plain"),
	)
})

var _ = Describe("Export format mapping", func() {
	DescribeTable("maps a format to the content type and extension a client expects",
		func(format, contentType, extension string, tabular bool) {
			Expect(ExportContentType(format)).To(Equal(contentType))
			Expect(ExportExtension(format)).To(Equal(extension))
			Expect(IsTabularExport(format)).To(Equal(tabular))
		},
		Entry("json", "json", "application/json", ".json", false),
		Entry("ndjson", "ndjson", "application/x-ndjson", ".ndjson", false),
		Entry("yaml", "yaml", "application/yaml", ".yaml", false),
		Entry("csv", "csv", "text/csv; charset=utf-8", ".csv", true),
		Entry("markdown", "markdown", "text/markdown; charset=utf-8", ".md", true),
		Entry("excel", "excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx", true),
		Entry("pdf", "pdf", "application/pdf", ".pdf", true),
		Entry("an unknown format", "brotli", "application/octet-stream", ".brotli", false),
	)
})

var _ = Describe("ParsePageRequest", func() {
	parse := func(rawQuery, accept string, limits PageLimits) (PageRequest, error) {
		r := &http.Request{URL: &url.URL{RawQuery: rawQuery}, Header: http.Header{}}
		if accept != "" {
			r.Header.Set("Accept", accept)
		}
		return ParsePageRequest(r, limits)
	}

	It("defaults to the first page in the default format", func() {
		req, err := parse("", "", PageLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(req).To(Equal(PageRequest{
			Scope: ScopePage, Format: DefaultExportFormat, Limit: DefaultPageSize,
		}))
	})

	It("resolves the format from Accept when none was named", func() {
		req, err := parse("", "text/csv", PageLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(req.Format).To(Equal("csv"))
	})

	It("prefers an explicit format over Accept and canonicalises its alias", func() {
		req, err := parse("format=xlsx", "text/csv", PageLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(req.Format).To(Equal("excel"))
	})

	It("waives the total and the position for an all-rows export", func() {
		req, err := parse("scope=all&offset=100&cursor=abc", "", PageLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(req.Scope).To(Equal(ScopeAll))
		Expect(req.Offset).To(BeZero())
		Expect(req.Cursor).To(BeEmpty())
		Expect(req.SkipTotal).To(BeTrue())
	})

	It("reads the download intent from either _download or filename", func() {
		req, err := parse("_download&filename=q1.csv", "", PageLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(req.Download).To(BeTrue())
		Expect(req.Filename).To(Equal("q1.csv"))
	})

	DescribeTable("refuses a request it cannot serve",
		func(rawQuery, accept, code string, status int) {
			_, err := parse(rawQuery, accept, PageLimits{MaxPageSize: 500})

			Expect(err).To(HaveOccurred())
			statusErr, ok := err.(*StatusError)
			Expect(ok).To(BeTrue(), "expected a StatusError, got %T", err)
			Expect(statusErr.Code).To(Equal(code))
			Expect(statusErr.Status).To(Equal(status))
		},
		Entry("an unknown scope", "scope=everything", "", "invalid_scope", http.StatusBadRequest),
		Entry("an unsupported format", "format=brotli", "", "unsupported_format", http.StatusBadRequest),
		Entry("a refused representation", "", "application/json;q=0", "not_acceptable", http.StatusNotAcceptable),
		Entry("a limit past the maximum", "limit=501", "", "invalid_limit", http.StatusBadRequest),
		Entry("a limit that is not a number", "limit=many", "", "invalid_limit", http.StatusBadRequest),
		Entry("a negative offset", "offset=-1", "", "invalid_offset", http.StatusBadRequest),
		Entry("a cursor combined with an offset", "cursor=abc&offset=10", "", "invalid_cursor", http.StatusBadRequest),
	)
})
