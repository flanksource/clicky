package rpc

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// responseNested is a struct field type that a component-based generator would
// emit as a $ref. Clicky inlines it instead, which is what lets a published
// schema be interpreted standalone.
type responseNested struct {
	Count int `json:"count"`
}

type responseRow struct {
	ID     string         `json:"id"`
	Name   string         `json:"name,omitempty"`
	Nested responseNested `json:"nested"`
}

type responseTagged struct {
	Region string `json:"region" clicky:"type=region-selector,order=3"`
}

// assertNoRefs walks a decoded schema and fails on any $ref key. Self-containment
// is the property that makes ResponseSchema consumable without an accompanying
// components section.
func assertNoRefs(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				t.Errorf("schema must be self-contained, found $ref = %v", child)
			}
			assertNoRefs(t, child)
		}
	case []any:
		for _, child := range typed {
			assertNoRefs(t, child)
		}
	}
}

func mustObject(t *testing.T, value any, path string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	require.True(t, ok, "%s should be an object, got %T", path, value)
	return object
}

func TestResponseSchema_StructResponse(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{
		Name:         "row-get",
		ResponseType: reflect.TypeOf(responseRow{}),
	})
	require.NoError(t, err)
	require.NotNil(t, schema)
	assertNoRefs(t, schema)

	assert.Equal(t, "object", schema["type"])
	properties := mustObject(t, schema["properties"], "properties")
	assert.Equal(t, "string", mustObject(t, properties["id"], "properties.id")["type"])

	nested := mustObject(t, properties["nested"], "properties.nested")
	assert.Equal(t, "object", nested["type"], "a nested struct is inlined, not referenced")
	assert.Contains(t, mustObject(t, nested["properties"], "properties.nested.properties"), "count")
}

func TestResponseSchema_PagedResponse(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{
		Name:          "row-list",
		ResponseType:  reflect.TypeOf(responseRow{}),
		ResponsePaged: true,
	})
	require.NoError(t, err)
	require.NotNil(t, schema)
	assertNoRefs(t, schema)

	assert.Equal(t, "object", schema["type"])
	assert.ElementsMatch(t, []any{"data", "page"}, schema["required"])

	properties := mustObject(t, schema["properties"], "properties")
	data := mustObject(t, properties["data"], "properties.data")
	assert.Equal(t, "array", data["type"])

	items := mustObject(t, data["items"], "properties.data.items")
	assert.Equal(t, "object", items["type"])
	assert.Contains(t, mustObject(t, items["properties"], "properties.data.items.properties"), "id")

	page := mustObject(t, properties["page"], "properties.page")
	assert.Equal(t, "object", page["type"])
	pageProperties := mustObject(t, page["properties"], "properties.page.properties")
	for _, key := range []string{"limit", "offset", "total"} {
		assert.Contains(t, pageProperties, key)
	}
}

func TestResponseSchema_ArrayResponse(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{
		Name:          "row-all",
		ResponseType:  reflect.TypeOf(responseRow{}),
		ResponseArray: true,
	})
	require.NoError(t, err)
	require.NotNil(t, schema)
	assertNoRefs(t, schema)

	assert.Equal(t, "array", schema["type"], "an array response stays a top-level array")
	assert.NotContains(t, schema, "properties")
	assert.Equal(t, "object", mustObject(t, schema["items"], "items")["type"])
}

func TestResponseSchema_NilResponseType(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{Name: "untyped"})
	require.NoError(t, err)
	assert.Nil(t, schema, "an operation with no declared response type publishes no schema")
}

func TestResponseSchema_EntityIDResponse(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{
		Name:             "row-list",
		ResponseType:     reflect.TypeOf(responseRow{}),
		ResponsePaged:    true,
		ResponseEntityID: true,
	})
	require.NoError(t, err)
	require.NotNil(t, schema)

	properties := mustObject(t, schema["properties"], "properties")
	data := mustObject(t, properties["data"], "properties.data")
	items := mustObject(t, data["items"], "properties.data.items")
	assert.Contains(t, mustObject(t, items["properties"], "properties.data.items.properties"), "_id")
}

func TestResponseSchema_PreservesExtensions(t *testing.T) {
	schema, err := ResponseSchema(RPCOperation{
		Name:         "tagged-get",
		ResponseType: reflect.TypeOf(responseTagged{}),
	})
	require.NoError(t, err)
	require.NotNil(t, schema)

	properties := mustObject(t, schema["properties"], "properties")
	region := mustObject(t, properties["region"], "properties.region")
	assert.Equal(t, "region-selector", region["x-clicky-component"], "vendor keys survive the marshal round-trip")
	assert.Equal(t, float64(3), region["x-clicky-order"])
}
