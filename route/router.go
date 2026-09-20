// Package route is the registration point for every HTTP route an application
// serves, so routes it writes itself are observed like generated operations.
package route

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/flanksource/clicky/entity"
)

// Meta is what a route declares about itself at its registration site.
// Nothing here is derived from the URL: a listener reading an event never has to
// decompose a path or infer intent from a method, which is the whole reason the
// declaration lives next to the handler.
type Meta struct {
	// Entity and Verb name the operation the way a generated one is named, so
	// one trail reads consistently whichever surface produced the entry.
	Entity string
	Verb   string
	// ReadOnly declares that the route changes nothing. It defaults to false so
	// an undeclared route is treated as a mutation: over-recording is recoverable,
	// a missing record is not.
	ReadOnly bool
	// IDParam names the wildcard in the pattern holding the record the call acts
	// on, e.g. "id" for "DELETE /api/v1/monitors/{id}". Empty when the route
	// addresses a collection.
	IDParam string
	// SelfRouted marks a mount whose handler does its own method and sub-path
	// dispatch, so no single declaration can describe what a request to it did.
	// Verb then comes from the request method and ReadOnly from whether that
	// method is safe - deliberately coarser than a declaration, and opt-in at
	// the registration site so it reads as a known gap rather than an accident.
	// A route setting it has not finished migrating: splitting it into declared
	// routes, or into entity operations, is what removes it.
	SelfRouted bool
}

// describe resolves what one request to this route did. A declared route
// answers the same way for every request; a self-routed mount can only answer
// from the method, which is why saying so is explicit.
func (m Meta) describe(method string) (verb string, readOnly bool) {
	if !m.SelfRouted {
		return m.Verb, m.ReadOnly
	}
	return strings.ToLower(method), safeMethod(method)
}

// safeMethod reports whether a method is defined never to change state.
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// Router registers routes onto a mux and observes the ones the application
// wrote itself. Generated routes are mounted unobserved: clicky already reports
// them at the entity layer, and reporting them again here would double every
// entry in the trail.
type Router struct {
	mux *http.ServeMux
}

// NewRouter routes through mux, or through a fresh one when mux is nil.
func NewRouter(mux *http.ServeMux) *Router {
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Router{mux: mux}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Raw registers a handler the application wrote itself. The handler is called
// exactly as it would be on a bare mux — its response, streaming and connection
// handling all reach the client untouched — and its completion is reported to
// every operation listener.
func (r *Router) Raw(pattern string, handler http.Handler, meta Meta) {
	r.mux.Handle(pattern, observeRoute(handler, meta))
}

// RawFunc is Raw for a handler function.
func (r *Router) RawFunc(pattern string, handler http.HandlerFunc, meta Meta) {
	r.Raw(pattern, handler, meta)
}

// MountGenerated registers a route clicky generated from a registered entity or
// command. It is deliberately unobserved: those operations are already reported
// at the entity layer, and reporting them again at the transport would double
// every generated entry in the trail. Application routes use Raw.
func (r *Router) MountGenerated(pattern string, handler http.Handler) {
	r.mux.Handle(pattern, handler)
}

// observeRoute reports one request as a completed operation.
func observeRoute(handler http.Handler, meta Meta) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorder := &routeWriter{ResponseWriter: w}
		ctx := entity.ContextWithOperationSurface(req.Context(), "http")
		_, _ = entity.ObserveOperation(ctx,
			func() entity.OperationEvent {
				verb, readOnly := meta.describe(req.Method)
				event := entity.OperationEvent{Entity: meta.Entity, Verb: verb, ReadOnly: &readOnly}
				if meta.IDParam != "" {
					event.TargetID = req.PathValue(meta.IDParam)
				}
				return event
			},
			func() (any, error) {
				handler.ServeHTTP(recorder, req.WithContext(ctx))
				return nil, recorder.failure()
			})
	})
}

// routeWriter records the response status so a request that failed is not
// reported as a completed success. It delegates the streaming interfaces and
// exposes Unwrap for http.ResponseController, so SSE handlers, reverse proxies
// and connection upgrades behave exactly as they would unwrapped.
type routeWriter struct {
	http.ResponseWriter
	status int
}

func (w *routeWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *routeWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// failure renders a non-success response as the operation's error. A handler
// that wrote nothing completed successfully, as an empty 200 would.
func (w *routeWriter) failure() error {
	if w.status < 400 {
		return nil
	}
	return fmt.Errorf("%d %s", w.status, http.StatusText(w.status))
}

func (w *routeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *routeWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *routeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}
