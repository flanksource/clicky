package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/aichat"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/docs"
	"github.com/flanksource/clicky/extensions"
	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/markdown"
	"github.com/flanksource/clicky/mcp"
	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// webappFS carries the Vite-built single-page app. `webapp/dist/index.html`
// is committed as a placeholder so this embed always resolves; running
// `pnpm build` inside webapp/ replaces the contents with hashed bundles.
//
//go:embed webapp/dist
var webappFS embed.FS

type stack struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Team       string    `json:"team"`
	ClusterID  string    `json:"clusterId"`
	Status     string    `json:"status"`
	Region     string    `json:"region"`
	Tags       []string  `json:"tags,omitempty"`
	Archived   bool      `json:"archived,omitempty"`
	Version    int       `json:"version"`
	LastDeploy time.Time `json:"lastDeploy"`
}

func (s stack) GetID() string   { return s.ID }
func (s stack) GetName() string { return s.Name }

func (s stack) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "ID"},
		{Name: "Name", Style: "font-semibold"},
		{Name: "Team"},
		{Name: "Cluster"},
		{Name: "Status"},
		{Name: "Region"},
		{Name: "Tags", Style: "text-slate-600"},
		{Name: "Version", Type: "int"},
		{Name: "LastDeploy", Label: "Last deploy", Type: "date"},
	}
}

func (s stack) Row() map[string]any {
	teamID := strings.TrimPrefix(s.Team, "team/")
	return map[string]any{
		"ID": clicky.LinkCommand("stack/get").
			WithArgs(s.ID).
			WithAutoRun(true).
			Append(s.ID, "text-sky-700 underline underline-offset-4"),
		"Name": s.Name,
		"Team": clicky.LinkCommand("team/get").
			WithTarget(clicky.LinkTargetDialog).
			WithArgs(teamID).
			WithAutoRun(true).
			Append(labelFromCanonicalTeam(s.Team), "text-sky-700 underline underline-offset-4"),
		"Cluster": clicky.LinkCommand("cluster/get").
			WithTarget(clicky.LinkTargetDialog).
			WithArgs(s.ClusterID).
			WithAutoRun(true).
			Append(s.ClusterID, "text-sky-700 underline underline-offset-4"),
		"Status":     api.Text{Content: labelFromCanonicalStatus(s.Status), Style: statusStyle(s.Status)},
		"Region":     s.Region,
		"Tags":       strings.Join(s.Tags, ", "),
		"Version":    s.Version,
		"LastDeploy": s.LastDeploy,
	}
}

type cluster struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Region   string `json:"region"`
}

type team struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Owner string `json:"owner"`
}

func (t team) GetID() string   { return t.ID }
func (t team) GetName() string { return t.Name }

func (t team) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "ID"},
		{Name: "Name", Style: "font-semibold"},
		{Name: "Slug"},
		{Name: "Owner"},
	}
}

func (t team) Row() map[string]any {
	return map[string]any{
		"ID": clicky.LinkCommand("team/get").
			WithArgs(t.ID).
			WithAutoRun(true).
			Append(t.ID, "text-sky-700 underline underline-offset-4"),
		"Name":  t.Name,
		"Slug":  t.Slug,
		"Owner": t.Owner,
	}
}

type teamListOpts struct {
	Owner string `flag:"owner" help:"Filter by team owner"`
}

func (c cluster) GetID() string   { return c.ID }
func (c cluster) GetName() string { return c.Name }

func (c cluster) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "ID"},
		{Name: "Name", Style: "font-semibold"},
		{Name: "Provider"},
		{Name: "Region"},
	}
}

func (c cluster) Row() map[string]any {
	return map[string]any{
		"ID": clicky.LinkCommand("cluster/get").
			WithArgs(c.ID).
			WithAutoRun(true).
			Append(c.ID, "text-sky-700 underline underline-offset-4"),
		"Name":     c.Name,
		"Provider": c.Provider,
		"Region":   c.Region,
	}
}

type stackWindowOpts struct {
	Tags []string  `flag:"tags" help:"Return only stacks containing all of these tags"`
	From time.Time `flag:"from" help:"Return stacks deployed on or after this time" default:"now-30d"`
	To   time.Time `flag:"to" help:"Return stacks deployed on or before this time" default:"now"`
}

type stackListOpts struct {
	stackWindowOpts
	Team            string `flag:"team" help:"Team slug or canonical backend team id"`
	Status          string `flag:"status" help:"Status label or canonical backend status id"`
	Region          string `flag:"region" help:"Region filter"`
	Filter          string `flag:"filter" help:"Switch bulk actions into filter mode when set"`
	IncludeArchived bool   `flag:"include-archived" help:"Include archived stacks"`
}

type clusterListOpts struct {
	Provider string `flag:"provider" help:"Cloud provider filter"`
	Region   string `flag:"region" help:"Region filter"`
}

