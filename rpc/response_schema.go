package rpc

import (
	"encoding/json"
	"fmt"
)

// ResponseSchema returns op's response body as a plain JSON-schema map, or nil
// when the operation declares no static response type. It is the public entry
// point for consumers that publish a schema alongside a tool or endpoint — the
// chat tool catalog's outputSchema — instead of embedding it in a full OpenAPI
// document.
//
// The result is self-contained: reflection inlines every nested type, so the map
// never carries a $ref and can be interpreted standalone.
//
// A nil map is the honest signal that the operation declares no typed response.
// Callers publish nothing rather than substitute a placeholder, and must not
// coerce the result to an object — a ResponseArray operation is legitimately a
// top-level array.
func ResponseSchema(op RPCOperation) (map[string]any, error) {
	if op.ResponseType == nil {
		return nil, nil
	}
	return schemaToMap(NewOpenAPIGenerator(nil).responseSchemaForOperation(op))
}

// schemaToMap round-trips a schema through JSON. The detour is deliberate:
// OpenAPISchema.MarshalJSON merges the Extensions map (x-clicky-*) into the
// object, so marshalling is the only conversion that preserves vendor keys.
// Mirrors schemaFromMap, which converts in the opposite direction.
func schemaToMap(schema *OpenAPISchema) (map[string]any, error) {
	if schema == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal response schema: %w", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, fmt.Errorf("decode response schema: %w", err)
	}
	return decoded, nil
}
