package clirender

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/commons/logger"
	"gopkg.in/yaml.v3"
)

// RenderDataFiles loads the PrettyObject schema from schemaFile and renders
// each JSON/YAML data file with it.
func RenderDataFiles(schemaFile string, files []string, opts Options) error {
	opts.Format.ResolveNoColor()
	opts.Format.Format = opts.Format.ResolveFormat()

	schema, err := api.NewStructParser().LoadSchemaFromYAML(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}
	opts.Format.Schema = schema
	opts.Format.Verbose = true

	manager := formatters.NewFormatManager()
	for _, dataFile := range files {
		if err := opts.formatDataFile(manager, dataFile); err != nil {
			return fmt.Errorf("error processing %s: %w", dataFile, err)
		}
	}
	return nil
}

func (o Options) formatDataFile(manager *formatters.FormatManager, dataFile string) error {
	data, err := loadDataFile(dataFile)
	if err != nil {
		return fmt.Errorf("failed to load data file: %w", err)
	}

	output, err := o.formatData(manager, data)
	if err != nil {
		return err
	}

	switch o.Format.Output {
	case "", "stdout", "-", "/dev/stdout":
		_, err = fmt.Fprint(o.Stdout, output)
		return err
	case "stderr", "/dev/stderr":
		_, err = fmt.Fprint(o.Stderr, output)
		return err
	}

	outputFile := resolveInputOutputPath(o.Format.Output, dataFile, o.Format.Format)
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}
	logger.Tracef("Output written to %s\n", outputFile)
	return nil
}

func (o Options) formatData(manager *formatters.FormatManager, data any) (string, error) {
	if o.Format.Schema == nil {
		output, err := manager.FormatWithOptions(o.Format, data)
		if err != nil {
			return "", fmt.Errorf("failed to format data: %w", err)
		}
		return output, nil
	}
	prettyData, err := api.NewStructParser().ParseDataWithSchema(data, o.Format.Schema)
	if err != nil {
		return "", fmt.Errorf("failed to parse data with schema: %w", err)
	}
	output, err := manager.FormatWithSchema(prettyData, o.Format)
	if err != nil {
		return "", fmt.Errorf("failed to format schema data: %w", err)
	}
	return output, nil
}

func loadDataFile(filename string) (any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		var jsonData any
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return jsonData, nil
	case ".yaml", ".yml":
		var yamlData any
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		return yamlData, nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}
