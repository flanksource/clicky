package rpc

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/formatters"
)

func (s *SwaggerServer) writeFormattedResponse(w http.ResponseWriter, r *http.Request, data any, options formatOptions, statusCode int) {
	if paged, ok := data.(clicky.Paged); ok {
		page := paged.PageMetadata()
		w.Header().Set("X-Total-Count", strconv.FormatInt(page.Total, 10))
		w.Header().Set("X-Page-Limit", strconv.Itoa(page.Limit))
		w.Header().Set("X-Page-Offset", strconv.Itoa(page.Offset))
		if shouldFormatPagedRows(options.Format) {
			data = paged.PageRows()
		}
	}

	output, err := formatters.NewFormatManager().FormatWithContext(r, formatters.FormatOptions{
		Format: options.Format, Page: options.Page, Limit: options.Limit,
	}, data)
	if err != nil {
		if s.structuredErrorResponses() {
			s.writeError(w, r, fmt.Errorf("format response: %w", err))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "format error: %v", err)
		return
	}

	w.Header().Set("Content-Type", formatToContentType(options.Format))
	if filename := sanitizedAttachmentFilename(r.URL.Query().Get("filename")); filename != "" {
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(output))
}

func sanitizedAttachmentFilename(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	base := strings.Trim(path.Base(strings.ReplaceAll(raw, "\\", "/")), ". ")
	if base == "" || base == "." || base == "/" {
		return ""
	}

	var output strings.Builder
	lastDash := false
	for _, character := range base {
		safe := (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-'
		if safe {
			output.WriteRune(character)
			lastDash = false
		} else if !lastDash {
			output.WriteByte('-')
			lastDash = true
		}
	}

	filename := strings.Trim(output.String(), ".-")
	if len(filename) > 160 {
		filename = strings.Trim(filename[:160], ".-")
	}
	return filename
}

func shouldFormatPagedRows(format string) bool {
	return format != "json" && format != "yaml" && format != "yml"
}

type formatOptions struct {
	Format string
	Page   int
	Limit  int
}

func extractFormatOpts(r *http.Request) formatOptions {
	options := formatOptions{}
	if page := r.URL.Query().Get("page"); page != "" {
		options.Page, _ = strconv.Atoi(page)
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		options.Limit, _ = strconv.Atoi(limit)
	}
	if format := r.URL.Query().Get("format"); format != "" && !isRenderFormat(format) {
		options.Format = format
		return options
	}
	options.Format = negotiateAcceptFormat(r.Header.Get("Accept"))
	return options
}

// acceptFormats maps a media type in an Accept header to the wire format it
// selects.
var acceptFormats = map[string]string{
	"application/json":        "json",
	"application/x-ndjson":    "ndjson",
	"application/ndjson":      "ndjson",
	"application/clicky+json": "clicky-json",
	"application/json+clicky": "clicky-json",
	"application/yaml":        "yaml",
	"text/yaml":               "yaml",
	"application/x-yaml":      "yaml",
	"text/csv":                "csv",
	"application/csv":         "csv",
	"text/html":               "html",
	"application/xhtml+xml":   "html",
	"text/markdown":           "markdown",
	"application/pdf":         "pdf",
	"text/plain":              "pretty",
}

// negotiateAcceptFormat picks the supported representation with the highest
// quality value, preserving declaration order for ties. A representation
// refused with q=0 is never selected — "application/json;q=0, text/csv" means
// CSV, not JSON. When nothing acceptable is named, the wire default is JSON.
func negotiateAcceptFormat(accept string) string {
	best := ""
	bestQuality := -1.0
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mediaType, params, err := mime.ParseMediaType(part)
		if err != nil {
			continue
		}
		format, supported := acceptFormats[strings.ToLower(mediaType)]
		if !supported {
			continue
		}
		quality := 1.0
		if raw, declared := params["q"]; declared {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			quality = parsed
		}
		if quality <= 0 {
			continue
		}
		if quality > bestQuality {
			bestQuality = quality
			best = format
		}
	}
	if best == "" {
		return "json"
	}
	return best
}

func isRenderFormat(format string) bool {
	return format == "pretty" || format == "tree"
}

func isStructuredWireFormat(format string) bool {
	return format == "json" || format == "clicky-json" || format == "yaml" || format == "yml"
}

func formatToContentType(format string) string {
	switch format {
	case "clicky-json":
		return "application/json+clicky"
	case "yaml", "yml":
		return "application/yaml"
	case "ndjson":
		return "application/x-ndjson"
	case "csv":
		return "text/csv; charset=utf-8"
	case "html", "html-react":
		return "text/html; charset=utf-8"
	case "markdown", "md":
		return "text/markdown; charset=utf-8"
	case "pdf":
		return "application/pdf"
	case "pretty", "tree":
		return "text/plain; charset=utf-8"
	default:
		return "application/json"
	}
}
