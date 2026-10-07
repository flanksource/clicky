package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
)

// The multiplexed events stream exists because a browser allows only six
// HTTP/1.1 connections per host, shared by every tab. A dashboard tab holding
// one EventSource per topic (item list, process status, task runs, …) lets a
// second tab or a reload starve ordinary fetches until they queue forever. The
// hub carries every topic of a tab over one connection:
//
//	GET    {prefix}                  → text/event-stream; first frame
//	                                   `event: __hello` /
//	                                   `data: {"conn":"<id>","build":"<build id>"}`
//	POST   {prefix}/{conn}/subs      {"id","path"} → 204 once the sub runs
//	DELETE {prefix}/{conn}/subs/{id} → 204 once the sub's handler stopped
//
// A sub runs the stream handler registered for its path in-process, through
// the server's own top-level handler, and re-emits each of its events as
// `event: <subId>/<name>` on the connection. When the handler ends on its own
// the sub emits `event: <subId>/__closed` / `data: {"status":…}`.

// DefaultPrefix is where a Hub mounts its routes unless HubOptions.Prefix says
// otherwise.
const DefaultPrefix = "/api/events"

const hubPingInterval = 15 * time.Second

var hubSubIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var errHubConnClosed = errors.New("events connection closed")

// HubOptions configures a Hub.
type HubOptions struct {
	// Build identifies the UI bundle the server ships. Every hello frame carries
	// it so a page loaded from an older bundle notices it is stale and reloads.
	// Required.
	Build string
	// Prefix is the path the hub's routes live under. Default DefaultPrefix.
	Prefix string
}

// Hub tracks the open events connections of one handler tree. mux resolves a
// sub's path to a registered route; root is the full top-level handler
// (middlewares included) the sub is served through.
type Hub struct {
	build  string
	prefix string

	mu    sync.Mutex
	mux   *http.ServeMux
	root  http.Handler
	conns map[string]*hubConn
}

// hubConn is one open events response and the subs multiplexed onto it.
type hubConn struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc

	writeMu sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
	closed  bool

	mu         sync.Mutex
	subs       map[string]*hubSub
	subsClosed bool
	wg         sync.WaitGroup
}

type hubSub struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewHub returns a Hub; it panics on options it cannot serve.
func NewHub(opts HubOptions) *Hub {
	if opts.Build == "" {
		panic("sse: HubOptions.Build is required")
	}
	if opts.Prefix == "" {
		opts.Prefix = DefaultPrefix
	}
	if !strings.HasPrefix(opts.Prefix, "/") || strings.HasSuffix(opts.Prefix, "/") {
		panic(fmt.Sprintf("sse: HubOptions.Prefix %q must start with / and not end with /", opts.Prefix))
	}
	return &Hub{build: opts.Build, prefix: opts.Prefix, conns: map[string]*hubConn{}}
}

// Register mounts the hub routes on mux. root must be the handler the server
// actually serves (mux wrapped in its middlewares, Guard included) so subs see
// exactly what a direct request to the same path would.
func (h *Hub) Register(mux *http.ServeMux, root http.Handler) {
	h.mu.Lock()
	if h.mux != nil {
		h.mu.Unlock()
		panic(fmt.Sprintf("sse: hub at %s is already registered", h.prefix))
	}
	h.mux, h.root = mux, root
	h.mu.Unlock()
	mux.HandleFunc("GET "+h.prefix, h.handleStream)
	mux.HandleFunc("POST "+h.prefix+"/{conn}/subs", h.handleSubscribe)
	mux.HandleFunc("DELETE "+h.prefix+"/{conn}/subs/{id}", h.handleUnsubscribe)
}

