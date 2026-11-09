package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"
)

// sessionDrainTimeout bounds how long a finished run's stream waits for its
// session follows to close before it sends done. A var so specs can shorten it.
var sessionDrainTimeout = 5 * time.Second

// SessionEvent is one item of a followed session: a transcript entry, a state
// change, or the error that ended the follow. Entry and State are opaque JSON
// owned by the session's type; the task stream relays them unread.
type SessionEvent struct {
	Entry json.RawMessage
	State json.RawMessage
	Err   error
}

// SessionSource follows the sessions of one SessionRef type.
type SessionSource interface {
	// Follow streams the entries and state written AFTER the call. The channel
	// closes when the session reaches a terminal state or ctx is done.
	Follow(ctx context.Context, ref SessionRef) (<-chan SessionEvent, error)
}

var (
	sessionSourcesMu sync.RWMutex
	sessionSources   = map[string]SessionSource{}
)

// RegisterSessionSource registers the source a task stream follows sessions of
// sessionType through. An empty type, a nil source or a type registered twice
// is a programming error and panics, matching RegisterRunner.
func RegisterSessionSource(sessionType string, source SessionSource) {
	if sessionType == "" || source == nil {
		panic("task: RegisterSessionSource requires a session type and a source")
	}
	sessionSourcesMu.Lock()
	defer sessionSourcesMu.Unlock()
	if _, exists := sessionSources[sessionType]; exists {
		panic(fmt.Sprintf("task: session source for type %q already registered", sessionType))
	}
	sessionSources[sessionType] = source
}

func sessionSourceFor(sessionType string) (SessionSource, bool) {
	sessionSourcesMu.RLock()
	defer sessionSourcesMu.RUnlock()
	source, ok := sessionSources[sessionType]
	return source, ok
}

// sessionKey identifies a followed session in its frames.
type sessionKey struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// sessionFrame is the payload of the session, session-state and session-error
// events: the session it belongs to plus exactly one of entry, state or error.
type sessionFrame struct {
	Session sessionKey      `json:"session"`
	Entry   json.RawMessage `json:"entry,omitempty"`
	State   json.RawMessage `json:"state,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// sessionFollows is one task stream connection's session follows, keyed by
// type:id. A retired follow keeps its key with nil events, so a session is
// followed at most once per connection.
type sessionFollows struct {
	follows map[string]*sessionFollow
	// drainBy is when a finished stream stops waiting for its follows.
	drainBy time.Time
}

type sessionFollow struct {
	key    sessionKey
	events <-chan SessionEvent
	cancel context.CancelFunc
	// openErr is why Follow refused the session, sent as its session-error on
	// the next drain — after the task frame naming the session.
	openErr error
}

func newSessionFollows() *sessionFollows {
	return &sessionFollows{follows: map[string]*sessionFollow{}}
}

// open follows ref unless it is already followed (or was), its type has no
// registered source, or terminal says its history is complete. A Follow
// failure becomes the session's session-error frame on the next drain.
func (s *sessionFollows) open(ctx context.Context, ref *SessionRef, terminal bool) {
	if ref == nil || terminal {
		return
	}
	key := sessionKey{Type: ref.Type, ID: ref.ID}
	id := key.Type + ":" + key.ID
	if _, seen := s.follows[id]; seen {
		return
	}
	source, ok := sessionSourceFor(ref.Type)
	if !ok {
		return
	}
	followCtx, cancel := context.WithCancel(ctx)
	events, err := source.Follow(followCtx, *ref)
	if err != nil {
		cancel()
		s.follows[id] = &sessionFollow{key: key, openErr: fmt.Errorf("follow session: %w", err)}
		return
	}
	s.follows[id] = &sessionFollow{key: key, events: events, cancel: cancel}
}

// drain writes every event the open follows hold without blocking, retiring a
// follow whose channel closed or that reported an error.
func (s *sessionFollows) drain(frames io.Writer) error {
	for _, follow := range s.follows {
		if err := follow.drain(frames); err != nil {
			return err
		}
	}
	return nil
}

func (f *sessionFollow) drain(frames io.Writer) error {
	if f.openErr != nil {
		err := f.openErr
		f.openErr = nil
		return writeEvent(frames, "session-error", sessionFrame{Session: f.key, Error: err.Error()})
	}
	for f.events != nil {
		select {
		case event, ok := <-f.events:
			if !ok {
				f.retire()
				return nil
			}
			if err := f.write(frames, event); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return nil
}

// write sends one event; an error event, or one carrying nothing, ends the
// follow with a session-error frame.
func (f *sessionFollow) write(frames io.Writer, event SessionEvent) error {
	if event.Err == nil && event.Entry == nil && event.State == nil {
		event.Err = errors.New("session source sent an event with no entry, state or error")
	}
	if event.Err != nil {
		f.retire()
		return writeEvent(frames, "session-error", sessionFrame{Session: f.key, Error: event.Err.Error()})
	}
	if event.Entry != nil {
		if err := writeEvent(frames, "session", sessionFrame{Session: f.key, Entry: event.Entry}); err != nil {
			return err
		}
	}
	if event.State != nil {
		return writeEvent(frames, "session-state", sessionFrame{Session: f.key, State: event.State})
	}
	return nil
}

func (f *sessionFollow) retire() {
	if f.cancel != nil {
		f.cancel()
	}
	f.events = nil
}

// drained reports whether a finished stream may send done: every follow has
// closed, or sessionDrainTimeout has passed since the stream first finished,
// in which case the follows still open are cancelled.
func (s *sessionFollows) drained(now time.Time) bool {
	var open []*sessionFollow
	for _, follow := range s.follows {
		if follow.events != nil {
			open = append(open, follow)
		}
	}
	if len(open) == 0 {
		return true
	}
	if s.drainBy.IsZero() {
		s.drainBy = now.Add(sessionDrainTimeout)
	}
	if now.Before(s.drainBy) {
		return false
	}
	for _, follow := range open {
		logger.Warnf("task stream: session %s:%s still open %s after its run finished; ending its follow", follow.key.Type, follow.key.ID, sessionDrainTimeout)
		follow.retire()
	}
	return true
}
