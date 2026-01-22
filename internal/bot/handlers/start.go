package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"gopkg.in/telebot.v4"
)

type StartHandler struct {
	storage    storage.Service
	translator *i18n.Service
}

func NewStartHandler(storage storage.Service, translator *i18n.Service) *StartHandler {
	return &StartHandler{
		storage:    storage,
		translator: translator,
	}
}

func (h *StartHandler) Register(r *router.Router) error {
	r.Handle(fsm.StateEmpty, "/start", func(c telebot.Context, s *router.StateService) error {
		if err := s.Set(fsm.NewState(StateStartWaitForContact)); err != nil {
			return fmt.Errorf("set state: %w", err)
		}

		return c.Send(h.translator.Translate("welcome_message"), h.makeShareContactKeyboard())
	})

	r.Handle(StateStartWaitForContact, telebot.OnContact, func(c telebot.Context, s *router.StateService) error {
		contact := c.Message().Contact
		if contact == nil {
			return c.Send(h.translator.Translate("send_contact"), h.makeShareContactKeyboard())
		}

		if contact.UserID != c.Chat().ID {
			return c.Send(h.translator.Translate("share_contact"), h.makeShareContactKeyboard())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := h.storage.Store(ctx, contact.PhoneNumber, c.Chat().ID); err != nil {
			if errors.Is(err, storage.ErrInvalidPhoneNumber) {
				return c.Send(h.translator.Translate("invalid_phone"))
			}
			return fmt.Errorf("set phone number: %w", err)
		}

		if err := s.Delete(); err != nil {
			return fmt.Errorf("delete state: %w", err)
		}

		return c.Send(h.translator.Translate("thanks_contact"), &telebot.ReplyMarkup{RemoveKeyboard: true})
	})

	r.Handle(StateStartWaitForContact, telebot.OnText, func(c telebot.Context, _ *router.StateService) error {
		return c.Send(h.translator.Translate("send_contact"), h.makeShareContactKeyboard())
	})

	return nil
}

func (h *StartHandler) makeShareContactKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}

	kb.Reply(
		kb.Row(kb.Contact(h.translator.Translate("share_contact_button"))),
	)

	return kb
}
