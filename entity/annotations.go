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
	annotationClickyEntityPath         = "clicky/entity-path"
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
	annotationClickyToolTitle          = "clicky/tool-title"
	annotationClickyToolIcon           = "clicky/tool-icon"
	annotationClickyToolParent         = "clicky/tool-parent"
	annotationClickyToolReadOnlyHint   = "clicky/tool-read-only-hint"
	annotationClickyToolDestructive    = "clicky/tool-destructive-hint"
	annotationClickyToolIdempotent     = "clicky/tool-idempotent-hint"
	annotationClickyToolOpenWorld      = "clicky/tool-open-world-hint"
	annotationClickyToolPermission     = "clicky/tool-default-permission"
	annotationClickyToolStrict         = "clicky/tool-strict"
	annotationClickyLocalOnly          = "clicky/local-only"
)

// MarkLocalOnly keeps a command off the generated HTTP surface.
//
// Publishing a CLI as an API publishes every runnable command in the tree, which
// is wrong for the ones that administer the process rather than serve a
// resource: `serve` would start a nested server, and `migrate` would let an
// unauthenticated request alter the schema. Such commands stay in `--help` and
// on the CLI — they are simply not routes.
//
// It marks the whole subtree: marking a parent covers every subcommand under it.
func MarkLocalOnly(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationClickyLocalOnly] = "true"
}

// IsLocalOnly reports whether a command, or any command it is nested under, is
// marked local-only.
func IsLocalOnly(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations != nil && parseAnnotationBool(c.Annotations[annotationClickyLocalOnly]) {
			return true
		}
	}
	return false
}

const (
	AnnotationClickyToolGroup             = annotationClickyToolGroup
	AnnotationClickyToolTitle             = annotationClickyToolTitle
	AnnotationClickyToolIcon              = annotationClickyToolIcon
	AnnotationClickyToolParent            = annotationClickyToolParent
	AnnotationClickyToolReadOnlyHint      = annotationClickyToolReadOnlyHint
	AnnotationClickyToolDestructiveHint   = annotationClickyToolDestructive
	AnnotationClickyToolIdempotentHint    = annotationClickyToolIdempotent
	AnnotationClickyToolOpenWorldHint     = annotationClickyToolOpenWorld
	AnnotationClickyToolDefaultPermission = annotationClickyToolPermission
	AnnotationClickyToolStrict            = annotationClickyToolStrict
)

// CommandOpenAPIMeta is the clicky-specific metadata attached to generated
// Cobra commands so the RPC/OpenAPI layer can expose entity semantics without
// guessing from paths or operationIds.
type CommandOpenAPIMeta struct {
	Entity  string
	Parent  string
	Aliases []string
	Admin   bool
	// Icon is an opaque UI icon name (e.g. "database"); clicky never interprets
	// it — it is emitted verbatim on the surface for the frontend to resolve.
	Icon string
	// Path is the entity's hierarchy position within Parent, PathSeparator-joined.
	Path string
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
	ToolHints MCPToolHints
}

