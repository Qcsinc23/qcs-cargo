package models

// LockerPackage is a package in customer storage (the inbox) per PRD 5.2.
type LockerPackage struct {
	ID                   string  `json:"id"`
	UserID               string  `json:"user_id"`
	SuiteCode            string  `json:"suite_code"`
	TrackingInbound       string  `json:"tracking_inbound,omitempty"`
	CarrierInbound       string  `json:"carrier_inbound,omitempty"`
	SenderName           string  `json:"sender_name,omitempty"`
	SenderAddress        string  `json:"sender_address,omitempty"`
	WeightLbs            float64 `json:"weight_lbs,omitempty"`
	LengthIn             float64 `json:"length_in,omitempty"`
	WidthIn              float64 `json:"width_in,omitempty"`
	HeightIn             float64 `json:"height_in,omitempty"`
	ArrivalPhotoURL      string  `json:"arrival_photo_url,omitempty"`
	Condition            string  `json:"condition,omitempty"`
	StorageBay           string  `json:"storage_bay,omitempty"`
	Status               string  `json:"status"` // stored|service_pending|ship_requested|shipped|expired|disposed
	ArrivedAt            string  `json:"arrived_at,omitempty"`
	FreeStorageExpiresAt string  `json:"free_storage_expires_at,omitempty"`
	DisposedAt           string  `json:"disposed_at,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}
