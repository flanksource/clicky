//go:build ignore

// Interactive forms over JSON Schema, on the terminal.
//
//	go run interactive_form.go deploy -i                # fill deploy's flags and args in a form
//	go run interactive_form.go deploy checkout -i --env prod   # the form starts from what you typed
//	go run interactive_form.go configure                # a schema with a discriminator and listeners
//	ACCESSIBLE=1 go run interactive_form.go configure   # line-based prompts (screen readers, pipes)
//
// `deploy -i` comes from extensions.CobraExtensions(root).Interactive(): any
// runnable command gets the flag, and its form is built from the same
// operation schema the OpenAPI document and clicky-ui's CommandForm use.
// `configure` calls clicky.PromptForm directly with a schema clicky-ui's
// JsonSchemaForm would render identically.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/flags"
	"github.com/spf13/cobra"
)

const notificationSchema = `{
  "type": "object",
  "x-discriminator": "kind",
  "required": ["kind", "name"],
  "properties": {
    "kind": {
      "type": "string",
      "title": "Channel",
      "enum": ["webhook", "email"],
      "x-enum-labels": {"webhook": "Webhook", "email": "Email"}
    },
    "name": {"type": "string", "title": "Name", "minLength": 3},
    "severity": {
      "type": "string",
      "title": "Minimum severity",
      "enum": ["info", "warning", "critical"],
      "default": "warning",
      "x-on-change": [
        {"when": {"const": "critical"}, "show": ["pager"], "require": ["pager"], "else": {"hide": ["pager"], "reset": ["pager"]}}
      ]
    },
    "pager": {"type": "string", "title": "Pager rotation"},
    "retries": {"type": "integer", "title": "Retries", "minimum": 0, "maximum": 5, "default": 2},
    "labels": {"type": "object", "title": "Labels", "additionalProperties": {"type": "string"}}
  },
  "allOf": [
    {"if": {"properties": {"kind": {"const": "webhook"}}, "required": ["kind"]},
     "then": {"properties": {"url": {"type": "string", "title": "URL", "pattern": "^https?://"}}, "required": ["url"]}},
    {"if": {"properties": {"kind": {"const": "email"}}, "required": ["kind"]},
     "then": {"properties": {"to": {"type": "array", "title": "Recipients", "minItems": 1, "items": {"type": "string"}}}, "required": ["to"]}}
  ]
}`

func main() {
	root := &cobra.Command{Use: "interactive-form", Short: "huh forms over JSON Schema"}
	root.AddCommand(deployCommand(), configureCommand())
	extensions.CobraExtensions(root).Interactive()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func deployCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy <service>",
		Short: "Deploy a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("env")
			replicas, _ := cmd.Flags().GetInt("replicas")
			tags, _ := cmd.Flags().GetStringSlice("tag")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			fmt.Printf("deploying %s to %s with %d replicas, tags=%v, dry-run=%v\n", args[0], env, replicas, tags, dryRun)
			return nil
		},
	}
	cmd.Flags().String("env", "", "Target environment")
	flags.SetAllowedValues(cmd.Flags().Lookup("env"), "dev", "staging", "prod")
	_ = cmd.MarkFlagRequired("env")
	cmd.Flags().Int("replicas", 1, "Replica count")
	cmd.Flags().StringSlice("tag", nil, "Release tags")
	cmd.Flags().Bool("dry-run", false, "Print the plan without applying it")
	return cmd
}

func configureCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Configure a notification channel",
		RunE: func(cmd *cobra.Command, _ []string) error {
			values, err := clicky.PromptForm(cmd.Context(), clicky.PromptFormOptions{
				Title:       "Notification channel",
				Description: "Where alerts are delivered",
				Schema:      json.RawMessage(notificationSchema),
			})
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(values, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		},
	}
}
