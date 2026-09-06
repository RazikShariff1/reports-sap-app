package report

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

var (
	detailColumns   = []string{"S.No", "Name", "Phone", "Email", "Halqa", "Masjid", "Profession", "Profession Type", "Address", "Last Met At"}
	detailColWidths = []float64{10, 32, 22, 42, 20, 20, 24, 30, 36, 34}
)

var (
	compactDetailColumns   = []string{"S.No", "Name", "Phone", "Email", "Profession", "Profession Type", "Address", "Last Met At"}
	compactDetailColWidths = []float64{10, 40, 26, 50, 28, 34, 44, 38}
)

func buildPDF(data *reportData) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, data.Title, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	if data.SingleMasjid {
		writeHeaderField(pdf, "Halqa", data.Halqa)
		writeHeaderField(pdf, "Masjid", data.Masjid)
	}

	writeHeaderField(pdf, "Generated On", data.GeneratedAt.Format("02-Jan-2006 03:04 PM"))
	writeHeaderField(pdf, "Total Individuals", fmt.Sprintf("%d", data.Total))
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Individual Details", "", 1, "L", false, 0, "")

	writeDetailTable(pdf, data)
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Profession Summary", "", 1, "L", false, 0, "")

	writeSummaryTable(pdf, data.ProfessionSummary, data.Total)

	var buf bytes.Buffer

	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}

	return buf.Bytes(), nil
}

func writeHeaderField(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(35, 6, label, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
}

func writeDetailTable(pdf *fpdf.Fpdf, data *reportData) {
	const rowHeight = 6

	columns, widths := detailColumns, detailColWidths
	if data.SingleMasjid {
		columns, widths = compactDetailColumns, compactDetailColWidths
	}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(239, 239, 239)

	for i, header := range columns {
		pdf.CellFormat(widths[i], rowHeight, header, "1", 0, "L", true, 0, "")
	}

	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)

	for _, r := range data.Rows {
		values := []string{fmt.Sprintf("%d", r.SNo), r.Name, r.Phone, r.Email}
		if !data.SingleMasjid {
			values = append(values, r.Halqa, r.Masjid)
		}

		values = append(values, r.Profession, r.ProfessionType, r.Address, r.LastMetAt)

		for i, v := range values {
			pdf.CellFormat(widths[i], rowHeight, v, "1", 0, "L", false, 0, "")
		}

		pdf.Ln(-1)
	}
}

func writeSummaryTable(pdf *fpdf.Fpdf, summary []professionCount, total int) {
	const (
		rowHeight  = 6
		labelWidth = 60
		countWidth = 30
	)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(239, 239, 239)
	pdf.CellFormat(labelWidth, rowHeight, "Profession", "1", 0, "L", true, 0, "")
	pdf.CellFormat(countWidth, rowHeight, "Count", "1", 1, "L", true, 0, "")

	pdf.SetFont("Helvetica", "", 9)

	for _, s := range summary {
		pdf.CellFormat(labelWidth, rowHeight, s.Profession, "1", 0, "L", false, 0, "")
		pdf.CellFormat(countWidth, rowHeight, fmt.Sprintf("%d", s.Count), "1", 1, "L", false, 0, "")
	}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(labelWidth, rowHeight, "Total", "1", 0, "L", false, 0, "")
	pdf.CellFormat(countWidth, rowHeight, fmt.Sprintf("%d", total), "1", 1, "L", false, 0, "")
}
