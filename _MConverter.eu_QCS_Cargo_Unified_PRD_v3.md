**QCS CARGO**

Unified Product Requirements Document

Parcel Forwarding · Air Freight · Warehouse Operations

Version 3.0 \| February 2026

Quiet Craft Solutions Inc.

Stack: go-app (WASM PWA) + Go Fiber + SQLite/Postgres

Build Method: AI-Agentic Development

1\. Executive Summary

QCS Cargo is a parcel forwarding and white-glove air freight service connecting the United States to the Caribbean. Customers receive a personal US mailing address at QCS Cargo\'s New Jersey warehouse. They shop at any US online store, and packages are received, stored, and forwarded to Guyana, Jamaica, Trinidad & Tobago, Barbados, and Suriname on the customer\'s schedule.

This PRD defines the complete build of the QCS Cargo platform as a unified Go stack: a go-app WebAssembly PWA frontend backed by a Go Fiber API server with SQLite (or PostgreSQL) for persistence.

1.1 Two Intake Models

QCS Cargo supports two workflows that converge at the warehouse:

> **Primary: Parcel Forwarding (Locker Flow)**
>
> Customer signs up, gets a US address with a unique suite code, shops online, packages arrive automatically at the warehouse, are stored in the customer\'s \'locker,\' and are shipped when the customer creates a Ship Request. This is the core business.
>
> **Secondary: Drop-Off Shipping (Booking Flow)**
>
> Customer physically brings packages to the warehouse on a scheduled date/time. This serves walk-in customers, large items, and items not purchased online. Retained from the legacy system.

Both flows share the same warehouse operations pipeline: receiving, staging, manifesting, and shipping. The customer dashboard surfaces both workflows but leads with the parcel forwarding (locker) experience.

1.2 Migration Rationale

**From:** SvelteKit + PocketBase (JavaScript/TypeScript)

**To:** go-app (Go WASM PWA) + Fiber (Go HTTP) + SQLite/PostgreSQL

- Unified Go stack: One language for frontend (WASM), backend, and tooling. Shared validation, models, and enums between client and server.

- Performance: Go compiles to native-speed WASM. Fiber is among the fastest Go HTTP frameworks. SQLite with WAL mode is ideal for single-server deployment.

- Offline-first PWA: go-app provides built-in service worker generation. Critical for warehouse receiving and scanning that must work without network.

- AI-agentic development: Go\'s strict typing, explicit error handling, and lack of hidden magic make it exceptionally suited for AI-assisted code generation.

- Deployment simplicity: Single binary (API server) plus static WASM bundle. No Node.js runtime, no npm, no build toolchain on the server.

1.3 Success Criteria

|                   |                           |                                       |
|-------------------|---------------------------|---------------------------------------|
| **Goal**          | **Metric**                | **Target**                            |
| Feature parity    | Routes implemented        | 100% of current + forwarding features |
| Performance       | Lighthouse PWA score      | \> 90                                 |
| Offline warehouse | Receiving without network | Full scan/weigh/photo flow            |
| Bundle size       | Initial WASM + assets     | \< 8 MB gzipped                       |
| API response time | p95 latency               | \< 100ms (local SQLite)               |
| Uptime            | Monthly availability      | 99.9%                                 |

1.4 Non-Goals

- Native mobile apps (iOS/Android). The PWA covers mobile use cases.

- Real-time WebSocket push. Use polling or SSE for status updates.

- Multi-region deployment. Single-server with SQLite is the initial target.

- Assisted purchase service (buying on behalf of customers). Deferred to a future release.

2\. Technology Stack

2.1 Stack Overview

|               |                               |                                                                    |
|---------------|-------------------------------|--------------------------------------------------------------------|
| **Layer**     | **Technology**                | **Role**                                                           |
| Frontend      | go-app v10+                   | Go-to-WASM PWA framework. Components, routing, lifecycle.          |
| Styling       | Tailwind CSS (CDN)            | Utility-first CSS loaded via CDN link, not compiled.               |
| Icons         | Lucide (SVG)                  | Inline SVG icons. Shipped as Go embed constants.                   |
| Backend API   | Fiber v2                      | Express-inspired Go HTTP framework. Fast, middleware-rich.         |
| Database      | SQLite (pure Go) / PostgreSQL | SQLite for dev + single-server prod. Postgres for scale-out.       |
| ORM / Query   | sqlc                          | Type-safe SQL. Write SQL, generate Go structs and query functions. |
| Migrations    | goose                         | SQL migration files. Up/down. Embeddable in binary.                |
| Auth          | JWT + Magic Link              | Short-lived access tokens, refresh via httpOnly cookie.            |
| Payments      | Stripe Go SDK                 | Server-side PaymentIntent creation and webhook handling.           |
| Email         | Resend Go SDK                 | Transactional emails (magic links, confirmations, notifications).  |
| Storage       | Local filesystem / S3         | Package photos, avatars, invoice PDFs.                             |
| PWA / Offline | go-app service worker         | Auto-generated SW. Custom cache strategies for warehouse.          |
| Testing       | Go testing + testify          | Unit, integration, and HTTP handler tests.                         |
| CI/CD         | GitHub Actions                | Lint, test, build WASM, build binary, deploy.                      |

2.2 Project Structure

Monorepo with frontend and backend together:

- cmd/server/main.go --- Entry point. Starts Fiber, serves WASM + API.

- cmd/migrate/main.go --- Database migration runner.

- internal/api/ --- Fiber route handlers (auth, bookings, locker, ship-requests, admin, warehouse).

- internal/db/ --- sqlc generated code, migration SQL files, connection logic.

- internal/models/ --- Shared domain types used by both API and frontend.

- internal/services/ --- Business logic (pricing, notifications, storage fees, payments).

- internal/middleware/ --- Auth, RBAC, rate limiting, request logging.

- internal/jobs/ --- Background jobs (storage fee processor, inbound tracking poller).

- frontend/pages/ --- One file per route.

- frontend/components/ --- Reusable UI (header, sidebar, forms, tables, modals, toasts).

- frontend/stores/ --- Client-side state (auth, locker selection, booking wizard, offline queue).

- frontend/static/ --- CSS, images, manifest.json.

- sql/ --- Raw SQL for sqlc, organized by domain.

- sql/migrations/ --- Goose migration files.

3\. System Architecture

3.1 High-Level Architecture

> **Browser (WASM PWA)**
>
> go-app compiles Go components to WebAssembly. Service worker caches static assets and warehouse data for offline use. All API calls use fetch(). Auth tokens stored in httpOnly cookies.
>
> **API Server (Fiber)**
>
> Single binary serves WASM app as static files AND handles /api/\* routes. Middleware stack: Logger \> RequestID \> CORS \> RateLimit \> Auth \> RBAC \> Handler.
>
> **Database (SQLite/Postgres)**
>
> All state in DB. WAL mode for concurrent reads. sqlc for type-safe queries.
>
> **Background Jobs**
>
> In-process goroutines for: storage fee processing (daily), inbound tracking polling (4h), storage expiry notifications (daily).
>
> **External Services**
>
> Stripe (payments), Resend (email), S3-compatible storage (photos/files). All server-side only.

3.2 Authentication Flow

3.2.1 Magic Link Flow

1.  User enters email on /login. Frontend POSTs to /api/v1/auth/magic-link/request.

2.  Server generates a single-use token (32-byte random, stored hashed in DB, 10-min expiry).

3.  Server sends email via Resend with link: /verify?token={token}&redirectTo={path}.

4.  User clicks link. Frontend sends token to /api/v1/auth/magic-link/verify.

5.  Server validates, creates session, returns JWT access token (15-min) in body and refresh token (7-day) in httpOnly cookie.

6.  Frontend stores access token in memory (Go variable). Includes in Authorization header.

7.  On 401, frontend calls /api/v1/auth/refresh. If refresh fails, redirect to /login.

3.2.2 Token Structure

|                |                    |              |                      |
|----------------|--------------------|--------------|----------------------|
| **Token**      | **Storage**        | **Lifetime** | **Contains**         |
| Access JWT     | In-memory (Go var) | 15 minutes   | user_id, role, email |
| Refresh JWT    | httpOnly cookie    | 7 days       | session_id           |
| Magic Link     | DB (hashed)        | 10 minutes   | user_id, redirect_to |
| Password Reset | DB (hashed)        | 1 hour       | user_id              |

3.3 Role-Based Access Control

|          |                                                                               |                           |
|----------|-------------------------------------------------------------------------------|---------------------------|
| **Role** | **Access**                                                                    | **Routes**                |
| customer | Own locker, ship requests, bookings, shipments, recipients, invoices, profile | /dashboard/\*             |
| staff    | Warehouse operations, receiving, staging, manifests, service queue            | /warehouse/\*             |
| admin    | Everything: all customers, all data, reports, settings, communications        | /admin/\* + /warehouse/\* |

3.4 Offline Architecture (Warehouse)

3.4.1 Offline-Capable Operations

|                                |                                                     |                                     |
|--------------------------------|-----------------------------------------------------|-------------------------------------|
| **Operation**                  | **Offline Behavior**                                | **Sync Strategy**                   |
| Scan package / read suite code | Camera/manual input works. Lookup cached customers. | Cache customer list on shift start. |
| Record weight / dimensions     | Stored in IndexedDB offline queue.                  | Sync on reconnect, FIFO.            |
| Set condition                  | Stored in IndexedDB queue.                          | Sync with package data.             |
| Capture photos                 | Stored as blobs in IndexedDB.                       | Upload after text data syncs.       |
| View today\'s schedule         | Cached on shift start.                              | Refresh when online.                |

