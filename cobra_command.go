package clicky

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/commons/logger"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// dataFuncRegistry maps cobra commands created by AddCommand to closures
// that can invoke the user function directly with flag values.
var dataFuncRegistry sync.Map // map[*cobra.Command]func(flags map[string]string, args []string) (any, error)

// lookupFuncRegistry maps cobra commands to filter metadata lookup closures.
var lookupFuncRegistry sync.Map // map[*cobra.Command]func(flags map[string]string, args []string) (any, error)

// GetDataFunc returns the direct data function registered for a command, if any.
// Used by the RPC converter to wire DataFunc on RPCOperation.
func GetDataFunc(cmd *cobra.Command) func(flags map[string]string, args []string) (any, error) {
	if v, ok := dataFuncRegistry.Load(cmd); ok {
		return v.(func(flags map[string]string, args []string) (any, error))
	}
	return nil
}

// GetLookupFunc returns the direct lookup function registered for a command, if any.
func GetLookupFunc(cmd *cobra.Command) func(flags map[string]string, args []string) (any, error) {
	if v, ok := lookupFuncRegistry.Load(cmd); ok {
		return v.(func(flags map[string]string, args []string) (any, error))
	}
	return nil
}

// AddCommand creates a Cobra command with automatic flag parsing from struct tags,
// execution, and result formatting.
//
// Type parameter T must be a struct with tags defining flags:
//   - flag:"name" - flag name (required)
//   - help:"description" - flag help text
//   - default:"value" - default value
//   - short:"x" - short flag variant
//   - required:"true" - mark flag as required
//   - stdin:"true" - mark field as default for stdin input (only one field)
//
// Supports @ prefix for file/URL loading:
//   - @file.txt - read from file
//   - @https://... - fetch from URL
//   - For slices: one value per line (skip empty lines and # comments)
//   - For slices: @file.csv:ColumnName reads a named column from a CSV
//     (first row is the header, empty cells are skipped)
//   - For slices: @file.xlsx:ColumnName / @file.xls:ColumnName reads a named
//     column from the first worksheet (case-insensitive header match)
//
// Supported types:
//   - string, int, bool
//   - duration.Duration (supports "30d", "2w", etc.)
//   - time.Time (supports datamath like "now-7d", "now/d", "now-1M/M")
//   - []string, []int (slices)
//
// Datamath expressions for time.Time fields (Elasticsearch compatible):
//   - now         : current time
//   - now-7d      : 7 days ago
//   - now+2h      : 2 hours from now
//   - now/d       : start of current day
//   - now-7d/d    : start of day 7 days ago
//   - now/w       : start of this week
//   - now-1M/M    : start of last month
//   - now/y       : start of this year
//   - Units: y (year), M (month), w (week), d (day), h (hour), m (minute), s (second)
//
// Example:
//
//	type ListOptions struct {
//	    Role   string            `flag:"role" help:"Filter by role" short:"r"`
//	    Limit  int               `flag:"limit" help:"Max results" default:"50"`
//	    Since  time.Time         `flag:"since" help:"Created since" default:"now-30d"`
//	    Tags   []string          `flag:"tags" help:"Filter tags" stdin:"true"`
//	    MaxAge duration.Duration `flag:"max-age" help:"Max age" default:"365d"`
//	}
//
//	cmd := &cobra.Command{Use: "list", Short: "List users"}
//	clicky.AddCommand(cmd, ListOptions{}, func(opts ListOptions) (any, error) {
//	    return fetchUsers(opts)
//	})
//
// Usage examples:
//
//	myapp list --tags @tags.txt              # load from file
//	myapp list --since now-30d               # datamath
//	echo -e "tag1\ntag2" | myapp list        # stdin
//	myapp list --max-age 60d --json          # with formatting
func AddCommand[T any, R any](parent *cobra.Command, opts T, fn func(opts T) (R, error)) *cobra.Command {
	optsType := reflect.TypeOf(opts)
	if optsType.Kind() != reflect.Struct {
		panic("AddCommand requires a struct type for opts parameter")
	}
	name := lo.KebabCase(strings.TrimSuffix(optsType.Name(), "Options"))
	return AddNamedCommand(name, parent, opts, fn)
}

