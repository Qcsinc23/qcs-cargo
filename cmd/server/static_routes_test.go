package main

import "testing"

func TestResolveStaticPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: "index.html"},
		{in: "/", want: "index.html"},
		{in: "dashboard", want: "dashboard/index.html"},
		{in: "admin", want: "admin/index.html"},
		{in: "admin/ship-requests", want: "admin/ship-requests.html"},
		{in: "admin/users/123", want: "admin/users.html"},
		{in: "admin/common.js", want: "admin/common.js"},
		{in: "warehouse", want: "warehouse/index.html"},
		{in: "warehouse/receiving", want: "warehouse/receiving.html"},
		{in: "warehouse/unknown", want: "warehouse/unknown.html"},
		{in: "dashboard/inbox/abc", want: "dashboard/inbox-detail.html"},
		{in: "dashboard/ship-requests/abc", want: "dashboard/ship-request-detail.html"},
		{in: "dashboard/ship-requests/abc/customs", want: "dashboard/customs.html"},
		{in: "dashboard/ship-requests/abc/pay", want: "dashboard/pay.html"},
		{in: "dashboard/ship-requests/abc/confirmation", want: "dashboard/confirmation.html"},
		{in: "dashboard/inbound/abc", want: "dashboard/inbound-detail.html"},
		{in: "dashboard/bookings/new", want: "dashboard/booking-wizard.html"},
		{in: "dashboard/bookings/abc", want: "dashboard/booking-detail.html"},
		// Dashboard UX review: shipment + invoice detail routes were missing.
		{in: "dashboard/shipments/abc", want: "dashboard/shipment-detail.html"},
		{in: "dashboard/invoices/abc", want: "dashboard/invoice-detail.html"},
		{in: "verify", want: "verify.html"},
		{in: "verify-email", want: "verify-email.html"},
		{in: "login", want: "login.html"},
		{in: "web/images/logo.png", want: "web/images/logo.png"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := resolveStaticPath(tc.in)
			if got != tc.want {
				t.Fatalf("resolveStaticPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
