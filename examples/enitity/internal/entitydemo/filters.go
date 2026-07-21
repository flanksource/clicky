package entitydemo

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
)

func (d *demoStore) matchingStacksLocked(opts stackListOpts, includeArchived bool) []stack {
	items := make([]stack, 0, len(d.stacks))
	for _, item := range d.stacks {
		if item.Archived && !includeArchived && !opts.IncludeArchived {
			continue
		}
		if opts.Team != "" && item.Team != opts.Team {
			continue
		}
		if opts.Status != "" && item.Status != opts.Status {
			continue
		}
		if opts.Region != "" && !strings.EqualFold(item.Region, opts.Region) {
			continue
		}
		if len(opts.Tags) > 0 && !containsAll(item.Tags, opts.Tags) {
			continue
		}
		if !opts.From.IsZero() && item.LastDeploy.Before(opts.From) {
			continue
		}
		if !opts.To.IsZero() && item.LastDeploy.After(opts.To) {
			continue
		}
		items = append(items, item)
	}
	return items
}

type stackTeamFilter struct{}

func (stackTeamFilter) Key() string { return "team" }

func (stackTeamFilter) Label() string { return "Team" }

func (stackTeamFilter) Lookup(opts *stackListOpts) (map[string]api.Textable, error) {
	if opts.Team == "" {
		return nil, nil
	}
	opts.Team = canonicalTeam(opts.Team)
	label := labelFromCanonicalTeam(opts.Team)
	return map[string]api.Textable{
		opts.Team: api.Text{
			Content: label,
			Style:   "font-semibold",
		},
	}, nil
}

func (stackTeamFilter) Options(opts stackListOpts) map[string]api.Textable {
	options := map[string]api.Textable{
		"team/platform": api.Text{Content: "Platform"},
		"team/core":     api.Text{Content: "Core"},
		"team/data":     api.Text{Content: "Data"},
	}
	if strings.EqualFold(opts.Region, "us-east-1") {
		return map[string]api.Textable{
			"team/core": options["team/core"],
		}
	}
	return options
}

type stackStatusFilter struct{}

func (stackStatusFilter) Key() string { return "status" }

func (stackStatusFilter) Label() string { return "Status" }

func (stackStatusFilter) Lookup(opts *stackListOpts) (map[string]api.Textable, error) {
	if opts.Status == "" {
		return nil, nil
	}
	opts.Status = canonicalStatus(opts.Status)
	label := labelFromCanonicalStatus(opts.Status)
	return map[string]api.Textable{
		opts.Status: api.Text{
			Content: label,
			Style:   statusStyle(opts.Status),
			Tooltip: api.Text{Content: "Canonical backend value: " + opts.Status},
		},
	}, nil
}

func (stackStatusFilter) Options(opts stackListOpts) map[string]api.Textable {
	options := map[string]api.Textable{
		"status:healthy":  api.Text{Content: "Healthy", Style: statusStyle("status:healthy")},
		"status:degraded": api.Text{Content: "Degraded", Style: statusStyle("status:degraded")},
		"status:paused":   api.Text{Content: "Paused", Style: statusStyle("status:paused")},
	}
	if opts.Team == "team/platform" {
		return map[string]api.Textable{
			"status:healthy": options["status:healthy"],
			"status:paused":  options["status:paused"],
		}
	}
	return options
}

type stackFromFilter struct{}

func (stackFromFilter) Key() string { return "from" }

func (stackFromFilter) Label() string { return "From" }

func (stackFromFilter) Lookup(_ *stackListOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (stackFromFilter) Options(_ stackListOpts) map[string]api.Textable {
	return nil
}

type stackToFilter struct{}

func (stackToFilter) Key() string { return "to" }

func (stackToFilter) Label() string { return "To" }

func (stackToFilter) Lookup(_ *stackListOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (stackToFilter) Options(_ stackListOpts) map[string]api.Textable {
	return nil
}

func registerEntities(store *demoStore) {
	stackFilters := []clicky.Filter[stackListOpts]{
		stackTeamFilter{},
		stackStatusFilter{},
		stackFromFilter{},
		stackToFilter{},
	}

	clicky.NewEntity[stack, stackListOpts, stackDetail]("stack").
		Aliases("stacks", "svc").
		Filters(stackFilters...).
		List(store.listStacks).
		GetWithFlags(inspectFlags{}, store.getStack).
		Create(store.createStack).
		Update(store.updateStack).
		Delete(store.deleteStack).
		WithAction(
			clicky.ActionWithFlags("restart", restartFlags{}, store.restartStack).
				WithShort("Restart a stack and record synthetic audit metadata"),
		).
		WithBulkAction(
			clicky.BulkActionWithFilter("pause", store.pauseStacks, store.pauseStacksByFilter).
				WithShort("Pause stacks directly by id or indirectly through filter mode"),
		).
		Admin(clicky.Entity[stack, stackListOpts, stackDetail]{
			Filters:      stackFilters,
			List:         store.listAdminStacks,
			GetFlags:     adminInspectFlags{},
			GetWithFlags: store.getAdminStack,
			Actions: []clicky.EntityAction{
				clicky.Action("reconcile", store.reconcileStack).
					WithShort("Force a synthetic admin reconcile for a stack"),
			},
		}).
		ValidArgs(store.completeStackIDs).
		Register()

	clicky.NewEntity[cluster, clusterListOpts, cluster]("cluster").
		Parent("catalog").
		List(store.listClusters).
		Get(store.getCluster).
		Register()

	clicky.NewEntity[team, teamListOpts, team]("team").
		List(store.listTeams).
		Get(store.getTeam).
		Register()
}

func registerSubCommands(store *demoStore) {
	clicky.RegisterSubCommand("stack", &cobra.Command{
		Use:   "seed",
		Short: "Reset the in-memory demo data set",
		RunE: func(cmd *cobra.Command, args []string) error {
			store.reset()
			stacks, clusters := store.counts()
			fmt.Fprintf(cmd.OutOrStdout(), "reset demo store with %d stacks and %d clusters\n", stacks, clusters)
			return nil
		},
	})

	clicky.RegisterSubCommandFn("stack", func(parent *cobra.Command) {
		clicky.AddNamedCommand("summary", parent, stackSummaryOpts{}, store.summarizeStacks)
	})
}
