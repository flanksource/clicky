package entity

import (
	"context"
	"net/http"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFamily(t *testing.T, name string) DynamicEntityFamily {
	t.Helper()
	t.Cleanup(func() { UnregisterDynamicEntityFamily(name) })
	return DynamicEntityFamily{
		Name: name,
		Resolve: func(_ context.Context, instance string) (DynamicEntitySpec, error) {
			return DynamicEntitySpec{Name: instance}, nil
		},
	}
}

func TestRegisterDynamicEntityFamily_ReplacesRatherThanShadows(t *testing.T) {
	family := testFamily(t, "report")
	family.Parent = "first"
	RegisterDynamicEntityFamily(family)

	family.Parent = "second"
	RegisterDynamicEntityFamily(family)

	registered := GetDynamicEntityFamilies()
	matches := 0
	for _, candidate := range registered {
		if candidate.Name == "report" {
			matches++
			assert.Equal(t, "second", candidate.Parent)
		}
	}
	assert.Equal(t, 1, matches, "one path segment cannot be owned by two families")
}

func TestRegisterDynamicEntityFamily_RejectsAnUnusableFamily(t *testing.T) {
	assert.Panics(t, func() { RegisterDynamicEntityFamily(DynamicEntityFamily{}) },
		"a family with no name owns no path")
	assert.Panics(t, func() { RegisterDynamicEntityFamily(DynamicEntityFamily{Name: "report"}) },
		"a family that cannot resolve an instance can never answer a request")
}

// The registry is read on the way into every executor request, so a consumer
// that registers no family must not pay for the feature.
func TestGetDynamicEntityFamilies_IsNilWhenEmpty(t *testing.T) {
	require.Nil(t, GetDynamicEntityFamilies())

	RegisterDynamicEntityFamily(testFamily(t, "report"))
	require.Len(t, GetDynamicEntityFamilies(), 1)

	assert.True(t, UnregisterDynamicEntityFamily("report"))
	assert.False(t, UnregisterDynamicEntityFamily("report"))
	assert.Nil(t, GetDynamicEntityFamilies())
}

func TestUnknownDynamicEntity_IsA404WithAStableCode(t *testing.T) {
	err := UnknownDynamicEntity("profile", "missing")

	assert.Equal(t, http.StatusNotFound, err.StatusCode())
	assert.Equal(t, "not_found", err.Code)
	assert.Equal(t, `no profile named "missing"`, err.Message)
}

// A family instance never enters the registry, so this is the only way its
// filters can describe themselves.
func TestResolveDynamicLookup_AnswersFromASpecThatWasNeverRegistered(t *testing.T) {
	spec := DynamicEntitySpec{
		Name: "daily",
		Filters: []DynamicFilter{{
			Key:        "env",
			Label:      "Environment",
			Searchable: true,
			Options: func(_ context.Context, _ map[string]string, query string, _ int) (map[string]api.Textable, int, error) {
				if query == "pr" {
					return map[string]api.Textable{"prod": api.Text{Content: "Production"}}, 1, nil
				}
				return map[string]api.Textable{
					"prod": api.Text{Content: "Production"},
					"dev":  api.Text{Content: "Development"},
				}, 2, nil
			},
		}},
	}

	head, err := ResolveDynamicLookup(context.Background(), spec, map[string]string{})
	require.NoError(t, err)
	response, ok := head.(entityLookupResponse)
	require.True(t, ok)
	assert.Len(t, response.Filters["env"].Options, 2)
	assert.Equal(t, "Environment", response.Filters["env"].Label)

	searched, err := ResolveDynamicLookup(context.Background(), spec, map[string]string{
		lookupFilterKeyParam: "env",
		lookupQueryParam:     "pr",
	})
	require.NoError(t, err)
	assert.Len(t, searched.(entityLookupResponse).Filters["env"].Options, 1,
		"__lookup_filter/__lookup_q reach the filter that searches on them")
}
