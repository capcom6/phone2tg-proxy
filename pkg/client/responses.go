package client

type ErrorResponse struct {
	// Error description
	Message string `json:"message"`
	// Error code
	Code int `json:"code,omitempty"`
	// Optional error details
	Details any `json:"details,omitempty"`
}

type MessagesPOSTResponse struct {
	// Message ID
	ID int `json:"id"`
}
