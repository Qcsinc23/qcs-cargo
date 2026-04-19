# QCS Cargo — User Guide

## For customers

Sign up with name, email, and password. Verify your email through the magic link or password flow. The system assigns a personal **suite code** that uniquely identifies your locker at the QCS warehouse.

After sign-in, the dashboard sidebar exposes the full customer flow:

- **Dashboard** — overview with packages-in-locker, expected arrivals, pending ship requests, and shipments-in-flight stats. Storage-fee banner surfaces when free storage is about to expire.
- **My Mailbox** — copy your US shipping address (street, suite code, city, ZIP). Use it at any US retailer checkout.
- **Expected Packages** — pre-alert inbound parcels by carrier and tracking number so the warehouse can match them to your suite as soon as they arrive.
- **My Packages** — packages currently in your locker. Per-package storage countdown bar; bulk-select to ship.
- **Ship a Package** — 4-step wizard: choose packages, destination, recipient, then review and submit. The wizard supports pre-selecting packages via `?packages=` query param.
- **Ship Requests** — list of submitted ship requests with status pills (`draft`, `pending_customs`, `pending_payment`, `paid`, `processing`, `staged`, `shipped`, `delivered`, etc.). Detail page surfaces a Customs-declaration link and a "Pay now" CTA when relevant.
- **Customs declaration** — per-item form (description, value, quantity, HS code, country of origin, weight) for ship requests in `draft` or `pending_customs`. Locked card appears once the request has moved past customs.
- **Shipments** — outbound shipments with carrier tracking, destination, and status. Detail page lets you copy the tracking number.
- **Bookings** — schedule a drop-off / pickup window. 5-step booking wizard with date and time-slot picker.
- **Recipients** — saved delivery contacts with default-recipient toggle and inline edit.
- **Templates** — reusable ship-request presets with inline edit.
- **Parcel+** — consolidation preview, package photos, customs documents, repack suggestion, loyalty summary.
- **Invoices** — billing records with line items, subtotal, tax, total.
- **Profile** — name, phone, mailing address. Suite code with one-click copy.
- **Settings** — `Notifications` (channel + event preferences with push enable), `Security` (password change), `Sessions` (active devices, revoke individual or all-other sessions), `Account lifecycle` (deactivate or permanently delete with anonymization).

Every page supports a floating utility dock for theme (light/dark) toggle, language (`en` / `es`), and a `?` keyboard shortcuts dialog. Press `g d` for Dashboard, `g i` for My Packages, `g s` for Shipments, `g n` for Notifications.

## For warehouse staff

Warehouse staff log in via the staff portal. The staff workflow covers locker receive, service queue, ship queue, receiving, and staging:

- **Locker receive** — record packages arriving for a customer's suite code; unmatched receives flow to a queue for resolution.
- **Service queue** — value-added service requests (photo, content inspection, repackage, fragile wrap, etc.) awaiting processing.
- **Ship queue** — ship requests ready to pack and ship.
- **Receiving / Staging** — inbound shipment intake, staging bays, manifest assembly.
- **Bay management** — move packages between bays with optimistic-concurrency guards (mismatched expected status returns 409).

## For admins

Admins access the admin dashboard at `/admin/`:

- **Overview** — locker, ship requests, revenue, and queue summaries.
- **Reports** — summaries by period, destination, or customer.
- **Users** — manage customer and staff accounts, set roles (`customer`, `staff`, `admin`).
- **System health** — DB health, Stripe / Resend config flags, queue counters, metrics endpoint pointer.
- **Insights** — analytics, performance (slow routes), error, and business summaries from `observability_events`.
- **Moderation** — admin moderation queue.
- **Bookings, ship requests, locker packages** — full lifecycle visibility.
- **Settings** — system options, Stripe and email configuration, feature flags, and other site-wide defaults.

To promote a user to admin, update the DB:

```bash
sqlite3 qcs.db "UPDATE users SET role = 'admin', updated_at = datetime('now') WHERE email = 'admin@example.com';"
```
