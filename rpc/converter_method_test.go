package rpc

import "testing"

// An entity operation states its verb. Before this was consulted, the method was
// guessed by looking for CRUD keywords anywhere in the command path, so an
// entity whose name happened to contain one was misrouted: "target" contains
// "get", which made `target update` a GET on the collection path and shadowed
// the entity's own list route.
func TestMethodForVerb(t *testing.T) {
	for verb, want := range map[string]string{
		"list":   "GET",
		"get":    "GET",
		"create": "POST",
		"update": "PUT",
		"delete": "DELETE",
		"Update": "PUT",
		"":       "",
		"cancel": "", // a custom action falls back to inference
	} {
		if got := methodForVerb(verb); got != want {
			t.Errorf("methodForVerb(%q) = %q, want %q", verb, got, want)
		}
	}
}

// The guess for unannotated commands matches the verb — the last segment —
// rather than searching the whole path. Searching the whole path read a CRUD
// keyword out of a parent's name: "target scan" contains "get", which published
// an action that starts a scan as a safe, prefetchable GET.
func TestInferHTTPMethodMatchesTheVerbNotTheWholePath(t *testing.T) {
	converter := &Converter{config: &Config{DefaultMethod: "POST"}}

	for path, want := range map[string]string{
		"target get":     "GET",
		"target list":    "GET",
		"target update":  "PUT",
		"target delete":  "DELETE",
		"target scan":    "POST", // an action, not a read
		"target refresh": "POST",
		"budget delete":  "DELETE",
		"asset set":      "PUT",
		"profile create": "POST",
		"engine install": "POST",
	} {
		if got := converter.inferHTTPMethod(nil, path); got != want {
			t.Errorf("inferHTTPMethod(%q) = %q, want %q", path, got, want)
		}
	}
}
