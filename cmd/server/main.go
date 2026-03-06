package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/jobs"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/static"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load() // load .env if present (ignore error)
	if services.IsProductionRuntime() && len(strings.TrimSpace(os.Getenv("JWT_SECRET"))) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters in production")
	}
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

	app := fiber.New(fiber.Config{
		DisableStartupMessage: os.Getenv("PORT") == "",
		ErrorHandler:          api.ErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(middleware.MetricsMiddleware)
	app.Use(logger.New(logger.Config{Format: "${time} ${status} ${method} ${path} ${latency} ${requestid}\n"}))
	app.Use(middleware.SecurityHeaders) // Add security headers after logger
	app.Use(compress.New())             // Safe-default response compression for API and static responses

	// CORS: ALLOWED_ORIGINS required in production; empty = allow all in dev
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	isProduction := services.IsProductionRuntime()

	if isProduction && allowedOrigins == "" {
		log.Fatal("ALLOWED_ORIGINS must be set in production (comma-separated list of allowed domains)")
	}

	corsConfig := cors.Config{
		AllowMethods: "GET,POST,PATCH,PUT,DELETE,OPTIONS",
		AllowHeaders: "Authorization,Content-Type",
		AllowOrigins: allowedOrigins,
	}
	if allowedOrigins == "" {
		// Dev mode: allow all
		corsConfig.AllowOrigins = "*"
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
		setAssetCacheHeaders(c, "wasm_exec.js")
		return c.SendFile("./web/wasm_exec.js", false)
	})
	app.Get("/app.wasm", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/wasm")
		setAssetCacheHeaders(c, "app.wasm")
		return c.SendFile("./web/app.wasm", false)
	})
	// Serve /web/* (images, etc.) from disk so frontend placeholders at /web/images/... resolve
	app.Static("/web", "./web")
	// Prometheus scraping endpoint (public, no auth).
	app.Get("/metrics", middleware.MetricsHandler)
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
		data, err := readStaticAsset(webRoot, path)
		if err != nil {
			data, _ = fs.ReadFile(webRoot, "index.html")
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.Send(data)
		}
		if strings.HasPrefix(path, "dashboard/") || path == "dashboard" || strings.HasPrefix(path, "verify") {
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
		} else {
			setAssetCacheHeaders(c, path)
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

func setAssetCacheHeaders(c *fiber.Ctx, path string) {
	cdnBase := strings.TrimSpace(os.Getenv("CDN_BASE_URL"))
	if cdnBase != "" {
		c.Set("X-CDN-Base-URL", cdnBase)
	}
	switch {
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".svg"),
		strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"),
		strings.HasSuffix(path, ".webp"), strings.HasSuffix(path, ".wasm"), strings.HasSuffix(path, ".ico"):
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		if c.Get("Cache-Control") == "" {
			c.Set("Cache-Control", "public, max-age=300")
		}
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
