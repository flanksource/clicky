package clirender

import (
	"path/filepath"
	"strings"
)

// resolveInputOutputPath expands a `*` wildcard or a directory output into a
// file named after sourceName with the format's extension.
func resolveInputOutputPath(outputPattern, sourceName, format string) string {
	if strings.Contains(outputPattern, "*") || filepath.Ext(outputPattern) == "" {
		base := "stdin"
		if sourceName != "" && sourceName != "stdin" {
			base = strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
		}
		ext := getOutputExtension(format)
		if strings.Contains(outputPattern, "*") {
			resolved := strings.ReplaceAll(outputPattern, "*", base)
			if filepath.Ext(resolved) == "" {
				resolved += ext
			}
			return resolved
		}
		return filepath.Join(outputPattern, base+ext)
	}
	return outputPattern
}

func isFileOnlyFormat(format string) bool {
	switch strings.ToLower(format) {
	case "excel", "xlsx":
		return true
	default:
		return false
	}
}

func getOutputExtension(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return ".json"
	case "clicky-json":
		return ".clicky.json"
	case "yaml", "yml":
		return ".yaml"
	case "csv":
		return ".csv"
	case "html", "html-react", "html-static":
		return ".html"
	case "markdown", "md":
		return ".md"
	case "pdf":
		return ".pdf"
	case "excel", "xlsx":
		return ".xlsx"
	case "slack":
		return ".json"
	default:
		return ".txt"
	}
}
