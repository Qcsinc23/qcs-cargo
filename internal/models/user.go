package models

// User is the shared user type per PRD 5.1. Used by API and frontend.
type User struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Email            string  `json:"email"`
	Phone            string  `json:"phone,omitempty"`
	Role             string  `json:"role"` // customer | staff | admin
	AvatarURL        string  `json:"avatar_url,omitempty"`
	SuiteCode        string  `json:"suite_code,omitempty"`
	AddressStreet    string  `json:"address_street,omitempty"`
	AddressCity      string  `json:"address_city,omitempty"`
	AddressState     string  `json:"address_state,omitempty"`
	AddressZip       string  `json:"address_zip,omitempty"`
	StoragePlan      string  `json:"storage_plan"` // free | premium
	FreeStorageDays  int     `json:"free_storage_days"`
	EmailVerified    bool    `json:"email_verified"`
	Status           string  `json:"status"` // active | suspended | etc.
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}