func (h *Hub) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	setStreamHeaders(w.Header())
	// The hub connection outlives any fixed response deadline; a write after
	// http.Server.WriteTimeout would drop every sub multiplexed onto it.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		http.Error(w, fmt.Sprintf("clear events write deadline: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	conn := &hubConn{id: uuid.NewString(), ctx: ctx, cancel: cancel, w: w, flusher: flusher, subs: map[string]*hubSub{}}
	h.mu.Lock()
	h.conns[conn.id] = conn
	h.mu.Unlock()
	defer h.closeConn(conn)

	hello, err := json.Marshal(hubHello{Conn: conn.id, Build: h.build})
	if err != nil {
		panic(fmt.Sprintf("marshal events hello: %v", err))
	}
	if conn.write(fmt.Appendf(nil, "event: __hello\ndata: %s\n\n", hello)) != nil {
		return
	}
	ticker := time.NewTicker(hubPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if conn.write([]byte(": ping\n\n")) != nil {
				return
			}
		}
	}
}

// closeConn forgets conn, stops its writes, cancels every sub and waits for
// their goroutines: the response writer is invalid once handleStream returns.
func (h *Hub) closeConn(conn *hubConn) {
	h.mu.Lock()
	delete(h.conns, conn.id)
	h.mu.Unlock()

	conn.writeMu.Lock()
	conn.closed = true
	conn.writeMu.Unlock()

	conn.mu.Lock()
	conn.subsClosed = true
	conn.mu.Unlock()
	conn.cancel()
	conn.wg.Wait()
}

func (h *Hub) lookup(id string) (*hubConn, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn, ok := h.conns[id]
	return conn, ok
}

// hubHello is the payload of the __hello frame.
type hubHello struct {
	Conn  string `json:"conn"`
	Build string `json:"build"`
}

type hubSubRequest struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (h *Hub) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	conn, ok := h.lookup(r.PathValue("conn"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown events connection %q", r.PathValue("conn")))
		return
	}
	var body hubSubRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode subscription: %w", err))
		return
	}
	if !hubSubIDPattern.MatchString(body.ID) {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("sub id %q must match %s", body.ID, hubSubIDPattern))
		return
	}
	req, err := h.subRequest(r, body.Path)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	status, err := conn.startSub(body.ID, req, h.root)
	if err != nil {
		writeJSONError(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Hub) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	conn, ok := h.lookup(r.PathValue("conn"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown events connection %q", r.PathValue("conn")))
		return
	}
	id := r.PathValue("id")
	conn.mu.Lock()
	sub, ok := conn.subs[id]
	conn.mu.Unlock()
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown sub %q on events connection %q", id, conn.id))
		return
	}
	sub.cancel()
	<-sub.done
	w.WriteHeader(http.StatusNoContent)
}

// startSub registers the sub and launches it. The returned status is the HTTP
// status to answer the subscribe request with when err is non-nil.
func (c *hubConn) startSub(id string, req *http.Request, root http.Handler) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subsClosed {
		return http.StatusNotFound, fmt.Errorf("events connection %q is closed", c.id)
	}
	if _, exists := c.subs[id]; exists {
		return http.StatusConflict, fmt.Errorf("sub %q is already active on events connection %q", id, c.id)
	}
	// The marker exempts the sub request from Guard: it is the hub-served
	// replacement for the direct stream that guard refuses.
	ctx, cancel := context.WithCancel(withHubSub(c.ctx))
	sub := &hubSub{id: id, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	c.subs[id] = sub
	c.wg.Add(1)
	go c.runSub(sub, req.WithContext(ctx), root)
	return 0, nil
}

func (c *hubConn) removeSub(sub *hubSub) {
	c.mu.Lock()
	if c.subs[sub.id] == sub {
		delete(c.subs, sub.id)
	}
	c.mu.Unlock()
}

// write sends one complete frame on the connection and flushes it. Every
// writer (hello, pings, every sub) goes through here, so frames never
// interleave. A failed write means the client is gone: the connection is
// cancelled, which tears down its subs.
func (c *hubConn) write(frame []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return errHubConnClosed
	}
	if _, err := c.w.Write(frame); err != nil {
		c.cancel()
		return fmt.Errorf("write to events connection %q: %w", c.id, err)
	}
	c.flusher.Flush()
	return nil
}

// writeJSONError answers a control request with {"error": "..."}.
func writeJSONError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encodeErr != nil {
		// The status is already out; the client is gone or the body broke.
		logger.Debugf("write events error response: %v", encodeErr)
	}
}
