package client

type MessagesPOSTRequest struct {
	PhoneNumber string `json:"phoneNumber" validate:"required,e164"`
	Text        string `json:"text" validate:"required,min=1,max=4096"`
}
