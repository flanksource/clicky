package flags

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// URLFetcher performs the HTTP GET behind @http(s) file references. The core
// package ships none, so binaries that never read URLs don't compile an HTTP
// client stack; opt in with urlfetch.Use().
type URLFetcher interface {
	// Fetch GETs url. defensive is set for values supplied by remote (RPC)
	// callers, which must be guarded against SSRF.
	Fetch(ctx context.Context, url string, defensive bool) (*http.Response, error)
}

var ErrNoURLFetcher = errors.New("no URL fetcher configured: call urlfetch.Use() from github.com/flanksource/clicky/flags/urlfetch")

var (
	urlFetcherMu sync.RWMutex
	urlFetcher   URLFetcher
)

func SetURLFetcher(f URLFetcher) {
	urlFetcherMu.Lock()
	defer urlFetcherMu.Unlock()
	urlFetcher = f
}

func fetchURL(ctx context.Context, url string, defensive bool) (*http.Response, error) {
	urlFetcherMu.RLock()
	fetcher := urlFetcher
	urlFetcherMu.RUnlock()
	if fetcher == nil {
		return nil, fmt.Errorf("reading URL file reference: %w", ErrNoURLFetcher)
	}
	return fetcher.Fetch(ctx, url, defensive)
}