3.4.2 Sync Queue Design

- Queue entry: id (UUID), endpoint, method (POST/PATCH), body (JSON), created_at, retry_count, status (pending/syncing/failed).

- On reconnect (navigator.onLine), sync worker processes oldest-first.

- 3 retries max. Failed entries surfaced in warehouse UI for manual resolution.

- Photos queued separately with parent reference. Sync only after parent text data confirmed.

- Sync Status indicator in warehouse header: green (synced), amber (syncing N), red (N failed).

3.4.3 Cache Warming

On warehouse login, pre-fetch and cache: today\'s expected drop-offs, all active customer suite codes (for offline matching), 200 most recent packages, staging bay configuration, and session data. Refresh every 5 minutes when online.

3.5 State Management (Frontend)

> **AuthStore**
>
> Current user, access token, role. Populated from /api/v1/auth/me on app init.
>
> **LockerSelectionStore**
>
> Tracks which packages the customer has selected in the Package Inbox for shipping. Persists across page navigations within the session.
>
> **ShipWizardStore**
>
> Multi-step ship request form state. Persists to sessionStorage so refresh doesn\'t lose progress.
>
> **BookingWizardStore**
>
> Multi-step booking form state for drop-off flow. Same persistence pattern.
>
> **OfflineQueueStore**
>
> IndexedDB-backed sync queue for warehouse. Exposes QueueLength(), FailedCount(), Sync().
>
> **ToastStore**
>
> Global notification queue. Components call Toast.Success() or Toast.Error().

3.6 Error Handling

3.6.1 API Error Format

