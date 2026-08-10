package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type primaryRPCItem struct {
	ID string `json:"id"`
}

func (i primaryRPCItem) GetID() string   { return i.ID }
func (i primaryRPCItem) GetName() string { return i.ID }

type primaryRPCOptions struct {
	Profile string   `flag:"profile" default:"safe"`
	Hosts   []string `flag:"host"`
}

func (primaryRPCOptions) ClickyActionFlags() {}

func TestPrimaryActionSharesCollectionPathWithList(t *testing.T) {
	name := "primary-action-rpc-spec"
	var received primaryRPCOptions
	clicky.NewEntity[primaryRPCItem, struct{}, primaryRPCItem](name).
		List(func(struct{}) ([]primaryRPCItem, error) {
			return []primaryRPCItem{{ID: "history"}}, nil
		}).
		WithPrimaryAction(entity.PrimaryActionWithContext(primaryRPCOptions{}, func(_ context.Context, opts primaryRPCOptions) (primaryRPCItem, error) {
			received = opts
			return primaryRPCItem{ID: "started"}, nil
		})).
		Register()

	root := &cobra.Command{Use: "test"}
	clicky.GenerateCLI(root)
	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	var methods []string
	for _, operation := range service.Operations {
		if operation.Path == "/api/v1/"+name {
			methods = append(methods, operation.Method)
		}
	}
	assert.ElementsMatch(t, []string{http.MethodGet, http.MethodPost}, methods)

	var post *RPCOperation
	for i := range service.Operations {
		operation := &service.Operations[i]
		if operation.Method == http.MethodPost && operation.Path == "/api/v1/"+name {
			post = operation
			break
		}
	}
	require.NotNil(t, post)
	assert.Equal(t, name, post.Name)
	assert.Contains(t, post.Parameters, RPCParameter{Name: "profile", In: "query", Type: "string", Required: false, Default: "safe"})

	executor := NewCommandExecutor(service, &ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"})
	request := httptest.NewRequest(http.MethodPost, post.Path, strings.NewReader(`{"profile":"full","host":["one.example.test","two.example.test"]}`))
	request.Header.Set("Content-Type", "application/json")
	execution, err := executor.ExtractRequestFromHTTP(request, post)
	require.NoError(t, err)
	require.NoError(t, executor.ValidateParameters(execution, post))
	data, _, err := executor.ExecuteCommand(post, execution)
	require.NoError(t, err)
	assert.Equal(t, primaryRPCItem{ID: "started"}, data)
	assert.Equal(t, primaryRPCOptions{Profile: "full", Hosts: []string{"one.example.test", "two.example.test"}}, received)
}
