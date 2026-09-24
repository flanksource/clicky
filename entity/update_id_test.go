package entity

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The positional <id> names the record an update acts on; a body field must
// not silently retarget the call to a different record.
func TestUpdatePositionalIDIsAuthoritative(t *testing.T) {
	previous := ParseArgs
	t.Cleanup(func() { ParseArgs = previous })
	ParseArgs = func(args []string) (map[string]any, error) {
		parsed := map[string]any{}
		for _, arg := range args {
			key, value, _ := strings.Cut(arg, "=")
			parsed[key] = value
		}
		return parsed, nil
	}

	var received map[string]string
	parent := &cobra.Command{Use: "todos"}
	generateBodyCommand(parent, "update", "Update a todo", EntityOperation{
		Verb: "update",
		DataFunc: func(flagMap map[string]string, _ []string) (any, error) {
			received = flagMap
			return nil, nil
		},
	})
	update, _, err := parent.Find([]string{"update"})
	require.NoError(t, err)

	err = update.RunE(update, []string{"todo-1", "id=todo-2"})
	require.ErrorContains(t, err, "todo-2")
	assert.Nil(t, received, "a conflicting id must be rejected before the update runs")

	require.NoError(t, update.RunE(update, []string{"todo-1", "id=todo-1", "status=done"}))
	assert.Equal(t, map[string]string{"id": "todo-1", "status": "done"}, received)
}
