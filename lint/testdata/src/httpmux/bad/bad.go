package bad

import "net/http"

// Taking the mux is how a package claims the right to register anything
// anywhere: the call site says nothing about what appears on the server.
func RegisterHandlers(mux *http.ServeMux, h *handlers) { // want `avoid accepting \*http\.ServeMux in RegisterHandlers`
	mux.HandleFunc("GET /api/v1/widgets", h.list)    // want `avoid registering net/http handlers directly`
	mux.HandleFunc("POST /api/v1/widgets", h.create) // want `avoid registering net/http handlers directly`
}

// The parameter is flagged wherever it sits in the signature.
func RegisterLate(prefix string, mux *http.ServeMux) { // want `avoid accepting \*http\.ServeMux in RegisterLate`
	_ = prefix
	_ = mux
}

type handlers struct{}

func (h *handlers) list(_ http.ResponseWriter, _ *http.Request)   {}
func (h *handlers) create(_ http.ResponseWriter, _ *http.Request) {}
