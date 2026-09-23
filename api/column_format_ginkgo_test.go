package api

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Column formatting", func() {
	DescribeTable("formats canonical units",
		func(value any, unit, expected string) {
			column := Column("value").Type("number").Unit(unit).Build()
			Expect(ColumnTextable(column, value).String()).To(Equal(expected))
			Expect(ColumnString(column, value)).To(Equal(expected))
		},
		Entry("none", 1200, ColumnUnitNone, "1.2K"),
		Entry("short", 1200, ColumnUnitShort, "1.2K"),
		Entry("percent", 12.34, ColumnUnitPercent, "12.3%"),
		Entry("fractional percent", 0.423, ColumnUnitPercentUnit, "42.3%"),
		Entry("binary bytes", 1536, ColumnUnitBytes, "1.5 KB"),
		Entry("decimal bytes", 1500, ColumnUnitDecimalBytes, "1.5 KB"),
		Entry("byte rate", 1536, ColumnUnitBytesPerSecond, "1.5 KB/s"),
		Entry("binary byte rate", 1536, ColumnUnitBinaryBytesPerSecond, "1.5 KB/s"),
		Entry("milliseconds", 1500, ColumnUnitMilliseconds, "1.5 s"),
		Entry("seconds", 90, ColumnUnitSeconds, "1.5 min"),
	)

	It("publishes the canonical profile format and unit values", func() {
		Expect(ColumnFormatValues()).To(Equal([]string{"date", "float", "integer", "duration", "bytes", "currency"}))
		Expect(ColumnUnitValues()).To(Equal([]string{
			"none", "short", "percent", "percentunit", "bytes", "decbytes", "Bps", "binBps", "ms", "s",
		}))
	})

	DescribeTable("formats an integer column without decimals, whatever numeric type decoded it",
		func(value any, expected string) {
			column := Column("sessionId").Type("number").Format(FormatInteger).Build()
			Expect(ColumnTextable(column, value).String()).To(Equal(expected))
		},
		Entry("an int from a typed row", 73, "73"),
		Entry("an int64 read from a sqlite INTEGER", int64(73), "73"),
		Entry("a float64 decoded from JSON or a sqlite NUMERIC", float64(73), "73"),
		Entry("a large count keeps every digit", float64(1234567), "1234567"),
		Entry("a non-integral value still shows its fraction", 2.5, "2.5"),
	)

	It("applies Unit after Format", func() {
		column := Column("ratio").Type("number").Format(FormatCurrency).Unit(ColumnUnitPercentUnit).Build()
		Expect(ColumnString(column, 0.42)).To(Equal("42%"))
	})

	It("uses the existing scalar Format metadata", func() {
		column := Column("amount").Type("number").Format(FormatCurrency).Build()
		Expect(ColumnString(column, 12.5)).To(Equal("$12.50"))
	})

	It("propagates Unit into the table column schema", func() {
		column := Column("latency").Type("number").Unit(ColumnUnitMilliseconds).Build()
		table := NewEmptyTable([]ColumnDef{column})
		Expect(table.Columns).To(HaveLen(1))
		Expect(table.Columns[0].Unit).To(Equal(ColumnUnitMilliseconds))
	})

	// Pinned to the golang.org/x/text/message AmericanEnglish printer output:
	// "," thousands groups, "." decimal point, round-half-even, and no sign on
	// negative zero while a negative value rounding to zero keeps "-".
	DescribeTable("groups thousands in decimal output",
		func(value float64, expected string) {
			Expect(formatDecimal(value)).To(Equal(expected))
		},
		Entry("zero", 0.0, "0"),
		Entry("negative zero", math.Copysign(0, -1), "0"),
		Entry("negative fraction rounding to zero", -0.04, "-0"),
		Entry("below a thousand", 999.0, "999"),
		Entry("rounds up into a new group", 999.95, "1,000"),
		Entry("a thousand", 1000.0, "1,000"),
		Entry("negative thousand", -1000.0, "-1,000"),
		Entry("fraction keeps one decimal", 1234.56, "1,234.6"),
		Entry("negative millions with fraction", -1234567.89, "-1,234,567.9"),
		Entry("half rounds to even", 0.25, "0.2"),
		Entry("binary above half rounds up", 0.05, "0.1"),
		Entry("int64 max as float", float64(math.MaxInt64), "9,223,372,036,854,775,808"),
		Entry("int64 min as float", float64(math.MinInt64), "-9,223,372,036,854,775,808"),
		Entry("beyond float exponent notation", 1e21, "1,000,000,000,000,000,000,000"),
		Entry("positive infinity", math.Inf(1), "∞"),
		Entry("negative infinity", math.Inf(-1), "-∞"),
	)

	DescribeTable("groups thousands in unit output",
		func(value float64, unit, expected string) {
			column := Column("value").Type("number").Unit(unit).Build()
			Expect(ColumnString(column, value)).To(Equal(expected))
		},
		Entry("sub-second milliseconds round half to even", 2.5, ColumnUnitMilliseconds, "2 ms"),
		Entry("negative sub-millisecond", -0.4, ColumnUnitMilliseconds, "-0 ms"),
		Entry("negative milliseconds", -999.0, ColumnUnitMilliseconds, "-999 ms"),
		Entry("hours group thousands", 3_600_000_000.0, ColumnUnitMilliseconds, "1,000 h"),
		Entry("percent groups thousands", 123456.7, ColumnUnitPercent, "123,456.7%"),
		Entry("percent unit groups thousands", 12345.678, ColumnUnitPercentUnit, "1,234,567.8%"),
		Entry("overflowing percent unit is infinite", math.MaxFloat64, ColumnUnitPercentUnit, "∞%"),
	)
})
