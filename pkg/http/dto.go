package http

type Error struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func NewErrorResponse(message string, code int, details any) ErrorResponse {
	return ErrorResponse{
		Error: Error{
			Message: message,
			Code:    code,
			Details: details,
		},
	}
}