type inspectFlags struct {
	Events       int  `flag:"events" help:"Number of synthetic events to include" default:"3"`
	IncludeAudit bool `flag:"include-audit" help:"Include audit metadata" default:"false"`
}

func (inspectFlags) ClickyActionFlags() {}

type restartFlags struct {
	Drain  bool   `flag:"drain" help:"Drain traffic before restart" default:"true"`
	Reason string `flag:"reason" help:"Reason to record for the restart" default:"manual"`
}

func (restartFlags) ClickyActionFlags() {}

type adminInspectFlags struct {
	IncludeSecret bool `flag:"include-secret" help:"Include simulated secret material" default:"false"`
}

func (adminInspectFlags) ClickyActionFlags() {}

type stackSummaryOpts struct {
	Team            string `flag:"team" help:"Only summarize one team"`
	IncludeArchived bool   `flag:"include-archived" help:"Include archived stacks in the summary"`
}

type stackDetail struct {
	Stack  stack             `json:"stack"`
	Events []string          `json:"events,omitempty"`
	Audit  map[string]any    `json:"audit,omitempty"`
	Notes  map[string]string `json:"notes,omitempty"`
	Secret map[string]string `json:"secret,omitempty"`
	Admin  map[string]string `json:"admin,omitempty"`
	Flags  map[string]string `json:"flags,omitempty"`
	Meta   map[string]any    `json:"meta,omitempty"`
}

