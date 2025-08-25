package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/pkg/client"
	"github.com/capcom6/phone2tg-proxy/pkg/handler"
	"github.com/capcom6/phone2tg-proxy/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type MessagesHandler struct {
	proxy proxy.Service

	handler.Base
}

func NewMessagesHandler(proxy proxy.Service, v *validator.Validate, logger *zap.Logger) *MessagesHandler {
	return &MessagesHandler{
		proxy: proxy,

		Base: handler.Base{
			Validator: v,
			Logger:    logger,
		},
	}
}

// @Summary Send message
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body client.MessagesPOSTRequest true "Request"
// @Success 200 {object} client.MessagesPOSTResponse
// @Failure 400 {object} client.ErrorResponse
// @Failure 404 {object} client.ErrorResponse
// @Failure 500 {object} client.ErrorResponse
// @Router /messages [post]
//
// Send message
func (h *MessagesHandler) post(c *fiber.Ctx) error {
	req := new(client.MessagesPOSTRequest)

	if err := h.BodyParserValidator(c, req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid request: %s", err.Error()))
	}

	ctx, cancel := context.WithTimeout(c.Context(), 1*time.Second)
	defer cancel()

	id, err := h.proxy.Send(ctx, req.PhoneNumber, req.Text)
	if errors.Is(err, proxy.ErrPhoneNumberNotFound) {
		return fiber.NewError(fiber.StatusNotFound, "phone number not found")
	}

	if err != nil {
		return fiber.NewError(
			fiber.StatusInternalServerError,
			"failed to send message, please try again later or contact support",
		)
	}

	return c.JSON(&client.MessagesPOSTResponse{
		ID: id,
	})
}

func (h *MessagesHandler) Register(r fiber.Router) {
	r.Post("", h.post)
}