func AddNamedCommand[T any, R any](name string, parent *cobra.Command, opts T, fn func(opts T) (R, error)) *cobra.Command {

	optsType := reflect.TypeOf(opts)
	if optsType.Kind() != reflect.Struct {
		panic("AddCommand requires a struct type for opts parameter")
	}

	name = strings.TrimPrefix(name, parent.Use+"-")
	optsValue := reflect.New(optsType).Elem()
	cmd := &cobra.Command{
		Use: name,
	}
	parent.AddCommand(cmd)
	SetCommandResponseMeta(cmd, ResponseOpenAPIMeta{Type: responseTypeOf[R]()})
	if meta := GetCommandOpenAPIMeta(parent); meta != nil {
		annotateEntityOperationCommand(cmd, parent, "action", "", "collection", name, "", false, false)
	}

	if namer, ok := optsValue.Interface().(Name); ok {
		cmd.Use = namer.GetName()
		if meta := GetCommandOpenAPIMeta(parent); meta != nil {
			actionName := strings.Fields(cmd.Use)
			if len(actionName) > 0 {
				setCommandAnnotation(cmd, annotationClickyOperationAction, actionName[0])
			}
		}
	}

	cmd.SilenceUsage = true
	if h, ok := optsValue.Interface().(Help); ok {
		cmd.Long = h.Help().ANSI()
	}

	// Parse struct fields including embedded structs
	fieldInfos, err := flags.ParseStructFields(optsType)
	if err != nil {
		panic(fmt.Sprintf("failed to parse struct fields: %v", err))
	}

	// Track stdin field
	var stdinFieldCount int
	for _, info := range fieldInfos {
		if info.IsStdin {
			stdinFieldCount++
		}
	}
	if stdinFieldCount > 1 {
		panic("multiple stdin fields defined")
	}

	// Create flag storage
	flagValues := make(map[string]*flags.FlagValue)

	// Bind all flags
	var argsField *flags.FlagValue
	for _, info := range fieldInfos {
		fv := flags.BindFlag(cmd, info)
		if fv != nil {
			// Use special key for args-only fields (no flag name)
			key := info.FlagName
			if key == "" && info.IsArgs {
				key = flags.ARGS
			}
			flagValues[key] = fv
			// Track the args-accepting field regardless of whether it
			// also has a flag name. A field tagged both `flag:"x"` and
			// `args:"true"` should accept the positional OR the flag;
			// the binder in flags/assignment.go already handles that
			// precedence.
			if info.IsArgs {
				argsField = fv
			}
		}
	}

	if argsField != nil {
		if argsField.Required {
			cmd.Args = cobra.MinimumNArgs(1)
		} else {
			cmd.Args = cobra.MinimumNArgs(0)
		}
	} else {
		cmd.Args = cobra.NoArgs
	}

	// Register data func for RPC direct invocation (bypasses stdout capture)
	dataFuncRegistry.Store(cmd, func(flagMap map[string]string, args []string) (any, error) {
		// Set cobra flags from the map (same as executor.ExecuteCommand does)
		for k, v := range flagMap {
			if flag := cmd.Flags().Lookup(k); flag != nil {
				_ = flag.Value.Set(v)
				flag.Changed = true
			}
		}

		// Build opts struct from the now-set flags (same as RunE does)
		optsValue := reflect.New(optsType).Elem()
		for _, fv := range flagValues {
			argsToPass := []string(nil)
			if fv.IsArgs {
				argsToPass = args
			}
			if err := flags.AssignFieldValue(optsValue, fv, argsToPass, false); err != nil {
				return nil, err
			}
		}
		return fn(optsValue.Interface().(T))
	})

	// Set RunE function
	cmd.RunE = func(c *cobra.Command, args []string) error {
		// Create new instance of opts
		optsValue := reflect.New(optsType).Elem()

		// First pass: Find the field with args:"true"
		var argsFieldValue *flags.FlagValue
		for _, fv := range flagValues {
			if fv.IsArgs {
				argsFieldValue = fv
				break
			}
		}

		// Process flags and populate struct
		for _, fv := range flagValues {
			// Only pass args to the field with args:"true", pass nil to all others
			argsToPass := []string(nil)
			if fv.IsArgs && argsFieldValue == fv {
				argsToPass = args
			}

			if err := flags.AssignFieldValue(optsValue, fv, argsToPass, isStdinAvailable()); err != nil {
				return err
			}
		}

		// Call the function
		result, err := fn(optsValue.Interface().(T))
		if err != nil {
			// An error that carries a clicky rendering interface
			// (Pretty/Textable/Tree*) is rendered through the same format
			// pipeline as a success result — honouring --format — instead of
			// being collapsed to its Error() line. The error is still
			// returned so cobra exits non-zero.
			if rich, ok := renderableError(err); ok {
				if specErr := Flags.ParseFormatSpec(); specErr != nil {
					return specErr
				}
				PrintAndWriteSinks(rich, Flags.FormatOptions)
			} else {
				logger.GetSlogLogger().WithSkipReportLevel(2).Errorf("Command %s failed: %v", name, err)
			}
			return err
		}

		if err := Flags.ParseFormatSpec(); err != nil {
			return err
		}
		PrintAndWriteSinks(result, Flags.FormatOptions)

		return nil
	}

	return cmd
}

// isStdinAvailable checks if stdin has data available
func isStdinAvailable() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// renderableError reports whether err — or any error in its Unwrap chain —
// carries a clicky rendering interface (Pretty / Textable / Tree*), so the
// command runner can render it through the format pipeline instead of just
// logging Error(). The chain is walked so a fmt-wrapped rich error still
// renders. The returned value is the matched error, ready to hand to
// PrintAndWriteSinks.
func renderableError(err error) (any, bool) {
	for e := err; e != nil; {
		if api.TryTypedValue(e) != nil {
			return e, true
		}
		// Check if this error wraps multiple errors (joined errors)
		if unwrapper, ok := e.(interface{ Unwrap() []error }); ok {
			for _, child := range unwrapper.Unwrap() {
				if result, found := renderableError(child); found {
					return result, true
				}
			}
			return nil, false
		}
		// Continue with single-error chain
		e = errors.Unwrap(e)
	}
	return nil, false
}

type Name interface {
	GetName() string
}