type actionResult struct {
	Action     string   `json:"action"`
	IDs        []string `json:"ids,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Drain      bool     `json:"drain,omitempty"`
	MatchedBy  string   `json:"matchedBy,omitempty"`
	MatchedIDs []string `json:"matchedIds,omitempty"`
}

type stackSummary struct {
	Total    int            `json:"total"`
	ByTeam   map[string]int `json:"byTeam"`
	ByStatus map[string]int `json:"byStatus"`
}

type demoStore struct {
	mu           sync.Mutex
	nextStackID  int
	stacks       map[string]stack
	clusters     map[string]cluster
	teams        map[string]team
	restartLog   []string
	reconcileLog []string
}

func newDemoStore() *demoStore {
	store := &demoStore{}
	store.reset()
	return store
}

func (d *demoStore) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextStackID = 4
	d.restartLog = nil
	d.reconcileLog = nil
	d.stacks = map[string]stack{
		"stk-001": {
			ID:         "stk-001",
			Name:       "checkout",
			Team:       "team/platform",
			ClusterID:  "cls-001",
			Status:     "status:healthy",
			Region:     "eu-west-1",
			Tags:       []string{"critical", "customer"},
			Version:    12,
			LastDeploy: time.Now().Add(-4 * time.Hour).UTC(),
		},
		"stk-002": {
			ID:         "stk-002",
			Name:       "billing",
			Team:       "team/core",
			ClusterID:  "cls-002",
			Status:     "status:degraded",
			Region:     "us-east-1",
			Tags:       []string{"payments", "database"},
			Version:    19,
			LastDeploy: time.Now().Add(-48 * time.Hour).UTC(),
		},
		"stk-003": {
			ID:         "stk-003",
			Name:       "marketing-site",
			Team:       "team/platform",
			ClusterID:  "cls-003",
			Status:     "status:paused",
			Region:     "eu-west-1",
			Tags:       []string{"public", "edge"},
			Archived:   true,
			Version:    7,
			LastDeploy: time.Now().Add(-14 * 24 * time.Hour).UTC(),
		},
	}
	d.clusters = map[string]cluster{
		"cls-001": {ID: "cls-001", Name: "shared-eu1", Provider: "aws", Region: "eu-west-1"},
		"cls-002": {ID: "cls-002", Name: "payments-us1", Provider: "aws", Region: "us-east-1"},
		"cls-003": {ID: "cls-003", Name: "labs-eu2", Provider: "gcp", Region: "europe-west2"},
	}
	d.teams = map[string]team{
		"platform": {ID: "platform", Name: "Platform", Slug: "team/platform", Owner: "alex"},
		"core":     {ID: "core", Name: "Core", Slug: "team/core", Owner: "jordan"},
		"data":     {ID: "data", Name: "Data", Slug: "team/data", Owner: "sam"},
	}
}

func (d *demoStore) listTeams(opts teamListOpts) ([]team, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	items := make([]team, 0, len(d.teams))
	for _, item := range d.teams {
		if opts.Owner != "" && !strings.EqualFold(item.Owner, opts.Owner) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (d *demoStore) getTeam(id string) (team, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.teams[id]
	if !ok {
		return team{}, fmt.Errorf("team %q not found", id)
	}
	return item, nil
}

func (d *demoStore) counts() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.stacks), len(d.clusters)
}

func (d *demoStore) listStacks(opts stackListOpts) ([]stack, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	items := d.matchingStacksLocked(opts, false)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (d *demoStore) listAdminStacks(opts stackListOpts) ([]stack, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	items := d.matchingStacksLocked(opts, true)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (d *demoStore) getStack(id string, flags map[string]string) (stackDetail, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.stacks[id]
	if !ok {
		return stackDetail{}, fmt.Errorf("stack %q not found", id)
	}

	detail := stackDetail{
		Stack: item,
		Events: syntheticEvents(
			item.Name,
			intFlag(flags, "events", 3),
		),
		Flags: copyStringMap(flags),
		Notes: map[string]string{
			"team":   labelFromCanonicalTeam(item.Team),
			"status": labelFromCanonicalStatus(item.Status),
		},
	}
	if boolFlag(flags, "include-audit") {
		detail.Audit = map[string]any{
			"lastRestart": lastEntry(d.restartLog),
			"lastDeploy":  item.LastDeploy,
			"version":     item.Version,
		}
	}

	return detail, nil
}

func (d *demoStore) getAdminStack(id string, flags map[string]string) (stackDetail, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.stacks[id]
	if !ok {
		return stackDetail{}, fmt.Errorf("stack %q not found", id)
	}

	detail := stackDetail{
		Stack: item,
		Admin: map[string]string{
			"canonicalTeam":   item.Team,
			"canonicalStatus": item.Status,
			"reconcileHint":   "run admin stack reconcile to refresh synthetic health",
		},
		Meta: map[string]any{
			"restartLog":   append([]string(nil), d.restartLog...),
			"reconcileLog": append([]string(nil), d.reconcileLog...),
		},
		Flags: copyStringMap(flags),
	}
	if boolFlag(flags, "include-secret") {
		detail.Secret = map[string]string{
			"token":    "demo-token",
			"rotation": "disabled in the example store",
		}
	}
	return detail, nil
}

func (d *demoStore) createStack(body map[string]any) (stackDetail, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	name := valueOrDefault(stringValue(body["name"]), "unnamed")
	team := canonicalTeam(valueOrDefault(stringValue(body["team"]), "platform"))
	status := canonicalStatus(valueOrDefault(stringValue(body["status"]), "healthy"))
	region := valueOrDefault(stringValue(body["region"]), "eu-west-1")
	id := fmt.Sprintf("stk-%03d", d.nextStackID)
	d.nextStackID++

	item := stack{
		ID:         id,
		Name:       name,
		Team:       team,
		Status:     status,
		Region:     region,
		Tags:       sliceValue(body["tags"]),
		Archived:   boolValue(body["archived"]),
		Version:    1,
		LastDeploy: time.Now().UTC(),
	}
	d.stacks[id] = item
	return stackDetail{Stack: item}, nil
}

func (d *demoStore) updateStack(id string, body map[string]any) (stackDetail, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.stacks[id]
	if !ok {
		return stackDetail{}, fmt.Errorf("stack %q not found", id)
	}

	if value := stringValue(body["name"]); value != "" {
		item.Name = value
	}
	if value := stringValue(body["team"]); value != "" {
		item.Team = canonicalTeam(value)
	}
	if value := stringValue(body["status"]); value != "" {
		item.Status = canonicalStatus(value)
	}
	if value := stringValue(body["region"]); value != "" {
		item.Region = value
	}
	if raw, ok := body["tags"]; ok {
		item.Tags = sliceValue(raw)
	}
	if raw, ok := body["archived"]; ok {
		item.Archived = boolValue(raw)
	}
	item.Version++
	item.LastDeploy = time.Now().UTC()
	d.stacks[id] = item
	return stackDetail{Stack: item}, nil
}

func (d *demoStore) deleteStack(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.stacks[id]; !ok {
		return fmt.Errorf("stack %q not found", id)
	}
	delete(d.stacks, id)
	return nil
}

func (d *demoStore) restartStack(id string, flags map[string]string) (actionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.stacks[id]
	if !ok {
		return actionResult{}, fmt.Errorf("stack %q not found", id)
	}
	item.Status = "status:healthy"
	item.Version++
	item.LastDeploy = time.Now().UTC()
	d.stacks[id] = item

	reason := valueOrDefault(strings.TrimSpace(flags["reason"]), "manual")
	drain := boolFlagDefault(flags, "drain", true)
	entry := fmt.Sprintf("%s restarted with reason=%s drain=%t", id, reason, drain)
	d.restartLog = append(d.restartLog, entry)

	return actionResult{
		Action: "restart",
		IDs:    []string{id},
		Reason: reason,
		Drain:  drain,
	}, nil
}

func (d *demoStore) reconcileStack(id string, flags map[string]string) (actionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.stacks[id]
	if !ok {
		return actionResult{}, fmt.Errorf("stack %q not found", id)
	}
	item.Status = "status:healthy"
	d.stacks[id] = item

	entry := fmt.Sprintf("%s reconciled", id)
	d.reconcileLog = append(d.reconcileLog, entry)

	return actionResult{
		Action: "reconcile",
		IDs:    []string{id},
	}, nil
}

func (d *demoStore) pauseStacks(ids []string, _ map[string]string) (actionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	updated := make([]string, 0, len(ids))
	for _, id := range ids {
		item, ok := d.stacks[id]
		if !ok {
			return actionResult{}, fmt.Errorf("stack %q not found", id)
		}
		item.Status = "status:paused"
		item.Version++
		d.stacks[id] = item
		updated = append(updated, id)
	}
	sort.Strings(updated)

	return actionResult{
		Action: "pause",
		IDs:    updated,
	}, nil
}

func (d *demoStore) pauseStacksByFilter(opts stackListOpts, _ map[string]string) (actionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	matched := d.matchingStacksLocked(opts, true)
	ids := make([]string, 0, len(matched))
	for _, item := range matched {
		current := d.stacks[item.ID]
		current.Status = "status:paused"
		current.Version++
		d.stacks[item.ID] = current
		ids = append(ids, item.ID)
	}
	sort.Strings(ids)

	return actionResult{
		Action:     "pause",
		MatchedBy:  opts.Filter,
		MatchedIDs: ids,
	}, nil
}

func (d *demoStore) summarizeStacks(opts stackSummaryOpts) (stackSummary, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	summary := stackSummary{
		ByTeam:   map[string]int{},
		ByStatus: map[string]int{},
	}
	team := canonicalTeam(opts.Team)
	for _, item := range d.stacks {
		if !opts.IncludeArchived && item.Archived {
			continue
		}
		if opts.Team != "" && item.Team != team {
			continue
		}
		summary.Total++
		summary.ByTeam[labelFromCanonicalTeam(item.Team)]++
		summary.ByStatus[labelFromCanonicalStatus(item.Status)]++
	}
	return summary, nil
}

func (d *demoStore) listClusters(opts clusterListOpts) ([]cluster, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	items := make([]cluster, 0, len(d.clusters))
	for _, item := range d.clusters {
		if opts.Provider != "" && !strings.EqualFold(item.Provider, opts.Provider) {
			continue
		}
		if opts.Region != "" && !strings.EqualFold(item.Region, opts.Region) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (d *demoStore) getCluster(id string) (cluster, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	item, ok := d.clusters[id]
	if !ok {
		return cluster{}, fmt.Errorf("cluster %q not found", id)
	}
	return item, nil
}

func (d *demoStore) completeStackIDs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ids := make([]string, 0, len(d.stacks))
	for id := range d.stacks {
		if strings.HasPrefix(id, toComplete) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, cobra.ShellCompDirectiveNoFileComp
}

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

func (stackTeamFilter) Key() string   { return "team" }
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

func (stackStatusFilter) Key() string   { return "status" }
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

func (stackFromFilter) Key() string   { return "from" }
func (stackFromFilter) Label() string { return "From" }

func (stackFromFilter) Lookup(_ *stackListOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (stackFromFilter) Options(_ stackListOpts) map[string]api.Textable {
	return nil
}

type stackToFilter struct{}

func (stackToFilter) Key() string   { return "to" }
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

// newServeUICommand wires the clicky RPC executor together with the Vite
// webapp embedded above. The webapp consumes `@flanksource/clicky-ui`'s
// metadata-driven `EntityExplorerApp` against `/api/openapi.json` and `/api/v1/...`.
func newServeUICommand() *cobra.Command {
	var (
		host   string
		port   int
		dev    bool
		uiPort int
	)

	cmd := &cobra.Command{
		Use:   "serve-ui",
		Short: "Start the HTTP API and embedded operation-catalog UI",
		Long: `Start an HTTP server that exposes both the executor-backed OpenAPI endpoints
and the embedded React UI built from clicky-ui's metadata-driven entity explorer.

The API is served at /api/openapi.json + /api/v1/..., the UI at /. Build the
Vite frontend with ` + "`cd webapp && pnpm install && pnpm build`" + ` before
compiling the Go binary so the embedded assets are current.

With --dev, this command additionally launches the Vite dev server (HMR) from
webapp/, which resolves @flanksource/clicky-ui from the sibling checked-out
clicky-ui repo (../../../../clicky-ui/packages/ui/dist) and proxies /api back to
this Go process. Open the printed Vite URL to develop against local clicky-ui
source — no embedded rebuild needed. Requires a source checkout with pnpm
available and ` + "`pnpm install`" + ` already run in webapp/.`,
		Example: `  entity-demo serve-ui --port 8080
  entity-demo serve-ui --host 0.0.0.0 --port 9090
  entity-demo serve-ui --dev               # API + Vite HMR against local clicky-ui`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port: %d", port)
			}

			rootCmd := cmd.Root()
			openAPIConfig := &rpc.OpenAPIConfig{
				Title:       "Clicky Entity Example",
				Description: "Entity example app with embedded metadata-driven explorer UI.",
				Version:     "1.0.0",
			}
			serveConfig := &rpc.ServeConfig{
				Host:    host,
				Port:    port,
				Title:   openAPIConfig.Title,
				Version: openAPIConfig.Version,
				Executor: &rpc.ExecutorConfig{
					Enabled:    true,
					SkipPreRun: true,
					PathPrefix: "/api/v1",
				},
			}

			server := rpc.NewSwaggerServer(serveConfig, rootCmd, openAPIConfig)

			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			mux.HandleFunc("/api/examples/links", serveLinkExamples)
			mux.HandleFunc("/api/examples/markdown-preview", serveMarkdownPreview)

			// AI chat backend: the demo's own entity operations become tools.
			// Requires a provider key (ANTHROPIC_API_KEY / OPENAI_API_KEY /
			// GOOGLE_API_KEY); it fails loud on the first request otherwise.
			chat := aichat.NewServer(aichat.Options{
				RootCmd: rootCmd,
				System: "You are an operator assistant for this entity demo " +
					"(stacks, clusters, teams). Prefer calling an operation over " +
					"guessing, and summarize results clearly.",
				// Persist conversations in-memory so the thread endpoints work.
				Threads: aichat.NewMemThreadStore(),
				// Demonstrate human-in-the-loop approvals: any mutating operation
				// (restart/delete/reconcile/pause) pauses for the user to approve.
				ApprovalPolicy: func(toolName string, _ any) bool {
					for _, verb := range []string{"restart", "delete", "reconcile", "pause", "destroy"} {
						if strings.Contains(toolName, verb) {
							return true
						}
					}
					return false
				},
			})
			// Mount as a subtree so /api/chat, /api/chat/models and the thread
			// endpoints all resolve.
			mux.Handle("/api/chat", chat.Handler())
			mux.Handle("/api/chat/", chat.Handler())

			uiHandler, err := newWebappHandler()
			if err != nil {
				return fmt.Errorf("load embedded webapp: %w", err)
			}
			mux.Handle("/", uiHandler)

			addr := fmt.Sprintf("%s:%d", host, port)
			httpSrv := &http.Server{
				Addr:        addr,
				Handler:     mux,
				ReadTimeout: 30 * time.Second,
				// No WriteTimeout: /api/chat streams SSE responses that stay open
				// well past any fixed deadline; a write timeout truncates them.
				IdleTimeout: 60 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				fmt.Fprintf(cmd.OutOrStdout(), "🚀 Entity UI listening on http://%s\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • UI:           http://%s/\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • OpenAPI JSON: http://%s/api/openapi.json\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • Executor API: http://%s/api/v1/...\n", addr)
				fmt.Fprintf(cmd.OutOrStdout(), "   • AI Chat:      http://%s/api/chat\n", addr)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			if dev {
				vite, err := startViteDevServer(ctx, cmd, host, port, uiPort)
				if err != nil {
					return err
				}
				// ctx cancellation (Ctrl-C) kills the Vite process group via the
				// CommandContext Cancel hook; Wait reaps it on shutdown.
				defer func() { _ = vite.Wait() }()
			}

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return httpSrv.Shutdown(shutdownCtx)
			case err := <-errCh:
				return err
			}
		},
	}

	cmd.Flags().StringVar(&host, "host", "localhost", "Host to bind the server to")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to bind the server to")
	cmd.Flags().BoolVar(&dev, "dev", false, "Launch the Vite dev server (HMR) against the checked-out clicky-ui instead of using the embedded build")
	cmd.Flags().IntVar(&uiPort, "ui-port", 5173, "Port for the Vite dev server (only used with --dev)")

	return cmd
}

