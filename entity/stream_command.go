// Registration for streaming commands: one registration serves the CLI, the
// generated REST surface and the OpenAPI document from the same producer.
package entity

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"
)

// streamFuncRegistry maps cobra commands to their event producers. The RPC
// converter reads it to wire RPCOperation.StreamFunc, the same way the data and
// lookup registries wire theirs.
var streamFuncRegistry sync.Map // map[*cobra.Command]StreamFunc

// GetStreamFunc returns the event producer registered for a command, if any.
func GetStreamFunc(cmd *cobra.Command) StreamFunc {
	if v, ok := streamFuncRegistry.Load(cmd); ok {
		return v.(StreamFunc)
	}
	return nil
}

// AddStreamCommand registers a streaming operation under parent: a producer of
// events over time rather than one answer.
//
// Over HTTP it is served as server-sent events; on the CLI the same producer
// prints each event as it arrives, so `app trace-runs events` tails what a
// browser subscribing to the route would see. The route follows the usual
// naming ("trace-runs events" serves /api/v1/trace-runs/events) and can be
// overridden with the clicky/operation-path annotation like any other.
//
// The producer must stop when the context ends — that is the client hanging up,
// or the CLI being interrupted.
func AddStreamCommand(parent *cobra.Command, name, short string, fn StreamFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(c *cobra.Command, args []string) error {
			return streamToWriter(c.Context(), c.OutOrStdout(), changedFlagMap(c), args, fn)
		},
	}
	streamFuncRegistry.Store(cmd, fn)
	parent.AddCommand(cmd)
	return cmd
}

// streamToWriter prints events as they arrive. The CLI has no framing to
// preserve, so each event is one line: its name, when it has one, then its data.
func streamToWriter(ctx context.Context, out io.Writer, flags map[string]string, args []string, fn StreamFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	err := fn(ctx, flags, args, func(event StreamEvent) error {
		line, err := streamLine(event)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(out, line); err != nil {
			return err
		}
		if flusher, ok := out.(interface{ Flush() error }); ok {
			return flusher.Flush()
		}
		return nil
	})
	// The caller interrupting is how a tail ends, not a failure to report.
	if err != nil && ctx.Err() != nil {
		return nil
	}
	return err
}

// streamLine renders one event for a terminal.
func streamLine(event StreamEvent) (string, error) {
	payload, err := StreamText(event.Data)
	if err != nil {
		return "", err
	}
	if event.Name != "" {
		return fmt.Sprintf("%s\t%s\n", event.Name, payload), nil
	}
	return payload + "\n", nil
}