All errors: { \"error\": { \"code\": \"BOOKING_NOT_FOUND\", \"message\": \"\...\", \"details\": {} } }

3.6.2 HTTP Status Codes

|          |                         |                                               |
|----------|-------------------------|-----------------------------------------------|
| **Code** | **Meaning**             | **Frontend Behavior**                         |
| 400      | Validation error        | Show inline field errors                      |
| 401      | Unauthenticated         | Attempt refresh. If fails, redirect to /login |
| 403      | Forbidden               | Show \'Access Denied\' page                   |
| 404      | Not found               | Show contextual empty state                   |
| 409      | Conflict                | Show message, suggest refresh                 |
| 422      | Business rule violation | Show error toast                              |
| 429      | Rate limited            | \'Please slow down\' toast                    |
| 500      | Server error            | Generic error toast                           |

3.6.3 Network Failure

- Timeout: 15s normal, 60s uploads.

- Offline (non-warehouse): Persistent banner, disable submissions, re-enable on reconnect.

- Offline (warehouse): Silently queue. Show sync indicator only.

- GET requests: retry 2x with backoff (1s, 3s). Mutations: no auto-retry.

4\. Design System

4.1 Color Palette

Primary

|            |          |                                                    |
|------------|----------|----------------------------------------------------|
| **Name**   | **Hex**  | **Usage**                                          |
| Deep Navy  | \#0F172A | Primary text, headings, sidebar backgrounds        |
| Ocean Blue | \#2563EB | Primary actions, links, active states, focus rings |
| Sky        | \#38BDF8 | Accent highlights, progress bars, hover states     |

Accent

|            |          |                                                                 |
|------------|----------|-----------------------------------------------------------------|
| **Name**   | **Hex**  | **Usage**                                                       |
| Warm Coral | \#F97316 | High-priority CTAs (Ship My Packages, Get Quote), urgent badges |
| Emerald    | \#10B981 | Success states, Delivered badge, positive metrics               |

Neutrals

|           |          |                                            |
|-----------|----------|--------------------------------------------|
| **Name**  | **Hex**  | **Usage**                                  |
| Slate-50  | \#F8FAFC | Page backgrounds                           |
| Slate-100 | \#F1F5F9 | Cards, alternating rows, input backgrounds |
| Slate-200 | \#E2E8F0 | Borders, dividers                          |
| Slate-400 | \#94A3B8 | Secondary text, placeholders               |
| Slate-600 | \#475569 | Body text (lighter variant)                |
| Slate-900 | \#0F172A | Primary text                               |

Status Badges

|            |                     |                     |                                         |
|------------|---------------------|---------------------|-----------------------------------------|
| **Style**  | **Background**      | **Text**            | **Statuses**                            |
| Info       | blue-100 \#DBEAFE   | blue-800 \#1E40AF   | Confirmed, In Transit                   |
| Success    | green-100 \#DCFCE7  | green-800 \#166534  | Delivered, Completed                    |
| Warning    | amber-100 \#FEF3C7  | amber-800 \#92400E  | Pending, Pending Payment, Expiring Soon |
| Danger     | red-100 \#FEE2E2    | red-800 \#991B1B    | Exception, Payment Failed, Cancelled    |
| Neutral    | slate-100 \#F1F5F9  | slate-600 \#475569  | Draft, Disposed                         |
| Processing | purple-100 \#F3E8FF | purple-800 \#6B21A8 | Processing, In Progress                 |

4.2 Typography

|                    |                   |          |            |
|--------------------|-------------------|----------|------------|
| **Element**        | **Font**          | **Size** | **Weight** |
| Hero H1            | Plus Jakarta Sans | 48px     | 800        |
| Page H1            | Plus Jakarta Sans | 32px     | 700        |
| Section H2         | Plus Jakarta Sans | 24px     | 600        |
| Card H3            | Inter             | 18px     | 600        |
| Body               | Inter             | 16px     | 400        |
| Small / Captions   | Inter             | 14px     | 400        |
| Badges / Labels    | Inter             | 12px     | 500        |
| Code / Tracking \# | JetBrains Mono    | 14px     | 400        |

4.3 Component Patterns

Buttons

|             |                         |                                          |
|-------------|-------------------------|------------------------------------------|
| **Variant** | **Style**               | **Use For**                              |
| Primary     | Ocean bg, white text    | Submit, Save, Confirm                    |
| Secondary   | White bg, slate border  | Cancel, Back                             |
| Accent      | Coral bg, white text    | Ship My Packages, Get Quote, New Booking |
| Danger      | Red bg, white text      | Delete, Cancel, destructive              |
| Ghost       | Transparent, ocean text | Inline links, tertiary                   |

Height: 40px default, 36px small, 48px large. Weight 500. Hover: darken 10%. Disabled: 50% opacity.

Cards, Forms, Tables, Modals, Toasts

- Cards: white bg, 1px slate-200 border, rounded-xl (12px), shadow-sm. Hover: shadow-md + translateY(-1px).

- Form inputs: 40px height, rounded-lg (8px), focus ring-2 ocean/20. Error: red border + message below.

- Tables: slate-50 header, alternating rows, hover slate-100. Stack to cards below 768px.

- Modals: black/50 backdrop, white rounded-xl, max-width 480px (confirm) or 640px (form).

- Toasts: top-right, max 3. Color-coded left border. Auto-dismiss 5s.

4.4 Spacing & Layout

- Base unit: 4px. All spacing in multiples of 4 (8, 12, 16, 24, 32, 48, 64).

- Max-width: 1280px centered. Card padding: 24px (desktop), 16px (mobile).

- Grid: 12-column, 24px gutters. Single column below 768px.

- Sidebar: 256px desktop, hamburger on mobile.

- Border radius: xl (12px) cards, lg (8px) buttons/inputs, full for avatars/badges.

5\. Database Schema

All tables use UUID primary keys. Timestamps are ISO 8601 (SQLite) or TIMESTAMPTZ (Postgres). Managed via goose migrations. This schema covers both parcel forwarding and drop-off workflows.

5.1 Core Tables

|             |                                 |                                                                                                                                                                                                                      |
|-------------|---------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Table**   | **Purpose**                     | **Key Fields**                                                                                                                                                                                                       |
| users       | Customer, staff, admin accounts | id, name, email, phone, role, avatar_url, suite_code, address_street, address_city, address_state, address_zip, storage_plan (free\|premium), free_storage_days (30), email_verified, status, created_at, updated_at |
| sessions    | Active auth sessions            | id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at                                                                                                                                      |
| magic_links | Passwordless auth tokens        | id, user_id, token_hash, redirect_to, used, expires_at, created_at                                                                                                                                                   |
| recipients  | Saved shipping recipients       | id, user_id, name, phone, destination_id, street, apt, city, delivery_instructions, is_default, use_count, created_at, updated_at                                                                                    |

5.2 Parcel Forwarding Tables (Primary Flow)

|                    |                                          |                                                                                                                                                                                                                                                                                                                                                               |
|--------------------|------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Table**          | **Purpose**                              | **Key Fields**                                                                                                                                                                                                                                                                                                                                                |
| locker_packages    | Packages in customer storage (the inbox) | id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name, sender_address, weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay, status (stored\|service_pending\|ship_requested\|shipped\|expired\|disposed), arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at                        |
| locker_photos      | Package photos (arrival + service)       | id, locker_package_id, photo_url, photo_type (arrival\|content\|detail\|condition), taken_by, created_at                                                                                                                                                                                                                                                      |
| ship_requests      | Customer forwarding requests             | id, user_id, confirmation_code, status (draft\|pending_customs\|pending_payment\|paid\|processing\|shipped\|delivered\|cancelled), destination_id, recipient_id, service_type, consolidate (bool), special_instructions, subtotal, service_fees, insurance, discount, total, payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at |
| ship_request_items | Packages in a ship request               | id, ship_request_id, locker_package_id, customs_description, customs_value, customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs                                                                                                                                                                                                  |
| service_requests   | Value-added service requests             | id, user_id, locker_package_id, service_type (photo_detail\|content_inspection\|repackage\|remove_invoice\|fragile_wrap\|gift_wrap), status (pending\|in_progress\|completed\|cancelled), notes, completed_by, price, created_at, completed_at                                                                                                                |
| inbound_tracking   | Pre-arrival package tracking             | id, user_id, carrier, tracking_number, retailer_name, expected_items, status (tracking\|in_transit\|delivered\|matched), locker_package_id, last_checked_at, created_at                                                                                                                                                                                       |
| storage_fees       | Daily storage charges                    | id, user_id, locker_package_id, fee_date, amount, invoiced, invoice_id                                                                                                                                                                                                                                                                                        |
| unmatched_packages | Packages without valid suite code        | id, carrier, tracking_number, label_text, photo_url, weight_lbs, status (pending\|matched\|returned\|disposed), matched_user_id, resolution_notes, received_at, resolved_at                                                                                                                                                                                   |

5.3 Booking & Shipping Tables (Both Flows)

|               |                                               |                                                                                                                                                                                                                                             |
|---------------|-----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Table**     | **Purpose**                                   | **Key Fields**                                                                                                                                                                                                                              |
| bookings      | Drop-off shipping bookings                    | id, user_id, confirmation_code, status, service_type, destination_id, recipient_id, scheduled_date, time_slot, special_instructions, subtotal, discount, insurance, total, payment_status, stripe_payment_intent_id, created_at, updated_at |
| packages      | Individual packages (from bookings or locker) | id, booking_id, shipment_id, locker_package_id (nullable FK), tracking_number, weight_estimated, weight_actual, length, width, height, declared_value, contents, condition, warehouse_location, status, created_at, updated_at              |
| shipments     | Grouped packages in transit                   | id, destination_id, manifest_id, ship_request_id (nullable FK), tracking_number, status, total_weight, package_count, carrier, estimated_delivery, actual_delivery, created_at, updated_at                                                  |
| manifests     | Shipping manifests for flights                | id, manifest_number, destination_id, carrier, flight_number, departure_date, total_pieces, total_weight, status, created_at                                                                                                                 |
| invoices      | Billing invoices                              | id, user_id, booking_id, ship_request_id, invoice_number, subtotal, tax, total, status, due_date, paid_at, notes, created_at                                                                                                                |
| invoice_items | Line items on invoices                        | id, invoice_id, description, quantity, unit_price, total                                                                                                                                                                                    |

5.4 Operations Tables

|                      |                                |                                                                                                                                                                                |
|----------------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Table**            | **Purpose**                    | **Key Fields**                                                                                                                                                                 |
| exceptions           | Package issues                 | id, package_id, booking_id, locker_package_id, type, priority, severity, description, resolution, status, customer_notified, reported_by, resolved_by, created_at, resolved_at |
| weight_discrepancies | Estimated vs actual weight     | id, package_id, user_id, estimated_weight, actual_weight, difference, additional_cost, status, created_at                                                                      |
| communications       | Messages to customers          | id, user_id, type, subject, content, sent_by, created_at                                                                                                                       |
| activity_log         | Audit trail                    | id, user_id, action, resource_type, resource_id, description, ip_address, created_at                                                                                           |
| warehouse_bays       | Staging bay config             | id, name, zone, destination_id, capacity, current_count                                                                                                                        |
| templates            | Saved booking templates        | id, user_id, name, service_type, destination_id, recipient_id, use_count, created_at                                                                                           |
| settings             | Admin key-value config         | key, value, updated_at, updated_by                                                                                                                                             |
| notification_prefs   | Per-user notification settings | id, user_id, email_enabled, sms_enabled, push_enabled, on_package_arrived, on_storage_expiry, on_ship_updates, on_inbound_updates, daily_digest (off\|morning\|evening)        |

5.5 Key Indexes

- users: UNIQUE(email), INDEX(suite_code), INDEX(role, status)

- locker_packages: INDEX(user_id, status), INDEX(suite_code), INDEX(arrived_at), INDEX(free_storage_expires_at, status)

- ship_requests: INDEX(user_id, status), INDEX(confirmation_code)

- ship_request_items: INDEX(ship_request_id), INDEX(locker_package_id)

- service_requests: INDEX(locker_package_id, status), INDEX(status)

- inbound_tracking: INDEX(user_id), INDEX(tracking_number)

- bookings: INDEX(user_id, status), INDEX(scheduled_date), INDEX(confirmation_code)

- packages: INDEX(booking_id), INDEX(shipment_id), INDEX(tracking_number)

- shipments: INDEX(destination_id, status), INDEX(tracking_number), INDEX(ship_request_id)

- activity_log: INDEX(resource_type, resource_id), INDEX(created_at DESC)

- unmatched_packages: INDEX(status), INDEX(received_at)

- storage_fees: INDEX(user_id, invoiced), INDEX(locker_package_id)

6\. API Specification

All endpoints prefixed /api/v1. JSON bodies. Bearer token auth. Standard pagination: page, limit, sort, order, search, status, from, to. Response envelope: { data: \[\...\], pagination: { page, limit, total, totalPages } }.

6.1 Authentication

|            |                          |                                                |
|------------|--------------------------|------------------------------------------------|
| **Method** | **Endpoint**             | **Purpose**                                    |
| POST       | /auth/magic-link/request | Request magic link (email, name?, redirectTo?) |
| POST       | /auth/magic-link/verify  | Verify magic link token                        |
| POST       | /auth/refresh            | Refresh access token (cookie)                  |
| POST       | /auth/logout             | End session                                    |
| POST       | /auth/password/forgot    | Request password reset                         |
| POST       | /auth/password/reset     | Reset password with token                      |
| PATCH      | /auth/password/change    | Change password (authenticated)                |

6.2 Customer: Locker (Package Inbox)

|            |                             |                                                                 |
|------------|-----------------------------|-----------------------------------------------------------------|
| **Method** | **Endpoint**                | **Purpose**                                                     |
| GET        | /locker                     | List packages in locker (filterable by status)                  |
| GET        | /locker/:id                 | Package detail with photos and service history                  |
| GET        | /locker/summary             | Inbox stats: stored count, value, next expiry, pending services |
| POST       | /locker/:id/photo-request   | Request detailed photos (creates service_request, charges fee)  |
| POST       | /locker/:id/service-request | Request value-added service (repackage, inspection, etc.)       |
| DELETE     | /locker/:id                 | Request voluntary disposal                                      |

6.3 Customer: Ship Requests

|            |                              |                                                                                     |
|------------|------------------------------|-------------------------------------------------------------------------------------|
| **Method** | **Endpoint**                 | **Purpose**                                                                         |
| GET        | /ship-requests               | List ship requests (filterable)                                                     |
| POST       | /ship-requests               | Create ship request (package_ids\[\], destination, recipient, service, consolidate) |
| GET        | /ship-requests/:id           | Detail with items, customs, cost, tracking                                          |
| PATCH      | /ship-requests/:id           | Modify (only while draft/pending_customs)                                           |
| DELETE     | /ship-requests/:id           | Cancel (before payment only)                                                        |
| POST       | /ship-requests/:id/customs   | Submit customs declarations for all items                                           |
| GET        | /ship-requests/:id/estimate  | Cost estimate (no side effects)                                                     |
| POST       | /ship-requests/:id/pay       | Create Stripe PaymentIntent                                                         |
| POST       | /ship-requests/:id/reconcile | Confirm payment after Stripe success                                                |

6.4 Customer: Inbound Tracking

|            |                       |                                                        |
|------------|-----------------------|--------------------------------------------------------|
| **Method** | **Endpoint**          | **Purpose**                                            |
| GET        | /inbound-tracking     | List pre-registered tracking numbers                   |
| POST       | /inbound-tracking     | Add tracking number (carrier, number, retailer, items) |
| GET        | /inbound-tracking/:id | Detail with carrier status                             |
| DELETE     | /inbound-tracking/:id | Stop tracking                                          |

6.5 Customer: Bookings (Drop-Off Flow)

|            |                                      |                                   |
|------------|--------------------------------------|-----------------------------------|
| **Method** | **Endpoint**                         | **Purpose**                       |
| GET        | /bookings                            | List bookings (filterable)        |
| POST       | /bookings                            | Create booking                    |
| GET        | /bookings/:id                        | Booking detail                    |
| PATCH      | /bookings/:id                        | Modify (date, time, instructions) |
| DELETE     | /bookings/:id                        | Cancel booking                    |
| GET        | /bookings/time-slots?date=YYYY-MM-DD | Available time slots              |

6.6 Customer: Profile & Shared

|            |                            |                                 |
|------------|----------------------------|---------------------------------|
| **Method** | **Endpoint**               | **Purpose**                     |
| GET        | /me                        | Current user profile            |
| PATCH      | /me                        | Update profile                  |
| POST       | /me/avatar                 | Upload avatar (max 5MB)         |
| GET        | /recipients                | List saved recipients           |
| POST       | /recipients                | Create recipient                |
| PATCH      | /recipients/:id            | Update recipient                |
| DELETE     | /recipients/:id            | Delete recipient                |
| GET        | /shipments                 | List shipments                  |
| GET        | /shipments/:id             | Shipment detail + tracking      |
| GET        | /invoices                  | List invoices                   |
| GET        | /invoices/:id              | Invoice detail                  |
| GET        | /invoices/:id/pdf          | Download invoice PDF            |
| GET        | /templates                 | List booking templates          |
| DELETE     | /templates/:id             | Delete template                 |
| GET        | /notifications/preferences | Get notification preferences    |
| PUT        | /notifications/preferences | Update notification preferences |
| GET        | /sessions                  | List active sessions            |
| DELETE     | /sessions/:id              | Revoke session                  |
| DELETE     | /sessions                  | Revoke all other sessions       |
| POST       | /account/delete            | Request account deletion        |

6.7 Public

|            |                        |                           |
|------------|------------------------|---------------------------|
| **Method** | **Endpoint**           | **Purpose**               |
| GET        | /track/:trackingNumber | Public tracking lookup    |
| POST       | /contact               | Contact form              |
| GET        | /status                | System status             |
| GET        | /destinations          | Destinations with rates   |
| GET        | /destinations/:id      | Destination detail        |
| GET        | /calculator            | Shipping cost calculation |

6.8 Payments

|            |                         |                                              |
|------------|-------------------------|----------------------------------------------|
| **Method** | **Endpoint**            | **Purpose**                                  |
| POST       | /payments/create-intent | Create PaymentIntent (for bookings)          |
| POST       | /payments/reconcile     | Confirm payment                              |
| POST       | /payments/webhook       | Stripe webhook (signature verified, no auth) |

6.9 Admin

All require admin role. Prefixed /api/v1/admin.

|            |                           |                                      |
|------------|---------------------------|--------------------------------------|
| **Method** | **Endpoint**              | **Purpose**                          |
| GET        | /dashboard                | Dashboard KPIs                       |
| GET        | /bookings                 | All bookings                         |
| POST       | /bookings                 | Create on behalf of customer         |
| PATCH      | /bookings/:id/status      | Update booking status                |
| GET        | /locker-packages          | All locker packages across customers |
| GET        | /ship-requests            | All ship requests                    |
| PATCH      | /ship-requests/:id/status | Update ship request status           |
| GET        | /service-requests         | All service requests (task queue)    |
| PATCH      | /service-requests/:id     | Complete/cancel service request      |
| GET        | /unmatched-packages       | All unmatched packages               |
| PATCH      | /unmatched-packages/:id   | Match, return, or dispose            |
| GET        | /storage-report           | Storage utilization report           |
| POST       | /storage-fees/generate    | Run daily storage fee job            |
| GET        | /shipments                | All shipments                        |
| POST       | /shipments                | Create shipment                      |
| PATCH      | /shipments/:id            | Update shipment                      |
| GET        | /users                    | All users                            |
| GET        | /users/:id                | User detail                          |
| PATCH      | /users/:id                | Update user role/status              |
| POST       | /communications           | Send email/SMS/note                  |
| GET        | /invoices                 | All invoices                         |
| PATCH      | /invoices/:id             | Mark paid, void, etc.                |
| GET        | /manifests                | All manifests                        |
| GET        | /exceptions               | All exceptions                       |
| POST       | /exceptions               | Report exception                     |
| PATCH      | /exceptions/:id           | Update exception                     |
| GET        | /weight-discrepancies     | All discrepancies                    |
| PATCH      | /weight-discrepancies/:id | Approve/reject                       |
| GET        | /reports                  | Report data by period                |
| GET        | /activity                 | Activity log                         |
| GET        | /search?q=                | Global search                        |
| GET        | /settings                 | All settings                         |
| PUT        | /settings/pricing         | Update pricing                       |
| PUT        | /settings/general         | Update general settings              |

6.10 Warehouse

Require staff or admin role. Prefixed /api/v1/warehouse.

|            |                           |                                            |
|------------|---------------------------|--------------------------------------------|
| **Method** | **Endpoint**              | **Purpose**                                |
| GET        | /stats                    | Dashboard KPIs                             |
| POST       | /locker-receive           | Receive carrier package (match suite code) |
| POST       | /locker-receive/unmatched | Flag as unmatched                          |
| GET        | /service-queue            | Pending service requests                   |
| PATCH      | /service-queue/:id        | Complete service request                   |
| GET        | /ship-queue               | Paid ship requests ready to process        |
| PATCH      | /ship-queue/:id/process   | Mark as being processed                    |
| PATCH      | /ship-queue/:id/weighed   | Record consolidated weight                 |
| PATCH      | /ship-queue/:id/staged    | Mark staged for manifest                   |
| GET        | /packages                 | All warehouse packages                     |
| GET        | /packages/lookup?code=    | Lookup by barcode/tracking                 |
| POST       | /packages/:id/receive     | Receive drop-off package                   |
| POST       | /packages/:id/photos      | Upload photos                              |
| GET        | /bays                     | List staging bays                          |
| POST       | /bays/move                | Move packages to bay                       |
| POST       | /manifests                | Create manifest                            |
| PATCH      | /manifests/:id            | Update manifest                            |
| GET        | /manifests/:id/documents  | Generate manifest PDF                      |
| GET        | /exceptions/:id           | Exception detail                           |
| POST       | /exceptions/:id/resolve   | Resolve exception                          |
| POST       | /sync                     | Bulk sync offline queue                    |

7\. Route Map

Every route maps to a go-app component. Routes are grouped by access level.

7.1 Public

|                        |                       |                                                 |
|------------------------|-----------------------|-------------------------------------------------|
| **Path**               | **Component**         | **Title**                                       |
| /                      | HomePage              | QCS Cargo --- Your Personal US Shipping Address |
| /how-it-works          | HowItWorksPage        | How It Works \| QCS Cargo                       |
| /about                 | AboutPage             | About Us \| QCS Cargo                           |
| /services              | ServicesPage          | Our Services \| QCS Cargo                       |
| /pricing               | PricingPage           | Pricing \| QCS Cargo                            |
| /faq                   | FAQPage               | FAQ \| QCS Cargo                                |
| /contact               | ContactPage           | Contact Us \| QCS Cargo                         |
| /status                | StatusPage            | System Status \| QCS Cargo                      |
| /track                 | TrackPage             | Track Shipment \| QCS Cargo                     |
| /legal/terms           | TermsPage             | Terms of Service \| QCS Cargo                   |
| /legal/privacy         | PrivacyPage           | Privacy Policy \| QCS Cargo                     |
| /legal/shipping-policy | ShippingPolicyPage    | Shipping Policy \| QCS Cargo                    |
| /prohibited-items      | ProhibitedItemsPage   | Prohibited Items \| QCS Cargo                   |
| /shipping-calculator   | CalculatorPage        | Shipping Calculator \| QCS Cargo                |
| /destinations          | DestinationsPage      | Destinations \| QCS Cargo                       |
| /destinations/:id      | DestinationDetailPage | Shipping to {name} \| QCS Cargo                 |

7.2 Auth

|                  |                    |                              |
|------------------|--------------------|------------------------------|
| **Path**         | **Component**      | **Title**                    |
| /login           | LoginPage          | Sign In \| QCS Cargo         |
| /register        | RegisterPage       | Sign Up \| QCS Cargo         |
| /verify          | VerifyPage         | Verify Email \| QCS Cargo    |
| /forgot-password | ForgotPasswordPage | Forgot Password \| QCS Cargo |
| /reset-password  | ResetPasswordPage  | Reset Password \| QCS Cargo  |

7.3 Dashboard (Customer)

|                                           |                       |                                    |
|-------------------------------------------|-----------------------|------------------------------------|
| **Path**                                  | **Component**         | **Title**                          |
| /dashboard                                | DashboardHome         | Dashboard \| QCS Cargo             |
| /dashboard/inbox                          | PackageInbox          | My Packages \| QCS Cargo           |
| /dashboard/inbox/:id                      | PackageDetail         | Package Details \| QCS Cargo       |
| /dashboard/ship                           | ShipRequestWizard     | Ship My Packages \| QCS Cargo      |
| /dashboard/ship-requests                  | ShipRequestsList      | Ship Requests \| QCS Cargo         |
| /dashboard/ship-requests/:id              | ShipRequestDetail     | Ship Request Details \| QCS Cargo  |
| /dashboard/ship-requests/:id/customs      | CustomsForm           | Customs Declaration \| QCS Cargo   |
| /dashboard/ship-requests/:id/pay          | ShipRequestPay        | Complete Payment \| QCS Cargo      |
| /dashboard/ship-requests/:id/confirmation | ShipConfirmation      | Shipment Confirmed \| QCS Cargo    |
| /dashboard/inbound                        | InboundTracking       | Expected Packages \| QCS Cargo     |
| /dashboard/bookings                       | BookingsList          | My Bookings \| QCS Cargo           |
| /dashboard/bookings/new                   | BookingWizard         | New Booking \| QCS Cargo           |
| /dashboard/bookings/:id                   | BookingDetail         | Booking Details \| QCS Cargo       |
| /dashboard/bookings/:id/modify            | BookingModify         | Modify Booking \| QCS Cargo        |
| /dashboard/bookings/:id/pay               | BookingPay            | Complete Payment \| QCS Cargo      |
| /dashboard/bookings/:id/confirmation      | BookingConfirmation   | Booking Confirmed \| QCS Cargo     |
| /dashboard/recipients                     | RecipientsList        | My Recipients \| QCS Cargo         |
| /dashboard/recipients/new                 | RecipientForm         | Add Recipient \| QCS Cargo         |
| /dashboard/recipients/:id                 | RecipientForm         | Edit Recipient \| QCS Cargo        |
| /dashboard/shipments                      | ShipmentsList         | My Shipments \| QCS Cargo          |
| /dashboard/shipments/:id                  | ShipmentDetail        | Shipment Details \| QCS Cargo      |
| /dashboard/invoices                       | InvoicesList          | Invoices \| QCS Cargo              |
| /dashboard/invoices/:id                   | InvoiceDetail         | Invoice \| QCS Cargo               |
| /dashboard/mailbox                        | MailboxPage           | My Mailbox \| QCS Cargo            |
| /dashboard/profile                        | ProfilePage           | My Profile \| QCS Cargo            |
| /dashboard/settings                       | SettingsPage          | Settings \| QCS Cargo              |
| /dashboard/settings/notifications         | NotificationsSettings | Notification Settings \| QCS Cargo |
| /dashboard/settings/security              | SecuritySettings      | Security Settings \| QCS Cargo     |
| /dashboard/settings/sessions              | SessionsPage          | Active Sessions \| QCS Cargo       |
| /dashboard/settings/delete-account        | DeleteAccountPage     | Delete Account \| QCS Cargo        |
| /dashboard/templates                      | TemplatesPage         | Booking Templates \| QCS Cargo     |

7.4 Admin

|                             |                     |                               |
|-----------------------------|---------------------|-------------------------------|
| **Path**                    | **Component**       | **Title**                     |
| /admin                      | AdminDashboard      | Admin Dashboard               |
| /admin/bookings             | AdminBookings       | Bookings \| Admin             |
| /admin/bookings/new         | AdminBookingNew     | New Booking \| Admin          |
| /admin/bookings/:id         | AdminBookingDetail  | Booking Details \| Admin      |
| /admin/locker-packages      | AdminLockerPkgs     | Locker Packages \| Admin      |
| /admin/ship-requests        | AdminShipRequests   | Ship Requests \| Admin        |
| /admin/unmatched            | AdminUnmatched      | Unmatched Packages \| Admin   |
| /admin/service-queue        | AdminServiceQueue   | Service Queue \| Admin        |
| /admin/shipments            | AdminShipments      | Shipments \| Admin            |
| /admin/shipments/new        | AdminShipmentNew    | Create Shipment \| Admin      |
| /admin/shipments/:id        | AdminShipmentDetail | Shipment Details \| Admin     |
| /admin/users                | AdminUsers          | Users \| Admin                |
| /admin/users/:id            | AdminUserDetail     | User Details \| Admin         |
| /admin/communications       | AdminComms          | Communications \| Admin       |
| /admin/invoices             | AdminInvoices       | Invoices \| Admin             |
| /admin/receiving            | AdminReceiving      | Receiving \| Admin            |
| /admin/manifests            | AdminManifests      | Manifests \| Admin            |
| /admin/reports              | AdminReports        | Reports \| Admin              |
| /admin/activity             | AdminActivity       | Activity Log \| Admin         |
| /admin/exceptions           | AdminExceptions     | Exceptions \| Admin           |
| /admin/weight-discrepancies | AdminWeightDisc     | Weight Discrepancies \| Admin |
| /admin/search               | AdminSearch         | Search \| Admin               |
| /admin/settings             | AdminSettings       | Settings \| Admin             |
| /admin/settings/pricing     | AdminPricing        | Pricing \| Admin              |

7.5 Warehouse

|                           |                     |                          |
|---------------------------|---------------------|--------------------------|
| **Path**                  | **Component**       | **Title**                |
| /warehouse                | WarehouseDashboard  | Warehouse Operations     |
| /warehouse/locker-receive | WarehouseLockerRecv | Receive Carrier Package  |
| /warehouse/receiving      | WarehouseReceiving  | Receive Drop-Off Package |
| /warehouse/service-queue  | WarehouseServiceQ   | Service Queue            |
| /warehouse/ship-queue     | WarehouseShipQ      | Ship Processing Queue    |
| /warehouse/inventory      | WarehouseInventory  | Inventory                |
| /warehouse/packages       | WarehousePackages   | All Packages             |
| /warehouse/staging        | WarehouseStaging    | Staging Area             |
| /warehouse/scan           | WarehouseScan       | Scan Package             |
| /warehouse/manifests      | WarehouseManifests  | Shipping Preparation     |
| /warehouse/exception/:id  | WarehouseException  | Exception Detail         |

8\. Company Data & Constants

8.1 Company Information

|                  |                                         |
|------------------|-----------------------------------------|
| **Key**          | **Value**                               |
| Company          | QCS Cargo (Quiet Craft Solutions Cargo) |
| Phone            | 201-249-0929                            |
| Email            | sales@qcs-cargo.com                     |
| Support          | support@qcs-cargo.com                   |
| Address          | 35 Obrien St, E12, Kearny, NJ 07032     |
| Coords           | 40.7676, -74.1502                       |
| Hours (Mon-Fri)  | 9:00 AM - 6:00 PM                       |
| Hours (Saturday) | 9:00 AM - 2:00 PM                       |
| Hours (Sunday)   | Closed                                  |
| Facebook         | https://facebook.com/qcscargo           |
| Instagram        | https://instagram.com/qcscargo          |
| WhatsApp         | +1-201-249-0929                         |

8.2 Destinations & Rates

|          |                   |          |               |           |             |
|----------|-------------------|----------|---------------|-----------|-------------|
| **ID**   | **Name**          | **Code** | **Capital**   | **\$/lb** | **Transit** |
| guyana   | Guyana            | GY       | Georgetown    | 3.50      | 3-5 days    |
| jamaica  | Jamaica           | JM       | Kingston      | 3.75      | 3-5 days    |
| trinidad | Trinidad & Tobago | TT       | Port of Spain | 3.50      | 3-5 days    |
| barbados | Barbados          | BB       | Bridgetown    | 4.00      | 4-6 days    |
| suriname | Suriname          | SR       | Paramaribo    | 4.25      | 4-6 days    |

8.3 Services

|              |                      |                 |             |
|--------------|----------------------|-----------------|-------------|
| **ID**       | **Name**             | **Price**       | **Transit** |
| standard     | Standard Air Freight | \$3.50/lb       | 3-5 days    |
| express      | Express Delivery     | +25%            | 1-2 days    |
| door-to-door | Door-to-Door         | \$25 pickup fee | 3-5 days    |
| consolidated | Consolidated Cargo   | Volume discount | 3-5 days    |
| customs      | Customs Clearance    | \$35/shipment   | N/A         |
| special      | Special Handling     | Custom quote    | Varies      |

8.4 Value-Added Services

|                    |           |                                             |
|--------------------|-----------|---------------------------------------------|
| **Service**        | **Price** | **Description**                             |
| Arrival Photo      | Free      | One photo at intake, every package          |
| Detailed Photos    | \$3.00    | 3-5 photos, multiple angles                 |
| Content Inspection | \$5.00    | Open, photograph contents, list items       |
| Repackaging        | \$5.00    | Remove excess packaging, re-measure         |
| Remove Invoices    | \$2.00    | Remove price tags and receipts              |
| Fragile Wrap       | \$3.00    | Bubble wrap + fragile labels                |
| Gift Wrap          | \$5.00    | Gift paper + optional message card          |
| Consolidation      | Free      | Combine multiple packages into one shipment |

8.5 Storage Policy

|                             |                                   |
|-----------------------------|-----------------------------------|
| **Parameter**               | **Value**                         |
| Free storage                | 30 days from arrival              |
| Daily fee after free period | \$1.50 / day / package            |
| Warning                     | 5 days before expiry              |
| Final notice                | Day 55 (5 days before disposal)   |
| Auto-disposal               | Day 60                            |
| Maximum storage             | 90 days (then mandatory disposal) |

8.6 Volume Discounts

|                  |              |
|------------------|--------------|
| **Weight (lbs)** | **Discount** |
| 100-249          | 5%           |
| 250-499          | 10%          |
| 500-999          | 15%          |
| 1,000+           | 20%          |

8.7 Status Labels

|                  |                   |                 |
|------------------|-------------------|-----------------|
| **Key**          | **Label**         | **Badge Style** |
| pending          | Pending           | Warning         |
| confirmed        | Confirmed         | Info            |
| received         | Received          | Info            |
| processing       | Processing        | Processing      |
| in_transit       | In Transit        | Info            |
| customs          | Customs Clearance | Warning         |
| out_for_delivery | Out for Delivery  | Info            |
| delivered        | Delivered         | Success         |
| cancelled        | Cancelled         | Neutral         |
| exception        | Exception         | Danger          |
| draft            | Draft             | Neutral         |
| pending_payment  | Pending Payment   | Warning         |
| payment_failed   | Payment Failed    | Danger          |
| in_progress      | In Progress       | Processing      |
| completed        | Completed         | Success         |
| stored           | Stored            | Info            |
| ship_requested   | Ship Requested    | Processing      |
| expired          | Expired           | Warning         |
| disposed         | Disposed          | Neutral         |
| service_pending  | Service Pending   | Processing      |

8.8 Time Slots

Mon-Fri: 09:00-10:00 through 17:00-18:00 (9 slots). Saturday: 09:00-10:00 through 13:00-14:00 (5 slots). Sunday: Closed.

8.9 Pricing Formula

|                    |                                    |
|--------------------|------------------------------------|
| **Component**      | **Formula**                        |
| Dimensional weight | (L x W x H) / 166 inches to lbs    |
| Billable weight    | max(actual, dimensional)           |
| Base cost          | billable_weight x destination_rate |
| Express surcharge  | base_cost x 0.25                   |
| Door-to-door fee   | \$25 flat                          |
| Insurance          | declared_value / 100               |
| Volume discount    | See 8.6 tiers                      |
| Minimum charge     | \$10 per shipment                  |

8.10 Code Formats

- Suite Code: QCS-{6 alphanumeric}. Example: QCS-A3F7K2

- Shipment Tracking: QCS-YYYY-NNNNNN. Example: QCS-2026-001234

- Package Tracking: QCS{YYMM}{DD}{NNNN}{4 alpha}. Example: QCS2412150001ABCD

- Ship Request Code: SR-{8 alphanumeric}. Example: SR-B4K9M2X7

- Invoice Number: INV-YYYY-NNNNNN. Example: INV-2026-000042

9\. Page Specifications

Detailed content, form fields, validation, and behavior per page. Pages are grouped by workflow priority.

9.1 Home Page (/)

> **Hero**
>
> Badge: \'TSA Licensed · Fully Insured\'. H1: \'Your Personal\' / \'US Shipping Address\' (shimmer). Subtext: \'Shop any US store. We receive, store, and ship your packages to the Caribbean. Sign up in 60 seconds.\' CTAs: \'Get My Free Address\' (→ /register), \'Track Shipment\' (→ /track). Stats: 15+ Years, 50K+ Delivered, 99% Satisfaction, 24/7 Tracking.
>
> **How It Works**
>
> 4 steps with icons: 1) Get Your Address --- sign up, get suite code. 2) Shop Anywhere --- use address at any US store. 3) We Receive & Store --- photo, weigh, 30 days free. 4) Ship When Ready --- consolidate, customs, pay, deliver.
>
> **Destinations**
>
> H2: \'Select Caribbean Destinations\'. 5 horizontal-scroll cards. CTA: \'View All\' → /destinations.
>
> **Calculator CTA**
>
> id=calculator-section. \'Ready to Ship?\' → /shipping-calculator.

