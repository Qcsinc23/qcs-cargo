package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RegisterParcelFeatures mounts parcel-plus endpoints under /api/v1.
func RegisterParcelFeatures(g fiber.Router) {
	g.Post("/parcel/consolidation-preview", middleware.RequireAuth, parcelConsolidationPreview)
	g.Post("/parcel/assisted-purchases", middleware.RequireAuth, parcelAssistedPurchaseCreate)
	g.Get("/parcel/assisted-purchases", middleware.RequireAuth, parcelAssistedPurchaseList)
	g.Get("/parcel/photos", middleware.RequireAuth, parcelPhotosList)
	g.Post("/parcel/customs-docs", middleware.RequireAuth, parcelCustomsDocCreate)
	g.Get("/parcel/customs-docs", middleware.RequireAuth, parcelCustomsDocList)
	g.Post("/parcel/delivery-signature", middleware.RequireAuth, parcelDeliverySignatureCapture)
	g.Get("/parcel/delivery-signature/:shipRequestID", middleware.RequireAuth, parcelDeliverySignatureGet)
	g.Post("/parcel/repack-suggestion", middleware.RequireAuth, parcelRepackSuggestion)
	g.Get("/parcel/loyalty-summary", middleware.RequireAuth, parcelLoyaltySummary)
	g.Get("/data/export", middleware.RequireAuth, dataExportUser)
	g.Post("/data/recipients/import", middleware.RequireAuth, dataRecipientsImport)
}

type parcelPackageRow struct {
	ID       string
	Sender   string
	Weight   float64
	LengthIn float64
	WidthIn  float64
	HeightIn float64
	Status   string
}

func parcelConsolidationPreview(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		LockerPackageIDs []string `json:"locker_package_ids"`
		DestinationID    string   `json:"destination_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	ids := normalizeIDs(body.LockerPackageIDs)
	if len(ids) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "locker_package_ids required"))
	}

	packages, err := parcelFetchPackagesByID(c.Context(), userID, ids)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load locker packages"))
	}
	if len(packages) != len(ids) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "one or more locker_package_ids are invalid"))
	}

	preBillable, totalWeight, totalVolume := parcelWeightTotals(packages)
	postDimWeight := parcelDimWeight(totalVolume * 0.82) // simple packing-efficiency heuristic
	postBillable := math.Max(totalWeight, postDimWeight)
	savings := math.Max(0, preBillable-postBillable)

	now := time.Now().UTC()
	idsJSON, _ := json.Marshal(ids)
	if _, err := db.DB().ExecContext(c.Context(), `
		INSERT INTO parcel_consolidation_previews (
			id, user_id, package_ids_json, package_count, total_weight_lbs,
			pre_consolidation_billable_lbs, post_consolidation_billable_lbs, estimated_savings_lbs,
			destination_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		uuid.NewString(), userID, string(idsJSON), len(ids), round2(totalWeight),
		round2(preBillable), round2(postBillable), round2(savings),
		strings.TrimSpace(body.DestinationID), now.Format(time.RFC3339Nano),
	); err != nil {
		log.Printf("[parcel] consolidation preview persist failed: %v", err)
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"package_count":                    len(packages),
			"packages":                         packages,
			"total_weight_lbs":                 round2(totalWeight),
			"pre_consolidation_billable_lbs":   round2(preBillable),
			"post_consolidation_billable_lbs":  round2(postBillable),
			"estimated_savings_lbs":            round2(savings),
			"estimated_efficiency_improvement": round2(parcelRatio(preBillable, postBillable)),
		},
	})
}

