package api

import (
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
		Expect(ColumnFormatValues()).To(Equal([]string{"date", "float", "duration", "bytes", "currency"}))
		Expect(ColumnUnitValues()).To(Equal([]string{
			"none", "short", "percent", "percentunit", "bytes", "decbytes", "Bps", "binBps", "ms", "s",
		}))
	})

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
})