9.2 How It Works (/how-it-works)

Dedicated conversion page. Hero: \'How QCS Cargo Works\'. 4 illustrated steps (same as home but expanded with details). Each step has a heading, illustration placeholder, and 2-3 sentences. CTA: \'Get Your Free US Address\'. Mini FAQ: \'Is sign up free?\', \'How long can I store?\', \'What stores work?\'

9.3 Dashboard Home (/dashboard)

> **Stats Row**
>
> 4 cards: \'Packages in Locker\' (count → /inbox), \'Expected Arrivals\' (inbound count → /inbound), \'Pending Ship Requests\' (count → /ship-requests), \'Shipped This Month\' (count → /shipments).
>
> **Primary CTA**
>
> Large coral: \'Ship My Packages\' (if locker has items). Secondary: \'Copy My QCS Address\'.
>
> **Storage Alerts**
>
> Amber banner if packages within 5 days of expiry. Links to inbox filtered by expiring.
>
> **Recent Arrivals**
>
> Last 5 packages with photo thumbs. \'View All\' → /inbox.
>
> **Quick Actions**
>
> Grid: Ship Packages, Track Inbound, My Recipients, Calculator, Get Help, Drop-Off Booking.

9.4 Package Inbox (/dashboard/inbox)

PRIMARY customer view. Shows all packages in storage.

