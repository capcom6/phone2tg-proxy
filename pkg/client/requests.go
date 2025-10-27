package client

type MessagesPOSTRequest struct {
	// Phone number of the recipient
	PhoneNumber string `json:"phoneNumber" validate:"required,e164"           example:"+15551234567"`
	// Message text
	Text string `json:"text"        validate:"required,min=1,max=4096" example:"Hello from phone2tg-proxy"`
}
