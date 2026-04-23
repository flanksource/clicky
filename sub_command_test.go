package clicky

import (
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

type nestedTestEntity struct {
	ID   string
	Name string
}

func (n nestedTestEntity) GetID() string   { return n.ID }
func (n nestedTestEntity) GetName() string { return n.Name }

type nestedTestOpts struct{}

// resetEntityRegistry clears the global entity registry. Tests that register
// entities must call this before and after to avoid leaking state.
func resetEntityRegistry(t *testing.T) {
	t.Helper()
	entityRegistryMu.Lock()
	entityRegistry = nil
	entityRegistryMu.Unlock()
	dataFuncRegistry = sync.Map{}
	lookupFuncRegistry = sync.Map{}
	pendingSubCommandsMu.Lock()
	pendingSubCommands = nil
	pendingSubCommandsMu.Unlock()
}

func TestEntityParentNesting(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[nestedTestEntity, nestedTestOpts]{
		Name:   "snapshot",
		Parent: "policy",
		List: func(_ nestedTestOpts) ([]nestedTestEntity, error) {
			return nil, nil
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	policy, _, err := root.Find([]string{"policy"})
	if err != nil || policy == nil || policy.Name() != "policy" {
		t.Fatalf("expected lazily-created policy parent command, got %v err=%v", policy, err)
	}

	snapshot, _, err := root.Find([]string{"policy", "snapshot"})
	if err != nil || snapshot == nil || snapshot.Name() != "snapshot" {
		t.Fatalf("expected snapshot nested under policy, got %v err=%v", snapshot, err)
	}

	list, _, err := root.Find([]string{"policy", "snapshot", "list"})
	if err != nil || list == nil || list.Name() != "list" {
		t.Fatalf("expected list under policy snapshot, got %v err=%v", list, err)
	}
}

func TestEntityAliases(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[nestedTestEntity, nestedTestOpts]{
		Name:    "correspondence",
		Aliases: []string{"osc"},
		List: func(_ nestedTestOpts) ([]nestedTestEntity, error) {
			return nil, nil
		},
	})

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	via, _, err := root.Find([]string{"osc"})
	if err != nil || via == nil || via.Name() != "correspondence" {
		t.Fatalf("expected osc alias to resolve to correspondence, got %v err=%v", via, err)
	}
}

func TestRegisterSubCommandUnderEntity(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	RegisterEntity(Entity[nestedTestEntity, nestedTestOpts]{
		Name: "correspondence",
		List: func(_ nestedTestOpts) ([]nestedTestEntity, error) {
			return nil, nil
		},
	})

	template := &cobra.Command{Use: "template", Short: "template ops"}
	RegisterSubCommand("correspondence", template)

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	found, _, err := root.Find([]string{"correspondence", "template"})
	if err != nil || found == nil || found.Name() != "template" {
		t.Fatalf("expected template under correspondence, got %v err=%v", found, err)
	}
}

func TestRegisterSubCommandCreatesParent(t *testing.T) {
	resetEntityRegistry(t)
	defer resetEntityRegistry(t)

	process := &cobra.Command{Use: "process"}
	RegisterSubCommand("correspondence", process)

	root := &cobra.Command{Use: "root"}
	GenerateCLI(root)

	found, _, err := root.Find([]string{"correspondence", "process"})
	if err != nil || found == nil {
		t.Fatalf("expected process under lazily-created correspondence, got err=%v", err)
	}
}
