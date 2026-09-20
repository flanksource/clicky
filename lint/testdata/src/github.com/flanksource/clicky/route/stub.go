package route

import "net/http"

// Meta is the subset of route.Meta the lint fixtures need.
type Meta struct {
	Entity   string
	Verb     string
	ReadOnly bool
	IDParam  string
}

type Router struct{}

func NewRouter(mux *http.ServeMux) *Router { return &Router{} }

func (r *Router) Raw(pattern string, h http.Handler, meta Meta) {}

func (r *Router) RawFunc(pattern string, h http.HandlerFunc, meta Meta) {}
