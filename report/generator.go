package report

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"reports-app/accountservice"
	"reports-app/entity"
	"reports-app/secondarydb"
	"reports-app/storage"
)

const rosterQuery = `
SELECT i.id, i.name, i.phone, i.email, i.h_id, i.m_id, i.r_id,
       i.profession_type_id, COALESCE(pt.name, ''), COALESCE(p.name, ''),
       COALESCE(i.address_detail, ''), i.latitude, i.longitude, i.last_met_at
FROM individuals i
LEFT JOIN profession_types pt ON pt.id = i.profession_type_id
LEFT JOIN professions p ON p.id = pt.profession_id
WHERE i.deleted_at IS NULL
  AND i.m_id = ANY($1)
  AND i.profession_type_id = ANY($2)
ORDER BY i.id
`

var csvHeader = []string{
	"id", "name", "phone", "email", "halqa_name", "masjid_name", "r_id",
	"profession_type_id", "profession", "profession_type", "address_detail",
	"latitude", "longitude", "last_met_at",
}

// rosterRow is a single individual as displayed in a generated PDF report.
type rosterRow struct {
	SNo            int
	Name           string
	Phone          string
	Email          string
	Halqa          string
	Masjid         string
	ProfessionType string
	Profession     string
	Address        string
	LastMetAt      string
}

// professionCount is one line of the profession breakdown summary table.
type professionCount struct {
	Profession string
	Count      int
}

// reportData is everything the PDF renderer needs to lay out a report.
type reportData struct {
	Title       string
	GeneratedAt time.Time
	Total       int
	// SingleMasjid is true when the report was filtered to exactly one m_id,
	// so every row shares the same Halqa/Masjid: the renderer prints them
	// once in the header instead of repeating them down the detail table.
	SingleMasjid      bool
	Halqa             string
	Masjid            string
	Rows              []rosterRow
	ProfessionSummary []professionCount
}

// Generate runs the roster query for the given filters, renders the result
// in the requested format, uploads it to Cloudinary under a name derived
// from fileName, and returns its URL.
func Generate(ctx context.Context, reportID int64, fileName string, format entity.Format,
	mIDs, professionTypeIDs []int) (outputPath string, err error) {
	switch format {
	case entity.FormatCSV:
		return generateCSV(ctx, reportID, fileName, mIDs, professionTypeIDs)
	case entity.FormatPDF:
		return generatePDF(ctx, reportID, fileName, mIDs, professionTypeIDs)
	default:
		return "", fmt.Errorf("unsupported report format %q", format)
	}
}

