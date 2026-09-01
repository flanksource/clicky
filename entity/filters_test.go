package entity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
)

type entityFilterTestEntity struct {
	ID   string
	Name string
}

func (e entityFilterTestEntity) GetID() string   { return e.ID }
func (e entityFilterTestEntity) GetName() string { return e.Name }

type entityFilterEmbeddedOpts struct {
	Tags   []string          `flag:"tags"`
	Window duration.Duration `flag:"window" default:"24h"`
}

type entityFilterTestOpts struct {
	entityFilterEmbeddedOpts
	Owner  string    `flag:"owner"`
	Status string    `flag:"status"`
	Amount string    `flag:"amount"`
	Active bool      `flag:"active"`
	Since  time.Time `flag:"since"`
	From   time.Time `flag:"from"`
	To     time.Time `flag:"to"`
}

type entityMultiFilterTestOpts struct {
	Status MultiFilter `flag:"status"`
}

type entityFilterOuterOpts struct {
	entityFilterTestOpts
}

type ownerEntityFilter struct{}

func (ownerEntityFilter) Key() string   { return "owner" }
func (ownerEntityFilter) Label() string { return "Owner" }

func (ownerEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	if opts.Owner == "" {
		return nil, nil
	}
	opts.Owner = "team/" + opts.Owner
	return map[string]api.Textable{
		opts.Owner: api.Text{Content: strings.TrimPrefix(opts.Owner, "team/")},
	}, nil
}

func (ownerEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return map[string]api.Textable{
		"team/platform": api.Text{Content: "Platform"},
		"team/core":     api.Text{Content: "Core"},
	}
}

type statusEntityFilter struct{}

func (statusEntityFilter) Key() string   { return "status" }
func (statusEntityFilter) Label() string { return "Status" }

func (statusEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	if opts.Status == "" {
		return nil, nil
	}

	switch opts.Status {
	case "healthy":
		opts.Status = "status:healthy"
		return map[string]api.Textable{
			opts.Status: api.Text{
				Content: "Healthy",
				Style:   "font-semibold",
				Tooltip: api.Text{Content: "Ready"},
			},
		}, nil
	case "degraded":
		opts.Status = "status:degraded"
		return map[string]api.Textable{
			opts.Status: api.Text{Content: "Degraded"},
		}, nil
	default:
		return nil, nil
	}
}

func (statusEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	options := map[string]api.Textable{
		"status:healthy":  api.Text{Content: "Healthy", Style: "font-semibold"},
		"status:degraded": api.Text{Content: "Degraded"},
	}
	if opts.Owner == "team/platform" {
		return map[string]api.Textable{
			"status:healthy": options["status:healthy"],
		}
	}
	return options
}

type tagsEntityFilter struct{}

func (tagsEntityFilter) Key() string   { return "tags" }
func (tagsEntityFilter) Label() string { return "Tags" }

func (tagsEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	if len(opts.Tags) == 0 {
		return nil, nil
	}

	selected := make(map[string]api.Textable, len(opts.Tags))
	for _, tag := range opts.Tags {
		selected[tag] = api.Text{Content: strings.ToUpper(tag)}
	}
	return selected, nil
}

func (tagsEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return map[string]api.Textable{
		"api":    api.Text{Content: "API"},
		"worker": api.Text{Content: "Worker"},
	}
}

type amountEntityFilter struct{}

func (amountEntityFilter) Key() string   { return "amount" }
func (amountEntityFilter) Label() string { return "Amount" }

