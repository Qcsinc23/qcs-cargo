package services

import (
	"strings"
	"testing"
)

// TestSanitizeEmailSubject_StripsCRLF is the HIGH-08 regression test.
// User-supplied subject lines must have control characters stripped so
// they cannot inject mail headers (e.g. "Bcc:") into the outgoing email.
func TestSanitizeEmailSubject_StripsCRLF(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantNot []string // substrings that MUST NOT appear in output
		wantIn  string   // substring that MUST appear
	}{
		// Note: with CR/LF stripped, the literal substring "Bcc:" cannot
		// inject a header (it becomes ordinary Subject text). The original
		// HIGH-08 vector is the CR/LF, so we assert those are gone.
		{"crlf header injection", "Help me\r\nBcc: attacker@x.com", []string{"\r", "\n"}, "Help me"},
		{"bare cr", "X\rY", []string{"\r"}, "XY"},
		{"bare lf", "X\nY", []string{"\n"}, "XY"},
		{"tabs and del", "A\tB\x7fC", []string{"\t", "\x7f"}, "ABC"},
		{"length cap", strings.Repeat("a", 500), nil, strings.Repeat("a", 200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeEmailSubject(tc.in)
			for _, bad := range tc.wantNot {
				if strings.Contains(got, bad) {
					t.Fatalf("sanitizeEmailSubject(%q) must not contain %q, got %q", tc.in, bad, got)
				}
			}
			if tc.wantIn != "" && !strings.Contains(got, tc.wantIn) {
				t.Fatalf("sanitizeEmailSubject(%q) expected to contain %q, got %q", tc.in, tc.wantIn, got)
			}
			if len(got) > 200 {
				t.Fatalf("sanitizeEmailSubject must cap at 200 chars, got %d", len(got))
			}
		})
	}
}
