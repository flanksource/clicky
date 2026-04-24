package clicky

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
	annotationClickyOperationVerb      = "clicky/operation-verb"
	annotationClickyOperationScope     = "clicky/operation-scope"
	annotationClickyOperationAction    = "clicky/operation-action-name"
	annotationClickyOperationIDParam   = "clicky/operation-id-param"
	annotationClickySupportsLookup     = "clicky/supports-lookup"
	annotationClickySupportsFilterMode = "clicky/supports-filter-mode"
)

// CommandOpenAPIMeta is the clicky-specific metadata attached to generated
// Cobra commands so the RPC/OpenAPI layer can expose entity semantics without
// guessing from paths or operationIds.
type CommandOpenAPIMeta struct {
	Entity             string
	Parent             string
	Aliases            []string
	Admin              bool
	Verb               string
	Scope              string
	ActionName         string
	IDParam            string
	SupportsLookup     bool
	SupportsFilterMode bool
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
		Verb:               cmd.Annotations[annotationClickyOperationVerb],
		Scope:              cmd.Annotations[annotationClickyOperationScope],
		ActionName:         cmd.Annotations[annotationClickyOperationAction],
		IDParam:            cmd.Annotations[annotationClickyOperationIDParam],
		SupportsLookup:     parseAnnotationBool(cmd.Annotations[annotationClickySupportsLookup]),
		SupportsFilterMode: parseAnnotationBool(cmd.Annotations[annotationClickySupportsFilterMode]),
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
}

func annotateEntityOperationCommand(
	cmd *cobra.Command,
	parent *cobra.Command,
	verb string,
	scope string,
	actionName string,
	idParam string,
	supportsLookup bool,
	supportsFilterMode bool,
) {
	if cmd == nil {
		return
	}

	inheritEntityAnnotations(cmd, parent)
	setCommandAnnotation(cmd, annotationClickyOperationVerb, verb)
	setCommandAnnotation(cmd, annotationClickyOperationScope, scope)
	setCommandAnnotation(cmd, annotationClickyOperationAction, actionName)
	setCommandAnnotation(cmd, annotationClickyOperationIDParam, idParam)
	setCommandAnnotation(cmd, annotationClickySupportsLookup, strconv.FormatBool(supportsLookup))
	setCommandAnnotation(cmd, annotationClickySupportsFilterMode, strconv.FormatBool(supportsFilterMode))
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
