// Tests that an operation exchanging something other than JSON is documented as
// what it actually exchanges.
package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky/entity"
)

func mediaOperation(request, response *entity.MediaSpec) *RPCOperation {
	return &RPCOperation{
		Name: "activemq bulk-send", Path: "/api/v1/activemq/bulk-json/send", Method: "POST",
		RequestMedia: request, ResponseMedia: response,
	}
}

func TestDeclaredRequestMediaReplacesTheAssumedJSONBody(t *testing.T) {
	g := NewOpenAPIGenerator(nil)
	op := mediaOperation(&entity.MediaSpec{
		ContentType: "multipart/form-data",
		Description: "A JSON file plus mapping and send options.",
		Required:    true,
		Schema: entity.Schema{Type: "object", Properties: map[string]entity.Property{
			"file": {Type: "string", Description: "The JSON file to send"},
		}},
	}, nil)

	converted := g.convertOperationToOpenAPI(*op)

	require.NotNil(t, converted.RequestBody)
	require.Contains(t, converted.RequestBody.Content, "multipart/form-data",
		"an upload must be documented as an upload")
	assert.NotContains(t, converted.RequestBody.Content, "application/json",
		"the assumed JSON body must not be documented alongside it")
	assert.True(t, converted.RequestBody.Required)
	assert.Equal(t, "A JSON file plus mapping and send options.", converted.RequestBody.Description)
}

func TestDeclaredResponseMediaReplacesTheAssumedJSONResponse(t *testing.T) {
	g := NewOpenAPIGenerator(nil)
	op := mediaOperation(nil, &entity.MediaSpec{
		ContentType: "application/zip",
		Description: "The exported bundle.",
	})

	converted := g.convertOperationToOpenAPI(*op)

	success := converted.Responses["200"]
	require.Contains(t, success.Content, "application/zip")
	assert.NotContains(t, success.Content, "application/json")
	assert.Equal(t, "The exported bundle.", success.Description)
	// An opaque payload has no shape worth describing beyond being bytes.
	assert.Equal(t, "string", success.Content["application/zip"].Schema.Type)
	assert.Equal(t, "binary", success.Content["application/zip"].Schema.Format)
}

func TestUndeclaredMediaKeepsTheJSONDocument(t *testing.T) {
	g := NewOpenAPIGenerator(nil)

	converted := g.convertOperationToOpenAPI(*mediaOperation(nil, nil))

	require.NotNil(t, converted.RequestBody)
	assert.Contains(t, converted.RequestBody.Content, "application/json",
		"an operation that declared nothing must be documented exactly as before")
	assert.Contains(t, converted.Responses["200"].Content, "application/json")
}