// startViteDevServer launches `pnpm dev` in webapp/ for --dev mode. It points
// Vite's /api proxy back at this Go process via CLICKY_EXAMPLE_API_URL (read by
// webapp/vite.config.ts), so the HMR dev server — which resolves
// @flanksource/clicky-ui from the sibling checked-out clicky-ui repo — talks to
// the live executor API. The returned command is bound to ctx: Ctrl-C kills the
// whole Vite process group (pnpm + its node/esbuild children) and the caller
// reaps it with Wait.
func startViteDevServer(ctx context.Context, cmd *cobra.Command, apiHost string, apiPort, uiPort int) (*exec.Cmd, error) {
	webappDir, err := webappDevDir()
	if err != nil {
		return nil, err
	}
	apiURL := fmt.Sprintf("http://%s:%d", apiHost, apiPort)

	// `pnpm exec vite` runs the dev server binary directly so --port/--strictPort
	// reach Vite; `pnpm dev -- ...` would forward them past Vite's arg parser and
	// it would silently fall back to the default port.
	vite := exec.CommandContext(ctx, "pnpm", "exec", "vite", "--port", strconv.Itoa(uiPort), "--strictPort")
	vite.Dir = webappDir
	vite.Env = append(os.Environ(), "CLICKY_EXAMPLE_API_URL="+apiURL)
	vite.Stdout = cmd.OutOrStdout()
	vite.Stderr = cmd.ErrOrStderr()
	// Run Vite in its own process group so we can signal the whole tree; the
	// default CommandContext kill only targets pnpm and would orphan node/vite.
	vite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	vite.Cancel = func() error {
		if vite.Process == nil {
			return nil
		}
		return syscall.Kill(-vite.Process.Pid, syscall.SIGTERM)
	}
	vite.WaitDelay = 5 * time.Second

	if err := vite.Start(); err != nil {
		return nil, fmt.Errorf("start vite dev server in %s (is pnpm installed and `pnpm install` run there?): %w", webappDir, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "   • Dev UI (Vite): http://localhost:%d/  (HMR, clicky-ui from ../../../../clicky-ui, /api → %s)\n", uiPort, apiURL)
	return vite, nil
}

// webappDevDir locates the webapp/ source directory next to this file. --dev is
// a from-source convenience, so it resolves via the compile-time source path
// (runtime.Caller) and fails loudly when run from a relocated binary.
func webappDevDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate webapp/ for --dev: runtime caller unavailable")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "webapp")
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return "", fmt.Errorf("webapp/package.json not found at %s — run --dev from a source checkout: %w", dir, err)
	}
	return dir, nil
}

