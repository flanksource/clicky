package valkey

import (
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
)

// NewStoreWithTimeouts is NewStore with its own op and payload budgets.
func NewStoreWithTimeouts(client valkey.Client, op, payload time.Duration) cache.Store {
	return &kvStore{client: client, opTimeout: op, payloadTimeout: payload}
}

// SetTimeout is the budget a store gives a write of size bytes.
func SetTimeout(size int) time.Duration {
	return (&kvStore{opTimeout: opTimeout, payloadTimeout: payloadTimeout}).setTimeout(size)
}
