package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/balance"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

type destinationFallback struct {
	ID       string
	Name     string
	Code     string
	Capital  string
	USDPerLb float64
	Transit  string
}

// Fallback destinations are used only when destination DB queries fail.
var fallbackDestinations = []destinationFallback{
	{ID: "guyana", Name: "Guyana", Code: "GY", Capital: "Georgetown", USDPerLb: 3.50, Transit: "3-5 days"},
	{ID: "jamaica", Name: "Jamaica", Code: "JM", Capital: "Kingston", USDPerLb: 3.75, Transit: "3-5 days"},
	{ID: "trinidad", Name: "Trinidad & Tobago", Code: "TT", Capital: "Port of Spain", USDPerLb: 3.50, Transit: "3-5 days"},
	{ID: "barbados", Name: "Barbados", Code: "BB", Capital: "Bridgetown", USDPerLb: 4.00, Transit: "4-6 days"},
	{ID: "suriname", Name: "Suriname", Code: "SR", Capital: "Paramaribo", USDPerLb: 4.25, Transit: "4-6 days"},
}

// RegisterPublic mounts public (no-auth) API routes.
func RegisterPublic(g fiber.Router) {
	g.Get("/destinations", listDestinations)
	g.Get("/destinations/:id", getDestination)
	g.Get("/status", systemStatus)
	g.Get("/config", publicConfig)
	g.Post("/contact", contactForm)
	g.Get("/track/:trackingNumber", publicTrack)
	g.Get("/calculator", shippingCalculator)
	g.Get("/stripe/verify", stripeVerify)
}

func publicConfig(c *fiber.Ctx) error {
	config := fiber.Map{}
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	pk := os.Getenv("STRIPE_PUBLISHABLE_KEY")
	config["stripe_configured"] = secretKey != ""
	if pk != "" {
		config["stripe_publishable_key"] = pk
	}
	return c.JSON(fiber.Map{"data": config})
}

// stripeVerify calls Stripe Balance API to verify STRIPE_SECRET_KEY. Returns stripe_ok and optional error.
func stripeVerify(c *fiber.Ctx) error {
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	if secretKey == "" {
		return c.JSON(fiber.Map{"data": fiber.Map{"stripe_ok": false, "error": "STRIPE_SECRET_KEY not set"}})
	}
	stripe.Key = secretKey
	_, err := balance.Get(nil)
	if err != nil {
		return c.JSON(fiber.Map{"data": fiber.Map{"stripe_ok": false, "error": err.Error()}})
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"stripe_ok": true}})
}

func listDestinations(c *fiber.Ctx) error {
	var cached []fiber.Map
	if getCachedJSON(c.Context(), publicDestinationsCacheKey, &cached) && len(cached) > 0 {
		return c.JSON(fiber.Map{"data": cached})
	}

	rows, err := listActiveDestinationsFromDB(c.Context())
	if err != nil {
		log.Printf("destinations list fallback: %v", err)
		out := fallbackDestinationMaps()
		setCachedJSON(c.Context(), publicDestinationsCacheKey, out, publicDestinationsCacheTTL)
		return c.JSON(fiber.Map{"data": out})
	}
	if len(rows) == 0 {
		out := fallbackDestinationMaps()
		setCachedJSON(c.Context(), publicDestinationsCacheKey, out, publicDestinationsCacheTTL)
		return c.JSON(fiber.Map{"data": out})
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		out = append(out, destinationRowToMap(row))
	}
	setCachedJSON(c.Context(), publicDestinationsCacheKey, out, publicDestinationsCacheTTL)
	return c.JSON(fiber.Map{"data": out})
}

func getDestination(c *fiber.Ctx) error {
	id := strings.ToLower(strings.TrimSpace(c.Params("id")))
	row, err := getActiveDestinationByIDFromDB(c.Context(), id)
	if err == nil {
		return c.JSON(fiber.Map{"data": destinationRowToMap(row)})
	}
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Destination not found"))
	}
	log.Printf("destination get fallback for %q: %v", id, err)
	if fallback, ok := fallbackDestinationByID(id); ok {
		return c.JSON(fiber.Map{"data": fallback})
	}
	return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Destination not found"))
}

