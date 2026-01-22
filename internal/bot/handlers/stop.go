package handlers

import (
	"context"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

type StopHandler struct {
	storage    storage.Service
	logger     *zap.Logger
	translator *i18n.Service
}

func NewStopHandler(storage storage.Service, logger *zap.Logger, translator *i18n.Service) *StopHandler {
	return &StopHandler{
		storage:    storage,
		logger:     logger,
		translator: translator,
	}
}

func (h *StopHandler) Register(r *router.Router) error {
	r.Handle(fsm.StateEmpty, "/stop", func(c telebot.Context, _ *router.StateService) error {
		telegramID := c.Chat().ID

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Perform atomic deletion
		if err := h.storage.Delete(ctx, telegramID); err != nil {
			h.logger.Error("failed to delete association", zap.Int64("telegram_id", telegramID), zap.Error(err))
			return c.Send(h.translator.Translate("delete_failed"))
		}

		// Audit logging
		h.logger.Info("Association deleted", zap.Int64("telegram_id", telegramID))

		return c.Send(h.translator.Translate("delete_success"))
	})

	return nil
}
