package report

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"reports-app/accountservice"
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
	"profession_type_id", "profession_type", "profession", "address_detail",
	"latitude", "longitude", "last_met_at",
}

// Generate runs the roster query for the given filters, resolves each row's
// halqa/masjid name via account-service-app, uploads the resulting CSV to
// Cloudinary under a name derived from fileName, and returns its URL.
func Generate(ctx context.Context, reportID int64, fileName string, mIDs, professionTypeIDs []int) (outputPath string, err error) {
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
			lastMetAt                                     sql.NullTime
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
			strconv.Itoa(professionTypeID), professionType, profession,
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

	publicID := fmt.Sprintf("reports/%d-%s", reportID, sanitizeFileName(fileName))

	url, err := storage.UploadCSV(ctx, &buf, publicID)
	if err != nil {
		return "", fmt.Errorf("upload csv: %w", err)
	}

	return url, nil
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
