# PWA and UX

## Shared dashboard shell (current state)

Every signed-in dashboard page renders through one shared shell defined in `internal/static/dashboard/pwa-shell.js` and `pwa-shell.css`. As of 2026-04-19 the unified shell is applied to every customer tab — `dashboard`, `inbox`, `inbox-detail`, `mailbox`, `inbound`, `inbound-detail`, `ship`, `ship-requests`, `ship-request-detail`, `customs`, `shipments`, `shipment-detail`, `bookings`, `booking-detail`, `booking-wizard`, `invoices`, `invoice-detail`, `recipients`, `templates`, `parcel-plus`, `pay`, `confirmation`, `profile`, `settings`, `settings/notifications`, `settings/security`, `settings/sessions`, `settings/delete-account`.

Each page gets:

- `QCSPWA.renderSidebar(activeKey)` — single source of truth for the customer sidebar. Adding or relabeling a tab is a one-line change in `pwa-shell.js`.
- `qcs-page-wrap` + `qcs-page-main` layout, skip link, and focused `<main>` on load for accessibility.
- Tailwind loaded with `defer`, Inter + Plus Jakarta fonts, design tokens from `pwa-shell.css`.
- `QCSPWA.fetchJson` — fetch wrapper with bearer auth, JSON body/parse, one-shot 401 refresh via `/api/v1/auth/refresh`, and a uniform `QCSHttpError` surface.
- `QCSPWA.bindLogout`, `toast`, `statusBadge`, `formatMoney`, `formatDate`, `safeURL`, `copyToClipboard`.
- `QCSPWA.mountLoading` / `mountError` / `renderEmptyState` for the three universal states. Failed loads show an accessible error card with retry instead of a stuck "Loading..." string.
- `QCSPWA.initBase({ registerSW, keyboard, utilityDock })` — registers the service worker, installs keyboard shortcuts, and mounts the floating theme/locale/help dock.
- All server-supplied strings are HTML-escaped via `window.qcsEscapeHTML`.

## Sidebar tabs (single source of truth)

Defined in `pwa-shell.js#SIDEBAR_LINKS`. Customer-facing order:

1. Dashboard — `/dashboard`
2. My Packages — `/dashboard/inbox`
3. My Mailbox — `/dashboard/mailbox`
4. Expected Packages — `/dashboard/inbound`
5. Ship a Package — `/dashboard/ship` (CTA style)
6. Ship Requests — `/dashboard/ship-requests`
7. Shipments — `/dashboard/shipments`
8. Bookings — `/dashboard/bookings`
9. Recipients — `/dashboard/recipients`
10. Templates — `/dashboard/templates`
11. Parcel+ — `/dashboard/parcel-plus`
12. Invoices — `/dashboard/invoices`
13. Profile — `/dashboard/profile`
14. Settings — `/dashboard/settings`

Settings nest: `notifications`, `security`, `sessions`, `delete-account`.

## Wave 11 baseline (still in place)

The original wave-11 PWA/UX baseline still ships:

- Dark mode persistence via `localStorage` (`qcs_theme`).
- Locale scaffold (`en`, `es`) with `qcs_locale` persistence and `<html lang>` toggling.
- Keyboard shortcuts (`g d` / `g i` / `g s` / `g n`, `/` to focus search, `t` for theme, `?` for help) and a floating utility dock.
- Notification SSE client helper (`openNotificationStream`).
- Push subscription helper (`subscribePush`) backed by `POST /api/v1/notifications/push/subscribe`.
- Service worker offline shell caching and queued warehouse action replay (see `internal/static/sw.js`).

## Current limits

- The production customer flows are server-rendered HTML enhanced by `QCSPWA`. `frontend/main.go` ships a routed WASM PWA shell at the same origin for offline/PWA hosting, but the canonical UI is the shared shell described above.
- Push subscription registration is persisted, but actual Web Push delivery transport is not yet connected to an external provider.