// newWebappHandler returns an http.Handler that serves the embedded Vite
// build. Unknown paths fall back to index.html so the React router can
// handle client-side routes on a full page load.
func newWebappHandler() (http.Handler, error) {
	sub, err := fs.Sub(webappFS, "webapp/dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(r.URL.Path, "/")
		if requested == "" {
			serveIndex(w, sub)
			return
		}
		if _, err := fs.Stat(sub, requested); err != nil {
			// File not found: assume a SPA route and return index.html.
			if !looksLikeAssetRequest(requested) {
				serveIndex(w, sub)
				return
			}
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func serveIndex(w http.ResponseWriter, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "webapp index.html missing — run `pnpm build` in webapp/", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func serveLinkExamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := clicky.Format(linkExamplesDocument(), clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render link examples: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json+clicky")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(payload))
}

func serveMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	source, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read markdown: %v", err), http.StatusBadRequest)
		return
	}
	doc, err := markdown.ParseString(string(source))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse markdown: %v", err), http.StatusBadRequest)
		return
	}

	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "clicky-json"
	}
	format = normalizeMarkdownPreviewFormat(format)
	if !markdownPreviewFormats[format] {
		http.Error(w, fmt.Sprintf("unsupported format: %s", format), http.StatusBadRequest)
		return
	}

	if format == "excel" {
		serveMarkdownPreviewExcel(w, markdownPreviewRows(doc))
		return
	}

	payloadData := any(doc)
	if format == "csv" {
		payloadData = markdownPreviewRows(doc)
	}
	payload, err := clicky.Format(payloadData, clicky.FormatOptions{Format: format})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to render %s preview: %v", format, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", markdownPreviewContentType(format))
	if _, err := w.Write([]byte(payload)); err != nil {
		fmt.Fprintf(os.Stderr, "write markdown preview response: %v\n", err)
	}
}

