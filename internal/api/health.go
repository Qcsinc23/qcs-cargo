package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
)

var startTime = time.Now()

// RegisterHealth mounts GET /health on the given group.
func RegisterHealth(g fiber.Router) {
	g.Get("/health", healthHandler)
}

func healthHandler(c *fiber.Ctx) error {
	status := "ok"
	dbStatus := "ok"
	if err := db.Ping(); err != nil {
		dbStatus = "error"
		status = "degraded"
	}
	return c.JSON(fiber.Map{
		"status": status,
		"db":     dbStatus,
		"uptime": time.Since(startTime).String(),
	})
}
