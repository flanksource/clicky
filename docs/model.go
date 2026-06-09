package docs

import (
	"sort"

	"github.com/flanksource/clicky/rpc"
	"github.com/spf13/cobra"
)

// Model is the structured docs model assembled from a CLI's command tree. It is
// the single source for every output: CLI reference, UI catalog, and the
// json/yaml structured dumps.
type Model struct {
	Title       string          `json:"title" yaml:"title"`
	Description string          `json:"description" yaml:"description"`
	Version     string          `json:"version" yaml:"version"`
	Controllers []ControllerDoc `json:"controllers" yaml:"controllers"`
	Commands    []CommandDoc    `json:"commands" yaml:"commands"`
	Surfaces    []SurfaceDoc    `json:"surfaces" yaml:"surfaces"`
}

// ControllerDoc groups the commands of one high-level command controller — a
// direct child of the root — into a single documentation unit (one page). A
// top-level leaf command (no subcommands) is its own controller.
type ControllerDoc struct {
	Name     string       `json:"name" yaml:"name"`         // controller name, e.g. "stack"
	Short    string       `json:"short" yaml:"short"`       // controller one-line summary
	Long     string       `json:"long" yaml:"long"`         // controller full description
	Commands []CommandDoc `json:"commands" yaml:"commands"` // commands within the depth limit
}

// CommandDoc documents a single runnable command for the CLI reference.
type CommandDoc struct {
	Path    string    `json:"path" yaml:"path"`       // e.g. "stack create"
	Use     string    `json:"use" yaml:"use"`         // raw cobra Use string
	Short   string    `json:"short" yaml:"short"`     // one-line summary
	Long    string    `json:"long" yaml:"long"`       // full description
	Example string    `json:"example" yaml:"example"` // example block
	Aliases []string  `json:"aliases" yaml:"aliases"` // command aliases
	Flags   []FlagDoc `json:"flags" yaml:"flags"`
}

// FlagDoc documents a single flag.
type FlagDoc struct {
	Name      string      `json:"name" yaml:"name"`
	Shorthand string      `json:"shorthand" yaml:"shorthand"`
	Type      string      `json:"type" yaml:"type"`
	Default   interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Required  bool        `json:"required" yaml:"required"`
	Usage     string      `json:"usage" yaml:"usage"`
}

// SurfaceDoc documents a clicky-ui operation surface: the entity/verb mapping,
// its UI-role parameters, lookup support, and the HTTP endpoint + CLI command
// the UI uses to invoke it.
type SurfaceDoc struct {
	Command    string         `json:"command" yaml:"command"` // CLI command path
	Entity     string         `json:"entity" yaml:"entity"`   // entity name
	Verb       string         `json:"verb" yaml:"verb"`       // list/get/create/update/delete/action
	Method     string         `json:"method" yaml:"method"`   // HTTP method
	Path       string         `json:"path" yaml:"path"`       // /api/v1/...
	Lookup     bool           `json:"lookup" yaml:"lookup"`   // SupportsLookup
	Parameters []ParameterDoc `json:"parameters" yaml:"parameters"`
}

// ParameterDoc documents a single UI parameter and its widget role.
type ParameterDoc struct {
	Name     string      `json:"name" yaml:"name"`
	Type     string      `json:"type" yaml:"type"`
	Role     string      `json:"role,omitempty" yaml:"role,omitempty"` // filter/limit/offset/time-from/time-to
	Required bool        `json:"required" yaml:"required"`
	Default  interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	In       string      `json:"in" yaml:"in"` // query/path/body
}

// BuildModel converts the root cobra command into a Model using rpc.Converter
// for operation metadata (paths, verbs, parameter roles) and walking the
// command tree directly for the CLI reference.
func BuildModel(root *cobra.Command, cfg *DocsConfig) (*Model, error) {
	m := &Model{
		Title:       title(root, cfg),
		Description: description(root, cfg),
		Version:     root.Version,
	}

	converter := rpc.NewConverter(rpc.DefaultConfig())
	service, err := converter.ConvertCommandTree(root)
	if err != nil {
		return nil, err
	}

	m.Controllers = buildControllers(root, cfg)
	m.Commands = flattenControllers(m.Controllers)
	m.Surfaces = buildSurfaceDocs(service, cfg)
	return m, nil
}

// buildControllers groups the command tree into one ControllerDoc per direct
// child of the root. Each controller collects its own runnable commands down to
// the configured depth (relative to the controller). Hidden, grouping, and
// excluded commands are omitted; a controller with no documented commands is
// dropped.
func buildControllers(root *cobra.Command, cfg *DocsConfig) []ControllerDoc {
	maxDepth := cfg.depth()
	var controllers []ControllerDoc

	for _, top := range root.Commands() {
		if top.Hidden {
			continue
		}
		ctrl := ControllerDoc{Name: top.Name(), Short: top.Short, Long: top.Long}
		walkCommands(top, func(cmd *cobra.Command) {
			if !isRunnable(cmd) || cmd.Hidden {
				return
			}
			if maxDepth != unlimitedDepth && depthBelow(top, cmd) > maxDepth {
				return
			}
			path := commandPath(cmd)
			if cfg.excluded(path) {
				return
			}
			ctrl.Commands = append(ctrl.Commands, commandDoc(cmd, path))
		})
		if len(ctrl.Commands) == 0 {
			continue
		}
		sort.SliceStable(ctrl.Commands, func(i, j int) bool {
			return ctrl.Commands[i].Path < ctrl.Commands[j].Path
		})
		controllers = append(controllers, ctrl)
	}

	sort.SliceStable(controllers, func(i, j int) bool {
		return controllers[i].Name < controllers[j].Name
	})
	return controllers
}

func commandDoc(cmd *cobra.Command, path string) CommandDoc {
	return CommandDoc{
		Path:    path,
		Use:     cmd.Use,
		Short:   cmd.Short,
		Long:    cmd.Long,
		Example: cmd.Example,
		Aliases: cmd.Aliases,
		Flags:   flagDocs(cmd),
	}
}

// flattenControllers returns every controller's commands as one sorted list,
// preserving the flat CLI-reference view alongside the grouped one.
func flattenControllers(controllers []ControllerDoc) []CommandDoc {
	var docs []CommandDoc
	for _, c := range controllers {
		docs = append(docs, c.Commands...)
	}
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs
}

func buildSurfaceDocs(service *rpc.RPCService, cfg *DocsConfig) []SurfaceDoc {
	var docs []SurfaceDoc
	for _, op := range service.Operations {
		if op.Clicky == nil || op.Clicky.Entity == "" {
			continue // not a clicky-ui surface
		}
		if cfg.excluded(op.Name) {
			continue
		}
		sd := SurfaceDoc{
			Command: op.Name,
			Entity:  op.Clicky.Entity,
			Verb:    op.Clicky.Verb,
			Method:  op.Method,
			Path:    op.Path,
			Lookup:  op.Clicky.SupportsLookup,
		}
		for _, p := range op.Parameters {
			sd.Parameters = append(sd.Parameters, ParameterDoc{
				Name:     p.Name,
				Type:     p.Type,
				Role:     rpc.ParamRole(op, p),
				Required: p.Required,
				Default:  p.Default,
				In:       p.In,
			})
		}
		docs = append(docs, sd)
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if docs[i].Entity != docs[j].Entity {
			return docs[i].Entity < docs[j].Entity
		}
		return docs[i].Command < docs[j].Command
	})
	return docs
}
