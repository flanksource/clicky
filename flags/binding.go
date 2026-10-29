package flags

import (
	"reflect"
	"strconv"
	"time"

	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// BindFlag creates and binds a flag to a Cobra command based on field info
func BindFlag(cmd *cobra.Command, info FieldInfo) *FlagValue {
	fv := &FlagValue{
		FieldName:    info.FieldName,
		FieldPath:    info.FieldPath,
		FieldType:    info.FieldType,
		DefaultValue: info.DefaultValue,
		Required:     info.Required,
		IsStdin:      info.IsStdin,
		IsArgs:       info.IsArgs,
		CLIFileRead:  info.CLIFileRead,
	}
	bindFlagValue(cmd.Flags(), info, fv)
	fv.Explicit = func() bool { return cmd.Flags().Changed(info.FlagName) }
	if info.FlagName != "" {
		annotateFlag(cmd, info)
	}
	return fv
}

// bindFlagValue allocates the value fv holds for info's type and, when info
// names a flag, registers it. pflag's XVar is XVarP with an empty shorthand.
func bindFlagValue(fs *pflag.FlagSet, info FieldInfo, fv *FlagValue) {
	register := info.FlagName != ""
	if info.FieldType == reflect.TypeOf((*bool)(nil)) {
		info.FieldType = reflect.TypeOf(false)
	}
	switch info.FieldType.Kind() {
	case reflect.String:
		val := info.DefaultValue
		fv.StringPtr = &val
		if register {
			fs.StringVarP(fv.StringPtr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	case reflect.Int:
		var val int
		if info.DefaultValue != "" {
			val, _ = strconv.Atoi(info.DefaultValue)
		}
		fv.IntPtr = &val
		if register {
			fs.IntVarP(fv.IntPtr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	case reflect.Float64:
		var val float64
		if info.DefaultValue != "" {
			val, _ = strconv.ParseFloat(info.DefaultValue, 64)
		}
		fv.Float64Ptr = &val
		if register {
			fs.Float64VarP(fv.Float64Ptr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	case reflect.Bool:
		var val bool
		if info.DefaultValue != "" {
			val, _ = strconv.ParseBool(info.DefaultValue)
		}
		fv.BoolPtr = &val
		if register {
			fs.BoolVarP(fv.BoolPtr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	case reflect.Slice:
		bindSliceFlag(fs, info, fv)
	default:
		bindNamedTypeFlag(fs, info, fv)
	}
}

func bindSliceFlag(fs *pflag.FlagSet, info FieldInfo, fv *FlagValue) {
	register := info.FlagName != ""
	switch info.FieldType.Elem().Kind() {
	case reflect.String:
		var val []string
		fv.StringSlicePtr = &val
		if !register {
			return
		}
		if info.LiteralSlice {
			fs.StringArrayVarP(fv.StringSlicePtr, info.FlagName, info.ShortFlag, val, info.Help)
		} else {
			fs.StringSliceVarP(fv.StringSlicePtr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	case reflect.Int:
		var val []int
		fv.IntSlicePtr = &val
		if register {
			fs.IntSliceVarP(fv.IntSlicePtr, info.FlagName, info.ShortFlag, val, info.Help)
		}
	}
}

func bindNamedTypeFlag(fs *pflag.FlagSet, info FieldInfo, fv *FlagValue) {
	var value pflag.Value
	switch info.FieldType.String() {
	case "duration.Duration":
		var val duration.Duration
		if info.DefaultValue != "" {
			val, _ = duration.ParseDuration(info.DefaultValue)
		}
		fv.DurationPtr = &val
		value = &durationValue{d: fv.DurationPtr}
	case "time.Time":
		var val time.Time
		if info.DefaultValue != "" {
			val, _ = parseTime(info.DefaultValue)
		}
		fv.TimePtr = &val
		value = &timeValue{t: fv.TimePtr}
	default:
		return
	}
	if info.FlagName == "" {
		return
	}
	fs.Var(value, info.FlagName, info.Help)
	if info.ShortFlag != "" {
		fs.Lookup(info.FlagName).Shorthand = info.ShortFlag
	}
}

func annotateFlag(cmd *cobra.Command, info FieldInfo) {
	if info.Required {
		_ = cmd.MarkFlagRequired(info.FlagName)
	}

	// Hidden flags stay usable on the CLI but are dropped from --help and from
	// generated tool/OpenAPI schemas (the RPC converter skips flag.Hidden).
	if info.Hidden {
		_ = cmd.Flags().MarkHidden(info.FlagName)
	}

	// An `enum:` tag rides on the same annotation the sort/order flags use, so
	// the RPC converter publishes it as the parameter's enum without knowing
	// anything about struct tags.
	if len(info.Enum) > 0 {
		SetAllowedValues(cmd.Flags().Lookup(info.FlagName), info.Enum...)
	}

	if info.Category != "" {
		SetCategory(cmd.Flags().Lookup(info.FlagName), info.Category)
	}
}