func destinationRowToMap(row gen.Destination) fiber.Map {
	return fiber.Map{
		"id":         row.ID,
		"name":       row.Name,
		"code":       row.Code,
		"capital":    row.Capital,
		"usd_per_lb": row.UsdPerLb,
		"transit":    fmt.Sprintf("%d-%d days", row.TransitDaysMin, row.TransitDaysMax),
	}
}

func fallbackDestinationMaps() []fiber.Map {
	out := make([]fiber.Map, 0, len(fallbackDestinations))
	for _, d := range fallbackDestinations {
		out = append(out, fiber.Map{
			"id":         d.ID,
			"name":       d.Name,
			"code":       d.Code,
			"capital":    d.Capital,
			"usd_per_lb": d.USDPerLb,
			"transit":    d.Transit,
		})
	}
	return out
}

func fallbackDestinationByID(id string) (fiber.Map, bool) {
	for _, d := range fallbackDestinations {
		if d.ID == id {
			return fiber.Map{
				"id":         d.ID,
				"name":       d.Name,
				"code":       d.Code,
				"capital":    d.Capital,
				"usd_per_lb": d.USDPerLb,
				"transit":    d.Transit,
			}, true
		}
	}
	return nil, false
}

func listActiveDestinationsFromDB(ctx context.Context) (rows []gen.Destination, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("destinations list panic: %v", r)
		}
	}()
	return db.Queries().ListActiveDestinations(ctx)
}

func getActiveDestinationByIDFromDB(ctx context.Context, id string) (row gen.Destination, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("destination get panic: %v", r)
		}
	}()
	return db.Queries().GetActiveDestinationByID(ctx, id)
}

func systemStatus(c *fiber.Ctx) error {
	dbOK := true
	if err := db.Ping(); err != nil {
		dbOK = false
	}
	stripeConfigured := os.Getenv("STRIPE_SECRET_KEY") != ""
	resendConfigured := os.Getenv("RESEND_API_KEY") != ""
	status := "operational"
	message := "All systems normal"
	if !dbOK {
		status = "degraded"
		message = "Database connectivity issue"
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"status":            status,
			"message":           message,
			"db_ok":             dbOK,
			"stripe_configured": stripeConfigured,
			"resend_configured": resendConfigured,
		},
	})
}

func contactForm(c *fiber.Ctx) error {
	var body struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Name == "" || body.Email == "" || body.Message == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name, email, and message required"))
	}
	if len(body.Message) > MaxContactMessageLength {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "message must not exceed 5000 characters"))
	}
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendContactFormSubmission(body.Name, body.Email, body.Subject, body.Message); err != nil {
			log.Printf("contact form email send: %v", err)
			return c.Status(503).JSON(ErrorResponse{}.withCode("SERVICE_UNAVAILABLE", "Unable to send message. Please try again later."))
		}
	} else {
		log.Printf("[Contact] %s <%s> %s: %s", body.Name, body.Email, body.Subject, body.Message)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Thank you. We will get back to you soon."}})
}

