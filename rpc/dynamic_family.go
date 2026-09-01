package rpc

import (
	"context"
	"net/http"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
)

func (s *SwaggerServer) matchDynamicFamily(path string) (entity.DynamicEntityFamily, string, bool) {
	families := entity.GetDynamicEntityFamilies()
	if len(families) == 0 {
		return entity.DynamicEntityFamily{}, "", false
	}
	relative := strings.Trim(strings.TrimPrefix(strings.TrimSuffix(path, "/"), s.pathPrefix()), "/")
	segment, name, found := strings.Cut(relative, "/")
	if !found || name == "" || strings.Contains(name, "/") {
		return entity.DynamicEntityFamily{}, "", false
	}
	for _, family := range families {
		if family.Name == segment {
			return family, name, true
		}
	}
	return entity.DynamicEntityFamily{}, "", false
}

func (s *SwaggerServer) pathPrefix() string {
	if s.converterCfg != nil && s.converterCfg.PathPrefix != "" {
		return s.converterCfg.PathPrefix
	}
	return entity.DefaultConfig().PathPrefix
}

var familyReadMethods = []string{http.MethodGet, http.MethodHead}

func isFamilyReadMethod(method string) bool {
	for _, allowed := range familyReadMethods {
		if strings.EqualFold(method, allowed) {
			return true
		}
	}
	return false
}

func (s *SwaggerServer) handleDynamicFamily(w http.ResponseWriter, r *http.Request, family entity.DynamicEntityFamily, name string) {
	if !isFamilyReadMethod(r.Method) {
		entity.SetCORSHeaders(w)
		w.Header().Set("Allow", strings.Join(familyReadMethods, ", "))
		s.writeError(w, r, entity.NewStatusErrorf(
			http.StatusMethodNotAllowed, "method_not_allowed", "%s is not supported for %s instances", r.Method, family.Name,
		))
		return
	}

	spec, err := family.Resolve(r.Context(), name)
	if err != nil {
		entity.SetCORSHeaders(w)
		s.writeError(w, r, err)
		return
	}
	operation := familyOperation(family, spec, r.URL.Path)
	if explicitLookup(r) {
		s.handleDynamicFamilyLookup(w, r, spec, operation)
		return
	}
	if family.Paged != nil {
		paged := func(ctx context.Context, request entity.PageRequest, flags map[string]string) (entity.PageResponse, error) {
			return family.Paged(ctx, spec, request, flags)
		}
		s.handlePagedCommand(w, r, operation, paged, instanceName(spec, name))
		return
	}
	data, metadata, statusCode, err := s.executeOperation(r, operation)
	if err != nil {
		s.writeOperationError(w, r, statusCode, err)
		return
	}
	s.writeExecutionResult(w, r, data, metadata, statusCode)
}

func (s *SwaggerServer) handleDynamicFamilyLookup(
	w http.ResponseWriter,
	r *http.Request,
	spec entity.DynamicEntitySpec,
	operation *RPCOperation,
) {
	flags, err := s.requestFlags(r, operation)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	for _, key := range []string{"__lookup_filter", "__lookup_q"} {
		if value := r.URL.Query().Get(key); value != "" {
			flags[key] = value
		}
	}

	data, err := entity.ResolveDynamicLookup(r.Context(), spec, flags)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.writeLookupResponse(w, r, data)
}

func familyOperation(family entity.DynamicEntityFamily, spec entity.DynamicEntitySpec, path string) *RPCOperation {
	name := instanceName(spec, family.Name)
	parent := spec.Parent
	if parent == "" {
		parent = family.Parent
	}
	operation := &RPCOperation{
		Name: family.Name + " " + name, Description: spec.Title, Path: path, Method: http.MethodGet,
		ContextDataFunc: spec.List, ResponseArray: true,
		Clicky: &entity.ClickyOperationMeta{
			SurfaceID: family.Name + "/" + name, Entity: name, Parent: parent, Verb: "list",
			Icon: spec.Icon, Path: spec.Path, Title: spec.Title, SupportsLookup: len(spec.Filters) > 0,
		},
	}
	if spec.ItemType != nil {
		operation.ResponseType = spec.ItemType
		operation.ResponseEntityID = true
	}
	for _, filter := range spec.Filters {
		parameterType := "string"
		if filter.Multi {
			parameterType = "array"
		}
		operation.Parameters = append(operation.Parameters, entity.RPCParameter{
			Name: filter.Key, Type: parameterType, In: "query", Description: filter.Label,
		})
	}
	return operation
}

func instanceName(spec entity.DynamicEntitySpec, fallback string) string {
	if spec.Name != "" {
		return spec.Name
	}
	return fallback
}

func (s *SwaggerServer) addFamilyPaths(ctx context.Context, spec *OpenAPISpec, families []entity.DynamicEntityFamily) error {
	prefix := s.pathPrefix()
	for _, family := range families {
		if family.List == nil {
			continue
		}
		instances, err := family.List(ctx)
		if err != nil {
			clicky.Errorf("Omitting %s entities from the OpenAPI document: %s", family.Name, s.errorWriter.SafeMessage(err))
			continue
		}
		for _, instance := range instances {
			if instance.Name == "" {
				continue
			}
			path := prefix + "/" + family.Name + "/" + instance.Name
			if pathOwnedByRegisteredOperation(spec.Paths, path) {
				continue
			}
			operation := familyOperation(family, instance, path)
			if spec.Paths == nil {
				spec.Paths = make(map[string]OpenAPIPath, len(instances))
			}
			spec.Paths[path] = OpenAPIPath{"get": s.generator.convertOperationToOpenAPI(*operation)}
			spec.Clicky = appendFamilySurface(spec.Clicky, operation.Clicky, instance)
		}
	}
	return nil
}

// pathOwnedByRegisteredOperation reports whether an existing spec path already
// describes this concrete path — verbatim, or through a templated path that
// captures it (e.g. /api/v1/config/{id} capturing /api/v1/config/daily). At
// runtime the registered operation wins (handleExecuteCommand consults
// FindOperation before matching a family), so writing the family operation at a
// captured path would document an operation the server never serves there.
func pathOwnedByRegisteredOperation(paths map[string]OpenAPIPath, path string) bool {
	for registered := range paths {
		if registered == path || matchTemplatePath(registered, path) {
			return true
		}
	}
	return false
}

func appendFamilySurface(meta *ClickySpecMeta, operation *entity.ClickyOperationMeta, instance entity.DynamicEntitySpec) *ClickySpecMeta {
	if meta == nil {
		meta = &ClickySpecMeta{}
	}
	title := instance.Title
	if title == "" {
		title = operation.Entity
	}
	operation.Surface = operation.Entity
	meta.Surfaces = append(meta.Surfaces, ClickySurface{
		Key: operation.Entity, Entity: operation.Entity, Parent: operation.Parent,
		Title: title, Icon: instance.Icon, Path: instance.Path,
	})
	return meta
}
