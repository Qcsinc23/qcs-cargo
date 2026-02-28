package api

import "testing"

func TestNormalizeAuthEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "user@example.com", want: "user@example.com"},
		{name: "trims whitespace", input: "  user@example.com  ", want: "user@example.com"},
		{name: "lowercases local and domain", input: "User.Name+Tag@Example.COM", want: "user.name+tag@example.com"},
		{name: "empty after trim", input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAuthEmail(tt.input); got != tt.want {
				t.Fatalf("normalizeAuthEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
