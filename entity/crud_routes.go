// Route overrides for the CRUD verbs: publishing an operation where its callers
// already address it, rather than where the naming algorithm would put it.
package entity

// RouteOverride publishes one operation at a declared URL or method instead of
// the derived one. An empty field keeps what the derivation produced.
//
// Path is absolute or relative to the configured prefix, the same as
// ActionSpec.WithPath. Method is an HTTP method name.
type RouteOverride struct {
	Path   string
	Method string
}

// applyRouteOverrides stamps declared routes onto the operations they name.
// Verbs with no override are left exactly as derived, so adding this to an
// entity moves only the operations it mentions.
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
		if override.Method != "" {
			operations[i].Method = override.Method
		}
	}
}
