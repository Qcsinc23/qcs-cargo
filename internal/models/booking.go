package models

// Booking is a drop-off shipping booking per PRD 5.3.
type Booking struct {
	ID                     string  `json:"id"`
	UserID                 string  `json:"user_id"`
	ConfirmationCode       string  `json:"confirmation_code"`
	Status                 string  `json:"status"`
	ServiceType            string  `json:"service_type"`
	DestinationID          string  `json:"destination_id"`
	RecipientID            string  `json:"recipient_id,omitempty"`
	ScheduledDate          string  `json:"scheduled_date"`
	TimeSlot               string  `json:"time_slot"`
	SpecialInstructions    string  `json:"special_instructions,omitempty"`
	Subtotal               float64 `json:"subtotal"`
	Discount               float64 `json:"discount"`
	Insurance              float64 `json:"insurance"`
	Total                  float64 `json:"total"`
	PaymentStatus          string  `json:"payment_status,omitempty"`
	StripePaymentIntentID  string  `json:"stripe_payment_intent_id,omitempty"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}
