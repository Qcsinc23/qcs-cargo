package api

import (
	"fmt"
	"log"
	"math"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/balance"
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
	// TODO: send via Resend when RESEND_API_KEY set
	log.Printf("[Contact] %s <%s> %s: %s", body.Name, body.Email, body.Subject, body.Message)
	if os.Getenv("RESEND_API_KEY") != "" {
		// Resend send would go here
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

// shippingCalculator applies the PRD 8.9 pricing formula.
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

	destRates := map[string]float64{
		"guyana": 3.50, "jamaica": 3.75, "trinidad": 3.50, "barbados": 4.00, "suriname": 4.25,
	}
	rate, ok := destRates[destID]
	if !ok {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "unknown destination"))
	}

	parseF := func(s string) float64 {
		var v float64
		fmt.Sscanf(s, "%f", &v)
		return v
	}

	weight := parseF(weightStr)
	L, W, H := parseF(lStr), parseF(wStr), parseF(hStr)
	declaredValue := parseF(valueStr)

	if weight <= 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "weight must be > 0"))
	}

	dimWeight := 0.0
	if L > 0 && W > 0 && H > 0 {
		dimWeight = (L * W * H) / 166.0
	}
	billable := weight
	if dimWeight > billable {
		billable = dimWeight
	}

	base := billable * rate
	surcharge := 0.0
	d2d := 0.0
	switch service {
	case "express":
		surcharge = base * 0.25
	case "door_to_door":
		d2d = 25.0
	}

	insurance := declaredValue / 100.0
	total := base + surcharge + d2d + insurance
	if total < 10 {
		total = 10
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"destination_id":   destID,
			"service":          service,
			"actual_weight":    weight,
			"dim_weight":       dimWeight,
			"billable_weight":  billable,
			"rate_per_lb":      rate,
			"base_cost":        math.Round(base*100) / 100,
			"surcharge":        math.Round(surcharge*100) / 100,
			"door_to_door_fee": d2d,
			"insurance":        math.Round(insurance*100) / 100,
			"total":            math.Round(total*100) / 100,
			"minimum_applied":  total == 10,
		},
	})
}
