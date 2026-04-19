package api

// parcel_features_data.go holds /data/export and /data/recipients/import
// (plus their CSV / row helpers) split out from parcel_features.go in
// Phase 3.3 (QAL-001). Routes remain registered by RegisterParcelFeatures
// in parcel_features.go.

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func dataExportUser(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	format := strings.ToLower(strings.TrimSpace(c.Query("format", "json")))

	recipients, locker, shipRequests, bookings, err := exportUserRows(c, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to prepare export"))
	}

	if format == "csv" {
		buf := &bytes.Buffer{}
		w := csv.NewWriter(buf)
		_ = w.Write([]string{"type", "id", "status", "created_at", "field_a", "field_b", "field_c"})
		for _, r := range recipients {
			_ = w.Write([]string{"recipient", r.ID, "", r.CreatedAt, r.Name, r.DestinationID, r.City})
		}
		for _, lp := range locker {
			_ = w.Write([]string{"locker_package", lp.ID, lp.Status, lp.CreatedAt, lp.SenderName, fmt.Sprintf("%.2f", lp.WeightLbs), lp.ArrivedAt})
		}
		for _, sr := range shipRequests {
			_ = w.Write([]string{"ship_request", sr.ID, sr.Status, sr.CreatedAt, sr.ConfirmationCode, sr.DestinationID, sr.ServiceType})
		}
		for _, b := range bookings {
			_ = w.Write([]string{"booking", b.ID, b.Status, b.CreatedAt, b.ConfirmationCode, b.DestinationID, b.ServiceType})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to generate CSV export"))
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="qcs_export.csv"`)
		return c.Send(buf.Bytes())
	}

	if format != "json" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "format must be json or csv"))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"generated_at":    time.Now().UTC().Format(time.RFC3339Nano),
			"user_id":         userID,
			"recipients":      recipients,
			"locker_packages": locker,
			"ship_requests":   shipRequests,
			"bookings":        bookings,
		},
	})
}


type importRecipientRow struct {
	Name          string `json:"name"`
	DestinationID string `json:"destination_id"`
	Street        string `json:"street"`
	City          string `json:"city"`
	Phone         string `json:"phone"`
	Apt           string `json:"apt"`
}


func dataRecipientsImport(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		CSV  string               `json:"csv"`
		Rows []importRecipientRow `json:"rows"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	rows := make([]importRecipientRow, 0)
	rows = append(rows, body.Rows...)
	if strings.TrimSpace(body.CSV) != "" {
		parsedRows, err := parseRecipientCSV(body.CSV)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		rows = append(rows, parsedRows...)
	}
	if len(rows) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "rows or csv payload required"))
	}
	if len(rows) > 250 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "maximum 250 rows per import"))
	}

	type rowError struct {
		Row   int    `json:"row"`
		Error string `json:"error"`
	}
	valid := make([]importRecipientRow, 0, len(rows))
	errorsOut := make([]rowError, 0)
	seen := map[string]bool{}

	for i, row := range rows {
		n := normalizeRecipientRow(row)
		key := strings.ToLower(strings.Join([]string{n.Name, n.DestinationID, n.Street, n.City}, "|"))
		if n.Name == "" || n.DestinationID == "" || n.Street == "" || n.City == "" {
			errorsOut = append(errorsOut, rowError{Row: i + 1, Error: "name, destination_id, street, city are required"})
			continue
		}
		if seen[key] {
			errorsOut = append(errorsOut, rowError{Row: i + 1, Error: "duplicate row in payload"})
			continue
		}
		seen[key] = true
		if err := services.ValidateDestination(n.DestinationID); err != nil {
			errorsOut = append(errorsOut, rowError{Row: i + 1, Error: err.Error()})
			continue
		}
		if err := services.ValidatePhone(n.Phone); err != nil {
			errorsOut = append(errorsOut, rowError{Row: i + 1, Error: err.Error()})
			continue
		}
		valid = append(valid, n)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	imported := 0
	if len(valid) > 0 {
		tx, err := db.DB().BeginTx(c.Context(), nil)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start import transaction"))
		}
		defer func() {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				log.Printf("dataRecipientsImport rollback failed: %v", rbErr)
			}
		}()

		stmt, err := tx.PrepareContext(c.Context(), `
			INSERT INTO recipients (id, user_id, name, phone, destination_id, street, apt, city, delivery_instructions, is_default, use_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, 0, 0, ?, ?)
		`)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to prepare import statement"))
		}
		defer stmt.Close()

		for _, row := range valid {
			if _, err := stmt.ExecContext(c.Context(),
				uuid.NewString(), userID, row.Name, nullString(row.Phone), row.DestinationID,
				row.Street, nullString(row.Apt), row.City, now, now,
			); err != nil {
				errorsOut = append(errorsOut, rowError{Row: imported + 1, Error: "failed to insert row"})
				continue
			}
			imported++
		}
		if err := tx.Commit(); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit import transaction"))
		}
	}

	jobID := uuid.NewString()
	errorSummary := ""
	if len(errorsOut) > 0 {
		errorSummary = errorsOut[0].Error
	}
	_, _ = db.DB().ExecContext(c.Context(), `
		INSERT INTO data_import_jobs (
			id, user_id, import_type, status, payload_preview, total_rows, imported_rows, failed_rows, error_summary, created_at, updated_at
		) VALUES (?, ?, 'recipients_csv', ?, ?, ?, ?, ?, ?, ?, ?)
	`, jobID, userID, importStatus(imported, len(rows)), strings.TrimSpace(body.CSV), len(rows), imported, len(rows)-imported, errorSummary, now, now)
	if imported > 0 {
		_ = createUserNotification(c.Context(), userID, "Recipient import completed", "Your recipient import finished and new delivery addresses are ready.", "info", "/dashboard/recipients")
	}

	if imported == 0 && len(errorsOut) > 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": ErrorResponse{}.withCode("VALIDATION_ERROR", "No valid recipient rows found").Error,
			"data": fiber.Map{
				"job_id":        jobID,
				"total_rows":    len(rows),
				"imported_rows": imported,
				"failed_rows":   len(rows),
				"errors":        errorsOut,
			},
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"job_id":          jobID,
			"total_rows":      len(rows),
			"imported_rows":   imported,
			"failed_rows":     len(rows) - imported,
			"errors":          errorsOut,
			"multi_address":   imported > 1,
			"distinct_cities": countDistinctCities(valid),
		},
	})
}


