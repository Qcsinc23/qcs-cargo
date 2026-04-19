from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.utils import simpleSplit
from reportlab.pdfgen import canvas


PAGE_WIDTH, PAGE_HEIGHT = letter
MARGIN_X = 40
MARGIN_Y = 36
COLUMN_GAP = 20
CONTENT_WIDTH = PAGE_WIDTH - (MARGIN_X * 2)
COLUMN_WIDTH = (CONTENT_WIDTH - COLUMN_GAP) / 2
LEFT_X = MARGIN_X
RIGHT_X = MARGIN_X + COLUMN_WIDTH + COLUMN_GAP
ACCENT = colors.HexColor("#10324A")
ACCENT_2 = colors.HexColor("#2A6F97")
TEXT = colors.HexColor("#10212B")
MUTED = colors.HexColor("#5A6B78")
PANEL = colors.HexColor("#F3F7FA")
RULE = colors.HexColor("#D7E1E8")


TITLE = "QCS Cargo"
SUBTITLE = "One-page repo-based app summary"

WHAT_IT_IS = (
    "QCS Cargo is a parcel-forwarding and warehouse-operations app for moving "
    "packages from a U.S. suite address to Caribbean destinations. The current "
    "repo ships a Go Fiber backend, embedded static web pages, and a lightweight Go/WASM shell."
)

WHO_ITS_FOR = (
    "Primary persona: a customer who shops U.S. online stores, sends purchases to "
    "a personal QCS suite address, then consolidates, pays for, and tracks outbound shipments."
)

FEATURES = [
    "Account flow with registration, email verification, password reset/change, and magic-link sign-in.",
    "Public customer tools for destinations, shipping calculator, contact form, and tracking by tracking number or confirmation code.",
    "Bookings and ship requests with server-side pricing, customs submission, Stripe payment intent creation, and shipment history.",
    "Locker package workflow with suite-code intake, package photos, free-storage expiry tracking, and service requests.",
    "Parcel-plus tools including consolidation preview, assisted purchases, customs docs, delivery signatures, loyalty summary, and recipient import/export.",
    "Notifications via in-app inbox, read state, preference controls, SSE stream, and push subscription endpoint.",
    "Staff/admin warehouse flows for receiving, service queue, ship queue, manifests, exceptions, and operational dashboards.",
]

ARCHITECTURE = [
    "Fiber server in cmd/server/main.go serves /api, embedded static pages from internal/static, uploads, /web assets, /metrics, and the optional app.wasm loader.",
    "internal/api/routes.go wires feature groups for auth, public pages/tools, locker, bookings, ship requests, shipments, warehouse, admin, notifications, security/compliance, parcel features, platform ops, and blog.",
    "Handlers call internal/services for auth, email, pricing, cache, and observability logic, then use sqlc-generated queries in internal/db/gen for persistence.",
    "The data layer is currently wired to SQLite through modernc.org/sqlite in internal/db/db.go, with WAL, foreign keys, and busy timeout enabled.",
    "cmd/migrate/main.go runs SQL migrations from sql/migrations; docs/database/SCHEMA.md mirrors the active schema from migrations and sqlc sources.",
    "internal/jobs runs storage-fee and expiry-notifier jobs once on startup and every 24 hours; cache defaults to memory and upgrades to redis+memory when REDIS_URL is set.",
]

RUN_STEPS = [
    "Install Go 1.25.0.",
    "Copy .env.example to .env and set at least JWT_SECRET; APP_URL/RESEND_API_KEY/STRIPE_SECRET_KEY are optional for local startup.",
    "Run make migrate.",
    "Run make run.",
    "Open http://localhost:8080. Optional checks: /api/v1/health and /metrics.",
]


def draw_heading(pdf: canvas.Canvas, x: float, y: float, text: str) -> float:
    pdf.setFont("Helvetica-Bold", 12.5)
    pdf.setFillColor(ACCENT)
    pdf.drawString(x, y, text.upper())
    pdf.setStrokeColor(RULE)
    pdf.setLineWidth(1)
    pdf.line(x, y - 4, x + 54, y - 4)
    return y - 16


