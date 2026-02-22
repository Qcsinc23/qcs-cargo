#!/usr/bin/env bash
# Verify Stripe configuration: app API and optionally Stripe CLI.
# Production URL: https://qcs-cargo.com (set APP_URL and add to Stripe Dashboard redirects).
set -e
cd "$(dirname "$0")/.."
BASE="${STRIPE_VERIFY_BASE:-http://127.0.0.1:8080}"

echo "== Stripe configuration verification"
echo "   Production: https://qcs-cargo.com (set APP_URL in prod; add domain in Stripe Dashboard)"
echo ""

# 1) App API verification (uses STRIPE_SECRET_KEY from env or .env when server runs)
echo "1. App API (server must be running with STRIPE_SECRET_KEY set):"
if curl -sf "$BASE/api/v1/stripe/verify" > /tmp/stripe_verify.json 2>/dev/null; then
  if grep -q '"stripe_ok"' /tmp/stripe_verify.json 2>/dev/null; then
    cat /tmp/stripe_verify.json
    if grep -q '"stripe_ok":true' /tmp/stripe_verify.json 2>/dev/null; then
      echo "   -> Stripe secret key is valid."
    else
      echo "   -> Stripe not configured or key invalid. Set STRIPE_SECRET_KEY and run server."
    fi
  else
    echo "   -> Unexpected response (ensure server is latest build with /api/v1/stripe/verify)."
  fi
else
  echo "   -> Server not reachable at $BASE. Start server (make run) with STRIPE_SECRET_KEY set, then re-run."
fi
echo ""

# 2) Config endpoint (publishable key and stripe_configured)
echo "2. App config (GET /api/v1/config):"
if curl -sf "$BASE/api/v1/config" > /tmp/stripe_config.json 2>/dev/null; then
  if grep -q '"data"' /tmp/stripe_config.json 2>/dev/null; then
    cat /tmp/stripe_config.json
    if grep -q '"stripe_configured":true' /tmp/stripe_config.json 2>/dev/null; then
      echo "   -> STRIPE_SECRET_KEY is set."
    fi
    if grep -q 'stripe_publishable_key' /tmp/stripe_config.json 2>/dev/null; then
      echo "   -> STRIPE_PUBLISHABLE_KEY is set (required for pay page)."
    fi
  else
    echo "   -> Unexpected response (ensure server is latest build)."
  fi
else
  echo "   -> Server not reachable."
fi
echo ""

# 3) Stripe CLI (optional)
if command -v stripe >/dev/null 2>&1; then
  echo "3. Stripe CLI:"
  stripe config --list 2>/dev/null || true
  echo "   Run 'stripe login' if needed; 'stripe balance retrieve' to test API key from CLI."
else
  echo "3. Stripe CLI: not installed. Optional: https://docs.stripe.com/stripe-cli"
fi
echo ""
echo "Production: In Stripe Dashboard add https://qcs-cargo.com to:"
echo "  Developers → Webhooks (if using); Checkout / Payment Intents redirect URLs as needed."
