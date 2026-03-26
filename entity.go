package clicky

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/flanksource/clicky/flags"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	entityRegistry   []EntityInfo
	entityRegistryMu sync.Mutex
)

// EntityItem is the interface that all entity types must implement.
type EntityItem interface {
	GetID() string
	GetName() string
}

// EntityInfo is the type-erased representation stored in the registry.
type EntityInfo struct {
	Name        string
	Type        reflect.Type
	ListType    reflect.Type
	Operations  []EntityOperation
	Actions     []ActionInfo
	BulkActions []BulkActionInfo
}

// EntityOperation represents a single CRUD operation.
type EntityOperation struct {
	Verb     string // "list", "get", "create", "update", "delete"
	DataFunc func(flags map[string]string, args []string) (any, error)
}

// ActionInfo is the type-erased representation of a single-entity action.
type ActionInfo struct {
	Name     string
	Short    string
	DataFunc func(flags map[string]string, args []string) (any, error)
}

// BulkActionInfo is the type-erased representation of a bulk action.
type BulkActionInfo struct {
	Name       string
	Short      string
	DataFunc   func(flags map[string]string, args []string) (any, error)
	FilterFunc func(flags map[string]string, args []string) (any, error)
	ListType   reflect.Type
}

// Action represents a custom operation on a single entity by ID.
type Action[T EntityItem] struct {
	Name  string
	Short string
	Run   func(id string, flags map[string]string) (any, error)
}

// BulkAction represents a custom operation on multiple entities.
type BulkAction[T EntityItem, ListOpts any] struct {
	Name      string
	Short     string
	Run       func(ids []string, flags map[string]string) (any, error)
	RunFilter func(opts ListOpts, flags map[string]string) (any, error)
}

// Entity configures a CRUD resource. All function fields are optional.
// T is the type returned by List and must implement EntityItem.
// Get/Create/Update return any to allow different detail representations.
type Entity[T EntityItem, ListOpts any] struct {
	Name   string
	List   func(opts ListOpts) ([]T, error)
	Get    func(id string) (any, error)
	Create func(body map[string]any) (any, error)
	Update func(id string, body map[string]any) (any, error)
	Delete func(id string) error

	Actions     []Action[T]
	BulkActions []BulkAction[T, ListOpts]

	// ValidArgs provides shell completion for the ID argument.
	ValidArgs func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
}

// RegisterEntity registers a CRUD entity. Call during init().
// CLI commands and HTTP routes are generated later via GenerateCLI/GenerateRoutes.
func RegisterEntity[T EntityItem, ListOpts any](e Entity[T, ListOpts]) {
	name := e.Name
	if name == "" {
		name = lo.KebabCase(reflect.TypeOf((*T)(nil)).Elem().Name())
	}

	info := EntityInfo{
		Name:     name,
		Type:     reflect.TypeOf((*T)(nil)).Elem(),
		ListType: reflect.TypeOf((*ListOpts)(nil)).Elem(),
	}

	if e.List != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb: "list",
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				opts := buildOpts[ListOpts](flagMap)
				return e.List(opts)
			},
		})
	}

	if e.Get != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb: "get",
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id := flagMap["id"]
				if id == "" && len(args) > 0 {
					id = args[0]
				}
				if id == "" {
					return nil, fmt.Errorf("id is required")
				}
				return e.Get(id)
			},
		})
	}

	if e.Create != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb: "create",
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				body := make(map[string]any)
				for k, v := range flagMap {
					body[k] = v
				}
				return e.Create(body)
			},
		})
	}

	if e.Update != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb: "update",
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id := flagMap["id"]
				if id == "" && len(args) > 0 {
					id = args[0]
				}
				if id == "" {
					return nil, fmt.Errorf("id is required")
				}
				body := make(map[string]any)
				for k, v := range flagMap {
					if k != "id" {
						body[k] = v
					}
				}
				return e.Update(id, body)
			},
		})
	}

	if e.Delete != nil {
		info.Operations = append(info.Operations, EntityOperation{
			Verb: "delete",
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id := flagMap["id"]
				if id == "" && len(args) > 0 {
					id = args[0]
				}
				if id == "" {
					return nil, fmt.Errorf("id is required")
				}
				return nil, e.Delete(id)
			},
		})
	}

	for _, action := range e.Actions {
		a := action
		info.Actions = append(info.Actions, ActionInfo{
			Name:  a.Name,
			Short: a.Short,
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				id := flagMap["id"]
				if id == "" && len(args) > 0 {
					id = args[0]
				}
				if id == "" {
					return nil, fmt.Errorf("id is required")
				}
				return a.Run(id, flagMap)
			},
		})
	}

	listType := reflect.TypeOf((*ListOpts)(nil)).Elem()
	for _, ba := range e.BulkActions {
		b := ba
		bai := BulkActionInfo{
			Name:     b.Name,
			Short:    b.Short,
			ListType: listType,
			DataFunc: func(flagMap map[string]string, args []string) (any, error) {
				return b.Run(args, flagMap)
			},
		}
		if b.RunFilter != nil {
			bai.FilterFunc = func(flagMap map[string]string, args []string) (any, error) {
				opts := buildOpts[ListOpts](flagMap)
				return b.RunFilter(opts, flagMap)
			}
		}
		info.BulkActions = append(info.BulkActions, bai)
	}

	entityRegistryMu.Lock()
	entityRegistry = append(entityRegistry, info)
	entityRegistryMu.Unlock()
}

// GetEntities returns all registered entities.
func GetEntities() []EntityInfo {
	entityRegistryMu.Lock()
	defer entityRegistryMu.Unlock()
	return append([]EntityInfo{}, entityRegistry...)
}

