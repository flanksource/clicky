package aichat_test

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/flanksource/clicky"
	clickyaichat "github.com/flanksource/clicky/aichat"
	"github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

type toolSchemaTarget struct {
	URI string `json:"uri"`
}

type toolSchemaFlags struct {
	Version  int64              `flag:"expectedVersion" help:"Current version"`
	Content  json.RawMessage    `flag:"content" help:"Structured content"`
	Target   *toolSchemaTarget  `flag:"target" help:"Target object"`
	Links    []toolSchemaTarget `flag:"links" help:"Target list"`
	Evidence []string           `flag:"evidence" help:"Evidence URIs"`
}

func (toolSchemaFlags) ClickyActionFlags() {}

// compileToolSchema compiles a tool input schema the way captain admits caller
// tools: against the JSON Schema 2020-12 metaschema.
func compileToolSchema(name string, schema map[string]any) error {
	raw, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	url := "mem:///spec/" + name + "/input-schema.json"
	if err := compiler.AddResource(url, document); err != nil {
		return err
	}
	_, err = compiler.Compile(url)
	return err
}

var _ = Describe("tool input schemas", func() {
	It("publishes structured action flags as schemas valid against JSON Schema 2020-12", func(ctx SpecContext) {
		const entityName = "tool-schema-spec"
		clicky.NewEntity[toolTodo, toolTodoListOptions, toolTodo](entityName).
			List(func(toolTodoListOptions) ([]toolTodo, error) { return nil, nil }).
			WithAction(entity.TypedActionWithContext("annotate", toolSchemaFlags{}, func(_ context.Context, id string, _ toolSchemaFlags) (toolTodo, error) {
				return toolTodo{ID: id}, nil
			}).WithMethod("POST")).
			Register()
		root := &cobra.Command{Use: "example"}
		clicky.GenerateCLI(root)
		provider, err := clickyaichat.NewCobraToolProvider(clickyaichat.CobraToolProviderOptions{Root: root})
		Expect(err).NotTo(HaveOccurred())
		set, err := provider.ToolSet(ctx)
		Expect(err).NotTo(HaveOccurred())

		var annotate map[string]any
		for _, definition := range set.Definitions {
			Expect(compileToolSchema(definition.Name, definition.InputSchema)).To(Succeed(), definition.Name)
			if definition.Name == entityName+"_annotate" {
				annotate = definition.InputSchema
			}
		}
		Expect(annotate).NotTo(BeNil())
		Expect(annotate["properties"]).To(MatchKeys(IgnoreExtras, Keys{
			"content":         Equal(map[string]any{"description": "Structured content"}),
			"target":          HaveKeyWithValue("type", "object"),
			"links":           HaveKeyWithValue("type", "array"),
			"evidence":        HaveKeyWithValue("type", "array"),
			"expectedVersion": HaveKeyWithValue("type", "integer"),
		}))
	})
})
