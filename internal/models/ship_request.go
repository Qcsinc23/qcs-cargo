package models

// ShipRequest is a customer forwarding request per PRD 5.2.
type ShipRequest struct {
	ID                     string   `json:"id"`
	UserID                 string   `json:"user_id"`
	ConfirmationCode       string   `json:"confirmation_code"`
	Status                 string   `json:"status"` // draft|pending_customs|pending_payment|paid|processing|shipped|delivered|cancelled
	DestinationID          string   `json:"destination_id"`
	RecipientID            string   `json:"recipient_id,omitempty"`
	ServiceType            string   `json:"service_type"`
	Consolidate            bool     `json:"consolidate"`
	SpecialInstructions    string   `json:"special_instructions,omitempty"`
	Subtotal               float64  `json:"subtotal"`
	ServiceFees            float64  `json:"service_fees"`
	Insurance              float64  `json:"insurance"`
	Discount               float64  `json:"discount"`
	Total                  float64  `json:"total"`
	PaymentStatus          string   `json:"payment_status,omitempty"`
	StripePaymentIntentID  string   `json:"stripe_payment_intent_id,omitempty"`
	CustomsStatus          string   `json:"customs_status,omitempty"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
}
