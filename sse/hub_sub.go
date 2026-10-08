package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
)

// hubErrorBodyLimit caps the body carried in a __closed frame's "error".
const hubErrorBodyLimit = 500

// hubSubStopTimeout bounds how long a cancelled sub waits for its handler to
// return before abandoning it. A var so specs can shorten it.
var hubSubStopTimeout = 5 * time.Second

// hubForwardedHeaders are the subscribe request's headers a sub request
// inherits: identity and proxy context a handler may consult. Accept-Encoding
// is deliberately absent — a sub's body is parsed in-process, never
// compressed.
var hubForwardedHeaders = []string{
	"Authorization", "Cookie", "Origin", "Referer", "User-Agent",
	"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto",
}

// subRequest builds the GET request a sub serves, rejecting any path that is
// not a registered /api route of this server. The mux is the allowlist: a path
// is allowed when the server's own pattern matching resolves it to a pattern
// under /api/ (other than the hub's routes), so no route list is duplicated
// here and a stream added later is subscribable without touching the hub.
// The "/" SPA catch-all, method mismatches and unregistered paths all fail it.
func (h *Hub) subRequest(from *http.Request, rawPath string) (*http.Request, error) {
	target, err := url.Parse(rawPath)
	if err != nil {
		return nil, fmt.Errorf("sub path %q: %w", rawPath, err)
	}
	if target.Scheme != "" || target.Host != "" || target.Opaque != "" {
		return nil, fmt.Errorf("sub path %q must be a server-relative path", rawPath)
	}
	if clean := pathpkg.Clean(target.Path); target.Path != clean && target.Path != clean+"/" {
		return nil, fmt.Errorf("sub path %q is not canonical (want %q)", rawPath, clean)
	}
	if !h.isSubscribablePath(target.Path) {
		return nil, fmt.Errorf("sub path %q must be under /api/ and outside %s", rawPath, h.prefix)
	}

	req, err := http.NewRequest(http.MethodGet, target.RequestURI(), nil)
	if err != nil {
		return nil, fmt.Errorf("build sub request for %q: %w", rawPath, err)
	}
	req.RequestURI = target.RequestURI()
	req.Host = from.Host
	req.RemoteAddr = from.RemoteAddr
	req.Proto, req.ProtoMajor, req.ProtoMinor = from.Proto, from.ProtoMajor, from.ProtoMinor
	for _, name := range hubForwardedHeaders {
		if values := from.Header.Values(name); len(values) > 0 {
			req.Header[name] = append([]string(nil), values...)
		}
	}
	req.Header.Set("Accept", "text/event-stream")

	_, pattern := h.mux.Handler(req)
	if !h.isSubscribablePath(patternPath(pattern)) {
		return nil, fmt.Errorf("sub path %q does not resolve to a registered GET /api route (matched %q)", rawPath, pattern)
	}
	return req, nil
}

func (h *Hub) isSubscribablePath(p string) bool {
	return strings.HasPrefix(p, "/api/") && p != h.prefix && !strings.HasPrefix(p, h.prefix+"/")
}

// patternPath strips the optional "METHOD " prefix from a ServeMux pattern.
func patternPath(pattern string) string {
	if _, rest, ok := strings.Cut(pattern, " "); ok {
		return rest
	}
	return pattern
}