func parcelAssistedPurchaseCreate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		RecipientID      string  `json:"recipient_id"`
		StoreURL         string  `json:"store_url"`
		ItemName         string  `json:"item_name"`
		Quantity         int     `json:"quantity"`
		EstimatedCostUSD float64 `json:"estimated_cost_usd"`
		Notes            string  `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.StoreURL = strings.TrimSpace(body.StoreURL)
	body.ItemName = strings.TrimSpace(body.ItemName)
	if body.StoreURL == "" || body.ItemName == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "store_url and item_name required"))
	}
	if body.Quantity <= 0 {
		body.Quantity = 1
	}
	if body.Quantity > 100 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "quantity must be between 1 and 100"))
	}

	var recipientID any
	if rid := strings.TrimSpace(body.RecipientID); rid != "" {
		var exists int
		if err := db.DB().QueryRowContext(c.Context(), `SELECT 1 FROM recipients WHERE id = ? AND user_id = ? LIMIT 1`, rid, userID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "recipient_id not found"))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate recipient"))
		}
		recipientID = rid
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.NewString()
	if _, err := db.DB().ExecContext(c.Context(), `
		INSERT INTO assisted_purchase_requests (
			id, user_id, recipient_id, store_url, item_name, quantity, estimated_cost_usd, notes, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)
	`, id, userID, recipientID, body.StoreURL, body.ItemName, body.Quantity, round2(body.EstimatedCostUSD), nullString(strings.TrimSpace(body.Notes)), now, now); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create assisted purchase request"))
	}
	_ = createUserNotification(c.Context(), userID, "Assisted purchase requested", "Your assisted purchase request was submitted for review.", "info", "/dashboard/parcel-plus")
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "status": "pending"}})
}

func parcelAssistedPurchaseList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	rows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, recipient_id, store_url, item_name, quantity, estimated_cost_usd, notes, status, created_at, updated_at
		FROM assisted_purchase_requests
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list assisted purchases"))
	}
	defer rows.Close()

	out := make([]fiber.Map, 0)
	for rows.Next() {
		var (
			id, storeURL, itemName, status, createdAt, updatedAt string
			recipientID, notes                                   sql.NullString
			quantity                                             int
			estimatedCost                                        float64
		)
		if err := rows.Scan(&id, &recipientID, &storeURL, &itemName, &quantity, &estimatedCost, &notes, &status, &createdAt, &updatedAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to read assisted purchases"))
		}
		out = append(out, fiber.Map{
			"id":                 id,
			"recipient_id":       nullableString(recipientID),
			"store_url":          storeURL,
			"item_name":          itemName,
			"quantity":           quantity,
			"estimated_cost_usd": round2(estimatedCost),
			"notes":              nullableString(notes),
			"status":             status,
			"created_at":         createdAt,
			"updated_at":         updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list assisted purchases"))
	}
	return c.JSON(fiber.Map{"data": out})
}

func parcelPhotosList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	lockerPackageID := strings.TrimSpace(c.Query("locker_package_id"))

	query := `
		SELECT
			lp.id,
			COALESCE(lp.sender_name, ''),
			lp.arrival_photo_url,
			lp.arrived_at,
			lph.id,
			lph.photo_url,
			lph.photo_type,
			lph.created_at
		FROM locker_packages lp
		LEFT JOIN locker_photos lph ON lph.locker_package_id = lp.id
		WHERE lp.user_id = ?
	`
	args := []any{userID}
	if lockerPackageID != "" {
		query += ` AND lp.id = ?`
		args = append(args, lockerPackageID)
	}
	query += ` ORDER BY lp.arrived_at DESC, lph.created_at ASC`

	rows, err := db.DB().QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package photos"))
	}
	defer rows.Close()

	type photoRow struct {
		URL       string `json:"photo_url"`
		PhotoType string `json:"photo_type"`
		CreatedAt any    `json:"created_at,omitempty"`
		Source    string `json:"source"`
	}
	out := make([]fiber.Map, 0)
	byPackage := map[string]int{}

	for rows.Next() {
		var (
			pkgID, sender string
			arrivalURL    sql.NullString
			arrivedAt     sql.NullString
			photoID       sql.NullString
			photoURL      sql.NullString
			photoType     sql.NullString
			photoCreated  sql.NullString
		)
		if err := rows.Scan(&pkgID, &sender, &arrivalURL, &arrivedAt, &photoID, &photoURL, &photoType, &photoCreated); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to parse package photos"))
		}

		idx, exists := byPackage[pkgID]
		if !exists {
			entry := fiber.Map{
				"locker_package_id": pkgID,
				"sender_name":       sender,
				"arrived_at":        nullableString(arrivedAt),
				"photos":            []photoRow{},
			}
			out = append(out, entry)
			idx = len(out) - 1
			byPackage[pkgID] = idx
			if arrivalURL.Valid && strings.TrimSpace(arrivalURL.String) != "" {
				out[idx]["photos"] = append(out[idx]["photos"].([]photoRow), photoRow{
					URL:       arrivalURL.String,
					PhotoType: "arrival",
					CreatedAt: nullableString(arrivedAt),
					Source:    "locker_packages.arrival_photo_url",
				})
			}
		}

		if photoID.Valid && photoURL.Valid && strings.TrimSpace(photoURL.String) != "" {
			out[idx]["photos"] = append(out[idx]["photos"].([]photoRow), photoRow{
				URL:       photoURL.String,
				PhotoType: firstNonEmpty(photoType.String, "detail"),
				CreatedAt: nullableString(photoCreated),
				Source:    "locker_photos",
			})
		}
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package photos"))
	}
	return c.JSON(fiber.Map{"data": out})
}

func parcelCustomsDocCreate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		ShipRequestID   string `json:"ship_request_id"`
		LockerPackageID string `json:"locker_package_id"`
		DocType         string `json:"doc_type"`
		FileName        string `json:"file_name"`
		FileURL         string `json:"file_url"`
		MimeType        string `json:"mime_type"`
		SizeBytes       int64  `json:"size_bytes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.DocType = strings.TrimSpace(body.DocType)
	body.FileName = strings.TrimSpace(body.FileName)
	body.FileURL = strings.TrimSpace(body.FileURL)
	if body.DocType == "" || body.FileName == "" || body.FileURL == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "doc_type, file_name, and file_url required"))
	}
	if strings.TrimSpace(body.ShipRequestID) == "" && strings.TrimSpace(body.LockerPackageID) == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "ship_request_id or locker_package_id required"))
	}
	// Pass 2 audit fix H-5: tighten validation on the user-supplied file_url
	// and file_name. Without this, a user could store javascript: URLs or
	// HTML payloads that would later be reflected to staff-facing UIs.
	if err := services.ValidateUploadedFileURL(body.FileURL); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	if err := services.ValidateFileName(body.FileName); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	if !isAllowedCustomsDocType(body.DocType) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid doc_type"))
	}
	if body.SizeBytes < 0 || body.SizeBytes > 25*1024*1024 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "size_bytes must be between 0 and 25MB"))
	}

	if sid := strings.TrimSpace(body.ShipRequestID); sid != "" {
		var exists int
		if err := db.DB().QueryRowContext(c.Context(), `SELECT 1 FROM ship_requests WHERE id = ? AND user_id = ? LIMIT 1`, sid, userID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "ship_request_id not found"))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate ship_request_id"))
		}
	}
	if lid := strings.TrimSpace(body.LockerPackageID); lid != "" {
		var exists int
		if err := db.DB().QueryRowContext(c.Context(), `SELECT 1 FROM locker_packages WHERE id = ? AND user_id = ? LIMIT 1`, lid, userID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "locker_package_id not found"))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate locker_package_id"))
		}
	}

	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.DB().ExecContext(c.Context(), `
		INSERT INTO customs_preclearance_docs (
			id, user_id, ship_request_id, locker_package_id, doc_type, file_name, file_url, mime_type, size_bytes, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'uploaded', ?)
	`, id, userID, nullString(strings.TrimSpace(body.ShipRequestID)), nullString(strings.TrimSpace(body.LockerPackageID)),
		body.DocType, body.FileName, body.FileURL, nullString(strings.TrimSpace(body.MimeType)), body.SizeBytes, now); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save customs document metadata"))
	}
	_ = createUserNotification(c.Context(), userID, "Customs document uploaded", "Your pre-clearance document was saved.", "info", "/dashboard/customs")

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "status": "uploaded"}})
}

