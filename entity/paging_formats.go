package entity

// Content negotiation and the download name: the half of the export contract
// that decides what a response is, as opposed to how far it goes.

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

// RequestedFormat resolves ?format, falling back to content negotiation.
func RequestedFormat(r *http.Request) (string, error) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	switch format {
	case "xlsx":
		return "excel", nil
	case "md":
		return "markdown", nil
	case "yml":
		return "yaml", nil
	case "":
		return AcceptedFormat(r.Header.Get("Accept"))
	}
	if !SupportedExportFormat(format) {
		return "", NewStatusErrorf(http.StatusBadRequest, "unsupported_format", "unsupported export format %q", format)
	}
	return format, nil
}

// AcceptedFormat picks the format an Accept header asks for.
//
// Accept is ranked, not ordered: a caller listing text/html first at q=0.1 and
// the clicky envelope at q=0.9 is asking for the envelope. Reading the first
// recognised entry answers with the one it weighted lowest. Ties keep the
// earlier entry, which is the order the caller wrote them in.
//
// A q=0 is a refusal, not a low preference, and a refusal has to be able to
// leave nothing acceptable behind. Seeding the search with the default format
// would make `Accept: application/json;q=0` answer with the very thing it just
// refused, so refusals are tracked and the fallback is withheld when the
// default is among them — the honest answer there is 406. A wildcard resolves
// to the default for exactly this reason, so `*/*;q=0` refuses everything
// rather than silently accepting the default.
//
// A refusal only decides the answer when nothing else was acceptable, which is
// what lets a named range override a blanket one: `*/*;q=0, text/csv` is a
// caller that wants csv and nothing else.
func AcceptedFormat(accept string) (string, error) {
	best, bestQuality := "", -1.0
	refused := map[string]bool{}
	for _, part := range strings.Split(accept, ",") {
		fields := strings.Split(part, ";")
		format, ok := formatForMediaType(strings.ToLower(strings.TrimSpace(fields[0])))
		if !ok {
			continue
		}
		quality := acceptQuality(fields[1:])
		if quality <= 0 {
			refused[format] = true
			continue
		}
		if quality <= bestQuality {
			continue
		}
		best, bestQuality = format, quality
	}
	if best != "" {
		return best, nil
	}
	if refused[DefaultExportFormat] {
		return "", NewStatusErrorf(http.StatusNotAcceptable, "not_acceptable",
			"no supported representation is acceptable to %q", accept)
	}
	return DefaultExportFormat, nil
}

// acceptQuality reads the q parameter of one Accept entry, defaulting to 1.
func acceptQuality(parameters []string) float64 {
	quality := 1.0
	for _, parameter := range parameters {
		name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if !found || strings.ToLower(strings.TrimSpace(name)) != "q" {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		quality = parsed
	}
	return quality
}

func formatForMediaType(media string) (string, bool) {
	switch media {
	case "application/json+clicky", "application/clicky+json":
		return "clicky-json", true
	case "application/x-ndjson", "application/ndjson":
		return "ndjson", true
	case "application/yaml", "application/x-yaml", "text/yaml":
		return "yaml", true
	case "text/csv", "application/csv":
		return "csv", true
	case "text/markdown":
		return "markdown", true
	case "text/html":
		return "html", true
	case "application/pdf":
		return "pdf", true
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "excel", true
	case "application/json":
		return "json", true
	case "*/*":
		// A wildcard is an opinion about the default, not an absence of one, and
		// resolving it to the default is what makes it carry a q: `*/*;q=0` says
		// nothing at all is acceptable and has to be able to reach the 406, which
		// it cannot do while it looks like a media type nobody recognised.
		//
		// Subtype wildcards (application/*) are deliberately not read: honouring
		// them means implementing RFC 7231's specificity precedence, and guessing
		// at it is worse than treating them as unrecognised, which is what every
		// caller sending one already gets.
		return DefaultExportFormat, true
	default:
		return "", false
	}
}

func SupportedExportFormat(format string) bool {
	switch format {
	case "clicky-json", "json", "ndjson", "yaml", "csv", "markdown", "html", "excel", "pdf":
		return true
	default:
		return false
	}
}

// IsTabularExport reports whether a format renders rows as a table, and so has
// to be written through formatters.WriteTableStream rather than serialized.
func IsTabularExport(format string) bool {
	switch format {
	case "csv", "markdown", "html", "excel", "pdf":
		return true
	default:
		return false
	}
}

func ExportContentType(format string) string {
	switch format {
	case "clicky-json":
		return "application/json+clicky"
	case "json":
		return "application/json"
	case "ndjson":
		return "application/x-ndjson"
	case "yaml":
		return "application/yaml"
	case "csv":
		return "text/csv; charset=utf-8"
	case "markdown":
		return "text/markdown; charset=utf-8"
	case "html":
		return "text/html; charset=utf-8"
	case "excel":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func ExportExtension(format string) string {
	switch format {
	case "markdown":
		return ".md"
	case "excel":
		return ".xlsx"
	case "ndjson":
		return ".ndjson"
	default:
		return "." + format
	}
}

// SanitizeExportFilename reduces a caller-supplied name to something that can
// only ever be a filename: no path to escape with, and nothing that could close
// the header parameter it is about to be written into.
func SanitizeExportFilename(filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	parts := strings.Split(filename, "/")
	filename = parts[len(parts)-1]
	filename = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`\";`, r) {
			return '_'
		}
		return r
	}, filename)
	filename = strings.Trim(filename, " .")
	if filename == "" {
		return "export.json"
	}
	return filename
}

// ContentDisposition renders an attachment header for a filename.
//
// RFC 6266's plain filename= is a quoted-string, which is ISO-8859-1: a UTF-8
// name written there travels as bytes the client is not obliged to decode, and
// Go's own quoting is not the escaping that header defines either. filename*
// names its charset, so it is the form that carries a non-ASCII name intact.
// Both are emitted — a client that understands filename* is required to prefer
// it, and one that does not still gets a usable, if flattened, name.
func ContentDisposition(filename string) string {
	name := SanitizeExportFilename(filename)
	return "attachment; filename=\"" + asciiFilename(name) + "\"; filename*=UTF-8''" + encodeRFC8187(name)
}

// asciiFilename flattens what a quoted-string cannot carry. Sanitizing has
// already removed the quote and backslash that would need escaping, so what is
// left only has to be made ASCII.
func asciiFilename(name string) string {
	flattened := strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return '_'
		}
		return r
	}, name)
	if strings.Trim(flattened, "_") == "" {
		return "export" + ExportExtension(DefaultExportFormat)
	}
	return flattened
}

// attrChars are the characters RFC 8187 allows unencoded in an ext-value.
const attrChars = "!#$&+-.^_`|~"

func encodeRFC8187(name string) string {
	const hex = "0123456789ABCDEF"
	var encoded strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			strings.IndexByte(attrChars, c) >= 0:
			encoded.WriteByte(c)
		default:
			encoded.WriteByte('%')
			encoded.WriteByte(hex[c>>4])
			encoded.WriteByte(hex[c&0x0f])
		}
	}
	return encoded.String()
}
