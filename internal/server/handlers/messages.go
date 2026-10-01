package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/pkg/client"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type MessagesHandler struct {
	handler.Base

	proxySvc proxy.Service

	logger *zap.Logger
}

// NewMessagesHandler builds the messages handler.
//
// It returns the handler.Handler interface, not *MessagesHandler: internal/server/module.go consumes the value as a []handler.Handler dig group, and a value group is keyed by the EXACT result type. A concrete result therefore resolves to an EMPTY slice - silently, with no dig error - and the route is never registered.
func NewMessagesHandler(proxySvc proxy.Service, v *validator.Validate, logger *zap.Logger) handler.Handler {
	return &MessagesHandler{
		proxySvc: proxySvc,

		Base: handler.Base{
			Validator: v,
		},

		logger: logger,
	}
}

//	@Summary	Send message
//	@Tags		Messages
//	@Accept		json
//	@Produce	json
//	@Param		request	body		client.MessagesPOSTRequest	true	"Request"
//	@Success	200		{object}	client.MessagesPOSTResponse
//	@Failure	400		{object}	client.ErrorResponse
//	@Failure	404		{object}	client.ErrorResponse
//	@Failure	500		{object}	client.ErrorResponse
//	@Router		/messages [post]
//
// Send message.
func (h *MessagesHandler) post(c *fiber.Ctx) error {
	req := new(client.MessagesPOSTRequest)

	if err := h.BodyParserValidator(c, req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid request: %s", err.Error()))
	}

	ctx, cancel := context.WithTimeout(c.Context(), 1*time.Second)
	defer cancel()

	id, err := h.proxySvc.Send(ctx, req.PhoneNumber, req.Text)
	if errors.Is(err, proxy.ErrPhoneNumberNotFound) {
		h.logger.Warn("phone number not found", zap.Error(err))
		return fiber.NewError(fiber.StatusNotFound, "phone number not found")
	}

	if err != nil {
		h.logger.Error("failed to send message", zap.Error(err))
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
	g := r.Group("/messages")

	g.Post("", h.post)
}
