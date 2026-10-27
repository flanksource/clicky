package exec

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type ExecPhase string

const (
	ExecStart ExecPhase = "start"
	ExecExit  ExecPhase = "exit"
	TailBytes           = 64 << 10
)

type ExecEvent struct {
	Phase          ExecPhase
	Name, Command  string
	Args, EnvKeys  []string
	Dir            string
	PID            int
	StartedAt      time.Time
	Duration       time.Duration
	ExitCode       int
	Err            error
	Stdout, Stderr string
}

type execObserver struct {
	deliver  func(ExecEvent)
	inflight sync.WaitGroup
}

var execObservers = struct {
	mu        sync.RWMutex
	next      uint64
	count     atomic.Int64
	observers map[uint64]*execObserver
}{observers: make(map[uint64]*execObserver)}

// ObserveExec registers a process-wide observer. Delivery is synchronous on
// the publishing goroutine, so deliver must not block. Release waits for
// in-flight deliveries and must not be called from a delivery callback.
func ObserveExec(deliver func(ExecEvent)) (release func()) {
	if deliver == nil {
		panic("exec observer must not be nil")
	}
	observer := &execObserver{deliver: deliver}
	execObservers.mu.Lock()
	execObservers.next++
	id := execObservers.next
	execObservers.observers[id] = observer
	execObservers.count.Add(1)
	execObservers.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			execObservers.mu.Lock()
			delete(execObservers.observers, id)
			execObservers.count.Add(-1)
			execObservers.mu.Unlock()
			observer.inflight.Wait()
		})
	}
}

func publishExec(p *Process, phase ExecPhase) {
	if execObservers.count.Load() == 0 {
		return
	}
	execObservers.mu.RLock()
	delivers := make([]*execObserver, 0, len(execObservers.observers))
	for _, observer := range execObservers.observers {
		observer.inflight.Add(1)
		delivers = append(delivers, observer)
	}
	execObservers.mu.RUnlock()
	if len(delivers) == 0 {
		return
	}
	p.mu.RLock()
	event := ExecEvent{Phase: phase, Name: p.Cmd, Command: p.Cmd, Args: append([]string(nil), p.Args...), Dir: p.Cwd, PID: p.pid}
	if p.Started != nil {
		event.StartedAt = *p.Started
	}
	if phase == ExecExit {
		event.Duration, event.Err = p.duration, p.Err
		event.ExitCode = -1
		if p.exitCode != nil {
			event.ExitCode = *p.exitCode
		}
	}
	var env []string
	if p.cmd != nil {
		event.Name = p.cmd.Path
		env = p.cmd.Env
	} else {
		env = p.environment()
	}
	p.mu.RUnlock()
	keys := make(map[string]struct{}, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		keys[key] = struct{}{}
	}
	for key := range keys {
		event.EnvKeys = append(event.EnvKeys, key)
	}
	sort.Strings(event.EnvKeys)
	if phase == ExecExit {
		p.captureOutput.mu.Lock()
		event.Stdout = observerTail(&p.captureOutput.stdout)
		event.Stderr = observerTail(&p.captureOutput.stderr)
		p.captureOutput.mu.Unlock()
	}
	for _, observer := range delivers {
		func() {
			defer observer.inflight.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Errorf("exec observer panicked: %v", recovered)
				}
			}()
			observer.deliver(event)
		}()
	}
}

func observerTail(buffer *captureBuffer) string {
	if buffer.size == 0 {
		return ""
	}
	size := min(buffer.size, TailBytes)
	start := (buffer.start + buffer.size - size) % len(buffer.data)
	for size > 0 && !utf8.RuneStart(buffer.data[start]) {
		start = (start + 1) % len(buffer.data)
		size--
	}
	first := min(size, len(buffer.data)-start)
	if first == size {
		return string(buffer.data[start : start+size])
	}
	return string(buffer.data[start:]) + string(buffer.data[:size-first])
}
