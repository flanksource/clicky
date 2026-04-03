package clicky

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"sync"

	"github.com/flanksource/clicky/api"
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
	IsAdmin     bool
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

	// Admin groups admin-only operations (inspect, configure, etc.)
	// that produce different views/columns from the main CRUD operations.
	// Generates subcommands under an "admin" subgroup and routes like /entity/admin/{verb}.
	Admin *Entity[T, ListOpts]

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
				items, err := e.List(opts)
				if err != nil {
					return nil, err
				}
				return withEntityIDs(items), nil
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

	if e.Admin != nil {
		admin := *e.Admin
		if admin.Name == "" || admin.Name == name {
			admin.Name = name
		}
		// Store as admin sub-entity — GenerateCLI nests under "admin" parent
		adminInfo := EntityInfo{
			Name:     admin.Name,
			Type:     info.Type,
			ListType: info.ListType,
			IsAdmin:  true,
		}
		if admin.List != nil {
			adminInfo.Operations = append(adminInfo.Operations, EntityOperation{
				Verb: "list",
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					opts := buildOpts[ListOpts](flagMap)
					return admin.List(opts)
				},
			})
		}
		if admin.Get != nil {
			adminInfo.Operations = append(adminInfo.Operations, EntityOperation{
				Verb: "get",
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					id := flagMap["id"]
					if id == "" && len(args) > 0 {
						id = args[0]
					}
					if id == "" {
						return nil, fmt.Errorf("id is required")
					}
					return admin.Get(id)
				},
			})
		}
		for _, action := range admin.Actions {
			a := action
			adminInfo.Actions = append(adminInfo.Actions, ActionInfo{
				Name:  a.Name,
				Short: a.Short,
				DataFunc: func(flagMap map[string]string, args []string) (any, error) {
					id := flagMap["id"]
					if id == "" && len(args) > 0 {
						id = args[0]
					}
					return a.Run(id, flagMap)
				},
			})
		}
		entityRegistryMu.Lock()
		entityRegistry = append(entityRegistry, adminInfo)
		entityRegistryMu.Unlock()
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
// Admin entities are nested under a shared "admin" parent command.
func GenerateCLI(parent *cobra.Command) {
	var adminCmd *cobra.Command
	for _, entity := range GetEntities() {
		if entity.IsAdmin {
			if adminCmd == nil {
				adminCmd = &cobra.Command{
					Use:   "admin",
					Short: "Administrative operations",
				}
				parent.AddCommand(adminCmd)
			}
			generateEntityCLI(adminCmd, entity)
		} else {
			generateEntityCLI(parent, entity)
		}
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
				_, _ = fmt.Sscanf(val, "%d", &n)
				field.SetInt(n)
			case reflect.Bool:
				field.SetBool(val == "true")
			}
		}
	}

	return v.Interface().(T)
}

// withEntityIDs wraps each EntityItem in the slice to include a _id field in JSON output.
func withEntityIDs[T EntityItem](items []T) []entityWithID[T] {
	result := make([]entityWithID[T], len(items))
	for i, item := range items {
		result[i] = entityWithID[T]{ID: item.GetID(), Inner: item}
	}
	return result
}

type entityWithID[T EntityItem] struct {
	ID    string `json:"_id"`
	Inner T
}

func (e entityWithID[T]) GetID() string   { return e.ID }
func (e entityWithID[T]) GetName() string { return e.Inner.GetName() }

func (e entityWithID[T]) Columns() []api.ColumnDef {
	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return tp.Columns()
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		columns := columnsFromPrettyRow(prettyRow)
		if len(columns) > 0 {
			return columns
		}
	}

	columns, _, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return columns
}

func (e entityWithID[T]) Row() map[string]any {
	if tp, ok := any(e.Inner).(api.TableProvider); ok {
		return tp.Row()
	}

	if prettyRow, ok := entityPrettyRow(any(e.Inner), nil); ok {
		row := make(map[string]any, len(prettyRow))
		for key, value := range prettyRow {
			row[key] = value
		}
		return row
	}

	_, row, ok := columnsAndRowFromStruct(any(e.Inner))
	if !ok {
		return nil
	}
	return row
}

func (e entityWithID[T]) PrettyRow(opts interface{}) map[string]api.Text {
	if row, ok := entityPrettyRow(any(e.Inner), opts); ok {
		return row
	}

	rowData := e.Row()
	if len(rowData) == 0 {
		return nil
	}

	row := make(map[string]api.Text)
	order := 1
	for _, col := range e.Columns() {
		if col.Hidden {
			continue
		}
		cell := api.Text{Style: fmt.Sprintf("order-%d %s", order, col.Style)}
		if value, ok := rowData[col.Name]; ok {
			cell = addPrettyRowValue(cell, value)
		}
		row[col.DisplayLabel()] = cell
		order++
	}
	return row
}

func entityPrettyRow(inner any, opts interface{}) (map[string]api.Text, bool) {
	if pr, ok := inner.(api.PrettyRow); ok {
		if row := pr.PrettyRow(opts); len(row) > 0 {
			return row, true
		}
	}
	return nil, false
}

func columnsFromPrettyRow(prettyRow map[string]api.Text) []api.ColumnDef {
	type orderedColumn struct {
		name  string
		style string
		order int
	}

	columns := make([]orderedColumn, 0, len(prettyRow))
	for name, text := range prettyRow {
		columns = append(columns, orderedColumn{
			name:  name,
			style: text.Style,
			order: api.ExtractOrderValue(text.Style),
		})
	}

	sort.Slice(columns, func(i, j int) bool {
		if columns[i].order != columns[j].order {
			return columns[i].order < columns[j].order
		}
		return columns[i].name < columns[j].name
	})

	defs := make([]api.ColumnDef, 0, len(columns))
	for _, col := range columns {
		defs = append(defs, api.ColumnDef{
			Name:  col.name,
			Label: col.name,
			Style: col.style,
		})
	}
	return defs
}

func columnsAndRowFromStruct(inner any) ([]api.ColumnDef, map[string]any, bool) {
	parser := api.NewStructParser()
	val := reflect.ValueOf(inner)
	if !val.IsValid() {
		return nil, nil, false
	}

	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil, false
		}
		val = val.Elem()
	}

	fields, err := parser.GetTableFields(val)
	if err != nil {
		return nil, nil, false
	}

	typedRow, err := parser.StructToRow(val)
	if err != nil {
		return nil, nil, false
	}

	columns := make([]api.ColumnDef, 0, len(fields))
	row := make(map[string]any, len(fields))
	for _, field := range fields {
		columns = append(columns, api.ColumnDef{
			Name:          field.Name,
			Label:         field.Label,
			Style:         field.Style,
			HeaderStyle:   field.LabelStyle,
			Type:          field.Type,
			Format:        field.Format,
			FormatOptions: field.FormatOptions,
		})
		if value, ok := typedRow[field.Name]; ok {
			row[field.Name] = value.Value()
		}
	}

	return columns, row, true
}

func addPrettyRowValue(cell api.Text, value any) api.Text {
	switch v := value.(type) {
	case api.Textable:
		return cell.Add(v)
	case api.Pretty:
		return cell.Add(v.Pretty())
	default:
		return cell.Add(api.Human(v))
	}
}

func (e entityWithID[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(e.Inner)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 || data[0] != '{' {
		return data, nil
	}
	idJSON, err := json.Marshal(e.ID)
	if err != nil {
		return nil, err
	}
	prefix := []byte(`{"_id":`)
	prefix = append(prefix, idJSON...)
	prefix = append(prefix, ',')
	return append(prefix, data[1:]...), nil
}
