package clicky

import (
	"context"

	"github.com/flanksource/clicky/entity"
	"github.com/spf13/cobra"
)

// This file re-exports the entity + operation model, whose canonical home is the
// entity/ subpackage (see the inversion described in entity/doc.go). Existing
// callers keep using clicky.NewEntity, clicky.Entity, clicky.RegisterEntity, etc.
// unchanged. The init() below wires the entity package's render/parse hooks to
// the root clicky globals (which own the CLI flag state), keeping entity/ free of
// a dependency back on this package.

func init() {
	entity.RenderResult = func(o any) error {
		if err := Flags.ParseFormatSpec(); err != nil {
			return err
		}
		PrintAndWriteSinks(o, Flags.FormatOptions)
		return nil
	}
	entity.ParseArgs = ParseArgumentsAsMap
}

// --- non-generic type aliases ---
type (
	EntityItem          = entity.EntityItem
	EntityInfo          = entity.EntityInfo
	EntityOperation     = entity.EntityOperation
	ActionInfo          = entity.ActionInfo
	BulkActionInfo      = entity.BulkActionInfo
	ActionFlags         = entity.ActionFlags
	EntityAction        = entity.EntityAction
	EntityBulkAction    = entity.EntityBulkAction
	CommandOpenAPIMeta  = entity.CommandOpenAPIMeta
	ResponseOpenAPIMeta = entity.ResponseOpenAPIMeta
	MCPToolHints        = entity.MCPToolHints
	ToolPermission      = entity.ToolPermission
	DynamicFilter       = entity.DynamicFilter
	DynamicEntitySpec   = entity.DynamicEntitySpec
	ContextDataFunc     = entity.ContextDataFunc
	ContextLookupFunc   = entity.ContextLookupFunc
	MultiFilter         = entity.MultiFilter
	PageInfo            = entity.PageInfo
	Paged               = entity.Paged
	Name                = entity.Name
	Help                = entity.Help
)

// --- generic type aliases (Go 1.24+; go.mod is 1.26.1) ---
type (
	Entity[T entity.EntityItem, ListOpts any, R any]        = entity.Entity[T, ListOpts, R]
	EntityBuilder[T entity.EntityItem, ListOpts any, R any] = entity.EntityBuilder[T, ListOpts, R]
	Filter[ListOpts any]                                    = entity.Filter[ListOpts]
	TypedFilter[ListOpts any]                               = entity.TypedFilter[ListOpts]
	SearchableFilter[ListOpts any]                          = entity.SearchableFilter[ListOpts]
	ContextFilter[ListOpts any]                             = entity.ContextFilter[ListOpts]
	ContextSearchableFilter[ListOpts any]                   = entity.ContextSearchableFilter[ListOpts]
	Filterable[T any]                                       = entity.Filterable[T]
	PagedResult[T any]                                      = entity.PagedResult[T]
	ActionSpec[R any]                                       = entity.ActionSpec[R]
	BulkActionSpec[R any]                                   = entity.BulkActionSpec[R]
)

// --- non-generic function/var aliases ---
var (
	GetEntities            = entity.GetEntities
	GetEntity              = entity.GetEntity
	GenerateCLI            = entity.GenerateCLI
	RegisterDynamicEntity  = entity.RegisterDynamicEntity
	RegisterSubCommand     = entity.RegisterSubCommand
	RegisterSubCommandFn   = entity.RegisterSubCommandFn
	GetDataFunc            = entity.GetDataFunc
	GetContextDataFunc     = entity.GetContextDataFunc
	GetLookupFunc          = entity.GetLookupFunc
	GetContextLookupFunc   = entity.GetContextLookupFunc
	GetCommandOpenAPIMeta  = entity.GetCommandOpenAPIMeta
	GetCommandResponseMeta = entity.GetCommandResponseMeta
	SetCommandResponseMeta = entity.SetCommandResponseMeta
	AnnotateTool           = entity.AnnotateTool
)

