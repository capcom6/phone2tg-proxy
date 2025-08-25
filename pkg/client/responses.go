package client

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

type MessagesPOSTResponse struct {
	ID int `json:"id"`
}
