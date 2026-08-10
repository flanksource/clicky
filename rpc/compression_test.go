package rpc

import (
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressGzipsWhatTheCallerAccepts(t *testing.T) {
	// Repetitive by design, like the rendered table format this exists for.
	body := strings.Repeat(`{"kind":"text","plain":"idle","text":"idle"},`, 500)

	for _, tc := range []struct {
		name        string
		accept      string
		contentType string
		wantGzip    bool
	}{
		{"json is compressed", "gzip", "application/json", true},
		{"the clicky envelope is compressed", "gzip, deflate", "application/json+clicky", true},
		{"ndjson is compressed", "gzip", "application/x-ndjson", true},
		{"csv is compressed", "gzip", "text/csv; charset=utf-8", true},
		{"a caller that does not ask is not given", "", "application/json", false},
		{"gzip;q=0 is a refusal", "gzip;q=0", "application/json", false},
		{"a pdf is already compressed", "gzip", "application/pdf", false},
		{"a spreadsheet is already compressed", "gzip", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", false},
		{"an event stream is left alone", "gzip", "text/event-stream", false},
		// Go sniffs an unset type from the first bytes written; compressing
		// would make those bytes the gzip header and relabel the response.
		{"an unset content type is left alone", "gzip", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				_, _ = io.WriteString(w, body)
			}))

			request := httptest.NewRequest(http.MethodGet, "/api/v1/profile/anything", nil)
			if tc.accept != "" {
				request.Header.Set("Accept-Encoding", tc.accept)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, request)

			encoding := rec.Header().Get("Content-Encoding")
			if tc.wantGzip != (encoding == "gzip") {
				t.Fatalf("Content-Encoding=%q for %s, wantGzip=%v", encoding, tc.contentType, tc.wantGzip)
			}
			// The same URL answers differently per Accept-Encoding, so a cache
			// that ignores it would serve one client the other's response.
			if vary := rec.Header().Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
				t.Fatalf("Vary=%q, want it to name Accept-Encoding", vary)
			}

			if !tc.wantGzip {
				if got := rec.Body.String(); got != body {
					t.Fatalf("uncompressed body changed: %d bytes, want %d", len(got), len(body))
				}
				return
			}
			if rec.Header().Get("Content-Length") != "" {
				t.Fatal("Content-Length survived compression, so it describes the wrong body")
			}
			reader, err := gzip.NewReader(rec.Body)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if string(decoded) != body {
				t.Fatalf("round trip changed the body: %d bytes, want %d", len(decoded), len(body))
			}
			if rec.Body.Len() >= len(body) {
				t.Fatalf("compressed to %d bytes from %d, which is no saving", rec.Body.Len(), len(body))
			}
		})
	}
}

// An export writes rows as it reads them. If compressing turned that into one
// buffer that only drained at the end, a long export would look like a hang.
//
// The handler here does the flushing, so what this pins down is that the
// wrapper forwards a Flush instead of swallowing it — not that any real
// endpoint flushes. Nothing fails here if a streaming handler forgets to.
func TestCompressPassesFlushThrough(t *testing.T) {
	flushed := make(chan struct{}, 1)
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, `{"id":1}`+"\n")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("compressing writer is not an http.Flusher")
			return
		}
		flusher.Flush()
		flushed <- struct{}{}
		_, _ = io.WriteString(w, `{"id":2}`+"\n")
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile/anything?scope=all", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)

	select {
	case <-flushed:
	default:
		t.Fatal("handler never reached its flush")
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":1}` + "\n" + `{"id":2}` + "\n"; string(decoded) != want {
		t.Fatalf("streamed body = %q, want %q", decoded, want)
	}
}

// A 204 has no body to compress, and declaring an encoding for one would
// describe content that does not exist.
func TestCompressLeavesEmptyResponsesAlone(t *testing.T) {
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/profile/anything", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if encoding := rec.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("Content-Encoding=%q on a bodiless response", encoding)
	}
}

