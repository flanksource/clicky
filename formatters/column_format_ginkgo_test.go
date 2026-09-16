package formatters

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xuri/excelize/v2"
)

type percentageTableRow struct {
	Ratio float64
}

func (r percentageTableRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{{
		Name: "ratio", Type: "number", Format: api.FormatFloat, Unit: api.ColumnUnitPercentUnit,
		FilterKey: "filter.ratio",
	}}
}

func (r percentageTableRow) Row() map[string]any {
	return map[string]any{"ratio": r.Ratio}
}

type capturedEventRow struct {
	At        any
	SessionID any
}

func (capturedEventRow) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		{Name: "timestamp", Kind: "timestamp", Type: "datetime", Format: api.FormatDate},
		{Name: "sessionId", Type: "number", Format: api.FormatInteger},
	}
}

func (r capturedEventRow) Row() map[string]any {
	return map[string]any{"timestamp": r.At, "sessionId": r.SessionID}
}

func clickyCells(row capturedEventRow) map[string]ClickyNode {
	output, err := (&ClickyJSONFormatter{}).Format(api.NewTableFrom([]capturedEventRow{row}), FormatOptions{})
	Expect(err).NotTo(HaveOccurred())
	var document ClickyDocument
	Expect(json.Unmarshal([]byte(output), &document)).To(Succeed())
	Expect(document.Node.Rows).To(HaveLen(1))
	return document.Node.Rows[0].Cells
}

var _ = Describe("Column format serialization", func() {
	const capturedAt = "2026-09-15T15:55:01.132Z"
	instant := time.Date(2026, time.September, 15, 15, 55, 1, 132_000_000, time.UTC)
	eastOfUTC := time.FixedZone("UTC+3", 3*60*60)

	DescribeTable("a time cell without a filter key carries its instant with its zone",
		func(at any, expected string) {
			cell := clickyCells(capturedEventRow{At: at, SessionID: 73})["timestamp"]

			Expect(cell.FilterValue).To(Equal(expected))
		},
		Entry("a UTC time.Time", instant, capturedAt),
		Entry("a *time.Time", &instant, capturedAt),
		Entry("a time.Time in a zone east of UTC", instant.In(eastOfUTC), "2026-09-15T18:55:01.132+03:00"),
		Entry("an RFC3339 string read back from an index", capturedAt, capturedAt),
	)

	It("leaves a zone-less time string without an instant rather than guessing its zone", func() {
		cell := clickyCells(capturedEventRow{At: "2026-09-15 15:55:01", SessionID: 73})["timestamp"]

		Expect(cell.FilterValue).To(BeNil())
	})

	It("renders an integer column decoded as a float64 as an integer", func() {
		cell := clickyCells(capturedEventRow{At: instant, SessionID: float64(1234567)})["sessionId"]

		Expect(cell.Plain).To(Equal("1234567"))
	})

	It("includes raw filter values while keeping rendered cells formatted", func() {
		table := api.NewTableFrom([]percentageTableRow{{Ratio: 0.42}})

		output, err := (&ClickyJSONFormatter{}).Format(table, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		var document ClickyDocument
		Expect(json.Unmarshal([]byte(output), &document)).To(Succeed())
		Expect(document.Node.Columns).To(HaveLen(1))
		Expect(document.Node.Columns[0].Format).To(Equal(api.FormatFloat))
		Expect(document.Node.Columns[0].Unit).To(Equal(api.ColumnUnitPercentUnit))
		Expect(document.Node.Rows[0].Cells["ratio"].Plain).To(Equal("42%"))
		Expect(document.Node.Rows[0].Cells["ratio"].FilterValue).To(Equal(0.42))
	})

	It("formats text exports but preserves raw JSON and Excel values", func() {
		columns := []api.ColumnDef{{Name: "ratio", Type: "number", Unit: api.ColumnUnitPercentUnit}}
		rows := []map[string]any{{"ratio": 0.42}}

		var csvOutput bytes.Buffer
		_, err := WriteTableStream(context.Background(), &csvOutput, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "csv"})
		Expect(err).NotTo(HaveOccurred())
		records, err := csv.NewReader(strings.NewReader(csvOutput.String())).ReadAll()
		Expect(err).NotTo(HaveOccurred())
		Expect(records[1][0]).To(Equal("42%"))

		var jsonOutput bytes.Buffer
		_, err = WriteTableStream(context.Background(), &jsonOutput, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "json"})
		Expect(err).NotTo(HaveOccurred())
		var decoded []map[string]any
		Expect(json.Unmarshal(jsonOutput.Bytes(), &decoded)).To(Succeed())
		Expect(decoded[0]["ratio"]).To(Equal(0.42))

		var excelOutput bytes.Buffer
		_, err = WriteTableStream(context.Background(), &excelOutput, &sliceRowIterator{columns: columns, rows: rows}, StreamOptions{Format: "excel"})
		Expect(err).NotTo(HaveOccurred())
		book, err := excelize.OpenReader(bytes.NewReader(excelOutput.Bytes()))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(book.Close)
		value, err := book.GetCellValue("Sheet1", "A2")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal("0.42"))
	})
})
