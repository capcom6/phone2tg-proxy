// Package stop implements the /stop command handler.
package stop

import (
	"context"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/telegofx/fsm"
	tgpred "github.com/go-core-fx/telegofx/predicates"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"go.uber.org/zap"
)

// Handler is the /stop flow: it deletes the phone-to-chat association. It
// never touches the FSM state (Python parity).
type Handler struct {
	storage    storage.Service
	logger     *zap.Logger
	translator *i18n.Service
	fsm        *fsm.Store
}

// New constructs the stop handler. It returns handler.Handler - not the
// concrete type - because dig keys value groups by the EXACT result type: a
// concrete return type would make the []handler.Handler slice resolve empty
// with no error.
func New(
	store *fsm.Store,
	storage storage.Service,
	logger *zap.Logger,
	translator *i18n.Service,
) handler.Handler {
	return &Handler{
		storage:    storage,
		logger:     logger,
		translator: translator,
		fsm:        store,
	}
}

// Register mounts the /stop route directly on the router. SC-1 holds through
// two predicate failures plus telegohandler's silent no-match: mid-onboarding
// the fsm.StateEqual(fsm.StateEmpty) gate is false, and the start handler's
// wait-state text route rejects the '/'-prefixed text with
// th.Not(th.AnyCommand()) -
// so /stop during wait_for_contact causes no reply, no storage.Delete and no
// state change. th.CommandEqual is case-insensitive and ignores the @suffix,
// so "/STOP" and "/stop@AnyBot" both match in StateEmpty.
func (h *Handler) Register(router *telegofx.Router) {
	router.Handle(
		h.handleStop,
		tgpred.MessageWithChatType(telego.ChatTypePrivate),
		fsm.StateEqual(h.fsm, fsm.StateEmpty),
		th.CommandEqual("stop"),
	)
}

func (h *Handler) handleStop(ctx *th.Context, update telego.Update) error {
	telegramID := update.Message.Chat.ID

	// The handler's context must not reach fsm/storage - the delete keeps
	// the fixed background+1s budget.
	deleteCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Perform atomic deletion
	if err := h.storage.Delete(deleteCtx, telegramID); err != nil {
		h.logger.Error("failed to delete association", zap.Int64("telegram_id", telegramID), zap.Error(err))
		if sendErr := handlers.Send(ctx, telegramID, h.translator.Translate("delete_failed"), nil); sendErr != nil {
			return fmt.Errorf("send delete failed: %w", sendErr)
		}
		return nil
	}

	// Audit logging
	h.logger.Info("Association deleted", zap.Int64("telegram_id", telegramID))

	if sendErr := handlers.Send(ctx, telegramID, h.translator.Translate("delete_success"), nil); sendErr != nil {
		return fmt.Errorf("send delete success: %w", sendErr)
	}
	return nil
}
