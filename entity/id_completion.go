package entity

import (
	"context"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// IDOption is one of an entity's ids as its <id> argument offers it.
type IDOption struct {
	ID, Name, Description string
}

// IDOptionsFunc lists the ids an <id> argument offers.
type IDOptionsFunc func(ctx context.Context) ([]IDOption, error)

// EntityDescriber is an OPTIONAL extension of EntityItem: an item's
// description, shown with its id where the id is offered (the interactive
// form's id picker).
type EntityDescriber interface {
	GetDescription() string
}

// idOptionsRegistry maps the generated commands that take an entity's <id>
// to the entity's id options.
var idOptionsRegistry sync.Map // map[*cobra.Command]IDOptionsFunc

// GetIDOptions returns the id options of a generated command's <id>
// argument, nil when the entity has no list (or declares its own ValidArgs).
func GetIDOptions(cmd *cobra.Command) IDOptionsFunc {
	if v, ok := idOptionsRegistry.Load(cmd); ok {
		return v.(IDOptionsFunc)
	}
	return nil
}

// entityIDOptions lists the entity's items as id options. Nil for an entity
// that declares its own ValidArgs (its ids are its own) or has no list.
func entityIDOptions[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) IDOptionsFunc {
	list := entityLister(e)
	if e.ValidArgs != nil || list == nil {
		return nil
	}
	return func(ctx context.Context) ([]IDOption, error) {
		opts, err := buildOpts[ListOpts](map[string]string{})
		if err != nil {
			return nil, err
		}
		items, err := list(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]IDOption, 0, len(items))
		for _, item := range items {
			option := IDOption{ID: item.GetID(), Name: item.GetName()}
			if option.ID == "" {
				continue
			}
			if describer, ok := any(item).(EntityDescriber); ok {
				option.Description = describer.GetDescription()
			}
			out = append(out, option)
		}
		return out, nil
	}
}

// entityIDCompletion is the shell completion of an entity's <id> argument: the
// entity's own ValidArgs when it declares one, otherwise its id options — each
// as "id<TAB>name", matched on either by prefix — so every get/update/delete/
// action command completes ids (and interactive forms offer them) without
// extra wiring. Nil for an entity with no list.
func entityIDCompletion[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	if e.ValidArgs != nil {
		return e.ValidArgs
	}
	options := entityIDOptions(e)
	if options == nil {
		return nil
	}
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ids, err := options(ctx)
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveError
		}
		out := make([]string, 0, min(len(ids), lookupOptionsLimit))
		for _, option := range ids {
			if !strings.HasPrefix(option.ID, toComplete) && !strings.HasPrefix(option.Name, toComplete) {
				continue
			}
			entry := option.ID
			if option.Name != "" && option.Name != option.ID {
				entry += "\t" + option.Name
			}
			if out = append(out, entry); len(out) == lookupOptionsLimit {
				break
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// storeIDOptions registers the entity's id options for the commands it just
// generated under entityCmd that take its <id> (those that complete it).
func storeIDOptions(entityCmd *cobra.Command, existing map[*cobra.Command]bool, options IDOptionsFunc) {
	if options == nil {
		return
	}
	for _, cmd := range entityCmd.Commands() {
		if !existing[cmd] && cmd.ValidArgsFunction != nil {
			idOptionsRegistry.Store(cmd, options)
		}
	}
}

// entityLister returns the entity's list as one context-aware func, whichever
// variant it declares.
func entityLister[T EntityItem, ListOpts any, R any](e Entity[T, ListOpts, R]) func(context.Context, ListOpts) ([]T, error) {
	switch {
	case e.ListWithContext != nil:
		return e.ListWithContext
	case e.List != nil:
		return func(_ context.Context, opts ListOpts) ([]T, error) { return e.List(opts) }
	case e.ListPagedWithContext != nil:
		return func(ctx context.Context, opts ListOpts) ([]T, error) {
			page, err := e.ListPagedWithContext(ctx, opts)
			return page.Data, err
		}
	case e.ListPaged != nil:
		return func(_ context.Context, opts ListOpts) ([]T, error) {
			page, err := e.ListPaged(opts)
			return page.Data, err
		}
	}
	return nil
}