def draw_paragraph(
    pdf: canvas.Canvas,
    x: float,
    y: float,
    width: float,
    text: str,
    font_name: str = "Helvetica",
    font_size: float = 9.5,
    leading: float = 12,
    color=TEXT,
) -> float:
    lines = simpleSplit(text, font_name, font_size, width)
    pdf.setFont(font_name, font_size)
    pdf.setFillColor(color)
    text_obj = pdf.beginText(x, y)
    text_obj.setLeading(leading)
    for line in lines:
        text_obj.textLine(line)
    pdf.drawText(text_obj)
    return y - (leading * len(lines))


def draw_bullets(
    pdf: canvas.Canvas,
    x: float,
    y: float,
    width: float,
    items: list[str],
    bullet_gap: float = 10,
    font_size: float = 9.2,
    leading: float = 11.5,
) -> float:
    bullet_width = width - bullet_gap
    pdf.setFont("Helvetica", font_size)
    pdf.setFillColor(TEXT)
    for item in items:
        lines = simpleSplit(item, "Helvetica", font_size, bullet_width)
        pdf.drawString(x, y, "-")
        text_obj = pdf.beginText(x + bullet_gap, y)
        text_obj.setLeading(leading)
        for line in lines:
            text_obj.textLine(line)
        pdf.drawText(text_obj)
        y -= leading * len(lines) + 4
    return y


def main() -> None:
    repo_root = Path(__file__).resolve().parents[2]
    output_dir = repo_root / "output" / "pdf"
    output_dir.mkdir(parents=True, exist_ok=True)
    pdf_path = output_dir / "qcs-cargo-app-summary.pdf"

    pdf = canvas.Canvas(str(pdf_path), pagesize=letter)
    pdf.setTitle("QCS Cargo App Summary")
    pdf.setAuthor("OpenAI Codex")
    pdf.setSubject("Repo-based one-page summary")

    # Header band
    pdf.setFillColor(PANEL)
    pdf.roundRect(MARGIN_X, PAGE_HEIGHT - 160, CONTENT_WIDTH, 112, 18, fill=1, stroke=0)

    y = PAGE_HEIGHT - 76
    pdf.setFont("Helvetica-Bold", 24)
    pdf.setFillColor(ACCENT)
    pdf.drawString(MARGIN_X + 16, y, TITLE)

    pdf.setFont("Helvetica", 10)
    pdf.setFillColor(MUTED)
    pdf.drawString(MARGIN_X + 16, y - 18, SUBTITLE)
    pdf.drawString(MARGIN_X + 16, y - 32, "Evidence source: current repository files only")

    y = PAGE_HEIGHT - 178
    y = draw_heading(pdf, MARGIN_X, y, "What it is")
    y = draw_paragraph(pdf, MARGIN_X, y, CONTENT_WIDTH, WHAT_IT_IS)

    left_y = y - 18
    right_y = y - 18

    left_y = draw_heading(pdf, LEFT_X, left_y, "Who it's for")
    left_y = draw_paragraph(pdf, LEFT_X, left_y, COLUMN_WIDTH, WHO_ITS_FOR)
    left_y -= 14
    left_y = draw_heading(pdf, LEFT_X, left_y, "What it does")
    left_y = draw_bullets(pdf, LEFT_X, left_y, COLUMN_WIDTH, FEATURES)

    right_y = draw_heading(pdf, RIGHT_X, right_y, "How it works")
    right_y = draw_bullets(pdf, RIGHT_X, right_y, COLUMN_WIDTH, ARCHITECTURE, bullet_gap=10, font_size=9.0, leading=11.0)
    right_y -= 6
    right_y = draw_heading(pdf, RIGHT_X, right_y, "How to run")
    right_y = draw_bullets(pdf, RIGHT_X, right_y, COLUMN_WIDTH, RUN_STEPS)

    min_y = min(left_y, right_y)
    if min_y < MARGIN_Y + 14:
        raise RuntimeError(f"Content overflowed the page (lowest y={min_y:.2f}).")

    pdf.setStrokeColor(RULE)
    pdf.setLineWidth(1)
    pdf.line(MARGIN_X, 28, PAGE_WIDTH - MARGIN_X, 28)
    pdf.setFont("Helvetica", 8)
    pdf.setFillColor(MUTED)
    pdf.drawString(MARGIN_X, 16, "Generated from repo evidence on 2026-03-06")
    pdf.drawRightString(PAGE_WIDTH - MARGIN_X, 16, "1 page")

    pdf.showPage()
    pdf.save()
    print(pdf_path)


if __name__ == "__main__":
    main()