func serveMarkdownPreviewExcel(w http.ResponseWriter, rows []markdownPreviewRow) {
	tmpDir, err := os.MkdirTemp("", "clicky-markdown-preview-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create excel preview: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	output := filepath.Join(tmpDir, "markdown-preview.xlsx")
	manager := formatters.NewFormatManager()
	if err := manager.ExcelToFile(rows, output); err != nil {
		http.Error(w, fmt.Sprintf("failed to render excel preview: %v", err), http.StatusInternalServerError)
		return
	}
	data, err := os.ReadFile(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read excel preview: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", clicky.FormatToContentType("excel"))
	w.Header().Set("Content-Disposition", `inline; filename="markdown-preview.xlsx"`)
	_, _ = w.Write(data)
}

// markdownPreviewFormats is the set of formats the preview endpoint accepts
// (after normalization). It mirrors clicky's known render formats so a bad
// client-supplied ?format= is rejected as 400 instead of surfacing as a 500.
var markdownPreviewFormats = map[string]bool{
	"clicky-json": true,
	"json":        true,
	"yaml":        true,
	"yml":         true,
	"csv":         true,
	"html":        true,
	"html-react":  true,
	"html-static": true,
	"markdown":    true,
	"pdf":         true,
	"slack":       true,
	"excel":       true,
	"tree":        true,
	"pretty":      true,
}

func normalizeMarkdownPreviewFormat(format string) string {
	switch strings.ToLower(format) {
	case "react":
		return "clicky-json"
	case "md":
		return "markdown"
	case "xlsx":
		return "excel"
	default:
		return strings.ToLower(format)
	}
}

func markdownPreviewContentType(format string) string {
	if format == "clicky-json" {
		return "application/json+clicky"
	}
	return clicky.FormatToContentType(format)
}

type markdownPreviewRow struct {
	Index int    `json:"index" pretty:"label=Index"`
	Kind  string `json:"kind" pretty:"label=Kind"`
	Level int    `json:"level,omitempty" pretty:"label=Level"`
	Text  string `json:"text" pretty:"label=Text"`
}

func markdownPreviewRows(doc *markdown.Document) []markdownPreviewRow {
	var rows []markdownPreviewRow
	var visit func(node markdown.Node)
	visit = func(node markdown.Node) {
		if node.Kind != "" && node.Kind != "document" {
			rows = append(rows, markdownPreviewRow{
				Index: len(rows) + 1,
				Kind:  node.Kind,
				Level: node.Level,
				Text:  strings.TrimSpace(node.String()),
			})
		}
		for _, child := range node.Children {
			visit(child)
		}
		for _, item := range node.Items {
			visit(item)
		}
	}
	if doc != nil {
		visit(doc.Root)
	}
	return rows
}

func linkExamplesDocument() api.DescriptionList {
	return api.DescriptionList{
		Items: []api.KeyValuePair{
			{
				Key: "Plain link targets",
				Value: api.DescriptionList{
					Items: []api.KeyValuePair{
						{
							Key: "default",
							Value: linkExampleValue(
								clicky.Link("/stacks").Append("Open the stacks surface", "text-sky-700 underline underline-offset-4"),
								"Uses a normal in-app anchor without forcing a specific browser target.",
							),
						},
						{
							Key: "_self",
							Value: linkExampleValue(
								clicky.Link("/clusters").WithTarget(clicky.LinkTargetSelf).
									Append("Navigate to clusters in this tab", "text-sky-700 underline underline-offset-4"),
								"Sets target=_self explicitly for same-tab navigation.",
							),
						},
						{
							Key: "_window",
							Value: linkExampleValue(
								clicky.Link("/explorer").WithTarget(clicky.LinkTargetWindow).
									Append("Open the API explorer in a new window", "text-sky-700 underline underline-offset-4"),
								"Renders as a browser new-context link using the _window target hint.",
							),
						},
						{
							Key: "_tab",
							Value: linkExampleValue(
								clicky.Link("/admin-stacks").WithTarget(clicky.LinkTargetTab).
									Append("Open admin stacks in a new tab", "text-sky-700 underline underline-offset-4"),
								"Uses the same browser new-context flow but advertises the _tab target intent.",
							),
						},
					},
				},
			},
			{
				Key: "LinkCommand targets",
				Value: api.DescriptionList{
					Items: []api.KeyValuePair{
						{
							Key: "Dialog auto-run",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetDialog).
									WithArgs("stk-001").
									WithFlag("events", "4").
									WithAutoRun(true).
									Append("Open a stack detail dialog", "text-cyan-700 underline underline-offset-4"),
								"Prefills id + events and runs immediately because every required parameter is already satisfied.",
							),
						},
						{
							Key: "Dialog waits for params",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetDialog).
									WithAutoRun(true).
									Append("Show the form before running", "text-cyan-700 underline underline-offset-4"),
								"Leaves required params empty so the dialog opens prefilled but waits for a manual run.",
							),
						},
						{
							Key: "Hover",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetHover).
									WithArgs("stk-002").
									WithFlag("events", "2").
									Append("Hover stack detail", "text-cyan-700 underline underline-offset-4"),
								"Resolves and executes lazily inside a hover preview.",
							),
						},
						{
							Key: "Expand",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetExpand).
									WithArgs("stk-001").
									WithFlag("events", "1").
									Append("Expand stack detail", "text-cyan-700 underline underline-offset-4"),
								"Loads inline beneath the trigger without leaving the page.",
							),
						},
						{
							Key: "_clicky",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetClicky).
									WithArgs("stk-001").
									WithFlag("events", "3").
									WithAutoRun(true).
									Append("Navigate inside Clicky", "text-cyan-700 underline underline-offset-4"),
								"Delegates navigation to the React host via commandRuntime.onNavigate.",
							),
						},
						{
							Key: "_self",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetSelf).
									WithArgs("stk-002").
									WithFlag("events", "2").
									WithAutoRun(true).
									Append("Navigate in this tab", "text-cyan-700 underline underline-offset-4"),
								"Builds a deep-link URL that lands on a prefilled command page and auto-runs there.",
							),
						},
						{
							Key: "_window",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetWindow).
									WithArgs("stk-001").
									WithFlag("events", "5").
									WithAutoRun(true).
									Append("Open in new window", "text-cyan-700 underline underline-offset-4"),
								"Uses the same deep-link URL builder but asks the browser for a new window context.",
							),
						},
						{
							Key: "_tab",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetTab).
									WithArgs("stk-002").
									WithFlag("events", "6").
									WithAutoRun(true).
									Append("Open in new tab", "text-cyan-700 underline underline-offset-4"),
								"Produces a deep-link URL that opens the command page in a new tab.",
							),
						},
					},
				},
			},
		},
	}
}