type exportRecipientRow struct {
	ID            string
	Name          string
	DestinationID string
	City          string
	CreatedAt     string
}


type exportLockerRow struct {
	ID         string
	SenderName string
	WeightLbs  float64
	Status     string
	ArrivedAt  string
	CreatedAt  string
}


type exportShipRequestRow struct {
	ID               string
	ConfirmationCode string
	Status           string
	DestinationID    string
	ServiceType      string
	CreatedAt        string
}


type exportBookingRow struct {
	ID               string
	ConfirmationCode string
	Status           string
	DestinationID    string
	ServiceType      string
	CreatedAt        string
}


func exportUserRows(c *fiber.Ctx, userID string) ([]exportRecipientRow, []exportLockerRow, []exportShipRequestRow, []exportBookingRow, error) {
	recipients := make([]exportRecipientRow, 0)
	recRows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, name, destination_id, city, created_at
		FROM recipients
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for recRows.Next() {
		var r exportRecipientRow
		if err := recRows.Scan(&r.ID, &r.Name, &r.DestinationID, &r.City, &r.CreatedAt); err != nil {
			recRows.Close()
			return nil, nil, nil, nil, err
		}
		recipients = append(recipients, r)
	}
	if err := recRows.Err(); err != nil {
		recRows.Close()
		return nil, nil, nil, nil, err
	}
	recRows.Close()

	locker := make([]exportLockerRow, 0)
	lockRows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, COALESCE(sender_name, ''), COALESCE(weight_lbs, 0), COALESCE(status, ''), COALESCE(arrived_at, ''), created_at
		FROM locker_packages
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for lockRows.Next() {
		var r exportLockerRow
		if err := lockRows.Scan(&r.ID, &r.SenderName, &r.WeightLbs, &r.Status, &r.ArrivedAt, &r.CreatedAt); err != nil {
			lockRows.Close()
			return nil, nil, nil, nil, err
		}
		locker = append(locker, r)
	}
	if err := lockRows.Err(); err != nil {
		lockRows.Close()
		return nil, nil, nil, nil, err
	}
	lockRows.Close()

	shipRequests := make([]exportShipRequestRow, 0)
	shipRows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, confirmation_code, status, destination_id, service_type, created_at
		FROM ship_requests
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for shipRows.Next() {
		var r exportShipRequestRow
		if err := shipRows.Scan(&r.ID, &r.ConfirmationCode, &r.Status, &r.DestinationID, &r.ServiceType, &r.CreatedAt); err != nil {
			shipRows.Close()
			return nil, nil, nil, nil, err
		}
		shipRequests = append(shipRequests, r)
	}
	if err := shipRows.Err(); err != nil {
		shipRows.Close()
		return nil, nil, nil, nil, err
	}
	shipRows.Close()

	bookings := make([]exportBookingRow, 0)
	bookingRows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, confirmation_code, status, destination_id, service_type, created_at
		FROM bookings
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for bookingRows.Next() {
		var r exportBookingRow
		if err := bookingRows.Scan(&r.ID, &r.ConfirmationCode, &r.Status, &r.DestinationID, &r.ServiceType, &r.CreatedAt); err != nil {
			bookingRows.Close()
			return nil, nil, nil, nil, err
		}
		bookings = append(bookings, r)
	}
	if err := bookingRows.Err(); err != nil {
		bookingRows.Close()
		return nil, nil, nil, nil, err
	}
	bookingRows.Close()

	return recipients, locker, shipRequests, bookings, nil
}


