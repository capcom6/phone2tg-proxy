package client

type MessagesPOSTRequest struct {
	// Phone number of the recipient
	PhoneNumber string `json:"phoneNumber" validate:"required,e164"`
	// Message text
	Text string `json:"text" validate:"required,min=1,max=4096"`
}