const (
	ToolPermissionAsk  = entity.ToolPermissionAsk
	ToolPermissionOff  = entity.ToolPermissionOff
	ToolPermissionOn   = entity.ToolPermissionOn
	ToolPermissionAuto = entity.ToolPermissionAuto

	AnnotationClickyToolGroup             = entity.AnnotationClickyToolGroup
	AnnotationClickyToolTitle             = entity.AnnotationClickyToolTitle
	AnnotationClickyToolIcon              = entity.AnnotationClickyToolIcon
	AnnotationClickyToolParent            = entity.AnnotationClickyToolParent
	AnnotationClickyToolReadOnlyHint      = entity.AnnotationClickyToolReadOnlyHint
	AnnotationClickyToolDestructiveHint   = entity.AnnotationClickyToolDestructiveHint
	AnnotationClickyToolIdempotentHint    = entity.AnnotationClickyToolIdempotentHint
	AnnotationClickyToolOpenWorldHint     = entity.AnnotationClickyToolOpenWorldHint
	AnnotationClickyToolDefaultPermission = entity.AnnotationClickyToolDefaultPermission
	AnnotationClickyToolStrict            = entity.AnnotationClickyToolStrict
)

// --- generic function wrappers (a generic func cannot be a var alias) ---

func NewEntity[T entity.EntityItem, ListOpts any, R any](name string) *entity.EntityBuilder[T, ListOpts, R] {
	return entity.NewEntity[T, ListOpts, R](name)
}

func RegisterEntity[T entity.EntityItem, ListOpts any, R any](e entity.Entity[T, ListOpts, R]) {
	entity.RegisterEntity[T, ListOpts, R](e)
}

func Action[R any](name string, fn func(id string, flags map[string]string) (R, error)) *entity.ActionSpec[R] {
	return entity.Action(name, fn)
}

func ActionWithFlags[R any](name string, flags entity.ActionFlags, fn func(id string, flags map[string]string) (R, error)) *entity.ActionSpec[R] {
	return entity.ActionWithFlags(name, flags, fn)
}

func ActionWithContext[R any](name string, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *entity.ActionSpec[R] {
	return entity.ActionWithContext(name, fn)
}

func ActionWithFlagsAndContext[R any](name string, flags entity.ActionFlags, fn func(ctx context.Context, id string, flags map[string]string) (R, error)) *entity.ActionSpec[R] {
	return entity.ActionWithFlagsAndContext(name, flags, fn)
}

func BulkAction[R any](name string, fn func(ids []string, flags map[string]string) (R, error)) *entity.BulkActionSpec[R] {
	return entity.BulkAction(name, fn)
}

func BulkFilterAction[ListOpts any, R any](name string, fn func(opts ListOpts, flags map[string]string) (R, error)) *entity.BulkActionSpec[R] {
	return entity.BulkFilterAction[ListOpts, R](name, fn)
}

func BulkActionWithFilter[ListOpts any, R any](name string, run func(ids []string, flags map[string]string) (R, error), runFilter func(opts ListOpts, flags map[string]string) (R, error)) *entity.BulkActionSpec[R] {
	return entity.BulkActionWithFilter[ListOpts, R](name, run, runFilter)
}

func LiftFilters[Outer any, Inner any](filters []entity.Filter[Inner], project func(*Outer) *Inner) []entity.Filter[Outer] {
	return entity.LiftFilters(filters, project)
}

func NewPagedResult[T any](rows []T, limit, offset int, total int64) entity.PagedResult[T] {
	return entity.NewPagedResult(rows, limit, offset, total)
}

func AddCommand[T any, R any](parent *cobra.Command, opts T, fn func(opts T) (R, error)) *cobra.Command {
	return entity.AddCommand(parent, opts, fn)
}

func AddNamedCommand[T any, R any](name string, parent *cobra.Command, opts T, fn func(opts T) (R, error)) *cobra.Command {
	return entity.AddNamedCommand(name, parent, opts, fn)
}

func AddCommandWithContext[T any, R any](parent *cobra.Command, opts T, fn func(ctx context.Context, opts T) (R, error)) *cobra.Command {
	return entity.AddCommandWithContext(parent, opts, fn)
}

func AddNamedCommandWithContext[T any, R any](name string, parent *cobra.Command, opts T, fn func(ctx context.Context, opts T) (R, error)) *cobra.Command {
	return entity.AddNamedCommandWithContext(name, parent, opts, fn)
}