// GenerateCLI creates cobra subcommands for all registered entities under parent.
func GenerateCLI(parent *cobra.Command) {
	for _, entity := range GetEntities() {
		generateEntityCLI(parent, entity)
	}
}

func generateEntityCLI(parent *cobra.Command, entity EntityInfo) {
	entityCmd := &cobra.Command{
		Use:   entity.Name,
		Short: fmt.Sprintf("Manage %s resources", entity.Name),
	}
	parent.AddCommand(entityCmd)

	for _, op := range entity.Operations {
		generateEntitySubcommand(entityCmd, entity, op)
	}

	for _, action := range entity.Actions {
		generateIDCommand(entityCmd, action.Name, action.Short, EntityOperation{
			Verb:     action.Name,
			DataFunc: action.DataFunc,
		})
	}

	for _, ba := range entity.BulkActions {
		generateBulkActionCommand(entityCmd, ba)
	}
}

func generateEntitySubcommand(parent *cobra.Command, entity EntityInfo, op EntityOperation) {
	switch op.Verb {
	case "list":
		generateListCommand(parent, entity, op)
	case "get":
		generateIDCommand(parent, "get", fmt.Sprintf("Get a %s by ID", entity.Name), op)
	case "create":
		generateBodyCommand(parent, "create", fmt.Sprintf("Create a %s", entity.Name), op)
	case "update":
		generateBodyCommand(parent, "update", fmt.Sprintf("Update a %s", entity.Name), op)
	case "delete":
		generateIDCommand(parent, "delete", fmt.Sprintf("Delete a %s", entity.Name), op)
	}
}

func generateListCommand(parent *cobra.Command, entity EntityInfo, op EntityOperation) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s resources", entity.Name),
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = f.Value.String()
			})
			result, err := op.DataFunc(flagMap, args)
			if err != nil {
				return err
			}
			MustPrint(result, Flags.FormatOptions)
			return nil
		},
	}

	// Bind filter flags from the ListOpts type
	bindTypeFlags(cmd, entity.ListType)

	parent.AddCommand(cmd)
	dataFuncRegistry.Store(cmd, op.DataFunc)
}

func generateIDCommand(parent *cobra.Command, verb, short string, op EntityOperation) {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <id>", verb),
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			result, err := op.DataFunc(nil, args)
			if err != nil {
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
			}
			return nil
		},
	}
	if verb == "get" {
		cmd.Aliases = []string{"inspect"}
	}
	parent.AddCommand(cmd)
	dataFuncRegistry.Store(cmd, op.DataFunc)
}

func generateBodyCommand(parent *cobra.Command, verb, short string, op EntityOperation) {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [key=value ...]", verb),
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = f.Value.String()
			})

			// For update, first arg is ID
			callArgs := args
			if verb == "update" && len(args) > 0 {
				flagMap["id"] = args[0]
				callArgs = args[1:]
			}

			if len(callArgs) > 0 {
				parsed, err := ParseArgumentsAsMap(callArgs)
				if err != nil {
					return err
				}
				for k, v := range parsed {
					flagMap[k] = fmt.Sprintf("%v", v)
				}
			} else if isStdinAvailable() {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var body map[string]any
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON from stdin: %w", err)
				}
				for k, v := range body {
					flagMap[k] = fmt.Sprintf("%v", v)
				}
			}

			result, err := op.DataFunc(flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
			}
			return nil
		},
	}
	parent.AddCommand(cmd)
	dataFuncRegistry.Store(cmd, op.DataFunc)
}

func generateBulkActionCommand(parent *cobra.Command, ba BulkActionInfo) {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <id> [id...]", ba.Name),
		Short: ba.Short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			flagMap := make(map[string]string)
			c.Flags().Visit(func(f *pflag.Flag) {
				flagMap[f.Name] = f.Value.String()
			})

			// Use filter mode if --filter flag is set and FilterFunc exists
			if ba.FilterFunc != nil && flagMap["filter"] != "" {
				result, err := ba.FilterFunc(flagMap, args)
				if err != nil {
					return err
				}
				if result != nil {
					MustPrint(result, Flags.FormatOptions)
				}
				return nil
			}

			result, err := ba.DataFunc(flagMap, args)
			if err != nil {
				return err
			}
			if result != nil {
				MustPrint(result, Flags.FormatOptions)
			}
			return nil
		},
	}

	if ba.FilterFunc != nil {
		bindTypeFlags(cmd, ba.ListType)
	}

	parent.AddCommand(cmd)
	dataFuncRegistry.Store(cmd, ba.DataFunc)
}

// bindTypeFlags registers cobra flags from a struct type's field tags.
func bindTypeFlags(cmd *cobra.Command, t reflect.Type) {
	fieldInfos, _ := flags.ParseStructFields(t)
	for _, info := range fieldInfos {
		flags.BindFlag(cmd, info)
	}
}

// buildOpts creates an instance of T and populates it from a flag map.
func buildOpts[T any](flagMap map[string]string) T {
	t := reflect.TypeOf((*T)(nil)).Elem()
	v := reflect.New(t).Elem()

	fieldInfos, _ := flags.ParseStructFields(t)
	for _, info := range fieldInfos {
		name := info.FlagName
		if name == "" {
			name = lo.KebabCase(info.FieldName)
		}
		if val, ok := flagMap[name]; ok && val != "" {
			field := v.FieldByIndex(info.FieldPath)
			switch field.Kind() {
			case reflect.String:
				field.SetString(val)
			case reflect.Int, reflect.Int64:
				var n int64
				fmt.Sscanf(val, "%d", &n)
				field.SetInt(n)
			case reflect.Bool:
				field.SetBool(val == "true")
			}
		}
	}

	return v.Interface().(T)
}