func GetCommandOpenAPIMeta(cmd *cobra.Command) *CommandOpenAPIMeta {
	if cmd == nil || cmd.Annotations == nil {
		return nil
	}

	toolIcon := cmd.Annotations[annotationClickyToolIcon]
	if toolIcon == "" {
		toolIcon = cmd.Annotations[annotationClickyEntityIcon]
	}
	toolTitle := cmd.Annotations[annotationClickyToolTitle]
	if toolTitle == "" {
		toolTitle = cmd.Annotations[annotationClickyEntityTitle]
	}
	toolParent := cmd.Annotations[annotationClickyToolParent]
	if toolParent == "" {
		toolParent = cmd.Annotations[annotationClickyEntityParent]
	}
	toolHints := MCPToolHints{
		Title:             toolTitle,
		ReadOnlyHint:      parseAnnotationBoolPtr(cmd.Annotations[annotationClickyToolReadOnlyHint]),
		DestructiveHint:   parseAnnotationBoolPtr(cmd.Annotations[annotationClickyToolDestructive]),
		IdempotentHint:    parseAnnotationBoolPtr(cmd.Annotations[annotationClickyToolIdempotent]),
		OpenWorldHint:     parseAnnotationBoolPtr(cmd.Annotations[annotationClickyToolOpenWorld]),
		Icon:              toolIcon,
		Group:             cmd.Annotations[annotationClickyToolGroup],
		Parent:            toolParent,
		DefaultPermission: normalizeToolPermission(ToolPermission(cmd.Annotations[annotationClickyToolPermission])),
		Strict:            parseAnnotationBoolPtr(cmd.Annotations[annotationClickyToolStrict]),
	}

	meta := &CommandOpenAPIMeta{
		Entity:             cmd.Annotations[annotationClickyEntityName],
		Parent:             cmd.Annotations[annotationClickyEntityParent],
		Aliases:            splitAnnotationList(cmd.Annotations[annotationClickyEntityAliases]),
		Admin:              parseAnnotationBool(cmd.Annotations[annotationClickyEntityAdmin]),
		Icon:               cmd.Annotations[annotationClickyEntityIcon],
		Path:               cmd.Annotations[annotationClickyEntityPath],
		Title:              cmd.Annotations[annotationClickyEntityTitle],
		Verb:               cmd.Annotations[annotationClickyOperationVerb],
		Method:             cmd.Annotations[annotationClickyOperationMethod],
		Scope:              cmd.Annotations[annotationClickyOperationScope],
		ActionName:         cmd.Annotations[annotationClickyOperationAction],
		IDParam:            cmd.Annotations[annotationClickyOperationIDParam],
		SupportsLookup:     parseAnnotationBool(cmd.Annotations[annotationClickySupportsLookup]),
		SupportsFilterMode: parseAnnotationBool(cmd.Annotations[annotationClickySupportsFilterMode]),
		OptionalID:         parseAnnotationBool(cmd.Annotations[annotationClickyOptionalID]),
		ToolGroup:          toolHints.Group,
		ToolHints:          toolHints,
	}

	if meta.Entity == "" && meta.ToolHints.isZero() {
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
	setCommandAnnotation(cmd, annotationClickyEntityPath, entity.Path)
	setCommandAnnotation(cmd, annotationClickyEntityTitle, entity.Title)
	hints := entity.ToolHints
	if hints.Group == "" {
		hints.Group = entity.ToolGroup
	}
	if hints.Parent == "" {
		hints.Parent = entity.Parent
	}
	if hints.Icon == "" {
		hints.Icon = entity.Icon
	}
	if hints.Title == "" {
		hints.Title = entity.Title
	}
	AnnotateTool(cmd, hints)
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
	toolHints MCPToolHints,
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
	// Applied after inheritance: non-empty per-action hints replace inherited
	// entity hints; empty values are no-ops.
	AnnotateTool(cmd, toolHints)
	inferVerbToolHints(cmd, verb)
}

// inferVerbToolHints fills the safety hints a standard entity verb implies, when
// neither the entity nor the action declared them.
//
// It replaces stamping a default permission into clicky/tool-default-permission.
// That slot now carries explicit registrations only, because writing a verb
// default into the same field an app configures is what made a group baseline
// like `provider.xero.read: off` unreachable: whichever writer ran last won, and
// clicky ran last. Hints are the honest thing for clicky to contribute — "this
// operation only reads" is a fact about the operation, whereas "this operation is
// auto-approved" was an authority decision clicky had no standing to make.
//
// list/get still auto-run, but now by consequence rather than by decree: the
// consumer's policy resolves an unset permission through these hints.
func inferVerbToolHints(cmd *cobra.Command, verb string) {
	readOnly, destructive, ok := verbSafetyHints(verb)
	if !ok {
		return
	}
	if cmd.Annotations[annotationClickyToolReadOnlyHint] == "" {
		setCommandAnnotation(cmd, annotationClickyToolReadOnlyHint, strconv.FormatBool(readOnly))
	}
	if cmd.Annotations[annotationClickyToolDestructive] == "" {
		setCommandAnnotation(cmd, annotationClickyToolDestructive, strconv.FormatBool(destructive))
	}
}

// verbSafetyHints reports the read-only and destructive facts a standard entity
// verb implies. Custom actions get nothing: their safety is theirs to declare.
//
// Both hints are set together because a consumer that infers "auto-approve"
// requires read-only AND non-destructive; leaving destructive unset would make a
// read fall through to "ask" and silently change today's behaviour.
func verbSafetyHints(verb string) (readOnly, destructive, ok bool) {
	switch verb {
	case "list", "get":
		return true, false, true
	case "create", "update":
		return false, false, true
	case "delete":
		return false, true, true
	default:
		return false, false, false
	}
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
	setCommandAnnotation(cmd, annotationClickyEntityPath, meta.Path)
	setCommandAnnotation(cmd, annotationClickyEntityTitle, meta.Title)
	AnnotateTool(cmd, meta.ToolHints)
}

// AnnotateTool attaches MCP-facing tool hints to cmd. It is the public helper
// for commands that are not generated from an EntityBuilder.
func AnnotateTool(cmd *cobra.Command, hints MCPToolHints) {
	if cmd == nil {
		return
	}
	setCommandAnnotation(cmd, annotationClickyToolTitle, hints.Title)
	setCommandAnnotation(cmd, annotationClickyToolIcon, hints.Icon)
	setCommandAnnotation(cmd, annotationClickyToolParent, hints.Parent)
	setCommandAnnotation(cmd, annotationClickyToolGroup, hints.Group)
	if hints.ReadOnlyHint != nil {
		setCommandAnnotation(cmd, annotationClickyToolReadOnlyHint, strconv.FormatBool(*hints.ReadOnlyHint))
	}
	if hints.DestructiveHint != nil {
		setCommandAnnotation(cmd, annotationClickyToolDestructive, strconv.FormatBool(*hints.DestructiveHint))
	}
	if hints.IdempotentHint != nil {
		setCommandAnnotation(cmd, annotationClickyToolIdempotent, strconv.FormatBool(*hints.IdempotentHint))
	}
	if hints.OpenWorldHint != nil {
		setCommandAnnotation(cmd, annotationClickyToolOpenWorld, strconv.FormatBool(*hints.OpenWorldHint))
	}
	setCommandAnnotation(cmd, annotationClickyToolPermission, string(normalizeToolPermission(hints.DefaultPermission)))
	if hints.Strict != nil {
		setCommandAnnotation(cmd, annotationClickyToolStrict, strconv.FormatBool(*hints.Strict))
	}
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

func parseAnnotationBoolPtr(value string) *bool {
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
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