func linkExampleValue(link api.Textable, note string) api.Text {
	return clicky.Text("").Add(link).Append(" ").Append(note, "text-slate-600")
}

// looksLikeAssetRequest returns true when the request targets a file with a
// known extension, so we don't swallow a genuine 404 (e.g. a missing image
// reference) with the SPA fallback.
func looksLikeAssetRequest(requested string) bool {
	ext := strings.ToLower(path.Ext(requested))
	switch ext {
	case ".js", ".mjs", ".css", ".map", ".ico", ".png", ".jpg", ".jpeg",
		".gif", ".svg", ".webp", ".woff", ".woff2", ".ttf", ".eot", ".txt":
		return true
	}
	return false
}

func main() {
	store := newDemoStore()

	rootCmd := &cobra.Command{
		Use:   "entity-demo",
		Short: "Entity example covering clicky entity generation and served execution",
		Long: `A self-contained example showing how clicky entities can power both a CLI
and the executor-backed OpenAPI serve mode from the same registrations.`,
	}

	registerEntities(store)
	registerSubCommands(store)
	clicky.GenerateCLI(rootCmd)

	extensions.CobraExtensions(rootCmd).OpenAPICommandWithConfig(&rpc.OpenAPIConfig{
		Title:       "Clicky Entity Example",
		Description: "Entity example app covering CRUD, actions, filters, admin views, nested parents, and executor-backed serve mode.",
		Version:     "1.0.0",
		Tags: []rpc.OpenAPITag{
			{Name: "stack", Description: "Stack entity operations"},
			{Name: "catalog", Description: "Nested catalog entity operations"},
			{Name: "admin", Description: "Administrative entity operations"},
		},
	})

	extensions.CobraExtensions(rootCmd).DocsCommandWithConfig(&docs.DocsConfig{
		Title:       "Clicky Entity Example",
		Description: "Entity example app with a CLI reference and clicky-ui surface catalog.",
		Exclude:     []string{"serve-ui"},
	})

	rootCmd.AddCommand(newServeUICommand())

	// Expose every entity command as an MCP tool so the same registrations
	// drive a Claude/Cursor-compatible server. Demonstrates the fluent
	// Builder API: hide infra-only commands, strip flags that don't make
	// sense over MCP, and force Markdown-without-color output so AI
	// clients receive predictable, parseable text.
	rootCmd.AddCommand(
		mcp.NewMcpServer(rootCmd).
			AutoExpose().
			WithExclude("serve-ui").
			IgnoreParams("*", "--host", "--port").
			WithFormat(formatters.FormatOptions{Markdown: true, NoColor: true}).
			Command(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func containsAll(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, value := range have {
		set[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, value := range want {
		if _, ok := set[strings.ToLower(strings.TrimSpace(value))]; !ok {
			return false
		}
	}
	return true
}

func canonicalTeam(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "team/platform":
		return "team/platform"
	case "platform":
		return "team/platform"
	case "team/core":
		return "team/core"
	case "core":
		return "team/core"
	case "team/data":
		return "team/data"
	case "data":
		return "team/data"
	default:
		if strings.HasPrefix(value, "team/") {
			return value
		}
		return "team/" + value
	}
}

func labelFromCanonicalTeam(value string) string {
	return titleCase(strings.TrimPrefix(value, "team/"))
}

func canonicalStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "status:healthy":
		return "status:healthy"
	case "healthy":
		return "status:healthy"
	case "status:degraded":
		return "status:degraded"
	case "degraded":
		return "status:degraded"
	case "status:paused":
		return "status:paused"
	case "paused":
		return "status:paused"
	default:
		if strings.HasPrefix(value, "status:") {
			return value
		}
		return "status:" + value
	}
}

func labelFromCanonicalStatus(value string) string {
	return titleCase(strings.TrimPrefix(value, "status:"))
}

func statusStyle(value string) string {
	switch value {
	case "status:healthy":
		return "text-green-600 font-semibold"
	case "status:degraded":
		return "text-amber-600 font-semibold"
	case "status:paused":
		return "text-slate-500"
	default:
		return "text-slate-700"
	}
}

func syntheticEvents(name string, count int) []string {
	if count <= 0 {
		return nil
	}
	events := make([]string, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, fmt.Sprintf("%s event %d", name, i+1))
	}
	return events
}

func lastEntry(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func intFlag(flags map[string]string, key string, fallback int) int {
	if value, err := strconv.Atoi(flags[key]); err == nil {
		return value
	}
	return fallback
}

func boolFlag(flags map[string]string, key string) bool {
	value, err := strconv.ParseBool(flags[key])
	return err == nil && value
}

func boolFlagDefault(flags map[string]string, key string, fallback bool) bool {
	value, err := strconv.ParseBool(flags[key])
	if err != nil {
		return fallback
	}
	return value
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func sliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringValue(item); value != "" {
				items = append(items, value)
			}
		}
		return items
	case string:
		if typed == "" {
			return nil
		}
		parts := strings.Split(typed, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			if value := strings.TrimSpace(part); value != "" {
				items = append(items, value)
			}
		}
		return items
	default:
		value := stringValue(value)
		if value == "" {
			return nil
		}
		return []string{value}
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	default:
		parsed, err := strconv.ParseBool(fmt.Sprintf("%v", typed))
		return err == nil && parsed
	}
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}
