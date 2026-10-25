package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/schemaform"
)

// newPromptRunCommand mirrors `captain prompt run <name>`: the prompt name
// completes from the available prompts, --model completes from a catalog that
// depends on --effort, and --file completes files (not a choice).
func newPromptRunCommand() *cobra.Command {
	root := &cobra.Command{Use: "captain"}
	run := &cobra.Command{
		Use:  "run <name>",
		Args: cobra.ExactArgs(1),
		RunE: func(*cobra.Command, []string) error { return nil },
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return []cobra.Completion{"review\tCode review", "summarize\tSummarize a diff"}, cobra.ShellCompDirectiveNoFileComp
		},
	}
	run.Flags().String("effort", "", "Reasoning effort")
	flags.SetAllowedValues(run.Flags().Lookup("effort"), "high", "low")
	run.Flags().String("model", "", "Model")
	_ = run.RegisterFlagCompletionFunc("model", func(cmd *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if effort, _ := cmd.Flags().GetString("effort"); effort == "low" {
			return []cobra.Completion{"haiku"}, cobra.ShellCompDirectiveNoFileComp
		}
		return []cobra.Completion{"opus", "sonnet"}, cobra.ShellCompDirectiveNoFileComp
	})
	run.Flags().String("file", "", "Prompt file")
	_ = run.RegisterFlagCompletionFunc("file", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return []cobra.Completion{"md"}, cobra.ShellCompDirectiveFilterFileExt
	})
	root.AddCommand(run)
	return run
}

type formPrompt struct{ ID, Name, Description string }

func (p formPrompt) GetID() string          { return p.ID }
func (p formPrompt) GetName() string        { return p.Name }
func (p formPrompt) GetDescription() string { return p.Description }

// An optional-id entity action (`prompt run [id]`) takes no id over HTTP, so
// its operation omits it; the interactive form adds it back and completes it
// from the entity's list.
func TestInteractiveOffersAnOptionalIDFromTheEntityList(t *testing.T) {
	clicky.RegisterEntity(clicky.Entity[formPrompt, struct{}, any]{
		Name: "form-prompt",
		List: func(struct{}) ([]formPrompt, error) {
			return []formPrompt{{ID: "p-review", Name: "review", Description: "Reviews a diff"}, {ID: "p-summarize", Name: "summarize"}}, nil
		},
		Actions: []clicky.EntityAction{
			clicky.ActionWithContext("run", func(context.Context, string, map[string]string) (any, error) { return nil, nil }).WithOptionalID(),
		},
	})
	root := &cobra.Command{Use: "app"}
	clicky.GenerateCLI(root)
	cmd, _, err := root.Find([]string{"form-prompt", "run"})
	require.NoError(t, err)

	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	require.NoError(t, err)
	for _, param := range op.Parameters {
		require.NotEqual(t, "id", param.Name, "the HTTP operation has no id parameter")
	}
	withPositional(cmd, op)
	require.Equal(t, "id", op.Parameters[0].Name)

	completions, err := commandCompletions(cmd, *op, nil)
	require.NoError(t, err)
	require.Contains(t, completions, "id")
	options, err := completions["id"](context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, []schemaform.Option{
		{Value: "p-review", Label: "review", Raw: "p-review", Description: "Reviews a diff"},
		{Value: "p-summarize", Label: "summarize", Raw: "p-summarize"},
	}, options)
}

type formPromptRunFlags struct {
	Vars string `flag:"vars" help:"JSON object of template variables"`
}

func (formPromptRunFlags) ClickyActionFlags() {}

// A flag declared with WithActionFlagSchema (prompt run --vars) is filled in
// the form with the fields of the schema loaded for the chosen prompt, and
// reaches the command as JSON.
func TestFlagSchemaFillsAJSONFlagFromTheChosenPrompt(t *testing.T) {
	var loaded []map[string]string
	clicky.RegisterEntity(clicky.Entity[formPrompt, struct{}, any]{
		Name: "vars-prompt",
		List: func(struct{}) ([]formPrompt, error) {
			return []formPrompt{{ID: "p-review", Name: "review"}}, nil
		},
		Actions: []clicky.EntityAction{
			clicky.WithActionFlagSchema(
				clicky.ActionWithFlagsAndContext("run", formPromptRunFlags{}, func(context.Context, string, map[string]string) (any, error) { return nil, nil }).WithOptionalID(),
				"vars", clicky.FlagSchema{Scope: []string{"id"}, Load: func(_ context.Context, values map[string]string) (json.RawMessage, error) {
					loaded = append(loaded, values)
					return json.RawMessage(`{"type":"object","required":["diff"],"properties":{"diff":{"type":"string"}}}`), nil
				}}),
		},
	})
	root := &cobra.Command{Use: "app"}
	clicky.GenerateCLI(root)
	cmd, _, err := root.Find([]string{"vars-prompt", "run"})
	require.NoError(t, err)

	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	require.NoError(t, err)
	withPositional(cmd, op)
	completions, err := commandCompletions(cmd, *op, nil)
	require.NoError(t, err)
	schemas := entity.GetFlagSchemas(cmd)
	schema, err := OperationInputSchema(*op, OperationSchemaOptions{Completions: []string{"id"}, Positional: []string{"id"}, FlagSchemas: flagSchemaScopes(schemas)})
	require.NoError(t, err)

	var output bytes.Buffer
	values, err := schemaform.Run(context.Background(), schema, schemaform.Options{
		Accessible: true,
		Input:      strings.NewReader("2\nHEAD~1\n"), // id: (none), review — then the prompt's diff
		Output:     &output,
		Lookup:     formLookup(*op, nil, completions),
		LoadSchema: flagSchemaLoader(schemas),
	})
	require.NoError(t, err, output.String())
	assert.Equal(t, []map[string]string{{"id": "p-review"}}, loaded)

	args, err := applyInteractive(cmd, *op, nil, values)
	require.NoError(t, err)
	assert.Equal(t, []string{"p-review"}, args)
	assert.JSONEq(t, `{"diff":"HEAD~1"}`, cmd.Flags().Lookup("vars").Value.String())
}

func TestShellCompletionsBecomeFormLookups(t *testing.T) {
	cmd := newPromptRunCommand()
	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	require.NoError(t, err)

	completions, err := commandCompletions(cmd, *op, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(completions))
	for name := range completions {
		names = append(names, name)
	}
	assert.ElementsMatch(t, []string{"name", "model"}, names, "file completion offers files, not choices")

	schema, err := OperationInputSchema(*op, OperationSchemaOptions{Completions: names})
	require.NoError(t, err)
	var parsed struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schema, &parsed))
	assert.Equal(t, map[string]any{"filter": "name"}, parsed.Properties["name"]["x-clicky-lookup"],
		"a completion lookup has no URL, so the web renders it as a plain field")

	var output bytes.Buffer
	values, err := schemaform.Run(context.Background(), schema, schemaform.Options{
		Accessible: true,
		Input: strings.NewReader(strings.Join([]string{
			"2",        // name (required by ExactArgs(1)): review, summarize
			"y",        // configure the optional flags
			"3",        // effort: (none), high, low
			"notes.md", // file: plain text
			"2",        // model: (none), haiku — narrowed by effort=low
		}, "\n") + "\n"),
		Output: &output,
		Lookup: formLookup(*op, nil, completions),
	})
	require.NoError(t, err, output.String())
	assert.Equal(t, map[string]any{"name": "summarize", "effort": "low", "file": "notes.md", "model": "haiku"}, values)
	assert.Contains(t, output.String(), "Summarize a diff", "a positional is labelled by its description")
}