func (amountEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (amountEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return nil
}

func (amountEntityFilter) LookupType() string { return "number" }

type activeEntityFilter struct{}

func (activeEntityFilter) Key() string   { return "active" }
func (activeEntityFilter) Label() string { return "Active" }

func (activeEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	if !opts.Active {
		return nil, nil
	}
	return map[string]api.Textable{
		"true": api.Text{Content: "Active only"},
	}, nil
}

func (activeEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return map[string]api.Textable{
		"true": api.Text{Content: "Active only"},
	}
}

type sinceEntityFilter struct{}

func (sinceEntityFilter) Key() string   { return "since" }
func (sinceEntityFilter) Label() string { return "Since" }

func (sinceEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (sinceEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return nil
}

type fromEntityFilter struct{}

func (fromEntityFilter) Key() string   { return "from" }
func (fromEntityFilter) Label() string { return "From" }

func (fromEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (fromEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return nil
}

type toEntityFilter struct{}

func (toEntityFilter) Key() string   { return "to" }
func (toEntityFilter) Label() string { return "To" }

func (toEntityFilter) Lookup(opts *entityFilterTestOpts) (map[string]api.Textable, error) {
	return nil, nil
}

func (toEntityFilter) Options(opts entityFilterTestOpts) map[string]api.Textable {
	return nil
}

func TestBuildOptsSupportsRichTypes(t *testing.T) {
	opts, err := buildOpts[entityFilterTestOpts](map[string]string{
		"tags":   "api,worker",
		"window": "48h",
		"since":  "2026-04-20T15:04:05Z",
		"owner":  "platform",
		"status": "healthy",
	})
	if err != nil {
		t.Fatalf("buildOpts returned error: %v", err)
	}

	if len(opts.Tags) != 2 || opts.Tags[0] != "api" || opts.Tags[1] != "worker" {
		t.Fatalf("expected tags to be parsed, got %#v", opts.Tags)
	}

	if time.Duration(opts.Window) != 48*time.Hour {
		t.Fatalf("expected 48h window, got %s", opts.Window)
	}

	expectedSince := time.Date(2026, 4, 20, 15, 4, 5, 0, time.UTC)
	if !opts.Since.Equal(expectedSince) {
		t.Fatalf("expected since %s, got %s", expectedSince, opts.Since)
	}

	if opts.Owner != "platform" || opts.Status != "healthy" {
		t.Fatalf("expected string fields to be populated, got %#v", opts)
	}
}

func TestBuildOptsSupportsMultiFilter(t *testing.T) {
	opts, err := buildOpts[entityMultiFilterTestOpts](map[string]string{
		"status": "ready,!failed",
	})
	if err != nil {
		t.Fatalf("buildOpts returned error: %v", err)
	}

	if len(opts.Status) != 2 || opts.Status[0] != "ready" || opts.Status[1] != "!failed" {
		t.Fatalf("expected multi filter values to be parsed, got %#v", opts.Status)
	}
}

func TestLookupMetadataDetectsMultiFilter(t *testing.T) {
	metadata := buildLookupMetadata[entityMultiFilterTestOpts]()
	status := metadata["status"]
	if !status.Multi || status.Type != "multi-filter" {
		t.Fatalf("expected status to be a multi-filter, got %#v", status)
	}
}

func TestLiftedFiltersPreserveTypedLookupOverrides(t *testing.T) {
	filters := LiftFilters[entityFilterOuterOpts, entityFilterTestOpts](
		[]Filter[entityFilterTestOpts]{
			amountEntityFilter{},
			activeEntityFilter{},
		},
		func(opts *entityFilterOuterOpts) *entityFilterTestOpts {
			return &opts.entityFilterTestOpts
		},
	)

	lookupAny, err := buildLookupFunc(filters)(nil, nil)
	if err != nil {
		t.Fatalf("lookup func returned error: %v", err)
	}
	lookup := lookupAny.(entityLookupResponse)

	if lookup.Filters["amount"].Type != "number" {
		t.Fatalf("expected lifted typed filter to preserve type override, got %#v", lookup.Filters["amount"])
	}
	if lookup.Filters["active"].Type != "bool" {
		t.Fatalf("expected lifted untyped filter to keep inferred bool type, got %#v", lookup.Filters["active"])
	}
}

func TestEntityListFiltersResolveAndLookup(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	var received entityFilterTestOpts

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, entityFilterTestEntity]{
		Name: "filtered-entity",
		Filters: []Filter[entityFilterTestOpts]{
			ownerEntityFilter{},
			statusEntityFilter{},
			tagsEntityFilter{},
			amountEntityFilter{},
			activeEntityFilter{},
			sinceEntityFilter{},
			fromEntityFilter{},
			toEntityFilter{},
		},
		List: func(opts entityFilterTestOpts) ([]entityFilterTestEntity, error) {
			received = opts
			return []entityFilterTestEntity{{ID: "1", Name: "alpha"}}, nil
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	listCmd, _, err := root.Find([]string{"filtered-entity", "list"})
	if err != nil || listCmd == nil {
		t.Fatalf("expected to find list command, got err=%v", err)
	}

	dataFunc := GetDataFunc(listCmd)
	if dataFunc == nil {
		t.Fatal("expected list command to register data func")
	}

	if _, err := dataFunc(map[string]string{
		"owner":  "platform",
		"status": "healthy",
		"tags":   "api,worker",
		"window": "48h",
		"since":  "2026-04-20T15:04:05Z",
	}, nil); err != nil {
		t.Fatalf("list data func returned error: %v", err)
	}

	if received.Owner != "team/platform" {
		t.Fatalf("expected owner to be resolved, got %q", received.Owner)
	}

	if received.Status != "status:healthy" {
		t.Fatalf("expected status to be resolved, got %q", received.Status)
	}

	if len(received.Tags) != 2 || received.Tags[0] != "api" || received.Tags[1] != "worker" {
		t.Fatalf("expected tags to be parsed, got %#v", received.Tags)
	}

	if time.Duration(received.Window) != 48*time.Hour {
		t.Fatalf("expected resolved window to be 48h, got %s", received.Window)
	}

	lookupFunc := GetLookupFunc(listCmd)
	if lookupFunc == nil {
		t.Fatal("expected list command to register lookup func")
	}

	lookupAny, err := lookupFunc(map[string]string{
		"owner":  "platform",
		"status": "healthy",
	}, nil)
	if err != nil {
		t.Fatalf("lookup func returned error: %v", err)
	}

	lookup, ok := lookupAny.(entityLookupResponse)
	if !ok {
		t.Fatalf("expected lookup response type, got %T", lookupAny)
	}

	statusLookup, ok := lookup.Filters["status"]
	if !ok {
		t.Fatalf("expected status lookup payload, got %#v", lookup.Filters)
	}

	if len(statusLookup.Options) != 1 {
		t.Fatalf("expected narrowed status options, got %#v", statusLookup.Options)
	}

	if !lookup.Filters["tags"].Multi {
		t.Fatalf("expected tags lookup to advertise multi=true, got %#v", lookup.Filters["tags"])
	}

	if lookup.Filters["amount"].Type != "number" {
		t.Fatalf("expected amount lookup type=number from filter override, got %#v", lookup.Filters["amount"])
	}

	if lookup.Filters["active"].Type != "bool" {
		t.Fatalf("expected active lookup type=bool, got %#v", lookup.Filters["active"])
	}

	if lookup.Filters["since"].Type != "date" {
		t.Fatalf("expected since lookup type=date, got %#v", lookup.Filters["since"])
	}

	if lookup.Filters["from"].Type != "from" {
		t.Fatalf("expected from lookup type=from, got %#v", lookup.Filters["from"])
	}

	if lookup.Filters["to"].Type != "to" {
		t.Fatalf("expected to lookup type=to, got %#v", lookup.Filters["to"])
	}

	selected := statusLookup.Selected["status:healthy"]
	if selected.Kind != "text" || selected.Text != "Healthy" {
		t.Fatalf("expected selected option to be clicky text, got %#v", selected)
	}

	if selected.Style == nil || selected.Style.ClassName != "font-semibold" {
		t.Fatalf("expected selected option style to be preserved, got %#v", selected.Style)
	}

	if selected.Tooltip == nil || selected.Tooltip.Text != "Ready" {
		t.Fatalf("expected selected option tooltip to be preserved, got %#v", selected.Tooltip)
	}

	data, err := json.Marshal(lookup)
	if err != nil {
		t.Fatalf("failed to marshal lookup response: %v", err)
	}

	if !strings.Contains(string(data), `"kind":"text"`) {
		t.Fatalf("expected marshaled lookup to contain clicky nodes, got %s", string(data))
	}
}

func TestEntityBulkActionUsesResolvedFilters(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	var received entityFilterTestOpts

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, entityFilterTestEntity]{
		Name:    "filtered-bulk",
		Filters: []Filter[entityFilterTestOpts]{ownerEntityFilter{}, statusEntityFilter{}},
		BulkActions: []EntityBulkAction{
			BulkActionWithFilter(
				"bulk-suspend",
				func(ids []string, flags map[string]string) (any, error) {
					return ids, nil
				},
				func(opts entityFilterTestOpts, flags map[string]string) (any, error) {
					received = opts
					return map[string]string{
						"owner":  opts.Owner,
						"status": opts.Status,
					}, nil
				},
			).WithShort("Suspend entities matched by filters"),
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	bulkCmd, _, err := root.Find([]string{"filtered-bulk", "bulk-suspend"})
	if err != nil || bulkCmd == nil {
		t.Fatalf("expected to find bulk action command, got err=%v", err)
	}

	dataFunc := GetDataFunc(bulkCmd)
	if dataFunc == nil {
		t.Fatal("expected bulk action to register data func")
	}

	if _, err := dataFunc(map[string]string{
		"filter": "status == 'healthy'",
		"owner":  "platform",
		"status": "healthy",
	}, []string{"123"}); err != nil {
		t.Fatalf("bulk action data func returned error: %v", err)
	}

	if received.Owner != "team/platform" {
		t.Fatalf("expected owner to be resolved for bulk action, got %q", received.Owner)
	}

	if received.Status != "status:healthy" {
		t.Fatalf("expected status to be resolved for bulk action, got %q", received.Status)
	}

	if GetLookupFunc(bulkCmd) == nil {
		t.Fatal("expected bulk action to register lookup func")
	}
}

func TestEntityListCommandsRegisterFilterCompletions(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, entityFilterTestEntity]{
		Name:    "completion-entity",
		Filters: []Filter[entityFilterTestOpts]{ownerEntityFilter{}, statusEntityFilter{}},
		List: func(opts entityFilterTestOpts) ([]entityFilterTestEntity, error) {
			return nil, nil
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	listCmd, _, err := root.Find([]string{"completion-entity", "list"})
	if err != nil || listCmd == nil {
		t.Fatalf("expected to find list command, got err=%v", err)
	}

	ownerCompletion, ok := listCmd.GetFlagCompletionFunc("owner")
	if !ok {
		t.Fatal("expected owner flag completion to be registered")
	}

	completions, directive := ownerCompletion(listCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected no-file completion directive, got %v", directive)
	}

	expectedOwner := []string{
		cobra.CompletionWithDesc("team/core", "Core"),
		cobra.CompletionWithDesc("team/platform", "Platform"),
	}
	if len(completions) != len(expectedOwner) {
		t.Fatalf("expected owner completions %#v, got %#v", expectedOwner, completions)
	}
	for i, completion := range expectedOwner {
		if completions[i] != completion {
			t.Fatalf("expected owner completion %q at index %d, got %q", completion, i, completions[i])
		}
	}

	if err := listCmd.Flags().Set("owner", "platform"); err != nil {
		t.Fatalf("failed to set owner flag: %v", err)
	}

	statusCompletion, ok := listCmd.GetFlagCompletionFunc("status")
	if !ok {
		t.Fatal("expected status flag completion to be registered")
	}

	completions, directive = statusCompletion(listCmd, nil, "status:")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected no-file completion directive, got %v", directive)
	}

	expectedStatus := []string{
		cobra.CompletionWithDesc("status:healthy", "Healthy"),
	}
	if len(completions) != len(expectedStatus) {
		t.Fatalf("expected status completions %#v, got %#v", expectedStatus, completions)
	}
	for i, completion := range expectedStatus {
		if completions[i] != completion {
			t.Fatalf("expected status completion %q at index %d, got %q", completion, i, completions[i])
		}
	}
}

func TestEntityBulkCommandsRegisterFilterCompletions(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, entityFilterTestEntity]{
		Name:    "completion-bulk",
		Filters: []Filter[entityFilterTestOpts]{ownerEntityFilter{}, statusEntityFilter{}},
		BulkActions: []EntityBulkAction{
			BulkActionWithFilter(
				"bulk-suspend",
				func(ids []string, flags map[string]string) (any, error) {
					return ids, nil
				},
				func(opts entityFilterTestOpts, flags map[string]string) (any, error) {
					return nil, nil
				},
			).WithShort("Suspend entities matched by filters"),
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	bulkCmd, _, err := root.Find([]string{"completion-bulk", "bulk-suspend"})
	if err != nil || bulkCmd == nil {
		t.Fatalf("expected to find bulk action command, got err=%v", err)
	}

	if err := bulkCmd.Flags().Set("owner", "platform"); err != nil {
		t.Fatalf("failed to set owner flag: %v", err)
	}

	statusCompletion, ok := bulkCmd.GetFlagCompletionFunc("status")
	if !ok {
		t.Fatal("expected bulk status flag completion to be registered")
	}

	completions, directive := statusCompletion(bulkCmd, nil, "status:")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected no-file completion directive, got %v", directive)
	}

	expected := []string{
		cobra.CompletionWithDesc("status:healthy", "Healthy"),
	}
	if len(completions) != len(expected) {
		t.Fatalf("expected status completions %#v, got %#v", expected, completions)
	}
	for i, completion := range expected {
		if completions[i] != completion {
			t.Fatalf("expected status completion %q at index %d, got %q", completion, i, completions[i])
		}
	}
}

func TestEntityValidArgsPropagateToGeneratedIDCommands(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	validArgs := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		all := []string{"alpha", "beta"}
		var matches []string
		for _, candidate := range all {
			if strings.HasPrefix(candidate, toComplete) {
				matches = append(matches, candidate)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, any]{
		Name:      "id-completion-entity",
		ValidArgs: validArgs,
		Get: func(id string) (any, error) {
			return id, nil
		},
		Delete: func(id string) error {
			return nil
		},
		Actions: []EntityAction{
			Action("restart", func(id string, flags map[string]string) (any, error) {
				return id, nil
			}).WithShort("Restart one entity"),
		},
		Admin: &Entity[entityFilterTestEntity, entityFilterTestOpts, any]{
			Get: func(id string) (any, error) {
				return id, nil
			},
			Actions: []EntityAction{
				Action("reconcile", func(id string, flags map[string]string) (any, error) {
					return id, nil
				}).WithShort("Reconcile one entity"),
			},
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	paths := [][]string{
		{"id-completion-entity", "get"},
		{"id-completion-entity", "delete"},
		{"id-completion-entity", "restart"},
		{"admin", "id-completion-entity", "get"},
		{"admin", "id-completion-entity", "reconcile"},
	}

	for _, path := range paths {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil {
			t.Fatalf("expected to find command %v, got err=%v", path, err)
		}
		if cmd.ValidArgsFunction == nil {
			t.Fatalf("expected valid args function on %v", path)
		}

		completions, directive := cmd.ValidArgsFunction(cmd, nil, "a")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("expected no-file completion directive for %v, got %v", path, directive)
		}
		if len(completions) != 1 || completions[0] != "alpha" {
			t.Fatalf("expected alpha completion for %v, got %#v", path, completions)
		}
	}
}

func TestActionWithOptionalIDAcceptsNoPositionalArg(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, any]{
		Name: "optional-id-entity",
		Get:  func(id string) (any, error) { return id, nil },
		Actions: []EntityAction{
			Action("scan", func(id string, _ map[string]string) (any, error) {
				return id, nil
			}).WithShort("Scan with no id").WithOptionalID(),
			Action("restart", func(id string, _ map[string]string) (any, error) {
				return id, nil
			}).WithShort("Restart one entity"),
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	scan, _, err := root.Find([]string{"optional-id-entity", "scan"})
	if err != nil || scan == nil {
		t.Fatalf("expected to find scan command, got err=%v", err)
	}
	// The optional-id action accepts zero args but still rejects more than one.
	if err := scan.Args(scan, nil); err != nil {
		t.Fatalf("optional-id action must accept zero args, got: %v", err)
	}
	if err := scan.Args(scan, []string{"x", "y"}); err == nil {
		t.Fatalf("optional-id action must still reject more than one arg")
	}
	if scan.Use != "scan [id]" {
		t.Fatalf("optional-id action use should show [id], got %q", scan.Use)
	}

	// A normal action still forces exactly one positional arg.
	restart, _, err := root.Find([]string{"optional-id-entity", "restart"})
	if err != nil || restart == nil {
		t.Fatalf("expected to find restart command, got err=%v", err)
	}
	if err := restart.Args(restart, nil); err == nil {
		t.Fatalf("a normal action must still require its id arg")
	}
	if restart.Use != "restart <id>" {
		t.Fatalf("normal action use should show <id>, got %q", restart.Use)
	}
}

func TestActionInfoOptionalIDSkipsIDRequiredCheck(t *testing.T) {
	var gotID = "sentinel"
	spec := Action("scan", func(id string, _ map[string]string) (any, error) {
		gotID = id
		return id, nil
	}).WithOptionalID()

	// The DataFunc must not error on a missing id when OptionalID is set;
	// the run func receives an empty id.
	if _, err := spec.actionInfo().DataFunc(map[string]string{}, nil); err != nil {
		t.Fatalf("optional-id DataFunc must not require an id: %v", err)
	}
	if gotID != "" {
		t.Fatalf("expected empty id passed to run func, got %q", gotID)
	}

	// Without WithOptionalID the missing-id check still fires.
	plain := Action("restart", func(id string, _ map[string]string) (any, error) {
		return id, nil
	})
	if _, err := plain.actionInfo().DataFunc(map[string]string{}, nil); err == nil {
		t.Fatalf("a normal action must still reject a missing id")
	}
}

type bulkSetStatusFlags struct {
	To     string `flag:"to" help:"Status to apply" enum:"open,done" required:"true"`
	Reason string `flag:"reason" help:"Why"`
	// Collides with the entity's own `status` filter on purpose: a selector and
	// an operation can want the same word, and pflag panics on a redefinition.
	Status string `flag:"status"`
}

func (bulkSetStatusFlags) ClickyActionFlags() {}

// A bulk action is a selection and an operation. Its parameters say what to do;
// the entity's ListOpts say which rows to do it to. Before WithFlags the second
// half had no way to be declared, so id mode received whatever the caller
// happened to send, unbound and unvalidated.
func TestEntityBulkActionFlagsReachHandlerInBothModes(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	var idModeFlags, filterModeFlags map[string]string
	var idModeIDs []string
	var filterModeOpts entityFilterTestOpts

	RegisterEntity(Entity[entityFilterTestEntity, entityFilterTestOpts, entityFilterTestEntity]{
		Name:    "flagged-bulk",
		Filters: []Filter[entityFilterTestOpts]{ownerEntityFilter{}, statusEntityFilter{}},
		BulkActions: []EntityBulkAction{
			BulkActionWithFilter(
				"set-status",
				func(ids []string, flags map[string]string) (any, error) {
					idModeIDs, idModeFlags = ids, flags
					return ids, nil
				},
				func(opts entityFilterTestOpts, flags map[string]string) (any, error) {
					filterModeOpts, filterModeFlags = opts, flags
					return opts, nil
				},
			).WithFlags(bulkSetStatusFlags{}).WithShort("Set status on many entities"),
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	cmd, _, err := root.Find([]string{"flagged-bulk", "set-status"})
	if err != nil || cmd == nil {
		t.Fatalf("expected to find bulk action command, got err=%v", err)
	}

	// The action's own parameters are real cobra flags, so `--to` is usable and
	// documented rather than an undeclared key the handler hopes for.
	if cmd.Flags().Lookup("to") == nil {
		t.Fatal("expected the action's own --to flag to be bound")
	}
	if cmd.Flags().Lookup("reason") == nil {
		t.Fatal("expected the action's own --reason flag to be bound")
	}
	// `--status` is declared by both the entity's filters and the action's
	// parameters. Reaching this line at all is the assertion: pflag panics on a
	// redefined flag, so an unguarded second bind would have taken the process
	// down inside GenerateCLI above.
	if cmd.Flags().Lookup("status") == nil {
		t.Fatal("expected the entity filter to keep ownership of --status")
	}

	dataFunc := GetDataFunc(cmd)
	if dataFunc == nil {
		t.Fatal("expected bulk action to register data func")
	}

	if _, err := dataFunc(map[string]string{"to": "done", "reason": "ship it"}, []string{"a", "b"}); err != nil {
		t.Fatalf("id-mode bulk action returned error: %v", err)
	}
	if len(idModeIDs) != 2 || idModeIDs[0] != "a" || idModeIDs[1] != "b" {
		t.Fatalf("expected both ids in id mode, got %v", idModeIDs)
	}
	if idModeFlags["to"] != "done" || idModeFlags["reason"] != "ship it" {
		t.Fatalf("expected action parameters in id mode, got %v", idModeFlags)
	}

	// Filter mode resolves the selection through the entity's filters, and the
	// action's parameters must survive that resolution untouched.
	if _, err := dataFunc(map[string]string{
		"filter": "status == 'healthy'",
		"owner":  "platform",
		"to":     "open",
	}, []string{"ignored"}); err != nil {
		t.Fatalf("filter-mode bulk action returned error: %v", err)
	}
	if filterModeOpts.Owner != "team/platform" {
		t.Fatalf("expected filter mode to resolve owner, got %q", filterModeOpts.Owner)
	}
	if filterModeFlags["to"] != "open" {
		t.Fatalf("expected action parameters in filter mode, got %v", filterModeFlags)
	}
}

// The published catalog is what a front end renders an action from, so an
// action that declares parameters and hints has to arrive carrying them.
func TestEntityBulkActionInfoCarriesFlagsAndHints(t *testing.T) {
	spec := BulkActionWithFilter(
		"archive",
		func(ids []string, _ map[string]string) (any, error) { return ids, nil },
		func(opts entityFilterTestOpts, _ map[string]string) (any, error) { return opts, nil },
	).WithFlags(bulkSetStatusFlags{}).WithToolHints(MCPToolHints{Icon: "trash", Group: "Danger"})

	info := spec.bulkActionInfo(func(map[string]string) (any, error) { return entityFilterTestOpts{}, nil })

	if info.FlagsType == nil || info.FlagsType.Name() != "bulkSetStatusFlags" {
		t.Fatalf("expected FlagsType to be reflected, got %v", info.FlagsType)
	}
	if info.ToolHints.Icon != "trash" || info.ToolHints.Group != "Danger" {
		t.Fatalf("expected tool hints to survive, got %+v", info.ToolHints)
	}
	if info.ToolGroup != "Danger" {
		t.Fatalf("expected WithToolHints to set the tool group, got %q", info.ToolGroup)
	}
	if info.FilterFunc == nil {
		t.Fatal("expected filter mode to remain available")
	}
}
