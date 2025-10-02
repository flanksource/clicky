package clicky

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
	"github.com/timberio/go-datemath"
	"github.com/tj/go-naturaldate"
)

func loadFromStdin() ([]string, error) {
	if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		return nil, nil // No stdin data
	} else if stat != nil {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines, nil
	}

	return nil, nil
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
func AddCommand[T any](cmd *cobra.Command, opts T, fn func(opts T) (any, error)) *cobra.Command {
	optsType := reflect.TypeOf(opts)
	if optsType.Kind() != reflect.Struct {
		panic("AddCommand requires a struct type for opts parameter")
	}

	// Track stdin field
	var stdinFieldName string
	var stdinFieldIndex = -1

	// Create flag storage
	flagValues := make(map[string]*flagValue)

	// Parse struct fields and add flags
	for i := 0; i < optsType.NumField(); i++ {
		field := optsType.Field(i)
		flagName := field.Tag.Get("flag")
		if flagName == "" {
			continue
		}

		help := field.Tag.Get("help")
		defaultVal := field.Tag.Get("default")
		shortFlag := field.Tag.Get("short")
		required := field.Tag.Get("required") == "true"
		isStdin := field.Tag.Get("stdin") == "true"

		if isStdin {
			if stdinFieldIndex != -1 {
				panic(fmt.Sprintf("multiple stdin fields defined: %s and %s", stdinFieldName, field.Name))
			}
			stdinFieldName = field.Name
			stdinFieldIndex = i
		}

		// Create flag value holder
		fv := &flagValue{
			fieldName:    field.Name,
			fieldType:    field.Type,
			defaultValue: defaultVal,
			required:     required,
			isStdin:      isStdin,
		}
		flagValues[flagName] = fv

		// Bind flag based on type
		switch field.Type.Kind() {
		case reflect.String:
			var val string
			if defaultVal != "" {
				val = defaultVal
			}
			fv.stringPtr = &val
			if shortFlag != "" {
				cmd.Flags().StringVarP(fv.stringPtr, flagName, shortFlag, val, help)
			} else {
				cmd.Flags().StringVar(fv.stringPtr, flagName, val, help)
			}

		case reflect.Int:
			var val int
			if defaultVal != "" {
				val, _ = strconv.Atoi(defaultVal)
			}
			fv.intPtr = &val
			if shortFlag != "" {
				cmd.Flags().IntVarP(fv.intPtr, flagName, shortFlag, val, help)
			} else {
				cmd.Flags().IntVar(fv.intPtr, flagName, val, help)
			}

		case reflect.Bool:
			var val bool
			if defaultVal != "" {
				val = defaultVal == "true"
			}
			fv.boolPtr = &val
			if shortFlag != "" {
				cmd.Flags().BoolVarP(fv.boolPtr, flagName, shortFlag, val, help)
			} else {
				cmd.Flags().BoolVar(fv.boolPtr, flagName, val, help)
			}

		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				var val []string
				fv.stringSlicePtr = &val
				if shortFlag != "" {
					cmd.Flags().StringSliceVarP(fv.stringSlicePtr, flagName, shortFlag, val, help)
				} else {
					cmd.Flags().StringSliceVar(fv.stringSlicePtr, flagName, val, help)
				}

			case reflect.Int:
				var val []int
				fv.intSlicePtr = &val
				if shortFlag != "" {
					cmd.Flags().IntSliceVarP(fv.intSlicePtr, flagName, shortFlag, val, help)
				} else {
					cmd.Flags().IntSliceVar(fv.intSlicePtr, flagName, val, help)
				}
			}

		default:
			// Handle special types by name
			typeName := field.Type.String()
			switch typeName {
			case "duration.Duration":
				var val duration.Duration
				if defaultVal != "" {
					val, _ = duration.ParseDuration(defaultVal)
				}
				fv.durationPtr = &val
				cmd.Flags().Var(&durationValue{d: fv.durationPtr}, flagName, help)
				if shortFlag != "" {
					cmd.Flags().Lookup(flagName).Shorthand = shortFlag
				}

			case "time.Time":
				var val time.Time
				if defaultVal != "" {
					val, _ = parseTime(defaultVal)
				}
				fv.timePtr = &val
				cmd.Flags().Var(&timeValue{t: fv.timePtr}, flagName, help)
				if shortFlag != "" {
					cmd.Flags().Lookup(flagName).Shorthand = shortFlag
				}
			}
		}

		if required {
			_ = cmd.MarkFlagRequired(flagName)
		}
	}

	cmd.SilenceUsage = true
	var err error

	optsValue := reflect.New(optsType).Elem()

	if h, ok := optsValue.Interface().(Help); ok {
		cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
			fmt.Println(h.Help().ANSI())
		})
	}

	// Set RunE function
	cmd.RunE = func(c *cobra.Command, args []string) error {
		// Create new instance of opts
		optsValue := reflect.New(optsType).Elem()

		// Process flags and populate struct
		for flagName, fv := range flagValues {
			flag := c.Flags().Lookup(flagName)
			if flag == nil {
				continue
			}

			fieldValue := optsValue.FieldByName(fv.fieldName)
			if !fieldValue.IsValid() || !fieldValue.CanSet() {
				continue
			}

			// Process based on type
			switch fv.fieldType.Kind() {
			case reflect.String:
				val := *fv.stringPtr

				if val == "" && fv.isStdin && len(args) > 0 {
					val = args[0]
				} else if val == "" && fv.isStdin && isStdinAvailable() {
					if lines, err := loadFromStdin(); err != nil {
						return err
					} else if len(lines) > 0 {
						val = lines[0]
					}
				}

				if loaded, err := loadFromFileOrURL(val); err != nil {
					return fmt.Errorf("loading %s: %w", flagName, err)
				} else {
					fieldValue.SetString(loaded)
				}

			case reflect.Int:
				fieldValue.SetInt(int64(*fv.intPtr))

			case reflect.Bool:
				fieldValue.SetBool(*fv.boolPtr)

			case reflect.Slice:
				switch fv.fieldType.Elem().Kind() {
				case reflect.String:
					val := *fv.stringSlicePtr

					if len(val) == 0 && fv.isStdin && len(args) > 0 {
						val = args
					} else if len(val) == 0 && fv.isStdin && isStdinAvailable() {
						if val, err = loadFromStdin(); err != nil {
							return err
						}
					}

					if len(val) == 1 {
						if lines, err := loadLinesFromFileOrURL(val[0]); err != nil {
							return err
						} else {
							fieldValue.Set(reflect.ValueOf(lines))
						}
					} else {
						fieldValue.Set(reflect.ValueOf(val))
					}

				case reflect.Int:
					fieldValue.Set(reflect.ValueOf(*fv.intSlicePtr))
				}

			default:
				// Handle special types
				typeName := fv.fieldType.String()
				switch typeName {
				case "duration.Duration":
					fieldValue.Set(reflect.ValueOf(*fv.durationPtr))

				case "time.Time":
					fieldValue.Set(reflect.ValueOf(*fv.timePtr))
				}
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

// flagValue holds information about a flag
type flagValue struct {
	fieldName      string
	fieldType      reflect.Type
	defaultValue   string
	required       bool
	isStdin        bool
	stringPtr      *string
	intPtr         *int
	boolPtr        *bool
	stringSlicePtr *[]string
	intSlicePtr    *[]int
	durationPtr    *duration.Duration
	timePtr        *time.Time
}

// durationValue implements pflag.Value for duration.Duration
type durationValue struct {
	d *duration.Duration
}

func (d *durationValue) String() string {
	if d.d == nil {
		return ""
	}
	return d.d.String()
}

func (d *durationValue) Set(s string) error {
	parsed, err := duration.ParseDuration(s)
	if err != nil {
		return err
	}
	*d.d = parsed
	return nil
}

func (d *durationValue) Type() string {
	return "duration"
}

// timeValue implements pflag.Value for time.Time
type timeValue struct {
	t *time.Time
}

func (t *timeValue) String() string {
	if t.t == nil {
		return ""
	}
	return t.t.Format(time.RFC3339)
}

func (t *timeValue) Set(s string) error {
	parsed, err := parseTime(s)
	if err != nil {
		return err
	}
	*t.t = parsed
	return nil
}

func (t *timeValue) Type() string {
	return "time"
}

// parseTime parses time strings with support for:
// - Elasticsearch datemath: "now-7d", "now+2h", "now/d", "now-7d/d"
// - Relative dates: "yesterday", "today", "tomorrow"
// - Standard formats: RFC3339, ISO8601, common date formats
//
// Datemath syntax (Elasticsearch compatible via go-datemath):
// - now-5d        : 5 days ago
// - now+2h        : 2 hours from now
// - now/d         : start of current day
// - now-7d/d      : start of day 7 days ago
// - now-1M/M      : start of month, 1 month ago
// - Units: y (year), M (month), w (week), d (day), h (hour), m (minute), s (second)
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Try go-datemath for Elasticsearch-style expressions
	// Handles: now, now-7d, now+2h, now/d, now-7d/d, etc.
	if strings.HasPrefix(s, "now") || strings.Contains(s, "+") || strings.Contains(s, "-") || strings.Contains(s, "/") {
		if expr, err := datemath.Parse(s); err == nil {
			t := expr.Time(datemath.WithNow(now))
			return t, nil
		}
	}

	// Try naturaldate for relative dates like "yesterday", "today"
	if parsed, err := naturaldate.Parse(s, now); err == nil {
		return parsed, nil
	}

	// Try standard time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006-01-02 15:04:05",
		time.RFC1123,
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, s); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

// loadFromFileOrURL loads content from a file or URL
func loadFromFileOrURL(path string) (string, error) {
	if !strings.HasPrefix(path, "@") {
		return path, nil
	}
	path = strings.TrimPrefix(path, "@")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadLinesFromFileOrURL loads lines from a file or URL
func loadLinesFromFileOrURL(path string) ([]string, error) {
	content, err := loadFromFileOrURL(path)
	if err != nil {
		return nil, err
	}

	return parseLines(content), nil
}

// readLinesFromReader reads lines from a reader
func readLinesFromReader(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return parseLines(string(data)), nil
}

// parseLines parses lines from content, skipping empty lines and comments
func parseLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}

// isStdinAvailable checks if stdin has data available
func isStdinAvailable() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
