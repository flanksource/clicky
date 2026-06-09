package docs

import (
	"fmt"

	"github.com/flanksource/clicky/formatters"
)

// RenderSingleFile renders the whole model to one document in the requested
// format. "markdown" (default) concatenates the CLI reference and UI catalog;
// "json"/"yaml" emit the structured Model via clicky's FormatManager. Other
// formats fail loudly — prose markdown is not converted to html/pdf here.
func RenderSingleFile(m *Model, format string) (string, error) {
	switch format {
	case "", "markdown", "md":
		return RenderCLIReference(m) + "\n" + RenderUISurfaces(m), nil
	case "json", "yaml", "yml":
		return formatters.NewFormatManager().Format(format, m)
	default:
		return "", fmt.Errorf("unsupported --format %q (supported: markdown, json, yaml)", format)
	}
}
