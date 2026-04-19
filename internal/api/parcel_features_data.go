package api

// parcel_features_data.go holds /data/export and /data/recipients/import
// (plus their CSV / row helpers) split out from parcel_features.go in
// Phase 3.3 (QAL-001). Routes remain registered by RegisterParcelFeatures
// in parcel_features.go.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// dataExportUser implements GDPR Article 15 (right of access / data
// portability). Pass 2.5 finding HIGH-07 expanded the response to cover
// every user-scoped table; the JSON envelope is additive only so older
// clients keep working. API key material, MFA secrets, and push crypto
// keys are deliberately excluded — only metadata is returned for those.
func dataExportUser(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	format := strings.ToLower(strings.TrimSpace(c.Query("format", "json")))

	exp, err := exportUserRows(c.Context(), userID)
	if err != nil {
		log.Printf("[data/export] aggregate failed user=%s: %v", userID, err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to prepare export"))
	}

	if format == "csv" {
		buf := &bytes.Buffer{}
		w := csv.NewWriter(buf)
		_ = w.Write([]string{"type", "id", "status", "created_at", "field_a", "field_b", "field_c"})
		writeUserExportCSV(w, userID, exp)
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
			"generated_at":                   time.Now().UTC().Format(time.RFC3339Nano),
			"user_id":                        userID,
			"user_profile":                   exp.Profile,
			"recipients":                     exp.Recipients,
			"locker_packages":                exp.LockerPackages,
			"ship_requests":                  exp.ShipRequests,
			"bookings":                       exp.Bookings,
			"customs_preclearance_docs":      exp.CustomsDocs,
			"delivery_signatures":            exp.DeliverySignatures,
			"locker_photos":                  exp.LockerPhotos,
			"assisted_purchase_requests":     exp.AssistedPurchases,
			"notification_prefs":             exp.NotificationPrefs,
			"cookie_consents":                exp.CookieConsents,
			"loyalty_ledger":                 exp.LoyaltyLedger,
			"user_mfa":                       exp.UserMFA,
			"api_keys":                       exp.APIKeys,
			"service_requests":               exp.ServiceRequests,
			"storage_fees":                   exp.StorageFees,
			"invoices":                       exp.Invoices,
			"templates":                      exp.Templates,
			"inbound_tracking":               exp.InboundTracking,
			"in_app_notifications":           exp.InAppNotifications,
			"push_subscriptions":             exp.PushSubscriptions,
			"parcel_consolidation_previews":  exp.ConsolidationPreviews,
			"data_import_jobs":               exp.DataImportJobs,
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


// userExportData aggregates every user-scoped collection returned by
// /data/export. Add a new field here when a new user-scoped table lands;
// the drift-guard test in parcel_features_data_test.go fails until the
// JSON envelope and CSV emitter are updated.
type userExportData struct {
	Profile               *exportUserProfile               `json:"user_profile"`
	Recipients            []exportRecipientRow             `json:"recipients"`
	LockerPackages        []exportLockerRow                `json:"locker_packages"`
	ShipRequests          []exportShipRequestRow           `json:"ship_requests"`
	Bookings              []exportBookingRow               `json:"bookings"`
	CustomsDocs           []exportCustomsDocRow            `json:"customs_preclearance_docs"`
	DeliverySignatures    []exportDeliverySignatureRow     `json:"delivery_signatures"`
	LockerPhotos          []exportLockerPhotoRow           `json:"locker_photos"`
	AssistedPurchases     []exportAssistedPurchaseRow      `json:"assisted_purchase_requests"`
	NotificationPrefs     *exportNotificationPrefsRow      `json:"notification_prefs"`
	CookieConsents        *exportCookieConsentRow          `json:"cookie_consents"`
	LoyaltyLedger         []exportLoyaltyLedgerRow         `json:"loyalty_ledger"`
	UserMFA               *exportUserMFARow                `json:"user_mfa"`
	APIKeys               []exportAPIKeyMetaRow            `json:"api_keys"`
	ServiceRequests       []exportServiceRequestRow        `json:"service_requests"`
	StorageFees           []exportStorageFeeRow            `json:"storage_fees"`
	Invoices              []exportInvoiceRow               `json:"invoices"`
	Templates             []exportTemplateRow              `json:"templates"`
	InboundTracking       []exportInboundTrackingRow       `json:"inbound_tracking"`
	InAppNotifications    []exportInAppNotificationRow     `json:"in_app_notifications"`
	PushSubscriptions     []exportPushSubscriptionMetaRow  `json:"push_subscriptions"`
	ConsolidationPreviews []exportConsolidationPreviewRow  `json:"parcel_consolidation_previews"`
	DataImportJobs        []exportDataImportJobRow         `json:"data_import_jobs"`
}

type exportUserProfile struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	Phone              *string `json:"phone"`
	Role               string  `json:"role"`
	AvatarURL          *string `json:"avatar_url"`
	SuiteCode          *string `json:"suite_code"`
	AddressStreet      *string `json:"address_street"`
	AddressCity        *string `json:"address_city"`
	AddressState       *string `json:"address_state"`
	AddressZip         *string `json:"address_zip"`
	FreeStorageDays    int     `json:"free_storage_days"`
	EmailVerified      bool    `json:"email_verified"`
	Status             string  `json:"status"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type exportRecipientRow struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Phone                *string `json:"phone"`
	DestinationID        string  `json:"destination_id"`
	Street               string  `json:"street"`
	Apt                  *string `json:"apt"`
	City                 string  `json:"city"`
	DeliveryInstructions *string `json:"delivery_instructions"`
	IsDefault            bool    `json:"is_default"`
	UseCount             int     `json:"use_count"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type exportLockerRow struct {
	ID                   string   `json:"id"`
	SuiteCode            string   `json:"suite_code"`
	SenderName           string   `json:"sender_name"`
	WeightLbs            float64  `json:"weight_lbs"`
	LengthIn             *float64 `json:"length_in"`
	WidthIn              *float64 `json:"width_in"`
	HeightIn             *float64 `json:"height_in"`
	Condition            *string  `json:"condition"`
	Status               string   `json:"status"`
	StorageBay           *string  `json:"storage_bay"`
	ArrivedAt            *string  `json:"arrived_at"`
	FreeStorageExpiresAt *string  `json:"free_storage_expires_at"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

type exportShipRequestItemRow struct {
	ID                      string   `json:"id"`
	LockerPackageID         string   `json:"locker_package_id"`
	CustomsDescription      *string  `json:"customs_description"`
	CustomsValue            *float64 `json:"customs_value"`
	CustomsQuantity         *int     `json:"customs_quantity"`
	CustomsHSCode           *string  `json:"customs_hs_code"`
	CustomsCountryOfOrigin  *string  `json:"customs_country_of_origin"`
	CustomsWeightLbs        *float64 `json:"customs_weight_lbs"`
}

type exportShipRequestRow struct {
	ID                  string                     `json:"id"`
	ConfirmationCode    string                     `json:"confirmation_code"`
	Status              string                     `json:"status"`
	DestinationID       string                     `json:"destination_id"`
	RecipientID         *string                    `json:"recipient_id"`
	ServiceType         string                     `json:"service_type"`
	Consolidate         bool                       `json:"consolidate"`
	SpecialInstructions *string                    `json:"special_instructions"`
	Subtotal            float64                    `json:"subtotal"`
	ServiceFees         float64                    `json:"service_fees"`
	Insurance           float64                    `json:"insurance"`
	Discount            float64                    `json:"discount"`
	Total               float64                    `json:"total"`
	PaymentStatus       *string                    `json:"payment_status"`
	CustomsStatus       *string                    `json:"customs_status"`
	CreatedAt           string                     `json:"created_at"`
	UpdatedAt           string                     `json:"updated_at"`
	Items               []exportShipRequestItemRow `json:"items"`
}

type exportBookingRow struct {
	ID                  string  `json:"id"`
	ConfirmationCode    string  `json:"confirmation_code"`
	Status              string  `json:"status"`
	ServiceType         string  `json:"service_type"`
	DestinationID       string  `json:"destination_id"`
	RecipientID         *string `json:"recipient_id"`
	ScheduledDate       string  `json:"scheduled_date"`
	TimeSlot            string  `json:"time_slot"`
	SpecialInstructions *string `json:"special_instructions"`
	WeightLbs           float64 `json:"weight_lbs"`
	LengthIn            float64 `json:"length_in"`
	WidthIn             float64 `json:"width_in"`
	HeightIn            float64 `json:"height_in"`
	ValueUSD            float64 `json:"value_usd"`
	AddInsurance        bool    `json:"add_insurance"`
	Subtotal            float64 `json:"subtotal"`
	Discount            float64 `json:"discount"`
	Insurance           float64 `json:"insurance"`
	Total               float64 `json:"total"`
	PaymentStatus       *string `json:"payment_status"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

type exportCustomsDocRow struct {
	ID              string  `json:"id"`
	ShipRequestID   *string `json:"ship_request_id"`
	LockerPackageID *string `json:"locker_package_id"`
	DocType         string  `json:"doc_type"`
	FileName        string  `json:"file_name"`
	MimeType        *string `json:"mime_type"`
	SizeBytes       int64   `json:"size_bytes"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
}

type exportDeliverySignatureRow struct {
	ID            string `json:"id"`
	ShipRequestID string `json:"ship_request_id"`
	SignerName    string `json:"signer_name"`
	CapturedAt    string `json:"captured_at"`
	CreatedAt     string `json:"created_at"`
}

// exportLockerPhotoRow exposes photo metadata only (URL + type), not the
// raw image bytes. The user already owns the image at the URL via the
// signed-storage flow.
type exportLockerPhotoRow struct {
	ID              string  `json:"id"`
	LockerPackageID string  `json:"locker_package_id"`
	PhotoURL        string  `json:"photo_url"`
	PhotoType       string  `json:"photo_type"`
	TakenBy         *string `json:"taken_by"`
	CreatedAt       string  `json:"created_at"`
}

type exportAssistedPurchaseRow struct {
	ID               string  `json:"id"`
	RecipientID      *string `json:"recipient_id"`
	StoreURL         string  `json:"store_url"`
	ItemName         string  `json:"item_name"`
	Quantity         int     `json:"quantity"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Notes            *string `json:"notes"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type exportNotificationPrefsRow struct {
	EmailEnabled       bool   `json:"email_enabled"`
	SMSEnabled         bool   `json:"sms_enabled"`
	PushEnabled        bool   `json:"push_enabled"`
	OnPackageArrived   bool   `json:"on_package_arrived"`
	OnStorageExpiry    bool   `json:"on_storage_expiry"`
	OnShipUpdates      bool   `json:"on_ship_updates"`
	OnInboundUpdates   bool   `json:"on_inbound_updates"`
	DailyDigest        string `json:"daily_digest"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type exportCookieConsentRow struct {
	ConsentVersion string  `json:"consent_version"`
	Necessary      bool    `json:"necessary"`
	Functional     bool    `json:"functional"`
	Analytics      bool    `json:"analytics"`
	Marketing      bool    `json:"marketing"`
	Source         *string `json:"source"`
	IPAddress      *string `json:"ip_address"`
	UserAgent      *string `json:"user_agent"`
	ConsentedAt    string  `json:"consented_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type exportLoyaltyLedgerRow struct {
	ID           string  `json:"id"`
	PointsDelta  int     `json:"points_delta"`
	Reason       string  `json:"reason"`
	ResourceType *string `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	CreatedAt    string  `json:"created_at"`
}

// exportUserMFARow exposes only that MFA is enrolled and which method.
// The TOTP/email-OTP secret material is NEVER included.
type exportUserMFARow struct {
	Method          string  `json:"method"`
	Enabled         bool    `json:"enabled"`
	LastVerifiedAt  *string `json:"last_verified_at"`
	LastChallengeAt *string `json:"last_challenge_at"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// exportAPIKeyMetaRow returns API key metadata only. The key_hash column
// (and obviously the raw secret which the server never stores) are
// deliberately excluded — possessing the prefix alone cannot authenticate.
type exportAPIKeyMetaRow struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	Scopes     string  `json:"scopes_json"`
	LastUsedAt *string `json:"last_used_at"`
	ExpiresAt  *string `json:"expires_at"`
	RevokedAt  *string `json:"revoked_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type exportServiceRequestRow struct {
	ID              string  `json:"id"`
	LockerPackageID string  `json:"locker_package_id"`
	ServiceType     string  `json:"service_type"`
	Status          string  `json:"status"`
	Notes           *string `json:"notes"`
	Price           float64 `json:"price"`
	CreatedAt       string  `json:"created_at"`
	CompletedAt     *string `json:"completed_at"`
}

type exportStorageFeeRow struct {
	ID              string  `json:"id"`
	LockerPackageID string  `json:"locker_package_id"`
	FeeDate         string  `json:"fee_date"`
	Amount          float64 `json:"amount"`
	Invoiced        bool    `json:"invoiced"`
	InvoiceID       *string `json:"invoice_id"`
	CreatedAt       string  `json:"created_at"`
}

type exportInvoiceItemRow struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Total       float64 `json:"total"`
}

type exportInvoiceRow struct {
	ID            string                 `json:"id"`
	BookingID     *string                `json:"booking_id"`
	ShipRequestID *string                `json:"ship_request_id"`
	InvoiceNumber string                 `json:"invoice_number"`
	Subtotal      float64                `json:"subtotal"`
	Tax           float64                `json:"tax"`
	Total         float64                `json:"total"`
	Status        string                 `json:"status"`
	DueDate       *string                `json:"due_date"`
	PaidAt        *string                `json:"paid_at"`
	Notes         *string                `json:"notes"`
	CreatedAt     string                 `json:"created_at"`
	Items         []exportInvoiceItemRow `json:"items"`
}

type exportTemplateRow struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ServiceType   string  `json:"service_type"`
	DestinationID string  `json:"destination_id"`
	RecipientID   *string `json:"recipient_id"`
	UseCount      int     `json:"use_count"`
	CreatedAt     string  `json:"created_at"`
}

type exportInboundTrackingRow struct {
	ID              string  `json:"id"`
	Carrier         string  `json:"carrier"`
	TrackingNumber  string  `json:"tracking_number"`
	RetailerName    *string `json:"retailer_name"`
	ExpectedItems   *string `json:"expected_items"`
	Status          string  `json:"status"`
	LockerPackageID *string `json:"locker_package_id"`
	LastCheckedAt   *string `json:"last_checked_at"`
	CreatedAt       string  `json:"created_at"`
}

type exportInAppNotificationRow struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Level     string  `json:"level"`
	LinkURL   *string `json:"link_url"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

// exportPushSubscriptionMetaRow exposes only the subscription endpoint
// (which the user already shared with us when subscribing). The p256dh
// and auth crypto material are NEVER included — they are server-side
// secrets used to encrypt push payloads.
type exportPushSubscriptionMetaRow struct {
	ID        string `json:"id"`
	Endpoint  string `json:"endpoint"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type exportConsolidationPreviewRow struct {
	ID                          string   `json:"id"`
	PackageIDsJSON              string   `json:"package_ids_json"`
	PackageCount                int      `json:"package_count"`
	TotalWeightLbs              float64  `json:"total_weight_lbs"`
	PreConsolidationBillableLbs float64  `json:"pre_consolidation_billable_lbs"`
	PostConsolidationBillable   float64  `json:"post_consolidation_billable_lbs"`
	EstimatedSavingsLbs         float64  `json:"estimated_savings_lbs"`
	DestinationID               *string  `json:"destination_id"`
	CreatedAt                   string   `json:"created_at"`
}

type exportDataImportJobRow struct {
	ID             string  `json:"id"`
	ImportType     string  `json:"import_type"`
	Status         string  `json:"status"`
	TotalRows      int     `json:"total_rows"`
	ImportedRows   int     `json:"imported_rows"`
	FailedRows     int     `json:"failed_rows"`
	ErrorSummary   *string `json:"error_summary"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// exportUserRows aggregates every user-scoped collection backing the
// /data/export response. New tables MUST be added here AND have a key
// in the JSON envelope in dataExportUser AND be reflected in the
// drift-guard test (parcel_features_data_test.go).
func exportUserRows(ctx context.Context, userID string) (*userExportData, error) {
	exp := &userExportData{
		Recipients:            []exportRecipientRow{},
		LockerPackages:        []exportLockerRow{},
		ShipRequests:          []exportShipRequestRow{},
		Bookings:              []exportBookingRow{},
		CustomsDocs:           []exportCustomsDocRow{},
		DeliverySignatures:    []exportDeliverySignatureRow{},
		LockerPhotos:          []exportLockerPhotoRow{},
		AssistedPurchases:     []exportAssistedPurchaseRow{},
		LoyaltyLedger:         []exportLoyaltyLedgerRow{},
		APIKeys:               []exportAPIKeyMetaRow{},
		ServiceRequests:       []exportServiceRequestRow{},
		StorageFees:           []exportStorageFeeRow{},
		Invoices:              []exportInvoiceRow{},
		Templates:             []exportTemplateRow{},
		InboundTracking:       []exportInboundTrackingRow{},
		InAppNotifications:    []exportInAppNotificationRow{},
		PushSubscriptions:     []exportPushSubscriptionMetaRow{},
		ConsolidationPreviews: []exportConsolidationPreviewRow{},
		DataImportJobs:        []exportDataImportJobRow{},
	}

	type loader struct {
		name string
		fn   func() error
	}
	loaders := []loader{
		{"user_profile", func() (err error) { exp.Profile, err = exportUserProfileRow(ctx, userID); return }},
		{"recipients", func() (err error) { exp.Recipients, err = exportRecipientRows(ctx, userID); return }},
		{"locker_packages", func() (err error) { exp.LockerPackages, err = exportLockerPackageRows(ctx, userID); return }},
		{"ship_requests", func() (err error) { exp.ShipRequests, err = exportShipRequestRows(ctx, userID); return }},
		{"bookings", func() (err error) { exp.Bookings, err = exportBookingRows(ctx, userID); return }},
		{"customs_preclearance_docs", func() (err error) { exp.CustomsDocs, err = exportCustomsDocRows(ctx, userID); return }},
		{"delivery_signatures", func() (err error) { exp.DeliverySignatures, err = exportDeliverySignatureRows(ctx, userID); return }},
		{"locker_photos", func() (err error) { exp.LockerPhotos, err = exportLockerPhotoRows(ctx, userID); return }},
		{"assisted_purchase_requests", func() (err error) { exp.AssistedPurchases, err = exportAssistedPurchaseRows(ctx, userID); return }},
		{"notification_prefs", func() (err error) { exp.NotificationPrefs, err = exportNotificationPrefsRowFor(ctx, userID); return }},
		{"cookie_consents", func() (err error) { exp.CookieConsents, err = exportCookieConsentRowFor(ctx, userID); return }},
		{"loyalty_ledger", func() (err error) { exp.LoyaltyLedger, err = exportLoyaltyLedgerRows(ctx, userID); return }},
		{"user_mfa", func() (err error) { exp.UserMFA, err = exportUserMFARowFor(ctx, userID); return }},
		{"api_keys", func() (err error) { exp.APIKeys, err = exportAPIKeyMetaRows(ctx, userID); return }},
		{"service_requests", func() (err error) { exp.ServiceRequests, err = exportServiceRequestRows(ctx, userID); return }},
		{"storage_fees", func() (err error) { exp.StorageFees, err = exportStorageFeeRows(ctx, userID); return }},
		{"invoices", func() (err error) { exp.Invoices, err = exportInvoiceRows(ctx, userID); return }},
		{"templates", func() (err error) { exp.Templates, err = exportTemplateRows(ctx, userID); return }},
		{"inbound_tracking", func() (err error) { exp.InboundTracking, err = exportInboundTrackingRows(ctx, userID); return }},
		{"in_app_notifications", func() (err error) { exp.InAppNotifications, err = exportInAppNotificationRows(ctx, userID); return }},
		{"push_subscriptions", func() (err error) { exp.PushSubscriptions, err = exportPushSubscriptionMetaRows(ctx, userID); return }},
		{"parcel_consolidation_previews", func() (err error) { exp.ConsolidationPreviews, err = exportConsolidationPreviewRows(ctx, userID); return }},
		{"data_import_jobs", func() (err error) { exp.DataImportJobs, err = exportDataImportJobRows(ctx, userID); return }},
	}
	for _, l := range loaders {
		if err := l.fn(); err != nil {
			return nil, fmt.Errorf("export %s: %w", l.name, err)
		}
	}
	return exp, nil
}

func exportUserProfileRow(ctx context.Context, userID string) (*exportUserProfile, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT id, name, email, phone, role, avatar_url, suite_code,
		       address_street, address_city, address_state, address_zip,
		       free_storage_days, email_verified, status,
		       created_at, updated_at
		FROM users WHERE id = ?
	`, userID)
	var (
		p                                                                      exportUserProfile
		phone, avatarURL, suiteCode, street, city, state, zip                  sql.NullString
		emailVerified                                                          int
	)
	if err := row.Scan(&p.ID, &p.Name, &p.Email, &phone, &p.Role, &avatarURL, &suiteCode,
		&street, &city, &state, &zip,
		&p.FreeStorageDays, &emailVerified, &p.Status,
		&p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.Phone = nullStringPointer(phone)
	p.AvatarURL = nullStringPointer(avatarURL)
	p.SuiteCode = nullStringPointer(suiteCode)
	p.AddressStreet = nullStringPointer(street)
	p.AddressCity = nullStringPointer(city)
	p.AddressState = nullStringPointer(state)
	p.AddressZip = nullStringPointer(zip)
	p.EmailVerified = emailVerified == 1
	return &p, nil
}

func exportRecipientRows(ctx context.Context, userID string) ([]exportRecipientRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, name, phone, destination_id, street, apt, city, delivery_instructions,
		       is_default, use_count, created_at, updated_at
		FROM recipients
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportRecipientRow{}
	for rows.Next() {
		var (
			r                                          exportRecipientRow
			phone, apt, instructions                   sql.NullString
			isDefault                                  int
		)
		if err := rows.Scan(&r.ID, &r.Name, &phone, &r.DestinationID, &r.Street, &apt, &r.City,
			&instructions, &isDefault, &r.UseCount, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Phone = nullStringPointer(phone)
		r.Apt = nullStringPointer(apt)
		r.DeliveryInstructions = nullStringPointer(instructions)
		r.IsDefault = isDefault == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportLockerPackageRows(ctx context.Context, userID string) ([]exportLockerRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, suite_code, sender_name, weight_lbs, length_in, width_in, height_in,
		       condition, status, storage_bay, arrived_at, free_storage_expires_at,
		       created_at, updated_at
		FROM locker_packages
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportLockerRow{}
	for rows.Next() {
		var (
			r                                                   exportLockerRow
			senderName, condition, storageBay, arrivedAt, freeExp sql.NullString
			weight, length, width, height                       sql.NullFloat64
		)
		if err := rows.Scan(&r.ID, &r.SuiteCode, &senderName, &weight, &length, &width, &height,
			&condition, &r.Status, &storageBay, &arrivedAt, &freeExp,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.SenderName = senderName.String
		if weight.Valid {
			r.WeightLbs = weight.Float64
		}
		r.LengthIn = nullFloatPointer(length)
		r.WidthIn = nullFloatPointer(width)
		r.HeightIn = nullFloatPointer(height)
		r.Condition = nullStringPointer(condition)
		r.StorageBay = nullStringPointer(storageBay)
		r.ArrivedAt = nullStringPointer(arrivedAt)
		r.FreeStorageExpiresAt = nullStringPointer(freeExp)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportShipRequestRows(ctx context.Context, userID string) ([]exportShipRequestRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, confirmation_code, status, destination_id, recipient_id, service_type,
		       consolidate, special_instructions, subtotal, service_fees, insurance,
		       discount, total, payment_status, customs_status, created_at, updated_at
		FROM ship_requests
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportShipRequestRow{}
	ids := []string{}
	for rows.Next() {
		var (
			r                                                                   exportShipRequestRow
			recipientID, specialInstr, paymentStatus, customsStatus            sql.NullString
			consolidate                                                         int
		)
		if err := rows.Scan(&r.ID, &r.ConfirmationCode, &r.Status, &r.DestinationID, &recipientID,
			&r.ServiceType, &consolidate, &specialInstr, &r.Subtotal, &r.ServiceFees, &r.Insurance,
			&r.Discount, &r.Total, &paymentStatus, &customsStatus, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.RecipientID = nullStringPointer(recipientID)
		r.SpecialInstructions = nullStringPointer(specialInstr)
		r.PaymentStatus = nullStringPointer(paymentStatus)
		r.CustomsStatus = nullStringPointer(customsStatus)
		r.Consolidate = consolidate == 1
		r.Items = []exportShipRequestItemRow{}
		out = append(out, r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return out, nil
	}
	itemsByReq, err := exportShipRequestItemsForReqs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if items, ok := itemsByReq[out[i].ID]; ok {
			out[i].Items = items
		}
	}
	return out, nil
}

func exportShipRequestItemsForReqs(ctx context.Context, shipRequestIDs []string) (map[string][]exportShipRequestItemRow, error) {
	if len(shipRequestIDs) == 0 {
		return map[string][]exportShipRequestItemRow{}, nil
	}
	placeholders := strings.Repeat("?,", len(shipRequestIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(shipRequestIDs))
	for i, id := range shipRequestIDs {
		args[i] = id
	}
	q := `
		SELECT id, ship_request_id, locker_package_id, customs_description, customs_value,
		       customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
		FROM ship_request_items
		WHERE ship_request_id IN (` + placeholders + `)
		ORDER BY ship_request_id, id
	`
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]exportShipRequestItemRow{}
	for rows.Next() {
		var (
			it                                          exportShipRequestItemRow
			shipReqID                                   string
			desc, hsCode, originCountry                 sql.NullString
			value, weight                               sql.NullFloat64
			qty                                         sql.NullInt64
		)
		if err := rows.Scan(&it.ID, &shipReqID, &it.LockerPackageID, &desc, &value, &qty,
			&hsCode, &originCountry, &weight); err != nil {
			return nil, err
		}
		it.CustomsDescription = nullStringPointer(desc)
		it.CustomsValue = nullFloatPointer(value)
		if qty.Valid {
			n := int(qty.Int64)
			it.CustomsQuantity = &n
		}
		it.CustomsHSCode = nullStringPointer(hsCode)
		it.CustomsCountryOfOrigin = nullStringPointer(originCountry)
		it.CustomsWeightLbs = nullFloatPointer(weight)
		out[shipReqID] = append(out[shipReqID], it)
	}
	return out, rows.Err()
}

func exportBookingRows(ctx context.Context, userID string) ([]exportBookingRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, confirmation_code, status, service_type, destination_id, recipient_id,
		       scheduled_date, time_slot, special_instructions, weight_lbs, length_in,
		       width_in, height_in, value_usd, add_insurance, subtotal, discount,
		       insurance, total, payment_status, created_at, updated_at
		FROM bookings
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportBookingRow{}
	for rows.Next() {
		var (
			b                                          exportBookingRow
			recipientID, specialInstr, paymentStatus  sql.NullString
			addInsurance                               int
		)
		if err := rows.Scan(&b.ID, &b.ConfirmationCode, &b.Status, &b.ServiceType, &b.DestinationID,
			&recipientID, &b.ScheduledDate, &b.TimeSlot, &specialInstr, &b.WeightLbs, &b.LengthIn,
			&b.WidthIn, &b.HeightIn, &b.ValueUSD, &addInsurance, &b.Subtotal, &b.Discount,
			&b.Insurance, &b.Total, &paymentStatus, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.RecipientID = nullStringPointer(recipientID)
		b.SpecialInstructions = nullStringPointer(specialInstr)
		b.PaymentStatus = nullStringPointer(paymentStatus)
		b.AddInsurance = addInsurance == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

func exportCustomsDocRows(ctx context.Context, userID string) ([]exportCustomsDocRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, ship_request_id, locker_package_id, doc_type, file_name, mime_type,
		       size_bytes, status, created_at
		FROM customs_preclearance_docs
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportCustomsDocRow{}
	for rows.Next() {
		var (
			r                              exportCustomsDocRow
			shipReqID, pkgID, mimeType     sql.NullString
		)
		if err := rows.Scan(&r.ID, &shipReqID, &pkgID, &r.DocType, &r.FileName, &mimeType,
			&r.SizeBytes, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.ShipRequestID = nullStringPointer(shipReqID)
		r.LockerPackageID = nullStringPointer(pkgID)
		r.MimeType = nullStringPointer(mimeType)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportDeliverySignatureRows(ctx context.Context, userID string) ([]exportDeliverySignatureRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, ship_request_id, signer_name, captured_at, created_at
		FROM delivery_signatures
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportDeliverySignatureRow{}
	for rows.Next() {
		var r exportDeliverySignatureRow
		if err := rows.Scan(&r.ID, &r.ShipRequestID, &r.SignerName, &r.CapturedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportLockerPhotoRows(ctx context.Context, userID string) ([]exportLockerPhotoRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT lph.id, lph.locker_package_id, lph.photo_url, lph.photo_type,
		       lph.taken_by, lph.created_at
		FROM locker_photos lph
		JOIN locker_packages lp ON lp.id = lph.locker_package_id
		WHERE lp.user_id = ?
		ORDER BY lph.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportLockerPhotoRow{}
	for rows.Next() {
		var (
			r       exportLockerPhotoRow
			takenBy sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.LockerPackageID, &r.PhotoURL, &r.PhotoType, &takenBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.TakenBy = nullStringPointer(takenBy)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportAssistedPurchaseRows(ctx context.Context, userID string) ([]exportAssistedPurchaseRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, recipient_id, store_url, item_name, quantity, estimated_cost_usd,
		       notes, status, created_at, updated_at
		FROM assisted_purchase_requests
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportAssistedPurchaseRow{}
	for rows.Next() {
		var (
			r                       exportAssistedPurchaseRow
			recipientID, notes      sql.NullString
		)
		if err := rows.Scan(&r.ID, &recipientID, &r.StoreURL, &r.ItemName, &r.Quantity,
			&r.EstimatedCostUSD, &notes, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.RecipientID = nullStringPointer(recipientID)
		r.Notes = nullStringPointer(notes)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportNotificationPrefsRowFor(ctx context.Context, userID string) (*exportNotificationPrefsRow, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT email_enabled, sms_enabled, push_enabled, on_package_arrived,
		       on_storage_expiry, on_ship_updates, on_inbound_updates, daily_digest,
		       created_at, updated_at
		FROM notification_prefs WHERE user_id = ?
	`, userID)
	var (
		r                                                                                 exportNotificationPrefsRow
		emailEnabled, smsEnabled, pushEnabled, pkgArrived, storageExp, shipUpd, inbUpd  int
	)
	if err := row.Scan(&emailEnabled, &smsEnabled, &pushEnabled, &pkgArrived, &storageExp,
		&shipUpd, &inbUpd, &r.DailyDigest, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.EmailEnabled = emailEnabled == 1
	r.SMSEnabled = smsEnabled == 1
	r.PushEnabled = pushEnabled == 1
	r.OnPackageArrived = pkgArrived == 1
	r.OnStorageExpiry = storageExp == 1
	r.OnShipUpdates = shipUpd == 1
	r.OnInboundUpdates = inbUpd == 1
	return &r, nil
}

func exportCookieConsentRowFor(ctx context.Context, userID string) (*exportCookieConsentRow, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT consent_version, necessary, functional, analytics, marketing,
		       source, ip_address, user_agent, consented_at, updated_at
		FROM cookie_consents WHERE user_id = ?
	`, userID)
	var (
		r                                              exportCookieConsentRow
		source, ipAddr, ua                             sql.NullString
		necessary, functional, analytics, marketing    int
	)
	if err := row.Scan(&r.ConsentVersion, &necessary, &functional, &analytics, &marketing,
		&source, &ipAddr, &ua, &r.ConsentedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.Necessary = necessary == 1
	r.Functional = functional == 1
	r.Analytics = analytics == 1
	r.Marketing = marketing == 1
	r.Source = nullStringPointer(source)
	r.IPAddress = nullStringPointer(ipAddr)
	r.UserAgent = nullStringPointer(ua)
	return &r, nil
}

func exportLoyaltyLedgerRows(ctx context.Context, userID string) ([]exportLoyaltyLedgerRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, points_delta, reason, resource_type, resource_id, created_at
		FROM loyalty_ledger
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportLoyaltyLedgerRow{}
	for rows.Next() {
		var (
			r                          exportLoyaltyLedgerRow
			resType, resID             sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.PointsDelta, &r.Reason, &resType, &resID, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.ResourceType = nullStringPointer(resType)
		r.ResourceID = nullStringPointer(resID)
		out = append(out, r)
	}
	return out, rows.Err()
}

// exportUserMFARowFor returns MFA enrollment metadata only. Secrets are
// never selected from the database, so they cannot leak into the export.
func exportUserMFARowFor(ctx context.Context, userID string) (*exportUserMFARow, error) {
	row := db.DB().QueryRowContext(ctx, `
		SELECT method, enabled, last_verified_at, last_challenge_at, created_at, updated_at
		FROM user_mfa WHERE user_id = ?
	`, userID)
	var (
		r                              exportUserMFARow
		lastVerified, lastChallenge    sql.NullString
		enabled                        int
	)
	if err := row.Scan(&r.Method, &enabled, &lastVerified, &lastChallenge, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	r.Enabled = enabled == 1
	r.LastVerifiedAt = nullStringPointer(lastVerified)
	r.LastChallengeAt = nullStringPointer(lastChallenge)
	return &r, nil
}

// exportAPIKeyMetaRows selects metadata only. The key_hash column is
// excluded from the SELECT statement so it cannot leak into the response
// even via reflection or future refactors.
func exportAPIKeyMetaRows(ctx context.Context, userID string) ([]exportAPIKeyMetaRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, name, key_prefix, scopes_json, last_used_at, expires_at, revoked_at,
		       created_at, updated_at
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportAPIKeyMetaRow{}
	for rows.Next() {
		var (
			r                                 exportAPIKeyMetaRow
			lastUsed, expiresAt, revokedAt    sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Name, &r.KeyPrefix, &r.Scopes, &lastUsed, &expiresAt,
			&revokedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.LastUsedAt = nullStringPointer(lastUsed)
		r.ExpiresAt = nullStringPointer(expiresAt)
		r.RevokedAt = nullStringPointer(revokedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportServiceRequestRows(ctx context.Context, userID string) ([]exportServiceRequestRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, locker_package_id, service_type, status, notes, price, created_at, completed_at
		FROM service_requests
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportServiceRequestRow{}
	for rows.Next() {
		var (
			r                       exportServiceRequestRow
			notes, completedAt      sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.LockerPackageID, &r.ServiceType, &r.Status, &notes,
			&r.Price, &r.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		r.Notes = nullStringPointer(notes)
		r.CompletedAt = nullStringPointer(completedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportStorageFeeRows(ctx context.Context, userID string) ([]exportStorageFeeRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, locker_package_id, fee_date, amount, invoiced, invoice_id, created_at
		FROM storage_fees
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportStorageFeeRow{}
	for rows.Next() {
		var (
			r          exportStorageFeeRow
			invoiceID  sql.NullString
			invoiced   int
		)
		if err := rows.Scan(&r.ID, &r.LockerPackageID, &r.FeeDate, &r.Amount, &invoiced,
			&invoiceID, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Invoiced = invoiced == 1
		r.InvoiceID = nullStringPointer(invoiceID)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportInvoiceRows(ctx context.Context, userID string) ([]exportInvoiceRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, booking_id, ship_request_id, invoice_number, subtotal, tax, total,
		       status, due_date, paid_at, notes, created_at
		FROM invoices
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportInvoiceRow{}
	ids := []string{}
	for rows.Next() {
		var (
			r                                                exportInvoiceRow
			bookingID, shipReqID, dueDate, paidAt, notes    sql.NullString
		)
		if err := rows.Scan(&r.ID, &bookingID, &shipReqID, &r.InvoiceNumber, &r.Subtotal,
			&r.Tax, &r.Total, &r.Status, &dueDate, &paidAt, &notes, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.BookingID = nullStringPointer(bookingID)
		r.ShipRequestID = nullStringPointer(shipReqID)
		r.DueDate = nullStringPointer(dueDate)
		r.PaidAt = nullStringPointer(paidAt)
		r.Notes = nullStringPointer(notes)
		r.Items = []exportInvoiceItemRow{}
		out = append(out, r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	itemsByInvoice, err := exportInvoiceItemsForInvoices(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if items, ok := itemsByInvoice[out[i].ID]; ok {
			out[i].Items = items
		}
	}
	return out, nil
}

func exportInvoiceItemsForInvoices(ctx context.Context, invoiceIDs []string) (map[string][]exportInvoiceItemRow, error) {
	if len(invoiceIDs) == 0 {
		return map[string][]exportInvoiceItemRow{}, nil
	}
	placeholders := strings.Repeat("?,", len(invoiceIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(invoiceIDs))
	for i, id := range invoiceIDs {
		args[i] = id
	}
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, invoice_id, description, quantity, unit_price, total
		FROM invoice_items
		WHERE invoice_id IN (`+placeholders+`)
		ORDER BY invoice_id, id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]exportInvoiceItemRow{}
	for rows.Next() {
		var (
			it       exportInvoiceItemRow
			invoiceID string
		)
		if err := rows.Scan(&it.ID, &invoiceID, &it.Description, &it.Quantity, &it.UnitPrice, &it.Total); err != nil {
			return nil, err
		}
		out[invoiceID] = append(out[invoiceID], it)
	}
	return out, rows.Err()
}

func exportTemplateRows(ctx context.Context, userID string) ([]exportTemplateRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, name, service_type, destination_id, recipient_id, use_count, created_at
		FROM templates
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportTemplateRow{}
	for rows.Next() {
		var (
			r           exportTemplateRow
			recipientID sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Name, &r.ServiceType, &r.DestinationID, &recipientID,
			&r.UseCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.RecipientID = nullStringPointer(recipientID)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportInboundTrackingRows(ctx context.Context, userID string) ([]exportInboundTrackingRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, carrier, tracking_number, retailer_name, expected_items, status,
		       locker_package_id, last_checked_at, created_at
		FROM inbound_tracking
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportInboundTrackingRow{}
	for rows.Next() {
		var (
			r                                            exportInboundTrackingRow
			retailer, expected, lockerPkgID, lastChecked sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Carrier, &r.TrackingNumber, &retailer, &expected, &r.Status,
			&lockerPkgID, &lastChecked, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.RetailerName = nullStringPointer(retailer)
		r.ExpectedItems = nullStringPointer(expected)
		r.LockerPackageID = nullStringPointer(lockerPkgID)
		r.LastCheckedAt = nullStringPointer(lastChecked)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportInAppNotificationRows(ctx context.Context, userID string) ([]exportInAppNotificationRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, title, body, level, link_url, read_at, created_at
		FROM in_app_notifications
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportInAppNotificationRow{}
	for rows.Next() {
		var (
			r              exportInAppNotificationRow
			linkURL, readAt sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Title, &r.Body, &r.Level, &linkURL, &readAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.LinkURL = nullStringPointer(linkURL)
		r.ReadAt = nullStringPointer(readAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// exportPushSubscriptionMetaRows excludes p256dh and auth crypto material.
func exportPushSubscriptionMetaRows(ctx context.Context, userID string) ([]exportPushSubscriptionMetaRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, endpoint, created_at, updated_at
		FROM push_subscriptions
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportPushSubscriptionMetaRow{}
	for rows.Next() {
		var r exportPushSubscriptionMetaRow
		if err := rows.Scan(&r.ID, &r.Endpoint, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportConsolidationPreviewRows(ctx context.Context, userID string) ([]exportConsolidationPreviewRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, package_ids_json, package_count, total_weight_lbs,
		       pre_consolidation_billable_lbs, post_consolidation_billable_lbs,
		       estimated_savings_lbs, destination_id, created_at
		FROM parcel_consolidation_previews
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportConsolidationPreviewRow{}
	for rows.Next() {
		var (
			r           exportConsolidationPreviewRow
			destID      sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.PackageIDsJSON, &r.PackageCount, &r.TotalWeightLbs,
			&r.PreConsolidationBillableLbs, &r.PostConsolidationBillable,
			&r.EstimatedSavingsLbs, &destID, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.DestinationID = nullStringPointer(destID)
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportDataImportJobRows(ctx context.Context, userID string) ([]exportDataImportJobRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, import_type, status, total_rows, imported_rows, failed_rows,
		       error_summary, created_at, updated_at
		FROM data_import_jobs
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportDataImportJobRow{}
	for rows.Next() {
		var (
			r            exportDataImportJobRow
			errorSummary sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.ImportType, &r.Status, &r.TotalRows, &r.ImportedRows,
			&r.FailedRows, &errorSummary, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.ErrorSummary = nullStringPointer(errorSummary)
		out = append(out, r)
	}
	return out, rows.Err()
}

// writeUserExportCSV writes one row per user-scoped record using the
// 7-column schema established by the original implementation:
//   type, id, status, created_at, field_a, field_b, field_c
// The CSV format intentionally flattens; consumers needing full fidelity
// should request format=json.
func writeUserExportCSV(w *csv.Writer, userID string, exp *userExportData) {
	if exp.Profile != nil {
		_ = w.Write([]string{"user_profile", exp.Profile.ID, exp.Profile.Status, exp.Profile.CreatedAt,
			exp.Profile.Email, exp.Profile.Name, exp.Profile.Role})
	}
	for _, r := range exp.Recipients {
		_ = w.Write([]string{"recipient", r.ID, "", r.CreatedAt, r.Name, r.DestinationID, r.City})
	}
	for _, lp := range exp.LockerPackages {
		_ = w.Write([]string{"locker_package", lp.ID, lp.Status, lp.CreatedAt, lp.SenderName,
			fmt.Sprintf("%.2f", lp.WeightLbs), strDeref(lp.ArrivedAt)})
	}
	for _, sr := range exp.ShipRequests {
		_ = w.Write([]string{"ship_request", sr.ID, sr.Status, sr.CreatedAt,
			sr.ConfirmationCode, sr.DestinationID, sr.ServiceType})
	}
	for _, b := range exp.Bookings {
		_ = w.Write([]string{"booking", b.ID, b.Status, b.CreatedAt,
			b.ConfirmationCode, b.DestinationID, b.ServiceType})
	}
	for _, d := range exp.CustomsDocs {
		_ = w.Write([]string{"customs_preclearance_doc", d.ID, d.Status, d.CreatedAt,
			d.DocType, d.FileName, strDeref(d.ShipRequestID)})
	}
	for _, ds := range exp.DeliverySignatures {
		_ = w.Write([]string{"delivery_signature", ds.ID, "captured", ds.CreatedAt,
			ds.ShipRequestID, ds.SignerName, ds.CapturedAt})
	}
	for _, lp := range exp.LockerPhotos {
		_ = w.Write([]string{"locker_photo", lp.ID, "", lp.CreatedAt,
			lp.LockerPackageID, lp.PhotoType, lp.PhotoURL})
	}
	for _, ap := range exp.AssistedPurchases {
		_ = w.Write([]string{"assisted_purchase", ap.ID, ap.Status, ap.CreatedAt,
			ap.StoreURL, ap.ItemName, fmt.Sprintf("%.2f", ap.EstimatedCostUSD)})
	}
	if exp.NotificationPrefs != nil {
		_ = w.Write([]string{"notification_prefs", userID, "", exp.NotificationPrefs.CreatedAt,
			boolStr(exp.NotificationPrefs.EmailEnabled), boolStr(exp.NotificationPrefs.SMSEnabled),
			exp.NotificationPrefs.DailyDigest})
	}
	if exp.CookieConsents != nil {
		_ = w.Write([]string{"cookie_consent", userID, "", exp.CookieConsents.ConsentedAt,
			exp.CookieConsents.ConsentVersion, boolStr(exp.CookieConsents.Analytics),
			boolStr(exp.CookieConsents.Marketing)})
	}
	for _, ll := range exp.LoyaltyLedger {
		_ = w.Write([]string{"loyalty_entry", ll.ID, "", ll.CreatedAt,
			ll.Reason, strconv.Itoa(ll.PointsDelta), strDeref(ll.ResourceID)})
	}
	if exp.UserMFA != nil {
		_ = w.Write([]string{"user_mfa", userID, boolStr(exp.UserMFA.Enabled),
			exp.UserMFA.CreatedAt, exp.UserMFA.Method, "", ""})
	}
	for _, k := range exp.APIKeys {
		_ = w.Write([]string{"api_key", k.ID, "", k.CreatedAt,
			k.Name, k.KeyPrefix, strDeref(k.RevokedAt)})
	}
	for _, sr := range exp.ServiceRequests {
		_ = w.Write([]string{"service_request", sr.ID, sr.Status, sr.CreatedAt,
			sr.ServiceType, sr.LockerPackageID, fmt.Sprintf("%.2f", sr.Price)})
	}
	for _, sf := range exp.StorageFees {
		_ = w.Write([]string{"storage_fee", sf.ID, boolStr(sf.Invoiced), sf.CreatedAt,
			sf.LockerPackageID, sf.FeeDate, fmt.Sprintf("%.2f", sf.Amount)})
	}
	for _, inv := range exp.Invoices {
		_ = w.Write([]string{"invoice", inv.ID, inv.Status, inv.CreatedAt,
			inv.InvoiceNumber, fmt.Sprintf("%.2f", inv.Total), strDeref(inv.PaidAt)})
	}
	for _, t := range exp.Templates {
		_ = w.Write([]string{"template", t.ID, "", t.CreatedAt,
			t.Name, t.ServiceType, t.DestinationID})
	}
	for _, it := range exp.InboundTracking {
		_ = w.Write([]string{"inbound_tracking", it.ID, it.Status, it.CreatedAt,
			it.Carrier, it.TrackingNumber, strDeref(it.RetailerName)})
	}
	for _, n := range exp.InAppNotifications {
		_ = w.Write([]string{"in_app_notification", n.ID, n.Level, n.CreatedAt,
			n.Title, "", strDeref(n.ReadAt)})
	}
	for _, ps := range exp.PushSubscriptions {
		_ = w.Write([]string{"push_subscription", ps.ID, "", ps.CreatedAt,
			ps.Endpoint, "", ""})
	}
	for _, cp := range exp.ConsolidationPreviews {
		_ = w.Write([]string{"consolidation_preview", cp.ID, "", cp.CreatedAt,
			strconv.Itoa(cp.PackageCount), fmt.Sprintf("%.2f", cp.EstimatedSavingsLbs),
			strDeref(cp.DestinationID)})
	}
	for _, j := range exp.DataImportJobs {
		_ = w.Write([]string{"data_import_job", j.ID, j.Status, j.CreatedAt,
			j.ImportType, strconv.Itoa(j.ImportedRows), strconv.Itoa(j.FailedRows)})
	}
}

func nullStringPointer(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func nullFloatPointer(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
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


