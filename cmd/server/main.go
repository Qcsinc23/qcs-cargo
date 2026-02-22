package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/jobs"
	"github.com/Qcsinc23/qcs-cargo/internal/static"
)

func main() {
	_ = godotenv.Load() // load .env if present (ignore error)
	if os.Getenv("RESEND_API_KEY") != "" {
		log.Print("Resend: configured (transactional email enabled)")
	} else {
		log.Print("Resend: not configured (magic link and contact form will log only)")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "file:qcs.db?_journal_mode=WAL"
	}
	if err := db.Connect(dbURL); err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	// Background jobs: storage fee + expiry notifier once per day (and once on startup for testing)
	go runDailyJobs()

	webRoot := static.Web

	staticOrIndex := func(path string) ([]byte, error) {
		if path == "" || path == "/" {
			path = "index.html"
		}
		if path == "dashboard" || path == "dashboard/" {
			path = "dashboard/index.html"
		} else if path == "admin" || path == "admin/" {
			path = "admin/index.html"
		} else if strings.HasPrefix(path, "admin/") {
			// /admin/ship-requests -> admin/ship-requests.html; /admin/users/123 -> admin/users.html; /admin/common.js -> admin/common.js
			rest := path[len("admin/"):]
			if rest != "" {
				if strings.Contains(rest, "/") {
					// e.g. users/123 -> serve admin/users.html
					segment := rest[:strings.Index(rest, "/")]
					path = "admin/" + segment + ".html"
				} else if !strings.Contains(rest, ".") {
					path = "admin/" + rest + ".html"
				}
				// else rest is e.g. common.js -> path stays admin/common.js
			}
		} else if path == "warehouse" || path == "warehouse/" {
			path = "warehouse/index.html"
		} else if strings.HasPrefix(path, "warehouse/") {
			rest := path[len("warehouse/"):]
			if rest != "" && !strings.Contains(rest, "/") && !strings.Contains(rest, ".") {
				segment := strings.TrimSuffix(rest, "/")
				switch segment {
				case "index", "locker-receive", "receiving", "service-queue", "ship-queue", "packages", "staging", "manifests", "exceptions":
					path = "warehouse/" + segment + ".html"
				}
			}
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

	// CORS: ALLOWED_ORIGINS empty = allow all (dev); comma-separated list in production
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	corsConfig := cors.Config{
		AllowMethods: "GET,POST,PATCH,PUT,DELETE,OPTIONS",
		AllowHeaders: "Authorization,Content-Type",
	}
	if allowedOrigins != "" {
		corsConfig.AllowOrigins = allowedOrigins
	}
	app.Use(cors.New(corsConfig))

	// Static asset: admin common.js from embed (before any wildcard so GET /admin/common.js wins)
	app.Get("/admin/common.js", func(c *fiber.Ctx) error {
		data, err := fs.ReadFile(webRoot, "admin/common.js")
		if err != nil {
			return c.Status(404).SendString("Not found")
		}
		c.Set("Content-Type", "application/javascript")
		return c.Send(data)
	})

	// API v1 + Stripe webhook (rate limit 100 req/min per IP; health excluded)
	api.RegisterAPIRoutes(app)

	// Uploaded files (avatars, etc.). UPLOAD_DIR defaults to ./uploads.
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	app.Static("/uploads", uploadDir)

	// Serve WASM and Go runtime (from disk so dev can build frontend separately)
	app.Get("/wasm_exec.js", func(c *fiber.Ctx) error {
		return c.SendFile("./web/wasm_exec.js", false)
	})
	app.Get("/app.wasm", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/wasm")
		return c.SendFile("./web/app.wasm", false)
	})
	// Serve /web/* (images, etc.) from disk so frontend placeholders at /web/images/... resolve
	app.Static("/web", "./web")
	// Static and SPA fallback from embed (do not serve HTML for unknown API paths)
	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if strings.HasPrefix(path, "api/") {
			return c.Status(404).JSON(fiber.Map{"status": "error", "error_code": "NOT_FOUND", "message": "Not found"})
		}
		if path == "" {
			path = "index.html"
		}
		// Serve admin/common.js with application/javascript so admin pages load the script correctly
		if path == "admin/common.js" {
			data, err := fs.ReadFile(webRoot, "admin/common.js")
			if err != nil {
				data, _ = fs.ReadFile(webRoot, "index.html")
				c.Set("Content-Type", "text/html; charset=utf-8")
				return c.Send(data)
			}
			c.Set("Content-Type", "application/javascript")
			return c.Send(data)
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

func runDailyJobs() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	run := func() {
		ctx := context.Background()
		if err := jobs.RunStorageFeeJob(ctx); err != nil {
			log.Printf("[jobs] RunStorageFeeJob: %v", err)
		}
		if err := jobs.RunExpiryNotifierJob(ctx); err != nil {
			log.Printf("[jobs] RunExpiryNotifierJob: %v", err)
		}
	}
	run() // run once on startup
	for range ticker.C {
		run()
	}
}
