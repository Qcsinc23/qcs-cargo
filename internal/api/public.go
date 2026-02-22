package api

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/balance"

	"github.com/Qcsinc23/qcs-cargo/internal/calc"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

// Destinations per PRD 8.2
var destinations = []fiber.Map{
	{"id": "guyana", "name": "Guyana", "code": "GY", "capital": "Georgetown", "usd_per_lb": 3.50, "transit": "3-5 days"},
	{"id": "jamaica", "name": "Jamaica", "code": "JM", "capital": "Kingston", "usd_per_lb": 3.75, "transit": "3-5 days"},
	{"id": "trinidad", "name": "Trinidad & Tobago", "code": "TT", "capital": "Port of Spain", "usd_per_lb": 3.50, "transit": "3-5 days"},
	{"id": "barbados", "name": "Barbados", "code": "BB", "capital": "Bridgetown", "usd_per_lb": 4.00, "transit": "4-6 days"},
	{"id": "suriname", "name": "Suriname", "code": "SR", "capital": "Paramaribo", "usd_per_lb": 4.25, "transit": "4-6 days"},
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
	return c.JSON(fiber.Map{"data": destinations})
}

func getDestination(c *fiber.Ctx) error {
	id := c.Params("id")
	for _, d := range destinations {
		if d["id"] == id {
			return c.JSON(fiber.Map{"data": d})
		}
	}
	return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Destination not found"))
}

func systemStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"status":  "operational",
			"message": "All systems normal",
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
	trackingNumber := c.Params("trackingNumber")
	if trackingNumber == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "tracking number required"))
	}
	// TODO: query shipments table when Phase 2 DB queries are generated.
	// For now return a not-found response. Frontend handles gracefully.
	return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "No shipment found with that tracking number"))
}

// shippingCalculator applies the PRD 8.9 pricing formula via internal/calc.
func shippingCalculator(c *fiber.Ctx) error {
	destID := c.Query("dest")
	weightStr := c.Query("weight")
	lStr := c.Query("l")
	wStr := c.Query("w")
	hStr := c.Query("h")
	valueStr := c.Query("value")
	service := c.Query("service", "standard")

	if destID == "" || weightStr == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "dest and weight are required"))
	}

	parseF := func(s string) float64 {
		var v float64
		_, _ = fmt.Sscanf(s, "%f", &v)
		return v
	}

	weight := parseF(weightStr)
	L, W, H := parseF(lStr), parseF(wStr), parseF(hStr)
	declaredValue := parseF(valueStr)

	if weight <= 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "weight must be > 0"))
	}

	result, ok := calc.CalculateShipping(calc.ShippingInput{
		Destination:   destID,
		Service:       service,
		ActualWeight:  weight,
		Length:        L, Width: W, Height: H,
		DeclaredValue: declaredValue,
		Insurance:     declaredValue > 0,
	})
	if !ok {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "unknown destination"))
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"destination_id":   result.DestinationID,
			"service":         result.Service,
			"actual_weight":   result.ActualWeight,
			"dim_weight":      result.DimWeight,
			"billable_weight": result.BillableWeight,
			"rate_per_lb":     result.RatePerLb,
			"base_cost":       result.BaseCost,
			"surcharge":       result.Surcharge,
			"door_to_door_fee": result.DoorToDoorFee,
			"insurance":       result.Insurance,
			"volume_discount": result.VolumeDiscount,
			"total":           result.Total,
			"minimum_applied": result.MinimumApplied,
		},
	})
}
