package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
	"github.com/spf13/cobra"
)

// Build information (set by goreleaser)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var schemaFile string
	var options formatters.FormatOptions

	rootCmd := &cobra.Command{
		Use:   "clicky",
		Short: "A CLI tool for formatting structured data using YAML schema definitions",
		Long: `Clicky is a flexible CLI tool that formats structured data (JSON, YAML, etc.)
using YAML schema definitions. It supports multiple output formats including
pretty-printed tables, HTML, PDF, Markdown, and more.

For backward compatibility, you can use the root command directly, or use the
'pretty' subcommand explicitly.`,
		Example: `  clicky --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  clicky version`,
		Args: func(cmd *cobra.Command, args []string) error {
			// If no subcommand and no args, show help
			if len(args) == 0 && schemaFile == "" {
				return fmt.Errorf("requires either a subcommand or data files with --schema flag")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no args but subcommands exist, user probably wants help
			if len(args) == 0 {
				return cmd.Help()
			}

			// Backward compatibility: behave like the old clicky
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required when using data files")
			}

			// Resolve format from format-specific flags
			if err := options.ResolveFormat(); err != nil {
				return err
			}

			// Create schema formatter
			schemaFormatter, err := clicky.LoadSchemaFromYAML(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}

			// Set verbose to true for CLI usage
			options.Verbose = true

			// Format all files
			err = schemaFormatter.FormatFiles(args, options)
			if err != nil {
				return fmt.Errorf("error formatting files: %w", err)
			}

			return nil
		},
	}

	// Add flags to root command for backward compatibility
	rootCmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema")
	formatters.BindPFlags(rootCmd.Flags(), &options)

	// Add subcommands
	rootCmd.AddCommand(newPrettyCommand())
	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newSchemaCommand())
	// TODO: Re-enable MCP command after fixing compatibility issues
	// rootCmd.AddCommand(mcp.NewCommand())

	return rootCmd
}

func newPrettyCommand() *cobra.Command {
	var schemaFile string
	var options formatters.FormatOptions

	cmd := &cobra.Command{
		Use:   "pretty [flags] <data-file1> [data-file2...]",
		Short: "Format data files using a YAML schema",
		Long: `Format structured data files (JSON, YAML, etc.) using a YAML schema definition.

The pretty command is the main functionality of clicky, allowing you to transform
raw data into beautifully formatted output using customizable schemas.`,
		Example: `  clicky pretty --schema order-schema.yaml order1.json order2.yaml
  clicky pretty --schema user-schema.yaml --format html --output reports/ users.json
  clicky pretty --schema product-schema.yaml --format csv products.json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaFile == "" {
				return fmt.Errorf("--schema flag is required")
			}

			// Resolve format from format-specific flags
			if err := options.ResolveFormat(); err != nil {
				return err
			}

			// Create schema formatter
			schemaFormatter, err := clicky.LoadSchemaFromYAML(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}

			// Set verbose to true for CLI usage
			options.Verbose = true

			// Format all files
			err = schemaFormatter.FormatFiles(args, options)
			if err != nil {
				return fmt.Errorf("error formatting files: %w", err)
			}

			return nil
		},
	}

	// Add schema flag
	cmd.Flags().StringVar(&schemaFile, "schema", "", "YAML file containing PrettyObject schema (required)")
	cmd.MarkFlagRequired("schema")

	// Add formatting flags using the new BindPFlags function
	formatters.BindPFlags(cmd.Flags(), &options)

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getVersionInfo())
		},
	}
}

func getVersionInfo() string {
	return fmt.Sprintf("clicky-schema %s (commit: %s, built: %s, go: %s)",
		version, commit, date, runtime.Version())
}

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema documentation and utilities",
		Long: `Work with clicky schema files - validate, generate examples, and view documentation.

The schema command provides tools for understanding and working with clicky's YAML schema format.`,
	}

	// Add subcommands
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
			fmt.Println(getSchemaDocumentation())
		},
	}
}

func newSchemaValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <schema-file>",
		Short: "Validate a schema file",
		Long:  `Check if a schema file is valid and report any errors or warnings.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaFile := args[0]

			// Try to load the schema
			_, err := clicky.LoadSchemaFromYAML(schemaFile)
			if err != nil {
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
			example := getExampleSchema()

			if outputFile != "" {
				// Write to file
				err := os.WriteFile(outputFile, []byte(example), 0644)
				if err != nil {
					return fmt.Errorf("failed to write example schema: %w", err)
				}
				fmt.Printf("Example schema written to %s\n", outputFile)
			} else {
				// Print to stdout
				fmt.Println(example)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for the example schema")

	return cmd
}

func getSchemaDocumentation() string {
	return `CLICKY SCHEMA DOCUMENTATION
