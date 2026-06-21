package good

import (
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
)

// gadget implements EntityItem AND api.TableProvider.
type gadget struct {
	ID   string
	Name string
}

func (g gadget) GetID() string   { return g.ID }
func (g gadget) GetName() string { return g.Name }

func (g gadget) Columns() []api.ColumnDef {
	return []api.ColumnDef{{Name: "id"}, {Name: "name"}}
}

func (g gadget) Row() map[string]any {
	return map[string]any{"id": g.ID, "name": g.Name}
}

type gadgetOpts struct{}

// OK: entity item type implements TableProvider — no diagnostic.
func registerGadget() {
	clicky.NewEntity[gadget, gadgetOpts, []gadget]("gadget").
		List(func(gadgetOpts) ([]gadget, error) { return nil, nil }).
		Register()
}

// OK: a bare grouping command (no Run/RunE) is structural — no diagnostic.
var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "Application root",
}

// OK: entities drive the CLI via GenerateCLI — no manual leaf command.
func buildCLI() {
	clicky.GenerateCLI(rootCmd)
}
