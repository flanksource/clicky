package entity

import (
	"context"
	"net/http"
	"sync"
)

// DynamicEntityFamily is a route for entities that do not exist yet.
//
// The registry pipeline — entityRegistry, GenerateCLI, Cobra, NewSwaggerServer,
// RegisterRoutes — is one snapshot taken at startup, so an entity that comes
// into being while the server runs has no route and no way to acquire one. A
// consumer whose entities are database rows is locked out of it entirely.
//
// A family registers one route for the whole shape — {prefix}/{Name}/{name} —
// and resolves the instance per request. Nothing is cached, so an entity created
// a moment ago is immediately reachable and none of the startup pipeline has to
// be re-run or invalidated.
type DynamicEntityFamily struct {
	// Name is the family's path segment: "profile" serves one route,
	// {prefix}/profile/{name}, for every profile that will ever exist.
	Name string

	// Parent groups the family's surfaces, as EntityInfo.Parent does.
	Parent string

	// Resolve returns the spec for one instance. It runs in front of every
	// request to the family, so it has to be cheap enough to sit there. A name
	// that does not exist is UnknownDynamicEntity rather than a zero spec: "no
	// such profile" and "a profile that does nothing" are different answers and a
	// caller has to be able to tell them apart.
	Resolve func(ctx context.Context, name string) (DynamicEntitySpec, error)

	// List enumerates the instances that exist right now, so the OpenAPI document
	// can describe them. It is called per request for the same reason Resolve is:
	// a document rendered once at startup would describe a set that has since
	// changed.
	List func(ctx context.Context) ([]DynamicEntitySpec, error)

	// Paged serves one page or one export of a resolved instance, putting a
	// family instance on the same export contract as RPCOperation.PagedFunc.
	//
	// It hangs off the family rather than off DynamicEntitySpec because the spec
	// says what the entity is and this says how the transport reads it, and
	// because one implementation serves every instance. Nil falls back to the
	// instance's List, served the way every other operation's data is.
	Paged func(ctx context.Context, spec DynamicEntitySpec, req PageRequest, flags map[string]string) (PageResponse, error)
}

var (
	dynamicFamilyMu sync.RWMutex
	dynamicFamilies []DynamicEntityFamily
)

// RegisterDynamicEntityFamily registers a family, replacing any family already
// registered under the same Name.
//
// Replacing rather than appending is deliberate: a family is a route, not an
// instance, and two families claiming one path segment is a wiring bug that
// would otherwise surface as whichever registration happened to be found first.
//
// Register every family before the server builds its mux (rpc.RegisterRoutes).
// The mux pattern for a family is written once, from the families registered at
// that moment: a family registered afterwards is described by the OpenAPI
// document — which is resolved per request — but has no pattern, so the mux
// answers 404 before the family dispatch is ever reached. Unregistering has the
// mirror shape: the pattern stays, and the path falls through to ordinary
// operation lookup and 404s there.
//
// Name must not collide with the first path segment of a registered operation.
// The dispatch resolves a registered operation first, so a colliding family is
// simply never reached for the paths the operation owns.
func RegisterDynamicEntityFamily(family DynamicEntityFamily) {
	if family.Name == "" {
		panic("clicky.RegisterDynamicEntityFamily: Name must not be empty")
	}
	if family.Resolve == nil {
		panic("clicky.RegisterDynamicEntityFamily: family " + family.Name + " has no Resolve func")
	}

	dynamicFamilyMu.Lock()
	defer dynamicFamilyMu.Unlock()
	for index := range dynamicFamilies {
		if dynamicFamilies[index].Name == family.Name {
			dynamicFamilies[index] = family
			return
		}
	}
	dynamicFamilies = append(dynamicFamilies, family)
}

// UnregisterDynamicEntityFamily removes a family by name, reporting whether one
// was registered. A family whose backing store is gone — a tenant disconnected,
// a plugin unloaded — has to be able to take its route with it.
func UnregisterDynamicEntityFamily(name string) bool {
	dynamicFamilyMu.Lock()
	defer dynamicFamilyMu.Unlock()
	for index := range dynamicFamilies {
		if dynamicFamilies[index].Name != name {
			continue
		}
		dynamicFamilies = append(dynamicFamilies[:index], dynamicFamilies[index+1:]...)
		return true
	}
	return false
}

// GetDynamicEntityFamilies returns the registered families.
//
// Nil when none are registered, rather than an empty slice: this is consulted on
// the way into every executor request, and a consumer that registers no family
// must not pay an allocation for the feature it does not use.
func GetDynamicEntityFamilies() []DynamicEntityFamily {
	dynamicFamilyMu.RLock()
	defer dynamicFamilyMu.RUnlock()
	if len(dynamicFamilies) == 0 {
		return nil
	}
	return append([]DynamicEntityFamily{}, dynamicFamilies...)
}

// UnknownDynamicEntity is the answer to a name a family cannot resolve.
func UnknownDynamicEntity(family, name string) *StatusError {
	return NewStatusErrorf(http.StatusNotFound, "not_found", "no %s named %q", family, name)
}

// ResolveDynamicLookup answers a filter-metadata lookup from a resolved spec.
//
// A registered entity reaches this resolution through the LookupFunc the
// registry built for it. A family instance never enters the registry, so the
// transport has to enter the same resolution from the spec it just resolved —
// otherwise a runtime entity would be the one kind that cannot describe its own
// filters.
func ResolveDynamicLookup(ctx context.Context, spec DynamicEntitySpec, flags map[string]string) (any, error) {
	response, err := resolveDynamicLookup(ctx, spec.Filters, flags)
	if err != nil {
		return nil, err
	}
	return response, nil
}
