package sse

import "sync"

// Notifier wakes every subscriber at once. A single shared channel would hand
// each signal to whichever reader got there first, so concurrent streams (one
// per open tab) would steal each other's wake-ups. Each subscriber instead owns
// a channel buffered to one: a Notify never blocks, and notifications a
// subscriber has not consumed yet coalesce into a single pending wake.
//
// The zero value is ready to use. A Notifier must not be copied after first use.
type Notifier struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// Subscribe returns a channel that receives a value after each Notify (or
// burst of them), and a cancel that unsubscribes. cancel is idempotent and does
// not close the channel, so a select over it never sees a spurious wake.
func (n *Notifier) Subscribe() (<-chan struct{}, func()) {
	wake := make(chan struct{}, 1)
	n.mu.Lock()
	if n.subs == nil {
		n.subs = map[chan struct{}]struct{}{}
	}
	n.subs[wake] = struct{}{}
	n.mu.Unlock()
	return wake, func() {
		n.mu.Lock()
		delete(n.subs, wake)
		n.mu.Unlock()
	}
}

// Notify wakes every current subscriber without blocking.
func (n *Notifier) Notify() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for wake := range n.subs {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}
