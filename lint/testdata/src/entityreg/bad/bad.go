package bad

import (
	"net/http"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
)

// widget implements EntityItem but NOT api.TableProvider.
type widget struct {
	ID   string
	Name string
}

func (w widget) GetID() string   { return w.ID }
func (w widget) GetName() string { return w.Name }

type widgetOpts struct{}

// Error: entity item type lacks a TableProvider implementation.
func registerWidget() {
	clicky.NewEntity[widget, widgetOpts, []widget]("widget"). // want `entity does not implement api\.TableProvider`
									List(func(widgetOpts) ([]widget, error) { return nil, nil }).
									Register()
}

// Error: hand-rolled cobra command with a RunE handler.
var manualCmd = &cobra.Command{ // want `avoid manual cobra\.Command with Run/RunE`
	Use:   "widget",
	Short: "Manage widgets",
	RunE: func(_ *cobra.Command, _ []string) error {
		return nil
	},
}

// Error: package-level http handler registration.
func serve() {
	http.HandleFunc("/widgets", handleWidgets) // want `avoid registering net/http handlers directly`

	mux := http.NewServeMux()
	mux.HandleFunc("/widgets/list", handleWidgets)        // want `avoid registering net/http handlers directly`
	mux.Handle("/widgets/static", http.NotFoundHandler()) // want `avoid registering net/http handlers directly`
}

func handleWidgets(_ http.ResponseWriter, _ *http.Request) {}