// publicTrack looks up a shipment/ship request by tracking number (PRD 6.7).
// Returns basic status information publicly. Full detail requires auth.
func publicTrack(c *fiber.Ctx) error {
	trackingNumber := strings.TrimSpace(c.Params("trackingNumber"))
	if trackingNumber == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "tracking number required"))
	}

	row := db.DB().QueryRowContext(c.Context(), `
		SELECT s.id, s.destination_id, s.manifest_id, s.ship_request_id, s.tracking_number, s.status,
		       s.total_weight, s.package_count, s.carrier, s.estimated_delivery, s.actual_delivery,
		       s.created_at, s.updated_at
		FROM shipments s
		LEFT JOIN ship_requests sr ON sr.id = s.ship_request_id
		WHERE LOWER(COALESCE(s.tracking_number, '')) = LOWER(?)
		   OR LOWER(COALESCE(sr.confirmation_code, '')) = LOWER(?)
		ORDER BY s.created_at DESC
		LIMIT 1
	`, trackingNumber, trackingNumber)

	var s gen.Shipment
	err := row.Scan(
		&s.ID,
		&s.DestinationID,
		&s.ManifestID,
		&s.ShipRequestID,
		&s.TrackingNumber,
		&s.Status,
		&s.TotalWeight,
		&s.PackageCount,
		&s.Carrier,
		&s.EstimatedDelivery,
		&s.ActualDelivery,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err == nil {
		return c.JSON(fiber.Map{"data": shipmentToMap(s)})
	}
	if err != sql.ErrNoRows {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load shipment status"))
	}

	// Confirmation-code fallback for requests that exist but have no shipment row yet.
	var id, confirmationCode, status, destinationID, serviceType, createdAt, updatedAt string
	err = db.DB().QueryRowContext(c.Context(), `
		SELECT id, confirmation_code, status, destination_id, service_type, created_at, updated_at
		FROM ship_requests
		WHERE LOWER(confirmation_code) = LOWER(?)
		ORDER BY created_at DESC
		LIMIT 1
	`, trackingNumber).Scan(&id, &confirmationCode, &status, &destinationID, &serviceType, &createdAt, &updatedAt)
	if err == nil {
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"id":                id,
				"tracking_number":   confirmationCode,
				"confirmation_code": confirmationCode,
				"status":            status,
				"destination_id":    destinationID,
				"service_type":      serviceType,
				"created_at":        createdAt,
				"updated_at":        updatedAt,
			},
		})
	}
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "No shipment found with that tracking number"))
	}
	return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load shipment status"))
}

// shippingCalculator applies the PRD 8.9 pricing formula via services.CalculatePricing.
func shippingCalculator(c *fiber.Ctx) error {
	destID := strings.TrimSpace(c.Query("dest"))
	if destID == "" {
		destID = strings.TrimSpace(c.Query("destination"))
	}
	weightStr := c.Query("weight")
	lStr := c.Query("l")
	wStr := c.Query("w")
	hStr := c.Query("h")
	valueStr := c.Query("value")
	service := c.Query("service", "standard")

	if destID == "" || strings.TrimSpace(weightStr) == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "dest and weight are required"))
	}

	parseF := func(field, s string) (float64, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s must be a valid number", field)
		}
		return v, nil
	}

	weight, err := parseF("weight", weightStr)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	L, err := parseF("l", lStr)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	W, err := parseF("w", wStr)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	H, err := parseF("h", hStr)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	declaredValue, err := parseF("value", valueStr)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}

	if weight <= 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "weight must be > 0"))
	}

	result := services.CalculatePricing(services.PricingInput{
		DestinationID: destID,
		ServiceType:   service,
		WeightLbs:     weight,
		LengthIn:      L,
		WidthIn:       W,
		HeightIn:      H,
		ValueUSD:      declaredValue,
		AddInsurance:  declaredValue > 0,
	})
	if _, ok := services.Rates[destID]; !ok {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "unknown destination"))
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"destination_id":  result.DestinationID,
			"service":         result.Service,
			"actual_weight":   result.ActualWeight,
			"dim_weight":      result.DimWeight,
			"billable_weight": result.BillableWeight,
			"rate_per_lb":     result.RatePerLb,
			"base_cost":       result.Subtotal,
			"surcharge":       result.ServiceFees,
			"door_to_door_fee": func() float64 {
				if result.Service == "door_to_door" {
					return result.ServiceFees
				}
				return 0.0
			}(),
			"insurance":       result.Insurance,
			"volume_discount": result.Discount,
			"total":           result.Total,
			"minimum_applied": result.MinimumApplied,
		},
	})
}