> **Header**
>
> H1: \'My Packages\'. \'{count} packages in your locker\'. Mailbox card with copy. Stats: stored, value, next expiry, pending services.
>
> **Action Bar**
>
> Primary: \'Ship My Packages\' (coral). Secondary: \'Add Inbound Tracking\'. Filters: All \| Stored \| Service Pending \| Shipped \| Expired. Sort: Newest \| Oldest \| Expiring Soon.
>
> **Package Grid**
>
> Card grid (2 col desktop, 1 mobile). Each: arrival photo, sender, weight, storage progress bar, status badge, checkbox. Selectable for ship requests.
>
> **Storage Bar Colors**
>
> 0-20 days: emerald. 21-27: amber. 28-30: red. 31+: red pulsing with daily fee.
>
> **Floating Selection Bar**
>
> Appears on 1+ selection. \'{N} packages ({weight} lbs)\'. Buttons: Ship Selected, Request Photos, Clear.
>
> **Empty State**
>
> Mailbox illustration. \'Your locker is empty\'. \'Copy My QCS Address\'. \'How It Works\' link.

9.5 Package Detail (/dashboard/inbox/:id)

> **Header**
>
> Back → inbox. Status badge. Sender name. Arrival date. Storage progress.
>
> **Photos**
>
> Gallery: arrival (always), content, detail (if requested). Lightbox on click. CTA: \'Want to see inside?\' if no content photos.
>
> **Details Card**
>
> Weight, dimensions, condition, bay, value, carrier, inbound tracking (linked).
>
> **Services Card**
>
> Grid of buttons: Photos (\$3), Inspection (\$5), Repackage (\$5), Remove Invoices (\$2), Fragile (\$3), Gift Wrap (\$5). Status for each if requested.
>
> **Actions**
>
> \'Ship This Package\' (coral). \'Dispose\' (danger, confirm dialog).

