// Tests that one stream registration serves both surfaces from one producer.
package entity

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamCommandRegistersItsProducer(t *testing.T) {
	parent := &cobra.Command{Use: "trace-runs"}
	cmd := AddStreamCommand(parent, "events", "Watch trace runs", func(context.Context, map[string]string, []string, StreamSend) error {
		return nil
	})

	require.NotNil(t, GetStreamFunc(cmd), "the RPC converter finds the producer through this registry")
	assert.Equal(t, "events", cmd.Name())
}

func TestStreamCommandPrintsEventsAsTheyArrive(t *testing.T) {
	parent := &cobra.Command{Use: "trace-runs"}
	cmd := AddStreamCommand(parent, "events", "Watch trace runs", func(_ context.Context, _ map[string]string, _ []string, send StreamSend) error {
		if err := send(StreamEvent{Data: map[string]string{"id": "one"}}); err != nil {
			return err
		}
		return send(StreamEvent{Name: "progress", Data: "halfway"})
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.RunE(cmd, nil))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, `{"id":"one"}`, lines[0])
	// A named event is labelled; a plain string is not re-quoted as JSON.
	assert.Equal(t, "progress\thalfway", lines[1])
}

// Interrupting a tail is how it ends, so the command must not report failure.
func TestStreamCommandTreatsAnInterruptionAsTheEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	parent := &cobra.Command{Use: "trace-runs"}
	cmd := AddStreamCommand(parent, "events", "Watch trace runs", func(ctx context.Context, _ map[string]string, _ []string, send StreamSend) error {
		if err := send(StreamEvent{Data: "first"}); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetContext(ctx)

	assert.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "first")
}
