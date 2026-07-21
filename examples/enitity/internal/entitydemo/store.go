package entitydemo

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

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
