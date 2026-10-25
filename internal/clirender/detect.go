package clirender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/flanksource/clicky"
	md "github.com/flanksource/clicky/markdown"
	"gopkg.in/yaml.v3"
)

func parseInputData(data []byte, sourceName, inputFormat string) (any, string, error) {
	format := inputFormat
	if format == "auto" {
		format = detectInputFormat(data, sourceName)
	}

	switch format {
	case "markdown":
		doc, err := md.Parse(data, md.WithFilename(sourceName))
		if err != nil {
			return nil, format, fmt.Errorf("failed to parse markdown: %w", err)
		}
		return doc, format, nil
	case "json":
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, format, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return value, format, nil
	case "yaml":
		var value any
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, format, fmt.Errorf("failed to parse YAML: %w", err)
		}
		return value, format, nil
	case "text":
		return clicky.Text(string(data)), format, nil
	default:
		return nil, format, fmt.Errorf("unsupported input format %q", inputFormat)
	}
}

func normalizeInputFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "auto":
		return "auto", nil
	case "markdown", "md":
		return "markdown", nil
	case "json":
		return "json", nil
	case "yaml", "yml":
		return "yaml", nil
	case "text", "txt", "plain":
		return "text", nil
	default:
		return "", fmt.Errorf("unsupported --input-format %q (supported: auto, markdown/md, json, yaml/yml, text)", format)
	}
}

func detectInputFormat(data []byte, sourceName string) string {
	if byExt := inputFormatFromExtension(sourceName); byExt != "" {
		return byExt
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "text"
	}
	if json.Valid(trimmed) {
		return "json"
	}
	if looksLikeMarkdown(trimmed) {
		return "markdown"
	}
	if looksLikeYAML(trimmed) {
		return "yaml"
	}
	return "text"
}

func inputFormatFromExtension(sourceName string) string {
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".txt", ".text", ".log":
		return "text"
	default:
		return ""
	}
}

func looksLikeMarkdown(data []byte) bool {
	text := string(data)
	if strings.HasPrefix(text, "---\n") && strings.Contains(text[4:], "\n---") {
		return true
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "#"):
			return true
		case strings.HasPrefix(trimmed, ">"):
			return true
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			return true
		case strings.HasPrefix(trimmed, "!!! "):
			return true
		case strings.HasPrefix(trimmed, "<details"), strings.HasPrefix(trimmed, "<section"), strings.HasPrefix(trimmed, "<table"):
			return true
		case strings.HasPrefix(trimmed, "[^") && strings.Contains(trimmed, "]:"):
			return true
		case strings.HasPrefix(trimmed, "- [ ] "), strings.HasPrefix(trimmed, "- [x] "), strings.HasPrefix(trimmed, "- [X] "):
			return true
		case strings.HasPrefix(trimmed, "|") && i+1 < len(lines) && strings.Contains(lines[i+1], "---"):
			return true
		case isOrderedMarkdownList(trimmed):
			return true
		}
	}
	return false
}

func isOrderedMarkdownList(line string) bool {
	dot := strings.Index(line, ".")
	if dot <= 0 || dot >= len(line)-1 {
		return false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return line[dot+1] == ' '
}

func looksLikeYAML(data []byte) bool {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return false
	}
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}
