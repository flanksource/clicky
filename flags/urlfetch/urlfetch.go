// Package urlfetch reads @http(s) file references for
// github.com/flanksource/clicky/flags. Values from remote (RPC) callers go
// through the Commons defensive client, which rejects private and loopback
// targets; it is the only flags package importing github.com/flanksource/commons/http.
package urlfetch

import (
	"context"
	"net/http"

	commonshttp "github.com/flanksource/commons/http"

	"github.com/flanksource/clicky/flags"
)

type Fetcher struct{}

func (Fetcher) Fetch(ctx context.Context, url string, defensive bool) (*http.Response, error) {
	if defensive {
		response, err := commonshttp.NewDefensive().R(ctx).Get(url)
		if err != nil {
			return nil, err
		}
		return response.Response, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(request)
}

// Use makes Fetcher the reader for @http(s) file references.
func Use() {
	flags.SetURLFetcher(Fetcher{})
}
