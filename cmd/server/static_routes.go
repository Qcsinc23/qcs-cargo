package main

import (
	"io/fs"
	"strings"
)

// resolveStaticPath maps app routes to embedded static asset paths.
func resolveStaticPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	if path == "" || path == "/" {
		path = "index.html"
	}
	if path == "dashboard" || path == "dashboard/" {
		path = "dashboard/index.html"
	} else if path == "admin" || path == "admin/" {
		path = "admin/index.html"
	} else if strings.HasPrefix(path, "admin/") {
		// /admin/ship-requests -> admin/ship-requests.html; /admin/users/123 -> admin/users.html;
		// /admin/common.js -> admin/common.js (file ext preserved);
		// Phase 2.4 / 3.1: /admin/scripts/<page>.js -> admin/scripts/<page>.js (extracted inline blocks).
		rest := path[len("admin/"):]
		if rest != "" {
			lastSegment := rest
			if i := strings.LastIndex(rest, "/"); i >= 0 {
				lastSegment = rest[i+1:]
			}
			if strings.Contains(lastSegment, ".") {
				// File reference (e.g. admin/scripts/users.js); leave as-is.
			} else if strings.Contains(rest, "/") {
				segment := rest[:strings.Index(rest, "/")]
				path = "admin/" + segment + ".html"
			} else {
				path = "admin/" + rest + ".html"
			}
		}
	} else if path == "warehouse" || path == "warehouse/" {
		path = "warehouse/index.html"
	} else if strings.HasPrefix(path, "warehouse/") {
		rest := path[len("warehouse/"):]
		if rest != "" {
			lastSegment := rest
			if i := strings.LastIndex(rest, "/"); i >= 0 {
				lastSegment = rest[i+1:]
			}
			// File reference (e.g. warehouse/scripts/staging.js); leave as-is.
			if !strings.Contains(lastSegment, ".") && !strings.Contains(rest, "/") {
				segment := strings.TrimSuffix(rest, "/")
				switch segment {
				case "index", "locker-receive", "receiving", "service-queue", "ship-queue", "packages", "staging", "manifests", "exceptions":
					path = "warehouse/" + segment + ".html"
				}
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
	} else if strings.HasPrefix(path, "dashboard/shipments/") && len(path) > len("dashboard/shipments/") {
		// Dashboard UX review fix: list pages link to /dashboard/shipments/:id
		// but the static router previously had no rule, so clicks 404'd.
		rest := path[len("dashboard/shipments/"):]
		if rest != "" && !strings.Contains(rest, "/") {
			path = "dashboard/shipment-detail.html"
		}
	} else if strings.HasPrefix(path, "dashboard/invoices/") && len(path) > len("dashboard/invoices/") {
		// Same fix for /dashboard/invoices/:id.
		rest := path[len("dashboard/invoices/"):]
		if rest != "" && !strings.Contains(rest, "/") {
			path = "dashboard/invoice-detail.html"
		}
	}
	if path != "" && !strings.HasSuffix(path, ".html") && !strings.Contains(path, ".") {
		path = path + ".html"
	}
	return path
}

func readStaticAsset(webRoot fs.FS, path string) ([]byte, error) {
	return fs.ReadFile(webRoot, resolveStaticPath(path))
}
