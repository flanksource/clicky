package http

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/commons/logger"
)

// FormatHandler wraps a function that returns data and automatically handles formatting
// based on the HTTP request (Accept header, query params, URL extension)
func FormatHandler(fn func(*http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Call user function to get data
		data, err := fn(r)
		if err != nil {
			logger.Errorf("Handler error: %v", err)
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
			return
		}

		// Handle nil data
		if data == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Extract format options from request
		opts := extractFormatOptions(r)

		output, err := formatters.FormatManager.FormatWithOptions(opts, data)
		if err != nil {
			logger.Errorf("Format error: %v", err)
			http.Error(w, fmt.Sprintf("Failed to format response: %v", err), http.StatusInternalServerError)
			return
		}

		// Set Content-Type header based on format
		contentType := formatToContentType(opts.Format)
		w.Header().Set("Content-Type", contentType)

		// Set paging headers if specified
		if opts.Page > 0 {
			w.Header().Set("X-Page", strconv.Itoa(opts.Page))
		}
		if opts.Limit > 0 {
			w.Header().Set("X-Per-Page", strconv.Itoa(opts.Limit))
		}

		// TODO: Set X-Total-Count if the data provides it (e.g., via a PagedResult interface)

		// Write response
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(output)); err != nil {
			logger.Errorf("Write error: %v", err)
		}
	}
}

// extractFormatOptions extracts format options from HTTP request
func extractFormatOptions(r *http.Request) FormatOptions {
	opts := FormatOptions{}

	// Extract paging parameters from query or headers
	opts.Page = getIntParam(r, "page", "X-Page")
	opts.Limit = getIntParam(r, "limit", "X-Limit")

	// 1. Check query parameter first (highest priority)
	if format := r.URL.Query().Get("format"); format != "" {
		opts.Format = format
		return opts
	}

	// 2. Check URL path extension
	if format := getFormatFromPath(r.URL.Path); format != "" {
		opts.Format = format
		return opts
	}

	// 3. Check Accept header
	accept := r.Header.Get("Accept")
	if accept != "" {
		if format := acceptToFormat(accept); format != "" {
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

// getFormatFromPath extracts format from URL path extension
func getFormatFromPath(path string) string {
	// Find last dot in path
	lastDot := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			lastDot = i
			break
		}
		// Stop at slash (no extension)
		if path[i] == '/' {
			break
		}
	}

	if lastDot == -1 {
		return ""
	}

	ext := path[lastDot+1:]
	return extensionToFormat(ext)
}

// extensionToFormat converts a file extension to a format string
func extensionToFormat(ext string) string {
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
	parts := splitAccept(accept)
	for _, part := range parts {
		// Remove quality values and whitespace
		contentType := trimQuality(part)

		switch contentType {
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
		}
	}

	return "json"
}

// splitAccept splits Accept header by comma
func splitAccept(accept string) []string {
	var result []string
	start := 0
	for i := 0; i < len(accept); i++ {
		if accept[i] == ',' {
			result = append(result, accept[start:i])
			start = i + 1
		}
	}
	if start < len(accept) {
		result = append(result, accept[start:])
	}
	return result
}

// trimQuality removes quality values and trims whitespace
func trimQuality(s string) string {
	// Find semicolon
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			s = s[:i]
			break
		}
	}
	// Trim whitespace manually
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// formatToContentType converts a format string to HTTP Content-Type
func formatToContentType(format string) string {
	switch format {
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
