package api

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var columnNumberPrinter = message.NewPrinter(language.AmericanEnglish)

// ColumnFormatValues returns the formats supported by schema-driven columns.
func ColumnFormatValues() []string {
	return []string{FormatDate, FormatFloat, FormatInteger, FieldTypeDuration, FieldTypeBytes, FormatCurrency}
}

// formatIntegerValue renders a whole number without decimals whatever numeric
// type decoded it: JSON and a sqlite NUMERIC column both yield a float64. A
// value with a fraction keeps it, and a non-number shows as it is.
func formatIntegerValue(value any) string {
	switch reflected := reflect.ValueOf(value); reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(reflected.Uint(), 10)
	}
	number := (FieldValue{Value: value}).Float()
	if number == nil {
		return fmt.Sprintf("%v", value)
	}
	if math.Trunc(*number) == *number && math.Abs(*number) < 1<<53 {
		return strconv.FormatInt(int64(*number), 10)
	}
	return strconv.FormatFloat(*number, 'f', -1, 64)
}

// ColumnUnitValues returns the Grafana-compatible units supported by columns.
func ColumnUnitValues() []string {
	return []string{
		ColumnUnitNone,
		ColumnUnitShort,
		ColumnUnitPercent,
		ColumnUnitPercentUnit,
		ColumnUnitBytes,
		ColumnUnitDecimalBytes,
		ColumnUnitBytesPerSecond,
		ColumnUnitBinaryBytesPerSecond,
		ColumnUnitMilliseconds,
		ColumnUnitSeconds,
	}
}

func formatColumnScalar(column ColumnDef, value any) (Textable, bool) {
	switch value.(type) {
	case PrettyShort, Textable, Pretty:
		return nil, false
	}

	if column.Unit != "" {
		if formatted, ok := formatColumnUnit(value, column.Unit); ok {
			return Text{Content: formatted, Style: "number"}, true
		}
	}
	if column.Format == "" {
		return nil, false
	}
	parsed, err := (PrettyField{
		Name:          column.Name,
		Type:          column.Type,
		Format:        column.Format,
		FormatOptions: column.FormatOptions,
	}).Parse(value)
	if err != nil || parsed.Text == nil {
		return nil, false
	}
	return parsed.Text, true
}

func formatColumnUnit(value any, unit string) (string, bool) {
	number := (FieldValue{Value: value}).Float()
	if number == nil || math.IsNaN(*number) || math.IsInf(*number, 0) {
		return "", false
	}
	switch unit {
	case ColumnUnitNone, ColumnUnitShort:
		return formatShortNumber(*number), true
	case ColumnUnitPercent:
		return formatDecimal(*number) + "%", true
	case ColumnUnitPercentUnit:
		return formatDecimal(*number*100) + "%", true
	case ColumnUnitBytes:
		return formatByteValue(*number, 1024, ""), true
	case ColumnUnitDecimalBytes:
		return formatByteValue(*number, 1000, ""), true
	case ColumnUnitBytesPerSecond, ColumnUnitBinaryBytesPerSecond:
		return formatByteValue(*number, 1024, "/s"), true
	case ColumnUnitMilliseconds:
		return formatDurationValue(*number), true
	case ColumnUnitSeconds:
		return formatDurationValue(*number * 1000), true
	default:
		return "", false
	}
}

func formatDecimal(value float64) string {
	if math.Trunc(value) == value {
		return columnNumberPrinter.Sprintf("%.0f", value)
	}
	return strings.TrimSuffix(columnNumberPrinter.Sprintf("%.1f", value), ".0")
}

func formatShortNumber(value float64) string {
	units := []string{"", "K", "M", "B", "T"}
	sign := ""
	if value < 0 {
		sign = "-"
		value = math.Abs(value)
	}
	if value < 1000 {
		return sign + formatDecimal(value)
	}
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if value >= 10 {
		return fmt.Sprintf("%s%.0f%s", sign, value, units[unit])
	}
	return fmt.Sprintf("%s%s%s", sign, strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0"), units[unit])
}

func formatByteValue(value, base float64, suffix string) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	if value <= 0 {
		return "0 B" + suffix
	}
	unit := 0
	for value >= base && unit < len(units)-1 {
		value /= base
		unit++
	}
	rendered := fmt.Sprintf("%.0f", value)
	if value < 10 && unit > 0 {
		rendered = strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
	}
	return fmt.Sprintf("%s %s%s", rendered, units[unit], suffix)
}

func formatDurationValue(milliseconds float64) string {
	abs := math.Abs(milliseconds)
	switch {
	case abs < 1000:
		return columnNumberPrinter.Sprintf("%.0f ms", milliseconds)
	case abs < 60_000:
		return formatDecimal(milliseconds/1000) + " s"
	case abs < 3_600_000:
		return formatDecimal(milliseconds/60_000) + " min"
	default:
		return formatDecimal(milliseconds/3_600_000) + " h"
	}
}
