package clicky

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/formatters"
)

// WithHttpRequest extracts format options from an HTTP request
// Priority: query param > path extension > Accept header > default
func WithHttpRequest(r *http.Request) formatters.FormatOptions {
	opts := formatters.FormatOptions{}

	// Extract paging parameters from query or headers
	opts.Page = getIntParam(r, "page", "X-Page")
	opts.Limit = getIntParam(r, "limit", "X-Limit")

	// 1. Check query parameter first (highest priority)
	if format := r.URL.Query().Get("format"); format != "" {
		opts.Format = format
		return opts
	}

	// 2. Check URL path extension
	path := r.URL.Path
	ext := filepath.Ext(path)
	if ext != "" {
		format := extensionToFormat(ext)
		if format != "" {
			opts.Format = format
			return opts
		}
	}

	// 3. Check Accept header
	accept := r.Header.Get("Accept")
	if accept != "" {
		format := acceptToFormat(accept)
		if format != "" {
			opts.Format = format
			return opts
		}
	}

	// 4. Default to JSON for HTTP requests
	opts.Format = "json"
	return opts
}

// getIntParam gets an integer parameter from query or header
func getIntParam(r *http.Request, queryKey, headerKey string) int {
	// Check query parameter first
	if val := r.URL.Query().Get(queryKey); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			return i
		}
	}

	// Check header
	if val := r.Header.Get(headerKey); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			return i
		}
	}

	return 0
}

// extensionToFormat converts a file extension to a format string
func extensionToFormat(ext string) string {
	// Remove leading dot
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ToLower(ext)

	switch ext {
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "csv":
		return "csv"
	case "html", "htm":
		return "html"
	case "md", "markdown":
		return "markdown"
	case "pdf":
		return "pdf"
	case "xlsx", "xls":
		return "excel"
	case "txt":
		return "pretty"
	default:
		return ""
	}
}

// acceptToFormat converts Accept header to format string
func acceptToFormat(accept string) string {
	// Handle multiple Accept values (take first match)
	parts := strings.Split(accept, ",")
	for _, part := range parts {
		// Remove quality values (e.g., "application/json;q=0.9" -> "application/json")
		contentType := strings.Split(strings.TrimSpace(part), ";")[0]
		contentType = strings.ToLower(contentType)

		switch contentType {
		case "application/json":
			return "json"
		case "application/yaml", "text/yaml", "application/x-yaml":
			return "yaml"
		case "text/csv", "application/csv":
			return "csv"
		case "text/html", "application/xhtml+xml":
			return "html"
		case "text/markdown":
			return "markdown"
		case "application/pdf":
			return "pdf"
		case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-excel":
			return "excel"
		case "text/plain":
			return "pretty"
		case "*/*", "":
			// Wildcard or empty - let caller decide default
			return ""
		}
	}

	return ""
}

// FormatToContentType converts a format string to HTTP Content-Type
func FormatToContentType(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "application/yaml"
	case "csv":
		return "text/csv"
	case "html":
		return "text/html; charset=utf-8"
	case "markdown", "md":
		return "text/markdown; charset=utf-8"
	case "pdf":
		return "application/pdf"
	case "excel", "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pretty", "tree":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
