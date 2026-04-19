package services

import (
	"os"
	"strings"
	"testing"
)

// TestEmailTemplates_EscapeUserInput is the HIGH-05 regression test.
// User-controlled fields interpolated into the branded HTML layout
// must be HTML-escaped to prevent stored XSS in email clients that
// render inline HTML.
func TestEmailTemplates_EscapeUserInput(t *testing.T) {
	// Force resendClient() to return nil so we exercise the layout
	// builders without trying to make a real HTTP call.
	_ = os.Setenv("RESEND_API_KEY", "")

	payload := `<script>alert(1)</script>`

	// We cannot call the Send* functions directly because they exit early
	// when client == nil. Instead, exercise escapeHTML and emailLayout/Body
	// helpers against a malicious payload to prove the contract.
	escaped := escapeHTML(payload)
	if strings.Contains(escaped, "<script>") {
		t.Fatalf("escapeHTML must encode <script>, got %q", escaped)
	}
	if !strings.Contains(escaped, "&lt;script&gt;") {
		t.Fatalf("escapeHTML missing escaped form, got %q", escaped)
	}

	// Apostrophe coverage — Pass 2.5 MED-09.
	apos := escapeHTML("O'Brien")
	if strings.Contains(apos, "'") {
		t.Fatalf("escapeHTML must encode apostrophe, got %q", apos)
	}
	if !strings.Contains(apos, "&#39;") {
		t.Fatalf("escapeHTML missing apostrophe entity, got %q", apos)
	}

	// Existing characters still encoded.
	quote := escapeHTML(`"&<>'`)
	expected := `&quot;&amp;&lt;&gt;&#39;`
	// Note: & must be processed first; verify the order doesn't double-encode.
	if quote != expected {
		t.Fatalf("escapeHTML order check: got %q, want %q", quote, expected)
	}
}

// TestEmailLayout_PreheaderEscaped verifies that when callers wrap
// user-controlled fields with escapeHTML() before passing them to
// emailLayout(), the rendered HTML does not contain executable markup
// in the hidden preheader <div>. This mirrors the contract enforced
// by the per-Send wrappers updated for HIGH-05.
func TestEmailLayout_PreheaderEscaped(t *testing.T) {
	payload := `<script>alert(1)</script>`

	// Simulate the SendShipRequestPaid call site after the HIGH-05 fix:
	// emailLayout("Ship request "+escapeHTML(code)+" confirmed", ...)
	preheader := "Ship request " + escapeHTML(payload) + " confirmed"
	html := emailLayout(preheader, "Ship Request Confirmed", emailParagraph("body"))

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("rendered layout must not contain executable <script> from preheader; got HTML containing raw payload")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("rendered layout missing escaped preheader payload")
	}

	// Double-interpolated case mirroring SendServiceComplete /
	// SendShipmentStatus where two user fields land in the preheader.
	preheader2 := "Shipment " + escapeHTML(payload) + " update: " + escapeHTML(payload)
	html2 := emailLayout(preheader2, "Shipment Update", emailParagraph("body"))
	if strings.Contains(html2, "<script>") {
		t.Fatalf("multi-field preheader must not contain raw <script>")
	}
}
