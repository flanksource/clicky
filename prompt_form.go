package clicky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/flanksource/clicky/prompt"
	"github.com/flanksource/clicky/schemaform"
)

// ErrPromptCancelled is returned by PromptForm when the user cancels the form.
var ErrPromptCancelled = errors.New("prompt cancelled")

// PromptFormOptions configures PromptForm.
type PromptFormOptions struct {
	Title       string
	Description string
	// Schema is a JSON Schema object: the same document clicky-ui's
	// JsonSchemaForm renders.
	Schema  json.RawMessage
	Default map[string]any
	// Lookup resolves x-clicky-lookup fields on the terminal; the dashboard
	// resolves them itself from the lookup URL.
	Lookup schemaform.Lookup
	// LoadSchema loads x-clicky-schema properties on the terminal; the dashboard
	// renders such a property as it is declared.
	LoadSchema schemaform.SchemaLoader
	// Accessible forces the line-based prompt (also selected by
	// ACCESSIBLE=1, huh's convention).
	Accessible bool
}

// PromptForm collects values for a JSON Schema form. It follows the routing of
// the other prompts: a prompt raised under a prompt.Scope with a sink installed
// goes to the dashboard (rendered by JsonSchemaForm); otherwise a terminal gets
// the huh form (schemaform); with neither, it fails.
func PromptForm(ctx context.Context, opts PromptFormOptions) (map[string]any, error) {
	if prompt.PreferSink(ctx) {
		return promptFormViaSink(ctx, prompt.GlobalManager(), opts)
	}
	session, err := newPromptSession()
	if err != nil {
		if m := prompt.GlobalManager(); m != nil {
			return promptFormViaSink(ctx, m, opts)
		}
		return nil, err
	}
	defer session.Close()

	values, err := schemaform.Run(ctx, opts.Schema, schemaform.Options{
		Title:       opts.Title,
		Description: opts.Description,
		Default:     opts.Default,
		Lookup:      opts.Lookup,
		LoadSchema:  opts.LoadSchema,
		Input:       session.terminal.input,
		Output:      session.terminal.output,
		Accessible:  opts.Accessible || accessibleEnv(),
		Theme:       huh.ThemeCharm(),
	})
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, ErrPromptCancelled
	}
	return values, err
}

func accessibleEnv() bool { return os.Getenv("ACCESSIBLE") != "" }

func promptFormViaSink(ctx context.Context, m *prompt.Manager, opts PromptFormOptions) (map[string]any, error) {
	values, ok, err := m.Form(ctx, prompt.FormOptions{
		Title:       opts.Title,
		Description: opts.Description,
		Schema:      opts.Schema,
		Default:     opts.Default,
	})
	if err != nil {
		return nil, fmt.Errorf("prompt %q: %w", opts.Title, err)
	}
	if !ok {
		return nil, ErrPromptCancelled
	}
	return values, nil
}
