package flags

import (
	"reflect"
	"strconv"
	"time"

	"github.com/flanksource/commons/duration"
	"github.com/spf13/cobra"
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
	}

	// Bind flag based on type (skip flag registration if no flag name)
	switch info.FieldType.Kind() {
	case reflect.String:
		var val string
		if info.DefaultValue != "" {
			val = info.DefaultValue
		}
		fv.StringPtr = &val
		if info.FlagName != "" {
			if info.ShortFlag != "" {
				cmd.Flags().StringVarP(fv.StringPtr, info.FlagName, info.ShortFlag, val, info.Help)
			} else {
				cmd.Flags().StringVar(fv.StringPtr, info.FlagName, val, info.Help)
			}
		}

	case reflect.Int:
		var val int
		if info.DefaultValue != "" {
			val, _ = strconv.Atoi(info.DefaultValue)
		}
		fv.IntPtr = &val
		if info.FlagName != "" {
			if info.ShortFlag != "" {
				cmd.Flags().IntVarP(fv.IntPtr, info.FlagName, info.ShortFlag, val, info.Help)
			} else {
				cmd.Flags().IntVar(fv.IntPtr, info.FlagName, val, info.Help)
			}
		}

	case reflect.Bool:
		var val bool
		if info.DefaultValue != "" {
			val = info.DefaultValue == "true"
		}
		fv.BoolPtr = &val
		if info.FlagName != "" {
			if info.ShortFlag != "" {
				cmd.Flags().BoolVarP(fv.BoolPtr, info.FlagName, info.ShortFlag, val, info.Help)
			} else {
				cmd.Flags().BoolVar(fv.BoolPtr, info.FlagName, val, info.Help)
			}
		}

	case reflect.Slice:
		switch info.FieldType.Elem().Kind() {
		case reflect.String:
			var val []string
			fv.StringSlicePtr = &val
			if info.FlagName != "" {
				if info.ShortFlag != "" {
					cmd.Flags().StringSliceVarP(fv.StringSlicePtr, info.FlagName, info.ShortFlag, val, info.Help)
				} else {
					cmd.Flags().StringSliceVar(fv.StringSlicePtr, info.FlagName, val, info.Help)
				}
			}

		case reflect.Int:
			var val []int
			fv.IntSlicePtr = &val
			if info.FlagName != "" {
				if info.ShortFlag != "" {
					cmd.Flags().IntSliceVarP(fv.IntSlicePtr, info.FlagName, info.ShortFlag, val, info.Help)
				} else {
					cmd.Flags().IntSliceVar(fv.IntSlicePtr, info.FlagName, val, info.Help)
				}
			}
		}

	default:
		// Handle special types by name
		typeName := info.FieldType.String()
		switch typeName {
		case "duration.Duration":
			var val duration.Duration
			if info.DefaultValue != "" {
				val, _ = duration.ParseDuration(info.DefaultValue)
			}
			fv.DurationPtr = &val
			if info.FlagName != "" {
				cmd.Flags().Var(&durationValue{d: fv.DurationPtr}, info.FlagName, info.Help)
				if info.ShortFlag != "" {
					cmd.Flags().Lookup(info.FlagName).Shorthand = info.ShortFlag
				}
			}

		case "time.Time":
			var val time.Time
			if info.DefaultValue != "" {
				val, _ = parseTime(info.DefaultValue)
			}
			fv.TimePtr = &val
			if info.FlagName != "" {
				cmd.Flags().Var(&timeValue{t: fv.TimePtr}, info.FlagName, info.Help)
				if info.ShortFlag != "" {
					cmd.Flags().Lookup(info.FlagName).Shorthand = info.ShortFlag
				}
			}
		}
	}

	if info.Required && info.FlagName != "" {
		_ = cmd.MarkFlagRequired(info.FlagName)
	}

	return fv
}
