package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/internal/clihelp"
)

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema documentation and utilities",
		Long: `Work with clicky schema files - validate, generate examples, and view documentation.

The schema command provides tools for understanding and working with clicky's YAML schema format.`,
	}

	cmd.AddCommand(newSchemaHelpCommand())
	cmd.AddCommand(newSchemaValidateCommand())
	cmd.AddCommand(newSchemaExampleCommand())

	return cmd
}

func newSchemaHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help",
		Short: "Show detailed schema documentation",
		Long:  `Display comprehensive documentation about the clicky schema format, including all available fields, types, and options.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(clihelp.SchemaDocumentation)
		},
	}
}

func newSchemaValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a schema file",
		Long:  `Check if a schema file is valid and report any errors or warnings.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaFile := args[0]
			if _, err := formatters.LoadSchemaFromYAML(schemaFile); err != nil {
				return fmt.Errorf("schema validation failed: %w", err)
			}
			fmt.Printf("✓ Schema file '%s' is valid\n", schemaFile)
			return nil
		},
	}
}

func newSchemaExampleCommand() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "example",
		Short: "Generate an example schema file",
		Long:  `Generate a comprehensive example schema file demonstrating all available features and options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile == "" {
				fmt.Println(clihelp.ExampleSchema)
				return nil
			}
			if err := os.WriteFile(outputFile, []byte(clihelp.ExampleSchema), 0o644); err != nil {
				return fmt.Errorf("failed to write example schema: %w", err)
			}
			fmt.Printf("Example schema written to %s\n", outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for the example schema")

	return cmd
}