func parcelCustomsDocList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	shipRequestID := strings.TrimSpace(c.Query("ship_request_id"))
	lockerPackageID := strings.TrimSpace(c.Query("locker_package_id"))

	query := `
		SELECT id, ship_request_id, locker_package_id, doc_type, file_name, file_url, mime_type, size_bytes, status, created_at
		FROM customs_preclearance_docs
		WHERE user_id = ?
	`
	args := []any{userID}
	if shipRequestID != "" {
		query += ` AND ship_request_id = ?`
		args = append(args, shipRequestID)
	}
	if lockerPackageID != "" {
		query += ` AND locker_package_id = ?`
		args = append(args, lockerPackageID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := db.DB().QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list customs document metadata"))
	}
	defer rows.Close()

	out := make([]fiber.Map, 0)
	for rows.Next() {
		var (
			id, docType, fileName, fileURL, status, createdAt string
			shipID, lockerID, mimeType                        sql.NullString
			sizeBytes                                         int64
		)
		if err := rows.Scan(&id, &shipID, &lockerID, &docType, &fileName, &fileURL, &mimeType, &sizeBytes, &status, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to parse customs document metadata"))
		}
		out = append(out, fiber.Map{
			"id":                id,
			"ship_request_id":   nullableString(shipID),
			"locker_package_id": nullableString(lockerID),
			"doc_type":          docType,
			"file_name":         fileName,
			"file_url":          fileURL,
			"mime_type":         nullableString(mimeType),
			"size_bytes":        sizeBytes,
			"status":            status,
			"created_at":        createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list customs document metadata"))
	}
	return c.JSON(fiber.Map{"data": out})
}

func parcelDeliverySignatureCapture(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		ShipRequestID string `json:"ship_request_id"`
		SignerName    string `json:"signer_name"`
		SignatureData string `json:"signature_data"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.ShipRequestID = strings.TrimSpace(body.ShipRequestID)
	body.SignerName = strings.TrimSpace(body.SignerName)
	body.SignatureData = strings.TrimSpace(body.SignatureData)
	if body.ShipRequestID == "" || body.SignerName == "" || body.SignatureData == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "ship_request_id, signer_name, and signature_data required"))
	}
	// Pass 2 audit fix M-10: bound payload sizes and validate the data URL
	// shape so callers cannot pad the signatures table or smuggle non-image
	// content.
	if cleaned, err := services.ValidateName(body.SignerName); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "signer_name "+err.Error()))
	} else {
		body.SignerName = cleaned
	}
	const maxSignatureBytes = 256 * 1024 // 256 KB
	if len(body.SignatureData) > maxSignatureBytes {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "signature_data exceeds 256KB"))
	}
	if !strings.HasPrefix(body.SignatureData, "data:image/png;base64,") &&
		!strings.HasPrefix(body.SignatureData, "data:image/jpeg;base64,") &&
		!strings.HasPrefix(body.SignatureData, "data:image/webp;base64,") {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "signature_data must be a base64 PNG/JPEG/WEBP data URL"))
	}

	// Freeze signatures once the ship request is in a terminal delivered/closed
	// state so a customer cannot rewrite the recorded delivery proof after
	// the fact. We accept first-time captures while still allowing legitimate
	// corrections during delivery.
	var existsRow struct {
		Status string
		PrevID sql.NullString
	}
	if err := db.DB().QueryRowContext(c.Context(), `
		SELECT s.status,
		       (SELECT id FROM delivery_signatures WHERE user_id = ? AND ship_request_id = ? LIMIT 1)
		FROM ship_requests s
		WHERE s.id = ? AND s.user_id = ? LIMIT 1
	`, userID, body.ShipRequestID, body.ShipRequestID, userID).Scan(&existsRow.Status, &existsRow.PrevID); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate ship request"))
	}
	if existsRow.PrevID.Valid && (strings.EqualFold(existsRow.Status, "delivered") || strings.EqualFold(existsRow.Status, "closed") || strings.EqualFold(existsRow.Status, "completed")) {
		return c.Status(409).JSON(ErrorResponse{}.withCode("SIGNATURE_LOCKED", "delivery signature can no longer be modified"))
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.NewString()
	if _, err := db.DB().ExecContext(c.Context(), `
		INSERT INTO delivery_signatures (
			id, user_id, ship_request_id, signer_name, signature_data, captured_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, ship_request_id)
		DO UPDATE SET
			signer_name = excluded.signer_name,
			signature_data = excluded.signature_data,
			captured_at = excluded.captured_at,
			updated_at = excluded.updated_at
	`, id, userID, body.ShipRequestID, body.SignerName, body.SignatureData, now, now, now); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to capture delivery signature"))
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"ship_request_id": body.ShipRequestID,
			"signer_name":     body.SignerName,
			"captured_at":     now,
		},
	})
}

func parcelDeliverySignatureGet(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	shipRequestID := strings.TrimSpace(c.Params("shipRequestID"))
	if shipRequestID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "shipRequestID required"))
	}

	var (
		id, signerName, signatureData, capturedAt, updatedAt string
	)
	err := db.DB().QueryRowContext(c.Context(), `
		SELECT id, signer_name, signature_data, captured_at, updated_at
		FROM delivery_signatures
		WHERE user_id = ? AND ship_request_id = ?
	`, userID, shipRequestID).Scan(&id, &signerName, &signatureData, &capturedAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Signature not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load signature"))
	}

	return c.JSON(fiber.Map{"data": fiber.Map{
		"id":              id,
		"ship_request_id": shipRequestID,
		"signer_name":     signerName,
		"signature_data":  signatureData,
		"captured_at":     capturedAt,
		"updated_at":      updatedAt,
	}})
}

func parcelRepackSuggestion(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		LockerPackageIDs []string `json:"locker_package_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	ids := normalizeIDs(body.LockerPackageIDs)
	if len(ids) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "locker_package_ids required"))
	}
	packages, err := parcelFetchPackagesByID(c.Context(), userID, ids)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load locker packages"))
	}
	if len(packages) != len(ids) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "one or more locker_package_ids are invalid"))
	}

	preBillable, totalWeight, totalVolume := parcelWeightTotals(packages)
	bulkyCount := 0
	for _, pkg := range packages {
		dim := parcelDimWeight(pkg.LengthIn * pkg.WidthIn * pkg.HeightIn)
		if dim > (pkg.Weight * 1.25) {
			bulkyCount++
		}
	}

	reductionPct := 8.0
	if len(packages) >= 2 {
		reductionPct += 8
	}
	reductionPct += float64(bulkyCount) * 4
	if reductionPct > 35 {
		reductionPct = 35
	}
	postVolume := totalVolume * (1.0 - (reductionPct / 100.0))
	postBillable := math.Max(totalWeight, parcelDimWeight(postVolume))
	savings := math.Max(0, preBillable-postBillable)

	action := "keep_current_packaging"
	if savings >= 0.5 {
		action = "repack_recommended"
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"package_count":               len(packages),
			"suggested_action":            action,
			"estimated_volume_reduction":  round2(reductionPct),
			"estimated_billable_before":   round2(preBillable),
			"estimated_billable_after":    round2(postBillable),
			"estimated_billable_savings":  round2(savings),
			"bulky_package_count":         bulkyCount,
			"optimization_confidence_pct": round2(65.0 + math.Min(30, float64(len(packages))*5)),
		},
	})
}

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

