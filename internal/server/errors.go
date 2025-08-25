package server

import (
	"github.com/capcom6/phone2tg-proxy/pkg/client"
)

func errorsFormatter(err error, code int) any {
	return &client.ErrorResponse{
		Message: err.Error(),
		Code:    code,
		Details: nil,
	}
}