func parseRecipientCSV(input string) ([]importRecipientRow, error) {
	r := csv.NewReader(strings.NewReader(input))
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid csv payload")
	}
	rows := make([]importRecipientRow, 0, len(records))
	for idx, rec := range records {
		if len(rec) == 0 {
			continue
		}
		if idx == 0 && strings.EqualFold(strings.TrimSpace(rec[0]), "name") {
			continue
		}
		if len(rec) < 4 {
			return nil, fmt.Errorf("csv row %d must include name,destination_id,street,city", idx+1)
		}
		row := importRecipientRow{
			Name:          rec[0],
			DestinationID: rec[1],
			Street:        rec[2],
			City:          rec[3],
		}
		if len(rec) > 4 {
			row.Phone = rec[4]
		}
		if len(rec) > 5 {
			row.Apt = rec[5]
		}
		rows = append(rows, row)
	}
	return rows, nil
}


func normalizeRecipientRow(row importRecipientRow) importRecipientRow {
	row.Name = strings.TrimSpace(row.Name)
	row.DestinationID = strings.ToLower(strings.TrimSpace(row.DestinationID))
	row.Street = strings.TrimSpace(row.Street)
	row.City = strings.TrimSpace(row.City)
	row.Phone = strings.TrimSpace(row.Phone)
	row.Apt = strings.TrimSpace(row.Apt)
	return row
}


func importStatus(imported int, total int) string {
	switch {
	case imported == 0:
		return "failed"
	case imported < total:
		return "completed_with_errors"
	default:
		return "completed"
	}
}


func countDistinctCities(rows []importRecipientRow) int {
	seen := map[string]bool{}
	for _, row := range rows {
		city := strings.ToLower(strings.TrimSpace(row.City))
		if city == "" {
			continue
		}
		seen[city] = true
	}
	return len(seen)
}


