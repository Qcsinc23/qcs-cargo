# Parcel Feature Additions

Wave 11 adds parcel-forwarding MVP endpoints for the remaining product gaps:

- Consolidation preview: `POST /api/v1/parcel/consolidation-preview`
- Assisted purchase requests: `POST/GET /api/v1/parcel/assisted-purchases`
- Customer package photos: `GET /api/v1/parcel/photos`
- Customs pre-clearance document metadata: `POST/GET /api/v1/parcel/customs-docs`
- Delivery signature capture/read: `POST /api/v1/parcel/delivery-signature`, `GET /api/v1/parcel/delivery-signature/:shipRequestID`
- Repack optimization suggestion: `POST /api/v1/parcel/repack-suggestion`
- User data export: `GET /api/v1/data/export?format=json|csv`
- Recipient import with validation: `POST /api/v1/data/recipients/import`
- Loyalty summary: `GET /api/v1/parcel/loyalty-summary`

UI surface:

- `/dashboard/parcel-plus` provides a lightweight customer view for consolidation preview, package photos, and loyalty summary.

Implementation notes:

- Consolidation and repack recommendations use heuristic estimates based on current dimensions and billable-weight reduction.
- Package photos unify `locker_packages.arrival_photo_url` and `locker_photos`.
- Recipient import supports multi-address payloads and records import-job metadata for auditability.
