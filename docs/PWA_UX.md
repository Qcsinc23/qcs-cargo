# PWA and UX Enhancements

Wave 11 upgrades the frontend/PWA baseline with the following MVP capabilities:

- Routed WASM shell in [frontend/main.go](/Users/sherwyngraham/development/Qcs%20Cargo%20Next/frontend/main.go)
- Shared dashboard utility shell in `internal/static/dashboard/pwa-shell.js`
- Dark mode persistence via `localStorage`
- Locale scaffold (`en`, `es`)
- Shared loading and empty-state renderers
- Keyboard shortcuts and utility dock
- Notification SSE client helper
- Push-subscription helper
- Expanded service worker offline shell caching and queued warehouse action replay

Current limits:

- The production customer/admin HTML flows remain the primary interface.
- The WASM shell is intentionally lightweight and supplements the static dashboard rather than replacing it.
- Push subscription registration is persisted, but actual Web Push delivery transport is not yet connected to an external provider.
