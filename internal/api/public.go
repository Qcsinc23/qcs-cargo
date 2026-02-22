package api

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
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
	g.Post("/contact", contactForm)
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
			"status": "operational",
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
