package flags

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/flanksource/commons/duration"
	"github.com/timberio/go-datemath"
	"github.com/tj/go-naturaldate"
)

// FlagValue holds information about a flag and its binding
type FlagValue struct {
	FieldName      string
	FieldPath      []int // Field indices for navigating embedded structs
	FieldType      reflect.Type
	DefaultValue   string
	Required       bool
	IsStdin        bool
	IsArgs         bool
	StringPtr      *string
	IntPtr         *int
	BoolPtr        *bool
	StringSlicePtr *[]string
	IntSlicePtr    *[]int
	DurationPtr    *duration.Duration
	TimePtr        *time.Time
}

// FieldInfo contains metadata about a struct field for flag parsing
type FieldInfo struct {
	FieldName    string
	FieldPath    []int // Indices to navigate from root struct to this field
	FieldType    reflect.Type
	FlagName     string
	Help         string
	DefaultValue string
	ShortFlag    string
	Required     bool
	Hidden       bool
	IsStdin      bool
	IsArgs       bool
	// Enum is the field's closed set of accepted values, from an
	// `enum:"a,b,c"` tag. It travels into the OpenAPI parameter and the
	// published action schema, so a front end renders the real choices instead
	// of hardcoding a list that drifts from the server's.
	Enum []string
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
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Try go-datemath for Elasticsearch-style expressions
	if strings.HasPrefix(s, "now") || strings.Contains(s, "+") || strings.Contains(s, "-") || strings.Contains(s, "/") {
		if expr, err := datemath.Parse(s); err == nil {
			t := expr.Time(datemath.WithNow(now))
			return t, nil
		}
	}

	// Try naturaldate for relative dates
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
