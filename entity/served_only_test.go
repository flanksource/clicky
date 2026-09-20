// Tests the counterpart to local-only: operations that belong to the running
// server and would mislead if offered as CLI commands.
package entity

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func servedOnlyTree() (*cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "app"}
	group := &cobra.Command{Use: "monitors"}
	list := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(group)
	group.AddCommand(list)
	return group, list
}

func TestServedOnlyCoversTheSubtree(t *testing.T) {
	group, list := servedOnlyTree()
	MarkServedOnly(group)

	assert.True(t, IsServedOnly(group))
	assert.True(t, IsServedOnly(list), "marking a group must cover what is under it")
}

func TestServedOnlyIsHiddenFromHelp(t *testing.T) {
	group, _ := servedOnlyTree()
	MarkServedOnly(group)

	assert.True(t, group.Hidden, "an operation with no meaning here must not be offered here")
}

// Refusing is the point: reporting this process's empty state would read as
// "the server has none", which is a different and wrong answer.
func TestServedOnlyRefusesToRunLocally(t *testing.T) {
	group, list := servedOnlyTree()
	MarkServedOnly(group)
	RefuseServedOnlyCommands(group.Root())

	err := list.RunE(list, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app monitors list")
	assert.Contains(t, err.Error(), "serve")
}

func TestUnmarkedCommandsStillRun(t *testing.T) {
	group, list := servedOnlyTree()
	RefuseServedOnlyCommands(group.Root())

	assert.NoError(t, list.RunE(list, nil))
	assert.False(t, group.Hidden)
}

// The two marks are opposites and must stay independent: served-only keeps a
// command on the HTTP surface, local-only keeps it off.
func TestServedOnlyIsNotLocalOnly(t *testing.T) {
	group, _ := servedOnlyTree()
	MarkServedOnly(group)

	assert.False(t, IsLocalOnly(group), "a served-only operation is still a route")
}

// A registration declares it once and every generated operation inherits it.
func TestEntityServedOnlyMarksItsGeneratedCommands(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	generateEntityCLI(root, EntityInfo{
		Name:       "monitors",
		ServedOnly: true,
		Type:       reflect.TypeOf(struct{}{}),
		ListType:   reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{{
			Verb:     "list",
			DataFunc: func(map[string]string, []string) (any, error) { return nil, nil },
		}},
	})

	var group *cobra.Command
	for _, candidate := range root.Commands() {
		if candidate.Name() == "monitors" {
			group = candidate
		}
	}
	require.NotNil(t, group)
	require.NotEmpty(t, group.Commands(), "the entity generated no commands to mark")
	for _, sub := range group.Commands() {
		assert.Truef(t, IsServedOnly(sub), "%s must be marked", sub.Name())
	}
}

// An entity named after a command group that already exists must join it, not
// stand a second one beside it. Cobra allows the duplicate; the user then sees
// two `monitors` in help and only one of them works.
func TestEntityJoinsAnExistingCommandGroup(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	existing := &cobra.Command{Use: "monitors", Short: "Start and inspect monitors"}
	start := &cobra.Command{Use: "start", RunE: func(*cobra.Command, []string) error { return nil }}
	existing.AddCommand(start)
	root.AddCommand(existing)

	generateEntityCLI(root, EntityInfo{
		Name:     "monitors",
		Type:     reflect.TypeOf(struct{}{}),
		ListType: reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{{
			Verb:     "list",
			DataFunc: func(map[string]string, []string) (any, error) { return nil, nil },
		}},
	})

	groups := 0
	for _, candidate := range root.Commands() {
		if candidate.Name() == "monitors" {
			groups++
		}
	}
	assert.Equal(t, 1, groups, "the entity must join the existing group")
	assert.Same(t, existing, root.Commands()[0], "and it must be that group, keeping what was already there")
}

// Marking a shared group served-only would hide commands the entity does not
// own. Only what the entity generated belongs to the server.
func TestServedOnlyLeavesASharedGroupsOtherCommandsAlone(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	existing := &cobra.Command{Use: "monitors"}
	start := &cobra.Command{Use: "start", RunE: func(*cobra.Command, []string) error { return nil }}
	existing.AddCommand(start)
	root.AddCommand(existing)

	generateEntityCLI(root, EntityInfo{
		Name:       "monitors",
		ServedOnly: true,
		Type:       reflect.TypeOf(struct{}{}),
		ListType:   reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{{
			Verb:     "list",
			DataFunc: func(map[string]string, []string) (any, error) { return nil, nil },
		}},
	})

	assert.False(t, IsServedOnly(start), "a command the entity did not generate must keep working")
	assert.False(t, start.Hidden)

	var list *cobra.Command
	for _, candidate := range existing.Commands() {
		if candidate.Name() == "list" {
			list = candidate
		}
	}
	require.NotNil(t, list, "the entity's own list operation")
	assert.True(t, IsServedOnly(list), "what the entity generated is served-only")
}

// A group the entity created holds nothing but served-only operations, so
// leaving it visible advertises a command with no subcommands under it. A group
// the application already had keeps its own visibility.
func TestServedOnlyHidesAGroupItOwnsButNotASharedOne(t *testing.T) {
	owned := &cobra.Command{Use: "app"}
	generateEntityCLI(owned, EntityInfo{
		Name: "cycle-runs", ServedOnly: true,
		Type: reflect.TypeOf(struct{}{}), ListType: reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{{Verb: "list", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }}},
	})
	require.True(t, owned.Commands()[0].Hidden, "a group with nothing else in it must not be advertised")

	shared := &cobra.Command{Use: "app"}
	existing := &cobra.Command{Use: "monitors"}
	existing.AddCommand(&cobra.Command{Use: "start", RunE: func(*cobra.Command, []string) error { return nil }})
	shared.AddCommand(existing)
	generateEntityCLI(shared, EntityInfo{
		Name: "monitors", ServedOnly: true,
		Type: reflect.TypeOf(struct{}{}), ListType: reflect.TypeOf(struct{}{}),
		Operations: []EntityOperation{{Verb: "list", DataFunc: func(map[string]string, []string) (any, error) { return nil, nil }}},
	})
	assert.False(t, existing.Hidden, "a group the application owns keeps its own commands reachable")
}
