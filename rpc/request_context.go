package rpc

import (
	"context"
	"net/http"
)

// requestContextKey keys the originating *http.Request stashed by the HTTP
// executor into a request's context.Context.
type requestContextKey struct{}

// ContextWithRequest returns a context carrying the originating *http.Request so
// context-based entity handlers (CreateWithContext/UpdateWithContext) can read the
// raw, nested JSON body the executor would otherwise flatten to string flags.
func ContextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestContextKey{}, r)
}

// RequestFromContext returns the originating *http.Request stashed by the HTTP
// executor. The second result is false on the CLI path, where there is no request.
func RequestFromContext(ctx context.Context) (*http.Request, bool) {
	r, ok := ctx.Value(requestContextKey{}).(*http.Request)
	return r, ok
}
