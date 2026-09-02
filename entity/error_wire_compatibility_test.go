package entity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusErrorWritePreservesLegacyWireShape(t *testing.T) {
	response := httptest.NewRecorder()

	NewStatusError(http.StatusConflict, "connection_required", "select a connection").Write(response)

	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.Empty(t, response.Header().Get("X-Trace-ID"))
	assert.JSONEq(t, `{"code":"connection_required","message":"select a connection"}`, response.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{
		"code":    "connection_required",
		"message": "select a connection",
	}, body)
}
