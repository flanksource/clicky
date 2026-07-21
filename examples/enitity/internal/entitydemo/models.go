package entitydemo

import (
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
)

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

func (s stack) GetID() string { return s.ID }

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

func (t team) GetID() string { return t.ID }

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

func (c cluster) GetID() string { return c.ID }

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