===========================

The clicky schema is a YAML file that defines how to format and display structured data.

## Basic Structure

fields:
  - name: "field_name"        # Required: name of the field in your data
    type: "string"             # Optional: field type (string, int, float, boolean, struct, array)
    format: "format_type"      # Optional: special formatting (currency, date, table, etc.)
    style: "tailwind_classes"  # Optional: Tailwind CSS classes for styling
    label: "Display Label"     # Optional: custom label for display

## Field Types

- string: Text values
- int: Integer numbers
- float: Decimal numbers
- boolean: true/false values
- struct: Nested objects
- array: Lists of items
- date: Date/time values

## Format Types

- currency: Format as currency (e.g., $1,234.56)
- date: Format as date/time
- float: Format with specific decimal places
- table: Display array as a table
- tree: Display as a tree structure

## Styling

Use Tailwind CSS classes for styling:
- Colors: text-red-500, bg-blue-100
- Typography: font-bold, italic, uppercase
- Spacing: px-2, py-1
- Decorations: underline, line-through

## Color Options

Dynamic coloring based on values:

color_options:
  green: "completed"    # Use green when value is "completed"
  red: "failed"         # Use red when value is "failed"
  yellow: ">= 50"       # Use yellow for numeric comparisons

## Table Options

For array fields with format: "table":

table_options:
  title: "Table Title"
  header_style: "bg-blue-50 font-bold"
  fields:
    - name: "column1"
      type: "string"
      style: "text-gray-700"

## Format Options

Additional formatting parameters:

format_options:
  format: "epoch"       # For dates: parse from epoch timestamp
  digits: "2"           # For floats: decimal places
  sort: "field_name"    # For tables: sort by field
  dir: "desc"           # Sort direction: asc/desc

## Nested Fields

For struct types, define nested fields:

fields:
  - name: "address"
    type: "struct"
    fields:
      - name: "street"
        type: "string"
      - name: "city"
        type: "string"

## Example Usage

clicky --schema my-schema.yaml data.json
clicky pretty --schema my-schema.yaml --format html data.json
clicky schema validate my-schema.yaml
clicky schema example -o example-schema.yaml
`
}

func getExampleSchema() string {
	return `# Example Clicky Schema
# This demonstrates all available features

fields:
  # Simple string field with styling
  - name: "id"
    type: "string"
    style: "text-blue-600 font-bold"
    label: "Order ID"

  # Nested struct field
  - name: "customer"
    type: "struct"
    style: "text-gray-700"
    fields:
      - name: "name"
        type: "string"
        style: "font-semibold"
      - name: "email"
        type: "string"
        style: "text-blue-500 underline"
      - name: "account_type"
        type: "string"
        style: "uppercase"
        color_options:
          green: "premium"
          yellow: "standard"
          gray: "basic"

  # Field with dynamic coloring
  - name: "status"
    type: "string"
    style: "font-bold uppercase"
    color_options:
      green: "completed"
      yellow: "processing"
      orange: "pending"
      red: "cancelled"

  # Currency field
  - name: "total_amount"
    type: "float"
    format: "currency"
    style: "text-green-600 font-bold text-lg"

  # Date field with epoch format
  - name: "order_date"
    type: "string"
    format: "date"
    style: "text-indigo-600"
    format_options:
      format: "epoch"

  # Array displayed as table
  - name: "items"
    type: "array"
    format: "table"
    format_options:
      sort: "price"
      dir: "desc"
    table_options:
      title: "Order Items"
      header_style: "bg-blue-50 text-blue-900 font-bold uppercase"
      fields:
        - name: "product_name"
          type: "string"
          style: "font-medium"
        - name: "quantity"
          type: "int"
          style: "text-center"
        - name: "price"
          type: "float"
          format: "currency"
          style: "text-green-600"
        - name: "discount"
          type: "float"
          format: "float"
          format_options:
            digits: "1"
          style: "text-red-500"
        - name: "warranty_months"
          type: "int"
          color_options:
            green: ">=36"
            yellow: ">=24"
            red: "<24"

  # Field with multiple style classes
  - name: "alert_message"
    type: "string"
    style: "uppercase text-red-600 bg-red-100 font-bold underline px-2 py-1 rounded"

  # Boolean field
  - name: "is_expedited"
    type: "boolean"
    style: "font-semibold"
    color_options:
      green: "true"
      gray: "false"

  # Tree structure field
  - name: "category_tree"
    type: "struct"
    format: "tree"
    tree_options:
      label_field: "name"
      children_field: "subcategories"
      style: "text-gray-700"
`
}
