package main

import (
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/static"
)

func main() {
	_ = godotenv.Load() // load .env if present (ignore error)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "file:qcs.db?_journal_mode=WAL"
	}
	if err := db.Connect(dbURL); err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	webRoot := static.Web

	staticOrIndex := func(path string) ([]byte, error) {
		if path == "" || path == "/" {
			path = "index.html"
		}
		if path == "dashboard" || path == "dashboard/" {
			path = "dashboard/index.html"
		} else if strings.HasPrefix(path, "dashboard/inbox/") && len(path) > len("dashboard/inbox/") {
			path = "dashboard/inbox-detail.html"
		} else if strings.HasPrefix(path, "dashboard/ship-requests/") {
			rest := path[len("dashboard/ship-requests/"):]
			if rest != "" && !strings.Contains(rest, "/") {
				path = "dashboard/ship-request-detail.html"
			} else if strings.HasSuffix(rest, "/customs") || rest == "customs" {
				path = "dashboard/customs.html"
			} else if strings.HasSuffix(rest, "/pay") {
				path = "dashboard/pay.html"
			} else if strings.HasSuffix(rest, "/confirmation") {
				path = "dashboard/confirmation.html"
			}
		} else if strings.HasPrefix(path, "dashboard/inbound/") && len(path) > len("dashboard/inbound/") {
			path = "dashboard/inbound-detail.html"
		} else if path == "dashboard/bookings/new" || path == "dashboard/bookings/new/" {
			path = "dashboard/booking-wizard.html"
		} else if strings.HasPrefix(path, "dashboard/bookings/") && len(path) > len("dashboard/bookings/") {
			rest := path[len("dashboard/bookings/"):]
			if rest != "" && !strings.Contains(rest, "/") {
				path = "dashboard/booking-detail.html"
			}
		}
		if path != "" && !strings.HasSuffix(path, ".html") && !strings.Contains(path, ".") {
			path = path + ".html"
		}
		return fs.ReadFile(webRoot, path)
	}

	app := fiber.New(fiber.Config{
		DisableStartupMessage: os.Getenv("PORT") == "",
		ErrorHandler:          api.ErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{Format: "${time} ${status} ${method} ${path} ${latency} ${requestid}\n"}))

	// Stripe webhook (no v1, raw body for signature verification)
	apiGroup := app.Group("/api")
	api.RegisterStripeWebhook(apiGroup)

	// API v1
	v1 := app.Group("/api/v1")
	api.RegisterHealth(v1)
	api.RegisterAuth(v1)
	api.RegisterPublic(v1)
	api.RegisterLocker(v1)
	api.RegisterRecipients(v1)
	api.RegisterShipRequests(v1)
	api.RegisterShipments(v1)
	api.RegisterInvoices(v1)
	api.RegisterTemplates(v1)
	api.RegisterBookings(v1)
	api.RegisterInboundTracking(v1)
	v1.Get("/me", middleware.RequireAuth, api.Me)
	api.RegisterMe(v1)
	api.RegisterNotifications(v1)
	api.RegisterSessions(v1)
	api.RegisterAccount(v1)

	// Serve WASM and Go runtime (from disk so dev can build frontend separately)
	app.Get("/wasm_exec.js", func(c *fiber.Ctx) error {
		return c.SendFile("./web/wasm_exec.js", false)
	})
	app.Get("/app.wasm", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/wasm")
		return c.SendFile("./web/app.wasm", false)
	})
	// Static and SPA fallback from embed
	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path == "" {
			path = "index.html"
		}
		data, err := staticOrIndex(path)
		if err != nil {
			data, _ = fs.ReadFile(webRoot, "index.html")
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.Send(data)
		}
		c.Set("Content-Type", contentType(path))
		return c.Send(data)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("QCS Cargo server listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}

func contentType(path string) string {
	switch {
	case len(path) > 4 && path[len(path)-4:] == ".css":
		return "text/css"
	case len(path) > 3 && path[len(path)-3:] == ".js":
		return "application/javascript"
	case len(path) > 4 && path[len(path)-4:] == ".wasm":
		return "application/wasm"
	case len(path) > 4 && path[len(path)-4:] == ".ico":
		return "image/x-icon"
	case len(path) > 4 && path[len(path)-4:] == ".png":
		return "image/png"
	case len(path) > 4 && path[len(path)-4:] == ".svg":
		return "image/svg+xml"
	default:
		return "text/html; charset=utf-8"
	}
}