// hubClosed is the payload of a sub's terminal __closed frame.
type hubClosed struct {
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

// runSub serves req through root, re-emitting its events on the connection.
// When the handler ends on its own the sub emits __closed; a sub stopped by
// DELETE or by the connection closing emits nothing more.
func (c *hubConn) runSub(sub *hubSub, req *http.Request, root http.Handler) {
	defer c.wg.Done()
	defer close(sub.done)
	defer c.removeSub(sub)

	pr, pw := io.Pipe()
	// A cancelled sub must unblock both ends: the handler's next Write fails
	// and the parser's Read returns, whatever either is doing.
	stop := context.AfterFunc(sub.ctx, func() { pr.CloseWithError(sub.ctx.Err()) })
	defer stop()

	rw := newSubResponseWriter(pw)
	handlerDone := make(chan struct{})
	go serveSub(root, rw, req, pw, handlerDone)

	select {
	case <-rw.committed:
	case <-sub.ctx.Done():
		awaitSubHandler(sub, req, handlerDone)
		return
	}
	closed := hubClosed{Status: rw.status}
	contentType, _, _ := mime.ParseMediaType(rw.header.Get("Content-Type"))
	switch {
	case rw.status < 200 || rw.status > 299:
		closed.Error = readErrorBody(pr)
	case contentType != "text/event-stream":
		closed.Error = fmt.Sprintf("content type %q is not text/event-stream: %s", rw.header.Get("Content-Type"), readErrorBody(pr))
	default:
		if err := readStream(pr, func(ev streamEvent) error { return c.write(encodeSubEvent(sub.id, ev)) }); err != nil {
			closed.Error = err.Error()
		}
	}
	stopped := sub.ctx.Err() != nil
	sub.cancel()
	if !awaitSubHandler(sub, req, handlerDone) {
		closed.Error = fmt.Sprintf("handler did not return within %s of cancellation", hubSubStopTimeout)
	} else if rw.panicked != nil {
		closed = hubClosed{Status: http.StatusInternalServerError, Error: fmt.Sprintf("handler panic: %v", rw.panicked)}
	}
	if stopped {
		return
	}
	payload, err := json.Marshal(closed)
	if err != nil {
		panic(fmt.Sprintf("marshal events __closed: %v", err))
	}
	if err := c.write(fmt.Appendf(nil, "event: %s/__closed\ndata: %s\n\n", sub.id, payload)); err != nil && !errors.Is(err, errHubConnClosed) {
		logger.Debugf("events sub %s: %v", sub.id, err)
	}
}

// awaitSubHandler waits for a cancelled sub's handler to return, at most
// hubSubStopTimeout. Go cannot stop a goroutine that ignores its request
// context, so a handler that overruns is abandoned: it only ever writes into
// the sub's closed pipe, never onto the connection, and DELETE and connection
// shutdown stay bounded. It reports whether the handler returned.
func awaitSubHandler(sub *hubSub, req *http.Request, handlerDone <-chan struct{}) bool {
	timer := time.NewTimer(hubSubStopTimeout)
	defer timer.Stop()
	select {
	case <-handlerDone:
		return true
	case <-timer.C:
		logger.Errorf("events sub %s: handler for %s ignored cancellation for %s; abandoning it", sub.id, req.URL, hubSubStopTimeout)
		return false
	}
}

// serveSub runs the handler, converting a panic into a recorded 500 the way
// net/http would recover it for a direct request, and closes the pipe so the
// parser sees the end of the body.
func serveSub(root http.Handler, rw *subResponseWriter, req *http.Request, pw *io.PipeWriter, done chan<- struct{}) {
	defer close(done)
	defer pw.Close()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Errorf("events sub handler for %s panicked: %v", req.URL, recovered)
			rw.WriteHeader(http.StatusInternalServerError)
			rw.panicked = recovered
		}
		rw.WriteHeader(http.StatusOK)
	}()
	root.ServeHTTP(rw, req)
}

// readErrorBody returns the first hubErrorBodyLimit bytes of the body.
func readErrorBody(r io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(r, hubErrorBodyLimit))
	if err != nil {
		return fmt.Sprintf("%s (reading body: %v)", body, err)
	}
	return string(body)
}

// subResponseWriter is the ResponseWriter a sub's handler writes into. The body
// flows through a pipe to the SSE parser; the status and headers are frozen at
// the first WriteHeader/Write/Flush and published by closing committed.
type subResponseWriter struct {
	pipe      *io.PipeWriter
	live      http.Header
	once      sync.Once
	committed chan struct{}
	status    int
	header    http.Header
	panicked  any
}

func newSubResponseWriter(pw *io.PipeWriter) *subResponseWriter {
	return &subResponseWriter{pipe: pw, live: http.Header{}, committed: make(chan struct{})}
}

func (w *subResponseWriter) Header() http.Header { return w.live }

func (w *subResponseWriter) WriteHeader(status int) {
	if status >= 100 && status <= 199 {
		return
	}
	w.once.Do(func() {
		w.status = status
		w.header = w.live.Clone()
		close(w.committed)
	})
}

func (w *subResponseWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.pipe.Write(b)
}

// Flush commits the headers; the pipe is unbuffered, so every Write has already
// reached the parser by the time it returns.
func (w *subResponseWriter) Flush() {
	w.WriteHeader(http.StatusOK)
}
