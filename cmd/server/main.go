package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
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
	// Pass 2 audit fix M-8: in production, refuse to start if Stripe is
	// configured but the webhook signing secret is missing. Without the
	// secret the webhook handler returns 503 to every Stripe event, which
	// silently breaks payment reconciliation and triggers Stripe's 3-day
	// retry storm.
	if services.IsProductionRuntime() &&
		strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")) == "" {
		log.Fatal("STRIPE_WEBHOOK_SECRET must be set when STRIPE_SECRET_KEY is configured in production")
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

	// Background jobs: storage fee + expiry notifier once per day (and once on startup for testing).
	// Phase 3.2: also drain the outbound_emails queue every minute.
	go runDailyJobs()
	go runOutboundEmailWorker()

	webRoot := static.Web

	app := fiber.New(fiber.Config{
		DisableStartupMessage: os.Getenv("PORT") == "",
		ErrorHandler:          api.ErrorHandler,
		// Pass 2 audit fix L-1: explicit body limit so the configured
		// per-feature limits (e.g. 5 MB avatar) are not silently capped by
		// the framework's 4 MiB default.
		BodyLimit: 8 << 20, // 8 MiB
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
			// DEF-010 (backlog) fix: only serve the SPA fallback HTML
			// when the unknown URL is plausibly a customer-facing client
			// route. Anything else (typos, scanner probes, asset misses)
			// gets a real 404 status so observability tools and search
			// engines see the failure instead of a 200 with the marketing
			// page.
			if isClientSPAFallback(path) {
				data, _ = fs.ReadFile(webRoot, "index.html")
				c.Set("Content-Type", "text/html; charset=utf-8")
				return c.Send(data)
			}
			c.Set("Content-Type", "text/html; charset=utf-8")
			c.Status(404)
			notFound, _ := fs.ReadFile(webRoot, "index.html")
			return c.Send(notFound)
		}
		if strings.HasPrefix(path, "dashboard/") || path == "dashboard" || strings.HasPrefix(path, "verify") {
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
		} else {
			setAssetCacheHeaders(c, path)
			// Pass 2.5 HIGH-09: emit ETag + Last-Modified so revalidating
			// clients get a 304 instead of a 200 + full body. Reduces
			// bandwidth on every dashboard load past the 5-minute cache
			// window. Skipped for the dashboard SPA shell HTML (no-store)
			// and for the SPA fallback path; those need to actually
			// re-render.
			etag := computeAssetETag(path, data)
			c.Set("ETag", etag)
			c.Set("Last-Modified", startTime.Format(http.TimeFormat))
			if match := c.Get("If-None-Match"); match != "" && match == etag {
				return c.SendStatus(304)
			}
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

// QAL-003 (backlog) fix: rely on Go's mime package for extension lookup
// instead of hand-rolled slice arithmetic, plus an override map for the
// few content types we need to ensure are stable across platforms (some
// hosts have unexpected entries in /etc/mime.types).
var contentTypeOverrides = map[string]string{
	".js":    "application/javascript",
	".mjs":   "application/javascript",
	".wasm":  "application/wasm",
	".css":   "text/css",
	".svg":   "image/svg+xml",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

func contentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ct, ok := contentTypeOverrides[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "text/html; charset=utf-8"
}

// setAssetCacheHeaders applies cache headers per the DEF-002 fix: until
// assets carry a content-hash in their filename, they MUST NOT be marked
// immutable. The previous policy combined no-store HTML with year-long
// immutable JS/CSS, so every redeploy with new HTML pointing at unchanged
// JS/CSS URLs left clients running stale code for up to a year.
//
// Only assets matching the hashed filename pattern (e.g. tailwind.<sha>.css)
// are served as immutable. All other static assets get a short revalidating
// max-age; browsers will conditional-GET them via ETag/Last-Modified, which
// Fiber sets automatically for the embedded FS.
func setAssetCacheHeaders(c *fiber.Ctx, path string) {
	cdnBase := strings.TrimSpace(os.Getenv("CDN_BASE_URL"))
	if cdnBase != "" {
		c.Set("X-CDN-Base-URL", cdnBase)
	}
	if isHashedAsset(path) {
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	switch {
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".svg"),
		strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"),
		strings.HasSuffix(path, ".webp"), strings.HasSuffix(path, ".wasm"), strings.HasSuffix(path, ".ico"),
		strings.HasSuffix(path, ".woff"), strings.HasSuffix(path, ".woff2"):
		c.Set("Cache-Control", "public, max-age=300, must-revalidate")
	default:
		if c.Get("Cache-Control") == "" {
			c.Set("Cache-Control", "public, max-age=300")
		}
	}
}

// hashedAssetRe matches filenames that embed a content hash, e.g.
// "tailwind.a1b2c3d4.css" or "app.0123abcd.wasm". Only these may be served
// as immutable; other static assets must revalidate.
var hashedAssetRe = regexp.MustCompile(`\.[0-9a-f]{8,}\.[a-z0-9]+$`)

func isHashedAsset(path string) bool {
	return hashedAssetRe.MatchString(strings.ToLower(path))
}

// Pass 2.5 HIGH-09 fix: cache content ETags for embed.FS-served assets.
// Embed.FS contents are immutable per binary, so an ETag computed once
// per (path, content) is stable for the process lifetime. The map key is
// the resolved path (e.g. "css/tailwind.css"); the value is the
// already-formatted ETag header value, e.g. `"sha256-abc123..."`.
var assetETagCache sync.Map // map[string]string

// computeAssetETag returns the cached ETag for path or computes and
// caches it from data. Returns the value formatted ready for the
// `ETag` response header (including the surrounding double quotes).
func computeAssetETag(path string, data []byte) string {
	if v, ok := assetETagCache.Load(path); ok {
		return v.(string)
	}
	sum := sha256.Sum256(data)
	etag := `"sha256-` + base64.RawURLEncoding.EncodeToString(sum[:])[:22] + `"`
	assetETagCache.Store(path, etag)
	return etag
}

// startTime is set at process start; used as Last-Modified for embed.FS
// assets since the embedded file system has no per-file mtime and the
// content is immutable per binary.
var startTime = time.Now().UTC()

// isClientSPAFallback reports whether an unknown path should be served
// the marketing landing page (200 OK), instead of a real 404. We scope
// this aggressively: only paths that have no extension and live under
// the customer-facing prefixes that the JS routers might own.
func isClientSPAFallback(path string) bool {
	if strings.Contains(path, ".") {
		return false
	}
	switch {
	case path == "" || path == "/":
		return true
	case strings.HasPrefix(path, "dashboard"):
		return true
	case strings.HasPrefix(path, "warehouse"):
		return true
	case strings.HasPrefix(path, "admin"):
		return true
	case strings.HasPrefix(path, "verify"):
		return true
	}
	return false
}

// runDailyJobs is the supervisor for background jobs that the single-replica
// deployment relies on. DEF-005 fix: each job is now wrapped in a recover()
// so a panic in one job cannot kill the entire daily loop, and each
// invocation publishes a Prometheus gauge so missed runs are observable.
func runDailyJobs() {
	middleware.EnsureMetricsRegistered()
	runOnce := func() {
		runJob("storage_fee", func(ctx context.Context) error {
			return jobs.RunStorageFeeJob(ctx)
		})
		runJob("expiry_notifier", func(ctx context.Context) error {
			return jobs.RunExpiryNotifierJob(ctx)
		})
	}
	runOnce() // run once on startup
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		runOnce()
	}
}

// runOutboundEmailWorker drains the outbound_emails queue (Phase 3.2 /
// INC-001 part B). Frequent invocation (every minute) keeps email
// latency low without overwhelming the provider; the worker itself
// claims a small batch per pass and applies bounded retry per row.
func runOutboundEmailWorker() {
	middleware.EnsureMetricsRegistered()
	tick := func() { runJob("outbound_email", jobs.RunOutboundEmailJob) }
	tick()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		tick()
	}
}

func runJob(name string, fn func(ctx context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[jobs] %s: panic recovered: %v\n%s", name, r, debug.Stack())
			middleware.RecordDailyJobPanic(name)
			middleware.RecordDailyJobRun(name, "error")
		}
	}()
	ctx := context.Background()
	if err := fn(ctx); err != nil {
		log.Printf("[jobs] %s: %v", name, err)
		middleware.RecordDailyJobRun(name, "error")
		return
	}
	middleware.RecordDailyJobRun(name, "success")
}
