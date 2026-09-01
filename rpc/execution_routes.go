package rpc

import (
	"net/http"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
)

func normalizeWildcardNames(pattern string) string {
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[i] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

func (s *SwaggerServer) registerExecutionRoutes(mux *http.ServeMux) {
	if s.executor == nil || s.executor.service == nil {
		clicky.Warnf("Executor has no service; no routes registered")
		return
	}

	registered := make(map[string]string)
	routeCount := 0
	registerRoute := func(method, path, operationName string) bool {
		sanitized, ok := sanitizePathParams(path)
		if !ok {
			clicky.Warnf("Skipping route with invalid path params: %s %s", method, path)
			return false
		}

		pattern := method + " " + sanitized
		dedupeKey := method + " " + normalizeWildcardNames(sanitized)
		if existingOperation, found := registered[dedupeKey]; found {
			clicky.Warnf("Duplicate endpoint %s already registered by %q; skipping %q", pattern, existingOperation, operationName)
			return false
		}
		registered[dedupeKey] = operationName
		mux.Handle(pattern, s.tracedHandler(pattern, http.HandlerFunc(s.handleExecuteCommand)))
		routeCount++
		return true
	}

	for _, operation := range s.executor.service.Operations {
		operationPath := strings.ReplaceAll(operation.Path, " ", "-")
		method := strings.ToUpper(operation.Method)
		registerRoute(method, operationPath, operation.Name)
		if !hasLookup(&operation) {
			continue
		}
		registerRoute(http.MethodHead, operationPath, operation.Name+" lookup")
		if !strings.EqualFold(method, http.MethodGet) {
			registerRoute(http.MethodGet, operationPath, operation.Name+" lookup")
		}
	}

	for _, family := range entity.GetDynamicEntityFamilies() {
		familyPath := s.pathPrefix() + "/" + family.Name + "/{name}"
		// Every write method routes to handleDynamicFamily too, so an unsupported
		// verb gets the family's 405 with Allow: GET, HEAD instead of a bare mux
		// fallback response.
		for _, method := range []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodDelete, http.MethodPatch, http.MethodOptions,
		} {
			registerRoute(method, familyPath, family.Name+" family")
		}
	}
	clicky.Infof("Registered %d executor routes", routeCount)
}

// sanitizePathParams normalizes an operation path into a pattern Go 1.22's
// ServeMux accepts. Ordinary wildcard names may carry '-' or '.' from CLI
// arg/flag names; those map unambiguously to '_'. The ServeMux special forms
// {$} and {name...} are preserved verbatim — rewriting them would silently
// change what the route matches — and any other unexpected character rejects
// the path (the caller warns and skips the route) instead of being rewritten.
func sanitizePathParams(path string) (string, bool) {
	if !strings.ContainsAny(path, "{}") {
		return path, true
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || len(segment) < 3 {
			return path, false
		}
		name, ok := sanitizeWildcard(segment[1:len(segment)-1], i == len(segments)-1)
		if !ok {
			return path, false
		}
		segments[i] = "{" + name + "}"
	}
	return strings.Join(segments, "/"), true
}

// sanitizeWildcard validates one wildcard name, mapping '-' and '.' to '_'.
// {$} and {name...} pass through unchanged, and only in the final segment —
// ServeMux panics on them anywhere else.
func sanitizeWildcard(name string, finalSegment bool) (string, bool) {
	if name == "$" {
		return name, finalSegment
	}
	if base, multi := strings.CutSuffix(name, "..."); multi {
		return name, finalSegment && isValidWildcard(base)
	}
	var value strings.Builder
	for _, character := range name {
		switch {
		case (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_':
			value.WriteRune(character)
		case character == '-' || character == '.':
			value.WriteRune('_')
		default:
			return name, false
		}
	}
	return value.String(), value.Len() > 0
}

func isValidWildcard(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}
