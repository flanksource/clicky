package rpc

import (
	"encoding/json"
	"net/http"

	"github.com/flanksource/clicky"
)

// EntityOperationDTO is the JSON shape of a registered CRUD operation.
type EntityOperationDTO struct {
	Verb string `json:"verb"`
}

// EntityActionDTO is the JSON shape of a registered action or bulk action.
type EntityActionDTO struct {
	Name  string `json:"name"`
	Short string `json:"short,omitempty"`
}

// EntityDTO is the JSON-serialisable projection of clicky.EntityInfo.
// reflect.Type and function fields are stripped so the registry can be
// exposed to HTTP clients (typically the web UI building its sidebar).
type EntityDTO struct {
	Name        string               `json:"name"`
	Aliases     []string             `json:"aliases,omitempty"`
	Parent      string               `json:"parent,omitempty"`
	IsAdmin     bool                 `json:"is_admin,omitempty"`
	Operations  []EntityOperationDTO `json:"operations,omitempty"`
	Actions     []EntityActionDTO    `json:"actions,omitempty"`
	BulkActions []EntityActionDTO    `json:"bulk_actions,omitempty"`
}

// EntitySnapshot returns a serialisable projection of the current entity
// registry. Callers that need to embed the registry in something other
// than the HTTP handler (tests, custom routes) can call this directly.
func EntitySnapshot() []EntityDTO {
	entities := clicky.GetEntities()
	out := make([]EntityDTO, 0, len(entities))
	for _, e := range entities {
		dto := EntityDTO{
			Name:    e.Name,
			Aliases: e.Aliases,
			Parent:  e.Parent,
			IsAdmin: e.IsAdmin,
		}
		for _, op := range e.Operations {
			dto.Operations = append(dto.Operations, EntityOperationDTO{Verb: op.Verb})
		}
		for _, a := range e.Actions {
			dto.Actions = append(dto.Actions, EntityActionDTO{Name: a.Name, Short: a.Short})
		}
		for _, b := range e.BulkActions {
			dto.BulkActions = append(dto.BulkActions, EntityActionDTO{Name: b.Name, Short: b.Short})
		}
		out = append(out, dto)
	}
	return out
}

// handleEntities serves the entity registry snapshot as JSON. Mounted at
// /api/entities by SwaggerServer.RegisterRoutes so UIs can enumerate
// registered entities without parsing the OpenAPI spec.
func (s *SwaggerServer) handleEntities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(EntitySnapshot()); err != nil {
		http.Error(w, "failed to encode entities: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