func parcelLoyaltySummary(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	windowStart := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339Nano)

	var (
		currentPoints int
		earned30d     int
		spent30d      int
	)
	err := db.DB().QueryRowContext(c.Context(), `
		SELECT
			COALESCE(SUM(points_delta), 0),
			COALESCE(SUM(CASE WHEN points_delta > 0 AND created_at >= ? THEN points_delta ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN points_delta < 0 AND created_at >= ? THEN -points_delta ELSE 0 END), 0)
		FROM loyalty_ledger
		WHERE user_id = ?
	`, windowStart, windowStart, userID).Scan(&currentPoints, &earned30d, &spent30d)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load loyalty summary"))
	}

	rows, err := db.DB().QueryContext(c.Context(), `
		SELECT id, points_delta, reason, resource_type, resource_id, created_at
		FROM loyalty_ledger
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load loyalty entries"))
	}
	defer rows.Close()

	recent := make([]fiber.Map, 0)
	for rows.Next() {
		var (
			id, reason, createdAt string
			pointsDelta           int
			resourceType          sql.NullString
			resourceID            sql.NullString
		)
		if err := rows.Scan(&id, &pointsDelta, &reason, &resourceType, &resourceID, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to parse loyalty entries"))
		}
		recent = append(recent, fiber.Map{
			"id":            id,
			"points_delta":  pointsDelta,
			"reason":        reason,
			"resource_type": nullableString(resourceType),
			"resource_id":   nullableString(resourceID),
			"created_at":    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load loyalty entries"))
	}

	tier := "basic"
	nextTierAt := 500
	if currentPoints >= 1000 {
		tier = "gold"
		nextTierAt = currentPoints
	} else if currentPoints >= 500 {
		tier = "silver"
		nextTierAt = 1000
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"current_points": currentPoints,
			"earned_30d":     earned30d,
			"spent_30d":      spent30d,
			"tier":           tier,
			"next_tier_at":   nextTierAt,
			"recent":         recent,
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

func normalizeIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func parcelFetchPackagesByID(ctx context.Context, userID string, ids []string) ([]parcelPackageRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, userID)
	for _, id := range ids {
		args = append(args, id)
	}
	query := `
		SELECT
			id,
			COALESCE(sender_name, ''),
			COALESCE(weight_lbs, 0),
			COALESCE(length_in, 0),
			COALESCE(width_in, 0),
			COALESCE(height_in, 0),
			COALESCE(status, '')
		FROM locker_packages
		WHERE user_id = ? AND id IN (` + placeholders + `)
	`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]parcelPackageRow, 0, len(ids))
	for rows.Next() {
		var row parcelPackageRow
		if err := rows.Scan(&row.ID, &row.Sender, &row.Weight, &row.LengthIn, &row.WidthIn, &row.HeightIn, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parcelWeightTotals(packages []parcelPackageRow) (preBillable float64, totalWeight float64, totalVolume float64) {
	for _, pkg := range packages {
		dimWeight := parcelDimWeight(pkg.LengthIn * pkg.WidthIn * pkg.HeightIn)
		preBillable += math.Max(pkg.Weight, dimWeight)
		totalWeight += pkg.Weight
		totalVolume += pkg.LengthIn * pkg.WidthIn * pkg.HeightIn
	}
	return preBillable, totalWeight, totalVolume
}

func parcelDimWeight(volumeCubicIn float64) float64 {
	if volumeCubicIn <= 0 {
		return 0
	}
	return volumeCubicIn / 166.0
}

func parcelRatio(before, after float64) float64 {
	if before <= 0 {
		return 0
	}
	return math.Max(0, (before-after)/before*100.0)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
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

func nullableString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

// isAllowedCustomsDocType is the closed enum of customs pre-clearance document
// types we accept. Pass 2 audit fix H-5.
func isAllowedCustomsDocType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "invoice", "packing_list", "id_proof", "permit", "certificate", "other":
		return true
	}
	return false
}
