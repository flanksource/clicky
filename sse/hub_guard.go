package sse

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

// hubSubKey marks a request the hub dispatches for a sub. It lives in the
// request context, which a client cannot set, so no header can forge it.
type hubSubKey struct{}

func withHubSub(ctx context.Context) context.Context {
	return context.WithValue(ctx, hubSubKey{}, true)
}

func isHubSub(ctx context.Context) bool {
	marked, _ := ctx.Value(hubSubKey{}).(bool)
	return marked
}

// Guard answers 410 Gone to a browser opening a native EventSource on a stream
// route the hub can serve instead. A page loaded from a bundle that predates
// the hub holds one such connection per topic and, under the browser's
// six-connections-per-host cap, starves every ordinary request of the tab and
// its siblings. EventSource treats a non-200 response as fatal and does not
// reconnect, so the stale page releases the connection.
//
// Only browsers are refused — they alone send Sec-Fetch-Mode, so CLI and Go
// clients keep direct streams — and only GETs, so POST launch streams are left
// alone. Paths the hub cannot subscribe to (outside /api/, e.g. a page's
// embedded bundle serving its own streams) have no replacement and stay served.
func (h *Hub) Guard(next http.Handler) http.Handler {
	refusal := "stream routes are served through " + h.prefix + "; reload the page"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.Header.Get("Sec-Fetch-Mode") != "" &&
			slices.ContainsFunc(r.Header.Values("Accept"), func(v string) bool { return strings.Contains(v, "text/event-stream") }) &&
			h.isSubscribablePath(r.URL.Path) &&
			!isHubSub(r.Context()) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, refusal, http.StatusGone)
			return
		}
		next.ServeHTTP(w, r)
	})
}