9.6 Ship Request Wizard (/dashboard/ship)

4-step flow. Can enter from inbox (pre-selected) or standalone.

Step 1: Select Packages

- Selectable cards for all \'stored\' packages. Pre-selected if coming from inbox.

- Running total: \'{N} packages, {weight} lbs\'.

- Toggle: \'Consolidate\' (default ON for 2+). Show estimated savings.

- Warning if package has pending service. Minimum 1 package.

Step 2: Destination & Service

- Destination select (5 countries). Service type: Standard, Express, Door-to-Door.

- Recipient: saved list or add new (name\*, phone\*, destination, street\*, apt, city\*, instructions, save checkbox).

- Live cost estimate as selections change.

Step 3: Customs Declarations

Per item in shipment:

|                   |               |              |                               |
|-------------------|---------------|--------------|-------------------------------|
| **Field**         | **Type**      | **Required** | **Validation**                |
| Description       | Text          | Yes          | 3-200 chars, no generic terms |
| Value (USD)       | Number        | Yes          | \> 0. Warn if \> \$800        |
| Quantity          | Integer       | Yes          | 1-100                         |
| Country of Origin | Select        | Yes          | Default: United States        |
| Weight (lbs)      | Number        | Yes          | Pre-filled, editable, 0.1-500 |
| HS/Tariff Code    | Text + search | No           | 6-10 digits                   |

Checkbox: \'I confirm these declarations are accurate.\' Total declared value shown.

Step 4: Review & Pay

- Summary: packages (with photos), destination, recipient, customs, cost breakdown.

- Cost: base + express + pickup + service fees + insurance - consolidation savings - volume discount + outstanding storage fees.

- Stripe payment. \'Submit Ship Request\'. Success → confirmation page.

- \'Save as Draft\' option.

9.7 Inbound Tracking (/dashboard/inbound)

> **Layout**
>
> H1: \'Expected Packages\'. \'Track a Package\' button. Card list of tracked items.
>
> **Add Form**
>
> Modal: Carrier (select), Tracking Number (required), Retailer (optional), Expected Items (optional). \'Start Tracking\'.
>
> **Card**
>
> Carrier icon, tracking number (linked), retailer, status badge, last update. When delivered + matched: \'Received! View in My Packages\' link.

9.8 Mailbox (/dashboard/mailbox)

- Large display of full address with per-line and full copy buttons.

- QR code of full address for mobile scanning.

- Format variations: \'For Amazon\', \'For eBay\', \'For Walmart\'.

- Shopping Tips: include suite code, use real name, avoid prohibited items.

- Compatible Stores: logos and links to popular US retailers.

- Link: \'Already ordered? Track incoming packages.\'

9.9 Shipping Calculator (/shipping-calculator)

|                |          |              |                                     |
|----------------|----------|--------------|-------------------------------------|
| **Field**      | **Type** | **Required** | **Validation**                      |
| Destination    | Select   | Yes          | 5 destinations                      |
| Service        | Radio    | Yes          | standard \| express \| door_to_door |
| Weight (lbs)   | Number   | Yes          | 0.1-500                             |
| L / W / H (in) | Numbers  | No           | 1-100 each                          |
| Declared Value | Number   | No           | \>= 0                               |
| Insurance      | Toggle   | No           | Default off                         |

Shows: dimensional weight, billable weight, base cost, surcharges, insurance, total.

9.10 Booking Wizard (/dashboard/bookings/new) --- Drop-Off Flow

5-step wizard for walk-in customers (secondary flow). Steps: Service → Packages → Recipient → Schedule → Review. Same spec as original PRD Section 5 (booking wizard). Date picker: next 14 days, no Sunday. Time slots from API. Max 20 packages. Cost breakdown at review.

