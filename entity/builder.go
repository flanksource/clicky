package entity

import (
	"context"

	"github.com/spf13/cobra"
)

// EntityBuilder provides a fluent API for registering typed entities.
type EntityBuilder[T EntityItem, ListOpts any, R any] struct {
	entity Entity[T, ListOpts, R]
}

// NewEntity starts a typed entity registration. The resulting entity is served
// identically on the CLI and over HTTP/RPC. See the package
// github.com/flanksource/clicky/entity for an authoring guide, the best
// practices for operations that behave well on both surfaces, and runnable
// examples.
func NewEntity[T EntityItem, ListOpts any, R any](name string) *EntityBuilder[T, ListOpts, R] {
	return &EntityBuilder[T, ListOpts, R]{
		entity: Entity[T, ListOpts, R]{Name: name},
	}
}

func (b *EntityBuilder[T, ListOpts, R]) Parent(parent string) *EntityBuilder[T, ListOpts, R] {
	b.entity.Parent = parent
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Aliases(aliases ...string) *EntityBuilder[T, ListOpts, R] {
	b.entity.Aliases = append(b.entity.Aliases, aliases...)
	return b
}

// ToolGroup names the group this entity's operations belong to. Every generated
// operation inherits it (an action can override it via WithToolGroup). AI
// tool-preference layers use the group to enable/disable related tools together.
func (b *EntityBuilder[T, ListOpts, R]) ToolGroup(group string) *EntityBuilder[T, ListOpts, R] {
	b.entity.ToolGroup = group
	b.entity.ToolHints.Group = group
	return b
}

// ToolHints sets MCP-facing annotations and Clicky UI metadata inherited by all
// generated operations. Per-action hints can override these values.
func (b *EntityBuilder[T, ListOpts, R]) ToolHints(hints MCPToolHints) *EntityBuilder[T, ListOpts, R] {
	b.entity.ToolHints = b.entity.ToolHints.merge(hints)
	if hints.Group != "" {
		b.entity.ToolGroup = hints.Group
	}
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) List(fn func(ListOpts) ([]T, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.List = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) ListWithContext(fn func(context.Context, ListOpts) ([]T, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.ListWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) ListPaged(fn func(ListOpts) (PagedResult[T], error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.ListPaged = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) ListPagedWithContext(fn func(context.Context, ListOpts) (PagedResult[T], error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.ListPagedWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Get(fn func(string) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.Get = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) GetWithFlags(flags ActionFlags, fn func(string, map[string]string) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.GetFlags = flags
	b.entity.GetWithFlags = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) GetWithContext(fn func(context.Context, string) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.GetWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Create(fn func(map[string]any) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.Create = fn
	return b
}

// CreateWithContext sets the context-aware create handler. The context carries
// request-scoped state (e.g. the originating *http.Request via
// rpc.RequestFromContext), letting handlers read the raw nested JSON body the
// executor would otherwise flatten to string flags.
func (b *EntityBuilder[T, ListOpts, R]) CreateWithContext(fn func(context.Context, map[string]any) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.CreateWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Update(fn func(string, map[string]any) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.Update = fn
	return b
}

// UpdateWithContext sets the context-aware update handler. See CreateWithContext.
func (b *EntityBuilder[T, ListOpts, R]) UpdateWithContext(fn func(context.Context, string, map[string]any) (R, error)) *EntityBuilder[T, ListOpts, R] {
	b.entity.UpdateWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Delete(fn func(string) error) *EntityBuilder[T, ListOpts, R] {
	b.entity.Delete = fn
	return b
}

// DeleteWithContext sets the context-aware delete handler. See CreateWithContext.
func (b *EntityBuilder[T, ListOpts, R]) DeleteWithContext(fn func(context.Context, string) error) *EntityBuilder[T, ListOpts, R] {
	b.entity.DeleteWithContext = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Filters(filters ...Filter[ListOpts]) *EntityBuilder[T, ListOpts, R] {
	b.entity.Filters = append(b.entity.Filters, filters...)
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) WithAction(action EntityAction) *EntityBuilder[T, ListOpts, R] {
	b.entity.Actions = append(b.entity.Actions, action)
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) WithBulkAction(action EntityBulkAction) *EntityBuilder[T, ListOpts, R] {
	b.entity.BulkActions = append(b.entity.BulkActions, action)
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Admin(admin Entity[T, ListOpts, R]) *EntityBuilder[T, ListOpts, R] {
	b.entity.Admin = &admin
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) ValidArgs(fn func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) *EntityBuilder[T, ListOpts, R] {
	b.entity.ValidArgs = fn
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Build() Entity[T, ListOpts, R] {
	return b.entity
}

func (b *EntityBuilder[T, ListOpts, R]) Register() {
	RegisterEntity(b.entity)
}
