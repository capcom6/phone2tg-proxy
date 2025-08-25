package server

import (
	"net/http"

	"github.com/capcom6/phone2tg-proxy/pkg/client"
)

func errorsFormatter(err error, code int) any {
	msg := err.Error()
	// For server-side errors, don't leak details.
	if code >= http.StatusInternalServerError {
		msg = http.StatusText(code)
	}
	return &client.ErrorResponse{
		Message: msg,
		Code:    code,
		Details: nil,
	}
}
