package docs

import (
	"fmt"
	"strings"
)

// RenderUISurfaces produces the markdown clicky-ui surface catalog body (no
// frontmatter). It documents, per operation: the surface entity + verb, the
// HTTP endpoint + CLI command the UI invokes, lookup support, and each
// parameter's UI-widget role.
func RenderUISurfaces(m *Model) string {
	md := &mdBuilder{}
	md.heading(1, "UI Surface Catalog")
	md.para("Operations exposed by the clicky-ui metadata-driven explorer. Each surface maps an entity verb to an HTTP endpoint the UI calls, and lists the parameters that render as filter, pagination, and time-range widgets.")

	if len(m.Surfaces) == 0 {
		md.para("_No clicky-ui surfaces are registered for this CLI._")
		return md.String()
	}

	for _, s := range m.Surfaces {
		renderSurface(md, s)
	}
	return md.String()
}

func renderSurface(md *mdBuilder, s SurfaceDoc) {
	heading := fmt.Sprintf("%s — %s", code(s.Command), strings.ToUpper(verbOrDash(s.Verb)))
	md.heading(2, heading)

	md.table(
		[]string{"Entity", "Verb", "HTTP", "Endpoint", "CLI", "Lookup"},
		[][]string{{
			code(s.Entity),
			verbOrDash(s.Verb),
			code(s.Method),
			code(s.Path),
			code(s.Command),
			boolMark(s.Lookup),
		}},
	)

	if len(s.Parameters) == 0 {
		md.para("_No parameters._")
		return
	}

	md.line("**Parameters:**").blank()
	rows := make([][]string, 0, len(s.Parameters))
	for _, p := range s.Parameters {
		rows = append(rows, []string{
			code(p.Name),
			code(p.Type),
			code(p.In),
			roleLabel(p.Role),
			boolMark(p.Required),
			defaultStr(p.Default),
		})
	}
	md.table([]string{"Name", "Type", "In", "UI Role", "Required", "Default"}, rows)
}

func verbOrDash(verb string) string {
	if verb == "" {
		return "—"
	}
	return verb
}

// roleLabel describes the UI widget a parameter role drives.
func roleLabel(role string) string {
	switch role {
	case "filter":
		return "filter (filter chip)"
	case "limit":
		return "limit (page size)"
	case "offset":
		return "offset (page)"
	case "time-from":
		return "time-from (range start)"
	case "time-to":
		return "time-to (range end)"
	case "":
		return ""
	default:
		return role
	}
}
