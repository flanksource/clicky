package clicky

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/flanksource/clicky/flags"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)


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
func AddCommand[T any](parent *cobra.Command, opts T, fn func(opts T) (any, error)) *cobra.Command {
	optsType := reflect.TypeOf(opts)
	if optsType.Kind() != reflect.Struct {
		panic("AddCommand requires a struct type for opts parameter")
	}

	name := lo.KebabCase(strings.TrimSuffix(optsType.Name(), "Options"))
	name = strings.TrimPrefix(name, parent.Use+"-")
	optsValue := reflect.New(optsType).Elem()
	cmd := &cobra.Command{
		Use: name,
	}
	parent.AddCommand(cmd)

	if namer, ok := optsValue.Interface().(Name); ok {
		cmd.Use = namer.GetName()
	}

	if h, ok := optsValue.Interface().(Help); ok {
		cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
			fmt.Println(h.Help().ANSI())
		})
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
	for _, info := range fieldInfos {
		fv := flags.BindFlag(cmd, info)
		flagValues[info.FlagName] = fv
	}

	// Set RunE function
	cmd.RunE = func(c *cobra.Command, args []string) error {
		// Create new instance of opts
		optsValue := reflect.New(optsType).Elem()

		// Process flags and populate struct
		for _, fv := range flagValues {
			if err := flags.AssignFieldValue(optsValue, fv, args, isStdinAvailable()); err != nil {
				return err
			}
		}

		// Call the function
		result, err := fn(optsValue.Interface().(T))
		if err != nil {
			return err
		}

		// Format and output result
		output, err := Format(result, Flags.FormatOptions)
		if err != nil {
			return fmt.Errorf("formatting result: %w", err)
		}

		fmt.Println(output)
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

type Name interface {
	GetName() string
}
