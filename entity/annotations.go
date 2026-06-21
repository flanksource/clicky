package entity

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	annotationClickyEntityName         = "clicky/entity-name"
	annotationClickyEntityParent       = "clicky/entity-parent"
	annotationClickyEntityAliases      = "clicky/entity-aliases"
	annotationClickyEntityAdmin        = "clicky/entity-admin"
	annotationClickyEntityIcon         = "clicky/entity-icon"
	annotationClickyEntityTitle        = "clicky/entity-title"
	annotationClickyOperationVerb      = "clicky/operation-verb"
	annotationClickyOperationMethod    = "clicky/operation-method"
	annotationClickyOperationScope     = "clicky/operation-scope"
	annotationClickyOperationAction    = "clicky/operation-action-name"
	annotationClickyOperationIDParam   = "clicky/operation-id-param"
	annotationClickySupportsLookup     = "clicky/supports-lookup"
	annotationClickySupportsFilterMode = "clicky/supports-filter-mode"
	annotationClickyOptionalID         = "clicky/operation-optional-id"
	annotationClickyToolGroup          = "clicky/tool-group"
)

// CommandOpenAPIMeta is the clicky-specific metadata attached to generated
// Cobra commands so the RPC/OpenAPI layer can expose entity semantics without
// guessing from paths or operationIds.
type CommandOpenAPIMeta struct {
	Entity             string
	Parent             string
	Aliases            []string
	Admin              bool
	// Icon is an opaque UI icon name (e.g. "database"); clicky never interprets
	// it — it is emitted verbatim on the surface for the frontend to resolve.
	Icon string
	// Title overrides the auto-generated surface title when non-empty.
	Title              string
	Verb               string
	Method             string
	Scope              string
	ActionName         string
	IDParam            string
	SupportsLookup     bool
	SupportsFilterMode bool
	// OptionalID is true when the operation's positional id is optional
	// (WithOptionalID). Such operations are invocable without an id, so the
	// generated REST path must NOT carry an {id} segment — otherwise the no-id
	// call collides with the entity's get-by-id route.
	OptionalID bool
	// ToolGroup names the group an operation belongs to. It is set on the entity
	// and inherited by every operation (a per-action override may replace it).
	// AI tool-preference layers use it to enable/disable a set of related tools
	// as one unit.
	ToolGroup string
}

func GetCommandOpenAPIMeta(cmd *cobra.Command) *CommandOpenAPIMeta {
	if cmd == nil || cmd.Annotations == nil {
		return nil
	}

	meta := &CommandOpenAPIMeta{
		Entity:             cmd.Annotations[annotationClickyEntityName],
		Parent:             cmd.Annotations[annotationClickyEntityParent],
		Aliases:            splitAnnotationList(cmd.Annotations[annotationClickyEntityAliases]),
		Admin:              parseAnnotationBool(cmd.Annotations[annotationClickyEntityAdmin]),
		Icon:               cmd.Annotations[annotationClickyEntityIcon],
		Title:              cmd.Annotations[annotationClickyEntityTitle],
		Verb:               cmd.Annotations[annotationClickyOperationVerb],
		Method:             cmd.Annotations[annotationClickyOperationMethod],
		Scope:              cmd.Annotations[annotationClickyOperationScope],
		ActionName:         cmd.Annotations[annotationClickyOperationAction],
		IDParam:            cmd.Annotations[annotationClickyOperationIDParam],
		SupportsLookup:     parseAnnotationBool(cmd.Annotations[annotationClickySupportsLookup]),
		SupportsFilterMode: parseAnnotationBool(cmd.Annotations[annotationClickySupportsFilterMode]),
		OptionalID:         parseAnnotationBool(cmd.Annotations[annotationClickyOptionalID]),
		ToolGroup:          cmd.Annotations[annotationClickyToolGroup],
	}

	if meta.Entity == "" {
		return nil
	}

	return meta
}

func annotateEntityCommand(cmd *cobra.Command, entity EntityInfo) {
	if cmd == nil {
		return
	}

	setCommandAnnotation(cmd, annotationClickyEntityName, entity.Name)
	setCommandAnnotation(cmd, annotationClickyEntityParent, entity.Parent)
	setCommandAnnotation(cmd, annotationClickyEntityAliases, strings.Join(entity.Aliases, ","))
	setCommandAnnotation(cmd, annotationClickyEntityAdmin, strconv.FormatBool(entity.IsAdmin))
	setCommandAnnotation(cmd, annotationClickyEntityIcon, entity.Icon)
	setCommandAnnotation(cmd, annotationClickyEntityTitle, entity.Title)
	setCommandAnnotation(cmd, annotationClickyToolGroup, entity.ToolGroup)
}

func annotateEntityOperationCommand(
	cmd *cobra.Command,
	parent *cobra.Command,
	verb string,
	method string,
	scope string,
	actionName string,
	idParam string,
	supportsLookup bool,
	supportsFilterMode bool,
	optionalID bool,
	toolGroup string,
) {
	if cmd == nil {
		return
	}

	inheritEntityAnnotations(cmd, parent)
	setCommandAnnotation(cmd, annotationClickyOperationVerb, verb)
	setCommandAnnotation(cmd, annotationClickyOperationMethod, strings.ToUpper(method))
	setCommandAnnotation(cmd, annotationClickyOperationScope, scope)
	setCommandAnnotation(cmd, annotationClickyOperationAction, actionName)
	setCommandAnnotation(cmd, annotationClickyOperationIDParam, idParam)
	setCommandAnnotation(cmd, annotationClickySupportsLookup, strconv.FormatBool(supportsLookup))
	setCommandAnnotation(cmd, annotationClickySupportsFilterMode, strconv.FormatBool(supportsFilterMode))
	setCommandAnnotation(cmd, annotationClickyOptionalID, strconv.FormatBool(optionalID))
	// Applied after inheritance: a non-empty per-action override replaces the
	// inherited entity group; empty is a no-op (setCommandAnnotation skips "").
	setCommandAnnotation(cmd, annotationClickyToolGroup, toolGroup)
}

func inheritEntityAnnotations(cmd *cobra.Command, parent *cobra.Command) {
	meta := GetCommandOpenAPIMeta(parent)
	if meta == nil {
		return
	}

	setCommandAnnotation(cmd, annotationClickyEntityName, meta.Entity)
	setCommandAnnotation(cmd, annotationClickyEntityParent, meta.Parent)
	setCommandAnnotation(cmd, annotationClickyEntityAliases, strings.Join(meta.Aliases, ","))
	setCommandAnnotation(cmd, annotationClickyEntityAdmin, strconv.FormatBool(meta.Admin))
	setCommandAnnotation(cmd, annotationClickyEntityIcon, meta.Icon)
	setCommandAnnotation(cmd, annotationClickyEntityTitle, meta.Title)
	setCommandAnnotation(cmd, annotationClickyToolGroup, meta.ToolGroup)
}

func setCommandAnnotation(cmd *cobra.Command, key string, value string) {
	if cmd == nil || value == "" {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[key] = value
}

func parseAnnotationBool(value string) bool {
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func splitAnnotationList(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
