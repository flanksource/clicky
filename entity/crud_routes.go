// Route overrides for the CRUD verbs: publishing an operation where its callers
// already address it, rather than where the naming algorithm would put it.
package entity

import (
	"fmt"
	"net/http"
	"strings"
)

// RouteOverride publishes one operation at a declared URL or method instead of
// the derived one. An empty field keeps what the derivation produced.
//
// Path is absolute or relative to the configured prefix, the same as
// ActionSpec.WithPath.
//
// Method is an HTTP method name. It may not be a safe method (GET, HEAD,
// OPTIONS) on create, update or delete: registration refuses that rather than
// publishing a state change where anything following links would call it.
type RouteOverride struct {
	Path   string
	Method string
}

// mutationVerbs are the CRUD operations that change state. A safe method must
// never reach one; see applyRouteOverrides.
var mutationVerbs = map[string]bool{"create": true, "update": true, "delete": true}

// SafeMethod reports whether an HTTP method is defined never to change state.
// Exported so transports classify a request the same way registration does.
func SafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// applyRouteOverrides stamps declared routes onto the operations they name.
// Verbs with no override are left exactly as derived, so adding this to an
// entity moves only the operations it mentions.
//
// Publishing a mutation under a safe method is refused rather than registered.
// Anything that follows links treats GET and HEAD as free to call — prefetchers,
// crawlers, a browser restoring tabs — so a delete reachable that way is a
// delete that happens without anyone asking for it. It panics because
// registration runs at init: the mistake is in the program, not in a request,
// and the earliest possible failure is the kindest one.
func applyRouteOverrides(operations []EntityOperation, routes map[string]RouteOverride) {
	if len(routes) == 0 {
		return
	}
	for i := range operations {
		override, ok := routes[operations[i].Verb]
		if !ok {
			continue
		}
		if override.Path != "" {
			operations[i].RoutePath = override.Path
		}
		if override.Method == "" {
			continue
		}
		if mutationVerbs[operations[i].Verb] && SafeMethod(override.Method) {
			panic(fmt.Sprintf(
				"route override publishes %s as %s: a safe method must not reach an operation that changes state",
				operations[i].Verb, strings.ToUpper(override.Method)))
		}
		operations[i].Method = override.Method
	}
}
