package report

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

// usableTableWidth is the printable width of the detail table on a portrait
// A4 page: 210mm - 10mm left margin - 10mm right margin.
const usableTableWidth = 190

// buildDetailColumns lays out the detail table's columns for data: when
// CustomColumns is empty, Address and Last Met At are shown; otherwise they
// are replaced by one blank (fillable-by-hand) column per custom name,
// sharing whatever width Address/Last Met At would otherwise have used.
func buildDetailColumns(data *reportData) (columns []string, widths []float64) {
	if data.SingleMasjid {
		columns = []string{"S.No", "Name", "Phone", "Profession", "Profession Type"}
		widths = []float64{9, 34, 22, 25, 29}
	} else {
		columns = []string{"S.No", "Name", "Phone", "Halqa", "Masjid", "Profession", "Profession Type"}
		widths = []float64{8, 27, 18, 17, 17, 20, 25}
	}

	if len(data.CustomColumns) == 0 {
		columns = append(columns, "Address", "Last Met At")
		if data.SingleMasjid {
			widths = append(widths, 47, 24)
		} else {
			widths = append(widths, 36, 22)
		}

		return columns, widths
	}

	used := 0.0
	for _, w := range widths {
		used += w
	}

	each := (usableTableWidth - used) / float64(len(data.CustomColumns))

	columns = append(columns, data.CustomColumns...)
	for range data.CustomColumns {
		widths = append(widths, each)
	}

	return columns, widths
}

func buildPDF(data *reportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
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

const tableLineHeight = 5

// wrappedRowHeight returns the height needed to draw values across widths,
// sizing the row to whichever cell wraps onto the most lines.
func wrappedRowHeight(pdf *fpdf.Fpdf, widths []float64, values []string) int {
	rowHeight := tableLineHeight
	for i, v := range values {
		if lines := len(pdf.SplitLines([]byte(v), widths[i]-2*pdf.GetCellMargin())); lines*tableLineHeight > rowHeight {
			rowHeight = lines * tableLineHeight
		}
	}

	return rowHeight
}

// writeWrappedRow draws one table row at the given height, wrapping each
// cell's text onto multiple lines instead of overflowing into the next
// column.
func writeWrappedRow(pdf *fpdf.Fpdf, widths []float64, values []string, rowHeight int, fill bool) {
	left, _, _, _ := pdf.GetMargins()
	x, y := pdf.GetXY()

	rectStyle := "D"
	if fill {
		rectStyle = "DF"
	}

	for i, v := range values {
		pdf.Rect(x, y, widths[i], float64(rowHeight), rectStyle)
		pdf.SetXY(x, y)
		pdf.MultiCell(widths[i], tableLineHeight, v, "", "L", false)
		x += widths[i]
	}

	pdf.SetXY(left, y+float64(rowHeight))
}

func writeDetailTable(pdf *fpdf.Fpdf, data *reportData) {
	columns, widths := buildDetailColumns(data)

	_, pageHeight := pdf.GetPageSize()
	_, _, _, bottom := pdf.GetMargins()
	pageBreakTrigger := pageHeight - bottom

	drawHeader := func() {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(239, 239, 239)
		writeWrappedRow(pdf, widths, columns, wrappedRowHeight(pdf, widths, columns), true)
		pdf.SetFont("Helvetica", "", 9)
	}

	drawHeader()

	for _, r := range data.Rows {
		values := []string{fmt.Sprintf("%d", r.SNo), r.Name, r.Phone}
		if !data.SingleMasjid {
			values = append(values, r.Halqa, r.Masjid)
		}

		values = append(values, r.Profession, r.ProfessionType)

		if len(data.CustomColumns) == 0 {
			values = append(values, r.Address, r.LastMetAt)
		} else {
			for range data.CustomColumns {
				values = append(values, "")
			}
		}

		rowHeight := wrappedRowHeight(pdf, widths, values)
		if pdf.GetY()+float64(rowHeight) > pageBreakTrigger {
			pdf.AddPage()
			drawHeader()
		}

		writeWrappedRow(pdf, widths, values, rowHeight, false)
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
