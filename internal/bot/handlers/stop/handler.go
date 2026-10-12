// Package stop implements the /stop command handler.
package stop

import (
	"context"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"go.uber.org/zap"
)

// Handler is the /stop flow: it deletes the phone-to-chat association.
type Handler struct {
	storage    storage.Service
	logger     *zap.Logger
	translator *i18n.Service
}

// New constructs the stop handler. It returns handler.Handler - not the
// concrete type - because dig keys value groups by the EXACT result type: a
// concrete return type would make the []handler.Handler slice resolve empty
// with no error.
func New(storage storage.Service, logger *zap.Logger, translator *i18n.Service) handler.Handler {
	return &Handler{
		storage:    storage,
		logger:     logger,
		translator: translator,
	}
}

// Register mounts the stop route on the shared dispatcher seam.
func (h *Handler) Register(r *router.Router) error {
	r.Handle(fsm.StateEmpty, "/stop", func(ctx *th.Context, update telego.Update, _ *router.StateService) error {
		telegramID := update.Message.Chat.ID

		// The handler's context must not reach fsm/storage - the delete keeps
		// the fixed background+1s budget.
		deleteCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Perform atomic deletion
		if err := h.storage.Delete(deleteCtx, telegramID); err != nil {
			h.logger.Error("failed to delete association", zap.Int64("telegram_id", telegramID), zap.Error(err))
			return handlers.Send(ctx, telegramID, h.translator.Translate("delete_failed"), nil)
		}

		// Audit logging
		h.logger.Info("Association deleted", zap.Int64("telegram_id", telegramID))

		return handlers.Send(ctx, telegramID, h.translator.Translate("delete_success"), nil)
	})

	return nil
}
