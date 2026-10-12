// Package start implements the /start onboarding handler.
package start

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// Handler is the /start flow: welcome message, contact request, contact
// storage.
type Handler struct {
	storage    storage.Service
	translator *i18n.Service
}

// New constructs the start handler. It returns handler.Handler - not the
// concrete type - because dig keys value groups by the EXACT result type: a
// concrete return type would make the []handler.Handler slice resolve empty
// with no error.
func New(storage storage.Service, translator *i18n.Service) handler.Handler {
	return &Handler{
		storage:    storage,
		translator: translator,
	}
}

// Register mounts the start routes on the shared dispatcher seam.
func (h *Handler) Register(r *router.Router) error {
	r.Handle(fsm.StateEmpty, "/start", func(ctx *th.Context, update telego.Update, s *router.StateService) error {
		if err := s.Set(fsm.NewState(handlers.StateStartWaitForContact)); err != nil {
			return fmt.Errorf("set state: %w", err)
		}

		return handlers.Send(
			ctx,
			update.Message.Chat.ID,
			h.translator.Translate("welcome_message"),
			h.makeShareContactKeyboard(),
		)
	})

	r.Handle(
		handlers.StateStartWaitForContact,
		router.OnContact,
		func(ctx *th.Context, update telego.Update, s *router.StateService) error {
			contact := update.Message.Contact
			if contact == nil {
				return handlers.Send(
					ctx,
					update.Message.Chat.ID,
					h.translator.Translate("send_contact"),
					h.makeShareContactKeyboard(),
				)
			}

			// SC-2: the contact-ownership check stays in the handler BODY on
			// purpose. As a Predicate it would degrade to a silent false and the
			// user would get no reply at all; here the other user's contact is
			// actively rejected with the share_contact text.
			if contact.UserID != update.Message.Chat.ID {
				return handlers.Send(
					ctx,
					update.Message.Chat.ID,
					h.translator.Translate("share_contact"),
					h.makeShareContactKeyboard(),
				)
			}

			// OD-B-style context: the handler's context must not reach
			// fsm/storage - storage ops keep the fixed background+1s budget.
			storeCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			if err := h.storage.Store(storeCtx, contact.PhoneNumber, update.Message.Chat.ID); err != nil {
				if errors.Is(err, storage.ErrInvalidPhoneNumber) {
					return handlers.Send(ctx, update.Message.Chat.ID, h.translator.Translate("invalid_phone"), nil)
				}
				return fmt.Errorf("set phone number: %w", err)
			}

			if err := s.Delete(); err != nil {
				return fmt.Errorf("delete state: %w", err)
			}

			return handlers.Send(
				ctx,
				update.Message.Chat.ID,
				h.translator.Translate("thanks_contact"),
				&telego.ReplyKeyboardRemove{RemoveKeyboard: true},
			)
		},
	)

	r.Handle(
		handlers.StateStartWaitForContact,
		router.OnText,
		func(ctx *th.Context, update telego.Update, _ *router.StateService) error {
			return handlers.Send(
				ctx,
				update.Message.Chat.ID,
				h.translator.Translate("send_contact"),
				h.makeShareContactKeyboard(),
			)
		},
	)

	return nil
}

func (h *Handler) makeShareContactKeyboard() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: h.translator.Translate("share_contact_button"), RequestContact: true}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}
