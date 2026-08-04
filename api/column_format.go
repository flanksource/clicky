package api

import (
	"fmt"
	"math"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var columnNumberPrinter = message.NewPrinter(language.AmericanEnglish)

// ColumnFormatValues returns the formats supported by schema-driven columns.
func ColumnFormatValues() []string {
	return []string{FormatDate, FormatFloat, FieldTypeDuration, FieldTypeBytes, FormatCurrency}
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
