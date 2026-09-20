package lint

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("clickylint", func(any) (register.LinterPlugin, error) {
		return &plugin{}, nil
	})
}

type plugin struct{}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

// GetLoadMode requests type information, not just syntax. Most of clickylint
// resolves what a node actually refers to — that a composite literal is
// cobra.Command, that a HandleFunc call is net/http's, that an entity's item
// type implements api.TableProvider — all of which read pass.TypesInfo. Under
// LoadModeSyntax that map is not populated and those rules silently never fire.
func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