9.11 Warehouse: Carrier Receiving (/warehouse/locker-receive)

Most-used warehouse page. Receives carrier-delivered packages.

1.  SCAN: Suite code or tracking number input. Auto-detects format.

2.  MATCH: Lookup suite code. Found: show customer + locker count. Not found: \'Flag as Unmatched\'.

3.  RECORD: Weight (mandatory), dimensions (optional), condition (select), carrier (auto/select), tracking, sender name.

4.  PHOTO: Arrival photo via camera. Mandatory, multiple angles supported.

5.  BAY: Auto-suggest based on destination. Staff can override.

6.  CONFIRM: Review, \'Receive Package\'. Auto-notifies customer.

Offline: queues to IndexedDB. Suite code lookup from cached customer list.

9.12 Warehouse: Service Queue (/warehouse/service-queue)

Photo, inspection, and repackage requests from customers. Cards: customer, sender, service type, time waiting, bay location. \'Start\' → fulfill → upload photos/notes → \'Complete\'. Customer notified.

9.13 Warehouse: Ship Queue (/warehouse/ship-queue)

Paid ship requests. Grouped by destination. Flow: Pull packages from bays → consolidate if flagged → weigh → apply customs labels → stage for manifest. Weight discrepancy check at weigh step.

10\. Notification System

Notifications cover the full parcel forwarding lifecycle plus the traditional shipping flow. Dispatched via a background goroutine consuming from an event channel.

10.1 Events

|                        |                |                                |                                                                  |
|------------------------|----------------|--------------------------------|------------------------------------------------------------------|
| **Event**              | **Channels**   | **Trigger**                    | **Template**                                                     |
| Package Arrived        | Email+SMS+Push | Staff completes locker receive | \'New package from {sender}! Weight: {weight}. Log in to view.\' |
| Photo Ready            | Email+Push     | Photo service completed        | \'{N} photos of your package from {sender} are ready.\'          |
| Service Complete       | Email+Push     | Any service completed          | \'{service} for package from {sender} is complete.\'             |
| Storage Warning (5d)   | Email+Push     | Daily job                      | \'Package from {sender} starts storage fees in 5 days.\'         |
| Storage Warning (1d)   | Email+SMS+Push | Daily job                      | \'Last day of free storage for {sender} package.\'               |
| Storage Fee Charged    | Email          | Daily job                      | \'Storage fee \${amount} charged for {sender} package.\'         |
| Storage Final Notice   | Email+SMS      | Day 55                         | \'Package from {sender} disposed in 5 days unless shipped.\'     |
| Ship Request Paid      | Email          | Payment success                | \'Ship request {code} confirmed! Being prepared.\'               |
| Ship Request Shipped   | Email+SMS+Push | Warehouse marks shipped        | \'Shipment {code} on its way! Tracking: {tracking}.\'            |
| Ship Request Delivered | Email+SMS+Push | Carrier confirms               | \'Shipment {code} delivered in {destination}!\'                  |
| Inbound Delivered      | Email+Push     | Tracking shows delivered       | \'{retailer} package delivered to QCS. Check locker shortly.\'   |
| Customs Hold           | Email+SMS      | Customs issue                  | \'Ship request {code} needs customs attention.\'                 |
| Booking Confirmed      | Email          | Booking payment                | \'Booking {code} confirmed for {date}.\'                         |
| Shipment Status        | Email+Push     | Status change                  | \'Shipment {tracking}: now {status}.\'                           |

10.2 Preferences

|                      |             |                              |
|----------------------|-------------|------------------------------|
| **Preference**       | **Default** | **Options**                  |
| Package arrival      | Email+Push  | Email, SMS, Push (any combo) |
| Storage warnings     | Email+Push  | Email, SMS, Push             |
| Ship request updates | Email       | Email, SMS, Push             |
| Inbound tracking     | Push        | Email, SMS, Push             |
| Daily digest         | Off         | Off / Morning / Evening      |

10.3 Implementation

- Event bus: Go channel. Publishers (API handlers, cron jobs) send events. Worker goroutine consumes.

- Dispatch: Resend (email), Twilio or similar (SMS), Web Push API via service worker.

- Logged to activity_log and notification_log tables.

11\. Phased Implementation Plan

6 phases, \~21 weeks. Each phase produces a deployable increment. Parcel forwarding is built as the primary flow in Phase 2.

Phase 0: Foundation (Week 1-2)

**Goal:** Skeleton compiles, serves WASM, connects to DB.

- Go module init. Fiber server with /api/v1/health.

- SQLite + WAL mode. goose migrations for ALL tables (including locker, ship_request tables).

- sqlc setup with initial queries.

- go-app skeleton: shell HTML, routing, Tailwind CDN, fonts.

- Shared models package (User, LockerPackage, ShipRequest, Booking, etc.).

- Service worker generation. Static asset caching.

- GitHub Actions: lint, test, build WASM, build binary. Dockerfile.

**Deliverable:** Hello World at /. Health endpoint returns 200. All DB tables exist.

Phase 1: Auth + Public Pages (Week 3-5)

**Goal:** All public pages. Registration with suite code generation. Login/logout.

- Magic-link auth flow. Password reset. JWT middleware. Session management.

- Suite code generation on registration (QCS-{6 random alphanumeric}).

- Public pages: Home (forwarding-first hero), How It Works (NEW), About, Services (with VAS), Pricing (with storage), FAQ, Contact, Status, Track.

- Legal pages. Destinations. Shipping Calculator. Prohibited Items.

- Header, footer, mobile nav. Contact form + Resend email.

**Deliverable:** Complete public site. User registers and gets suite code. Login works.

Phase 2: Customer Dashboard + Forwarding (Week 6-11)

**Goal:** Full parcel forwarding experience. Customer can view locker, request services, create ship requests, and pay.

Build order within this phase:

1.  Dashboard layout (sidebar, header, auth guard).

2.  Mailbox page (address display, QR code, store tips).

3.  Package Inbox (locker view, card grid, selection, storage bars).

4.  Package Detail (photos, details, service request buttons).

5.  Ship Request Wizard Steps 1-2 (package selection, destination/service).

6.  Ship Request Wizard Step 3 (customs declarations form).

7.  Ship Request Wizard Step 4 (review + Stripe payment).

8.  Ship Request list and detail pages.

9.  Inbound Tracking (add, list, detail).

10. Service request UI (photo, repackage, etc.).

11. Storage alert banners and expiry warnings.

12. Booking wizard (5-step, for drop-off secondary flow).

13. Recipients CRUD, shipments list/detail, invoices, templates.

14. Profile, settings, notifications, security, sessions, delete account.

**Deliverable:** Customer signs up, gets address, sees packages in locker, creates ship request, pays, tracks shipment. Also can book drop-offs.

Phase 3: Admin Console (Week 12-15)

**Goal:** Full admin operations management.

- Admin layout: sidebar, global search (Cmd+K), notification bell.

- Dashboard KPIs + pending actions + today\'s schedule.

- Locker packages management (view all, search by customer/suite code).

- Ship request management (list, status updates, processing queue).

- Service queue management (assign, complete, cancel).

- Unmatched packages resolution workflow.

- Storage report (utilization, fees, expirations).

- Bookings management (list, create, status, bulk).

- Shipments, users, communications, invoices, manifests.

- Reports (revenue, shipments, customers, by period).

- Activity log. Exception management. Weight discrepancies.

- Settings: pricing, storage config, business hours, general.

**Deliverable:** Admin manages all operations from browser.

Phase 4: Warehouse + Offline (Week 16-19)

**Goal:** Warehouse staff receives, stages, and ships packages. Carrier receiving works offline.

- Warehouse layout: tab nav, mobile-optimized.

- Carrier receiving flow (/warehouse/locker-receive): scan, match, weigh, photo, bay, confirm.

- IndexedDB integration for offline queue. Service worker cache for customer data.

- Offline queue UI: sync status, failed item resolution.

- Cache warming on login.

- Drop-off receiving flow (/warehouse/receiving): existing booking check-in.

- Service queue fulfillment: photos, repackage, inspection.

- Ship queue processing: pull, consolidate, weigh, customs labels, stage.

- Staging area: drag-drop / select-move packages to bays.

- Manifest creation and document generation.

- Scan page, inventory view, exception handling.

**Deliverable:** Warehouse runs full shift offline. Carrier and drop-off packages received. Ship requests processed.

Phase 5: Polish & Launch (Week 20-21)

**Goal:** Production-ready with tests and documentation.

- E2E tests: forwarding flow (signup → receive → ship → deliver), drop-off flow, admin flow.

- Integration tests for all API endpoints. Unit tests for business logic.

- Background jobs: storage fee processor, inbound tracker, expiry notifier.

- Performance: Lighthouse \> 90. WASM bundle optimization.

- Accessibility: keyboard nav, ARIA, contrast.

- Security: CORS, rate limiting, input validation, file upload safety.

- Data seeding scripts. Deployment docs. User guides.

**Deliverable:** Production deployment. All features tested.

12\. Testing & Quality Assurance

12.1 Testing Pyramid