// generateCSV runs the roster query, resolves each row's halqa/masjid name
// via account-service-app, uploads the resulting CSV to Cloudinary, and
// returns its URL.
func generateCSV(ctx context.Context, reportID int64, fileName string, mIDs, professionTypeIDs []int) (string, error) {
	rows, err := secondarydb.DB.QueryContext(ctx, rosterQuery, pq.Array(mIDs), pq.Array(professionTypeIDs))
	if err != nil {
		return "", fmt.Errorf("query individuals: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}

	for rows.Next() {
		var (
			id, hID, mID, rID, professionTypeID          int
			name, professionType, profession, addressDet string
			phone, email                                 sql.NullString
			latitude, longitude                          sql.NullFloat64
			lastMetAt                                    sql.NullTime
		)

		scanErr := rows.Scan(&id, &name, &phone, &email, &hID, &mID, &rID,
			&professionTypeID, &professionType, &profession,
			&addressDet, &latitude, &longitude, &lastMetAt)
		if scanErr != nil {
			return "", fmt.Errorf("scan individual row: %w", scanErr)
		}

		halqaName, err := accountservice.HalqaName(hID)
		if err != nil {
			return "", fmt.Errorf("resolve halqa name: %w", err)
		}

		masjidName, err := accountservice.MasjidName(mID)
		if err != nil {
			return "", fmt.Errorf("resolve masjid name: %w", err)
		}

		record := []string{
			strconv.Itoa(id), name, phone.String, email.String,
			halqaName, masjidName, strconv.Itoa(rID),
			strconv.Itoa(professionTypeID), profession, professionType,
			addressDet, formatFloat(latitude), formatFloat(longitude), formatTime(lastMetAt),
		}

		if writeErr := w.Write(record); writeErr != nil {
			return "", fmt.Errorf("write csv row: %w", writeErr)
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate individuals: %w", err)
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return "", fmt.Errorf("flush csv: %w", err)
	}

	publicID := reportPublicID(reportID, fileName, time.Now())

	url, err := storage.Upload(ctx, buf.Bytes(), publicID, "csv")
	if err != nil {
		return "", fmt.Errorf("upload csv: %w", err)
	}

	return url, nil
}

// generatePDF runs the roster query, resolves each row's halqa/masjid name
// via account-service-app, renders a PDF report, uploads it to Cloudinary,
// and returns its URL.
func generatePDF(ctx context.Context, reportID int64, fileName string, mIDs, professionTypeIDs []int) (string, error) {
	data, err := fetchReportData(ctx, fileName, mIDs, professionTypeIDs)
	if err != nil {
		return "", err
	}

	buf, err := buildPDF(data)
	if err != nil {
		return "", fmt.Errorf("build pdf report: %w", err)
	}

	publicID := reportPublicID(reportID, fileName, data.GeneratedAt)

	url, err := storage.Upload(ctx, buf, publicID, "pdf")
	if err != nil {
		return "", fmt.Errorf("upload pdf: %w", err)
	}

	return url, nil
}

func fetchReportData(ctx context.Context, fileName string, mIDs, professionTypeIDs []int) (*reportData, error) {
	rows, err := secondarydb.DB.QueryContext(ctx, rosterQuery, pq.Array(mIDs), pq.Array(professionTypeIDs))
	if err != nil {
		return nil, fmt.Errorf("query individuals: %w", err)
	}
	defer rows.Close()

	data := &reportData{
		Title:        fileName,
		GeneratedAt:  time.Now(),
		SingleMasjid: len(mIDs) == 1,
	}

	professionCounts := map[string]int{}

	for rows.Next() {
		var (
			id, hID, mID, rID, professionTypeID          int
			name, professionType, profession, addressDet string
			phone, email                                 sql.NullString
			latitude, longitude                          sql.NullFloat64
			lastMetAt                                    sql.NullTime
		)

		scanErr := rows.Scan(&id, &name, &phone, &email, &hID, &mID, &rID,
			&professionTypeID, &professionType, &profession,
			&addressDet, &latitude, &longitude, &lastMetAt)
		if scanErr != nil {
			return nil, fmt.Errorf("scan individual row: %w", scanErr)
		}

		halqaName, err := accountservice.HalqaName(hID)
		if err != nil {
			return nil, fmt.Errorf("resolve halqa name: %w", err)
		}

		masjidName, err := accountservice.MasjidName(mID)
		if err != nil {
			return nil, fmt.Errorf("resolve masjid name: %w", err)
		}

		if data.SingleMasjid && data.Total == 0 {
			data.Halqa = halqaName
			data.Masjid = masjidName
		}

		data.Total++
		data.Rows = append(data.Rows, rosterRow{
			SNo:            data.Total,
			Name:           name,
			Phone:          phone.String,
			Email:          email.String,
			Halqa:          halqaName,
			Masjid:         masjidName,
			ProfessionType: professionType,
			Profession:     profession,
			Address:        addressDet,
			LastMetAt:      formatTime(lastMetAt),
		})

		professionCounts[profession]++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate individuals: %w", err)
	}

	data.ProfessionSummary = sortedProfessionCounts(professionCounts)

	return data, nil
}

// sortedProfessionCounts returns counts ordered alphabetically by profession
// name so the summary table renders in a stable, predictable order.
func sortedProfessionCounts(counts map[string]int) []professionCount {
	summary := make([]professionCount, 0, len(counts))
	for profession, count := range counts {
		summary = append(summary, professionCount{Profession: profession, Count: count})
	}

	sort.Slice(summary, func(i, j int) bool { return summary[i].Profession < summary[j].Profession })

	return summary
}

// reportPublicID builds the Cloudinary public ID for a generated report:
// the reportID keeps assets from different requests apart, while the
// visible filename portion (ReportName_timestamp) is what downloaders see.
func reportPublicID(reportID int64, fileName string, generatedAt time.Time) string {
	return fmt.Sprintf("reports/%d/%s_%s", reportID, sanitizeFileName(fileName), generatedAt.Format("20060102150405"))
}

// sanitizeFileName restricts fileName to characters safe for a Cloudinary
// public ID, replacing everything else (spaces, slashes, etc.) with a dash.
func sanitizeFileName(fileName string) string {
	var b strings.Builder

	for _, r := range fileName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	return b.String()
}

func formatFloat(v sql.NullFloat64) string {
	if !v.Valid {
		return ""
	}

	return strconv.FormatFloat(v.Float64, 'f', -1, 64)
}

func formatTime(v sql.NullTime) string {
	if !v.Valid {
		return ""
	}

	return v.Time.Format(time.RFC3339)
}
