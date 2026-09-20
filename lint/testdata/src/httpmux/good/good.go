package good

import (
	"net/http"

	"github.com/flanksource/clicky/route"
)

// OK: the router is the parameter, and every route declares what it is.
func RegisterHandlers(router *route.Router, h *handlers) {
	router.RawFunc("GET /api/v1/widgets", h.list,
		route.Meta{Entity: "widget", Verb: "list", ReadOnly: true})
	router.RawFunc("POST /api/v1/widgets", h.create,
		route.Meta{Entity: "widget", Verb: "create"})
}

// OK: building a mux and handing it to the router is the intended shape — only
// accepting one as a parameter is the smell.
func Serve(h *handlers) http.Handler {
	mux := http.NewServeMux()
	router := route.NewRouter(mux)
	RegisterHandlers(router, h)
	return mux
}

// OK: a route that genuinely cannot be declared as an operation says so where
// the exception is made, and only for the rule it needs.
func RegisterProxy(router *route.Router, proxy http.Handler) {
	//clicky:allow direct-http-handler the JMX console proxies arbitrary methods
	router.Raw("/jolokia/proxy/{rest...}", proxy, route.Meta{Entity: "jolokia", Verb: "proxy"})
}

type handlers struct{}

func (h *handlers) list(_ http.ResponseWriter, _ *http.Request)   {}
func (h *handlers) create(_ http.ResponseWriter, _ *http.Request) {}