// Accept-Encoding must be named in Vary exactly once. A second copy is not a
// stronger instruction — it is a header every cache in the path has to
// normalise, and one that makes an otherwise identical response differ.
func TestCompressDeclaresVaryOnce(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing []string
		want     []string
	}{
		{"nothing declared yet", nil, []string{"Accept-Encoding"}},
		{"already declared", []string{"Accept-Encoding"}, []string{"Accept-Encoding"}},
		// Field names are case-insensitive, so a lowercase token is the same
		// declaration and adding another would only duplicate it.
		{"declared in another case", []string{"accept-encoding"}, []string{"accept-encoding"}},
		// A handler is free to pack several field names into one value.
		{"declared inside a list", []string{"Origin, accept-encoding"}, []string{"Origin, accept-encoding"}},
		{"declared as separate values", []string{"Origin", "Accept-Encoding"}, []string{"Origin", "Accept-Encoding"}},
		// Some other field varying the response says nothing about encoding.
		{"a different field is declared", []string{"Origin"}, []string{"Origin", "Accept-Encoding"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"rows":[]}`)
			}))

			request := httptest.NewRequest(http.MethodGet, "/api/v1/profile/anything", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			for _, value := range tc.existing {
				rec.Header().Add("Vary", value)
			}
			handler.ServeHTTP(rec, request)

			got := rec.Header().Values("Vary")
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("Vary=%q, want %q", got, tc.want)
			}
		})
	}
}

// flakyResponseWriter is a connection that dies on command. failNow is flipped
// by the handler once it has written everything it means to, which leaves the
// gzip footer — written when the wrapper is closed — as the only thing left to
// fail.
type flakyResponseWriter struct {
	*httptest.ResponseRecorder
	failNow bool
}

func (f *flakyResponseWriter) Write(body []byte) (int, error) {
	if f.failNow {
		return 0, errors.New("write: connection reset by peer")
	}
	return f.ResponseRecorder.Write(body)
}

// A gzip member whose footer never made it is truncated, and the status line
// promising a complete body left the server long before that could be known.
// Aborting the connection is the only signal a client can act on, so the
// middleware must not swallow the error and return a well-formed-looking 200.
func TestCompressAbortsWhenTheGzipFooterFails(t *testing.T) {
	writer := &flakyResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat(`{"kind":"text","plain":"idle"},`, 200))
		writer.failNow = true
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile/anything", nil)
	request.Header.Set("Accept-Encoding", "gzip")

	defer func() {
		// http.ErrAbortHandler specifically: net/http drops the connection
		// without a terminating chunk and without logging a stack trace, so
		// the client sees a transfer error rather than a short body.
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", recovered)
		}
	}()
	handler.ServeHTTP(writer, request)
	t.Fatal("a failed gzip Close was swallowed, leaving a truncated body under a 200")
}

// The nastiest case for the abort above: a handler panics with a real bug and
// the connection it was writing to is gone, so closing the member fails too.
// Raising ErrAbortHandler over the top of that would discard the bug — it is
// the one value net/http logs no stack for — and leave nothing to debug.
func TestCompressKeepsAnInFlightPanic(t *testing.T) {
	sentinel := errors.New("the handler's own bug")
	writer := &flakyResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat(`{"kind":"text","plain":"idle"},`, 200))
		writer.failNow = true
		panic(sentinel)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile/anything", nil)
	request.Header.Set("Accept-Encoding", "gzip")

	defer func() {
		if recovered := recover(); recovered != sentinel { //nolint:errorlint // the panic value must be the same one, not merely a match
			t.Fatalf("recovered %v, want the handler's own panic value", recovered)
		}
	}()
	handler.ServeHTTP(writer, request)
	t.Fatal("the handler's panic never made it out of the middleware")
}

// Re-panicking with the handler's own value has to leave the connection in the
// same state ErrAbortHandler would: net/http aborts on any unrecovered handler
// panic, so a client that already has the headers must see the response end
// without its terminating chunk rather than read a short body as complete.
func TestCompressPanicStillAbortsTheConnection(t *testing.T) {
	server := httptest.NewUnstartedServer(Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, strings.Repeat(`{"id":1}`+"\n", 200))
		// Puts the headers and some body on the wire, so the client is left
		// mid-response when the panic takes the connection out from under it.
		w.(http.Flusher).Flush()
		panic(errors.New("the handler's own bug"))
	})))
	// The panic is the point of the test; its stack trace is not the test's
	// output to own.
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.Start()
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := server.Client().Do(request)
	if err != nil {
		// The connection died before the headers arrived, which is the same
		// answer told earlier.
		return
	}
	defer response.Body.Close()

	if _, err := io.ReadAll(response.Body); err == nil {
		t.Fatal("the body read cleanly to EOF, so the client took a panicked response as complete")
	}
}

// The wrapper is closed after the handler returns but before net/http writes
// the trailers it declared. Closing later would put the gzip footer after the
// trailer — or never write it at all — and closing it inside the handler would
// end the member before the trailer was known. This runs over a real server
// because httptest.NewRecorder has no trailer or chunked framing to get wrong.
func TestCompressDeliversTrailerAfterTheCompressedBody(t *testing.T) {
	body := strings.Repeat(`{"kind":"text","plain":"idle","text":"idle"},`, 500)
	server := httptest.NewServer(Compress(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Trailer", "X-Truncated")
		_, _ = io.WriteString(w, body)
		// Only knowable once the rows have been written, which is the whole
		// reason it is a trailer and not a header.
		w.Header().Set("X-Truncated", "true")
	})))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Asking explicitly stops the transport from decompressing for us, so the
	// bytes asserted below are the ones that went over the wire.
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if encoding := response.Header.Get("Content-Encoding"); encoding != "gzip" {
		t.Fatalf("Content-Encoding=%q, want gzip", encoding)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		// An unexpected EOF here is the footer landing after the trailer, or
		// not landing at all.
		t.Fatalf("reading the compressed body: %v", err)
	}
	if string(decoded) != body {
		t.Fatalf("round trip changed the body: %d bytes, want %d", len(decoded), len(body))
	}
	// Trailers are only populated once the body has been read to EOF, so this
	// value existing proves it arrived after a complete gzip member.
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	if truncated := response.Trailer.Get("X-Truncated"); truncated != "true" {
		t.Fatalf("X-Truncated=%q after reading the body, want true", truncated)
	}
}