|             |                      |               |                                                                   |
|-------------|----------------------|---------------|-------------------------------------------------------------------|
| **Layer**   | **Tool**             | **Target**    | **Tests**                                                         |
| Unit        | Go testing + testify | 80% services  | Pricing calc, storage fees, weight thresholds, status transitions |
| Integration | httptest + test DB   | All endpoints | Request/response, auth, error codes                               |
| Component   | Go testing           | Critical UI   | Wizard state, offline queue, form validation                      |
| E2E         | Playwright           | Happy paths   | Forwarding flow, drop-off flow, admin, warehouse                  |

12.2 Per-Phase Acceptance Criteria

- All new endpoints have integration tests.

- All service functions have unit tests (happy path + 2 error paths).

- All pages render at 375px, 768px, 1280px.

- No WCAG 2.1 AA violations.

- WASM bundle delta \< 500KB from previous phase.

- All existing tests pass. golangci-lint passes.

12.3 Parity Checklist

- Text content matches spec. Links navigate correctly.

- Form fields present with correct types and validation.

- API calls use correct endpoints, methods, bodies.

- Empty states, loading states, confirmations present.

- Status badges use correct colors. Mobile layout functional.

13\. Build, Deployment & Operations

13.1 Build Pipeline

|              |                                                         |                 |
|--------------|---------------------------------------------------------|-----------------|
| **Step**     | **Command**                                             | **Output**      |
| Lint         | golangci-lint run                                       | Pass/fail       |
| Test         | go test ./\... -race -cover                             | Coverage report |
| Build WASM   | GOOS=js GOARCH=wasm go build -o web/app.wasm ./frontend | \~5-8MB         |
| Compress     | brotli -9 web/app.wasm                                  | \~1-2MB         |
| Build Server | CGO_ENABLED=0 go build -o qcs-server ./cmd/server       | Binary          |
| Docker       | docker build -t qcs-cargo .                             | Image           |

13.2 Deployment

> **Initial**
>
> Single VPS (4 CPU, 8GB). SQLite on SSD. Litestream to S3. Caddy for HTTPS. One binary serves everything.
>
> **Scale Path**
>
> Replace SQLite with PostgreSQL. Multiple instances behind LB. S3 for files. No code changes needed.

13.3 Environment Variables

|                       |              |                                            |
|-----------------------|--------------|--------------------------------------------|
| **Variable**          | **Required** | **Description**                            |
| DATABASE_URL          | Yes          | SQLite path or Postgres connection string  |
| JWT_SECRET            | Yes          | 256-bit signing secret                     |
| STRIPE_SECRET_KEY     | Yes          | Stripe API key                             |
| STRIPE_WEBHOOK_SECRET | Yes          | Webhook signing secret                     |
| RESEND_API_KEY        | Yes          | Email API key                              |
| FROM_EMAIL            | Yes          | Sender address                             |
| APP_URL               | Yes          | Base URL (e.g., https://app.qcs-cargo.com) |
| S3_BUCKET             | No           | File storage bucket                        |
| S3_ENDPOINT           | No           | S3-compatible endpoint                     |
| S3_ACCESS_KEY         | No           | S3 access key                              |
| S3_SECRET_KEY         | No           | S3 secret key                              |
| PORT                  | No           | Server port (default 8080)                 |
| LOG_LEVEL             | No           | Logging level (default info)               |

13.4 Monitoring

- Structured JSON logging via zerolog. Request ID on every request.

- Health: GET /api/v1/health returns { status, db, uptime }.

- Metrics: /metrics (Prometheus). Request count, latency, errors, connections.

- Alerts: error rate \> 1%, p95 \> 500ms, disk \> 80%.

- Backup: weekly automated restore test from Litestream.

13.5 Security

- HTTPS via Caddy with auto Let\'s Encrypt.

- CORS: app origin only. No wildcard.

- Rate limits: 60/min auth, 300/min API, 10/min contact form.

- Input validation server-side. sqlc prevents SQL injection.

- File uploads: MIME validation, size limits (5MB avatar, 10MB photo).

- Passwords: bcrypt cost 12. Tokens: 32-byte crypto/rand, SHA-256 hashed.

- Session revocation: delete from DB, refresh token invalidated immediately.

- Dependency scanning: GitHub Dependabot.

14\. AI-Agentic Development Guide

Guidance for building the application with AI coding agents (Claude, Cursor, etc.). Every prompt should reference a specific PRD section.

14.1 Session Structure

1.  Provide: PRD section number, current file structure (ls -la), specific component being built.

2.  Ask AI to generate complete files (package, imports, types, functions).

3.  Compile immediately (go build ./\...). Fix in same session.

4.  Run tests (go test ./\... -v) before moving to next component.

5.  Commit after each working component.

14.2 Prompt Patterns by Phase

|                |                                                                                                                                                                                              |
|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Phase**      | **Example Prompt**                                                                                                                                                                           |
| 0: Foundation  | \'Create internal/db/db.go: SQLite connection with WAL mode using modernc.org/sqlite. Include Migrate() that runs embedded goose migrations.\'                                               |
| 1: Auth        | \'Create internal/api/auth.go: Fiber handlers for magic-link request/verify/refresh/logout per PRD 6.1. Suite code generation on register per PRD 8.10.\'                                    |
| 2: Inbox       | \'Create frontend/pages/package_inbox.go: go-app component fetching GET /api/v1/locker. Card grid with photo, sender, weight, storage bar (PRD 9.4). Checkbox selection with floating bar.\' |
| 2: Ship Wizard | \'Create frontend/pages/ship_wizard.go: 4-step form per PRD 9.6. Step 1 selects locker packages. Step 3 customs per PRD 9.6 table. Step 4 Stripe payment.\'                                  |
| 3: Admin       | \'Create internal/api/admin/ship_requests.go: handlers for GET/PATCH per PRD 6.9. Include status transitions and activity logging.\'                                                         |
| 4: Warehouse   | \'Create frontend/pages/warehouse_locker_receive.go: 6-step carrier receiving per PRD 9.11. Offline queue via stores.OfflineQueueStore. Camera photo capture via syscall/js.\'               |
| 4: Storage     | \'Create internal/jobs/storage_cron.go: daily ProcessStorageFees(). Find expired packages, create storage_fees records, send notifications per PRD 10.1.\'                                   |

14.3 Code Conventions

- Files: lowercase_snake_case.go. Packages: single word matching directory.

- Errors: always return up. Wrap with fmt.Errorf(\'operation: %w\', err).

- Handlers: func(c \*fiber.Ctx) error. Parse → call service → return JSON.

- Services: pure logic. context.Context first param. No HTTP concerns.

- DB: all queries through sqlc. No raw SQL in handlers.

- Frontend: one component per file. OnMount for data fetching. c.Update() after state changes.

- Validation: shared functions in internal/validation/ used by API and frontend.

14.4 Common Pitfalls

- Frontend code runs in browser. No os.\* or file I/O. Use syscall/js for browser APIs.

- Fiber is NOT net/http. Use c.JSON(), c.Bind(), c.SendStatus().

- Use modernc.org/sqlite (pure Go, no CGO) for portability.

- WASM size: every import adds bytes. Keep heavy logic server-side.

- Stripe Elements: load via script tag, interact via syscall/js.

- Camera: navigator.mediaDevices.getUserMedia via syscall/js.

- IndexedDB: needs syscall/js wrappers or thin JS bridge file.

14.5 Context Window Tips

- Never paste entire PRD. Reference section numbers.

- Keep Go files under 400 lines. Split large components.

- For debugging: error message + file + one PRD section.

- For frontend: provide API response shape (Section 6) + page spec (Section 9).

- Use \'go doc ./internal/models\' output as context.

15\. Appendices

15.1 Background Jobs

|                       |                |                                                                   |
|-----------------------|----------------|-------------------------------------------------------------------|
| **Job**               | **Schedule**   | **Purpose**                                                       |
| StorageFeeProcessor   | Daily midnight | Find expired free storage, create fee records, send notifications |
| StorageExpiryNotifier | Daily 8am      | Send 5-day and 1-day warnings for expiring packages               |
| InboundTrackingPoller | Every 4 hours  | Check carrier APIs for inbound tracking status updates            |
| StorageDisposal       | Daily midnight | Mark packages past 60 days as disposed (with prior notice)        |

15.2 Key Dependencies

|                    |             |                        |
|--------------------|-------------|------------------------|
| **Package**        | **Version** | **Purpose**            |
| go-app             | v10+        | Frontend PWA framework |
| fiber              | v2.x        | HTTP framework         |
| sqlc               | v1.x        | SQL code generation    |
| goose              | v3.x        | Database migrations    |
| golang-jwt/jwt     | v5.x        | JWT handling           |
| stripe-go          | v76+        | Payments               |
| resend-go          | latest      | Email                  |
| zerolog            | v1.x        | Structured logging     |
| modernc.org/sqlite | latest      | Pure Go SQLite         |
| testify            | v1.x        | Test assertions        |

15.3 Document History

|             |            |                                                                                                        |
|-------------|------------|--------------------------------------------------------------------------------------------------------|
| **Version** | **Date**   | **Notes**                                                                                              |
| 1.0         | 2025-02-21 | Initial: SvelteKit frontend extraction                                                                 |
| 2.0         | 2026-02-21 | Full-stack Go architecture, Fiber, SQLite, design system, phasing                                      |
| 3.0         | 2026-02-21 | Unified: parcel forwarding as primary flow, storage, services, customs, notifications, merged addendum |

*End of Document*
