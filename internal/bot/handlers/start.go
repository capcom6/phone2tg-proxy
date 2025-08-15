package handlers

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"gopkg.in/telebot.v4"
)

type StartHandler struct {
}

func NewStartHandler() *StartHandler {
	return &StartHandler{}
}

func (h *StartHandler) Register(r *router.Router) error {
	r.Handle(fsm.StateEmpty, "/start", func(c telebot.Context, s *router.StateService) error {
		if err := s.Set(fsm.NewState(StateStartWaitForContact)); err != nil {
			return fmt.Errorf("set state: %w", err)
		}

		return c.Send("Please, send me your contact", h.makeShareContactKeyboard())
	})

	r.Handle(StateStartWaitForContact, telebot.OnContact, func(c telebot.Context, s *router.StateService) error {
		contact := c.Message().Contact
		if contact == nil {
			return c.Send("Please, send me your contact", h.makeShareContactKeyboard())
		}

		if contact.UserID != c.Chat().ID {
			return c.Send("You must share your contact with me", h.makeShareContactKeyboard())
		}

		if err := s.Delete(); err != nil {
			return fmt.Errorf("delete state: %w", err)
		}

		return c.Send("Thanks for sharing your contact!", &telebot.ReplyMarkup{RemoveKeyboard: true})
	})

	r.Handle(StateStartWaitForContact, telebot.OnText, func(c telebot.Context, _ *router.StateService) error {
		return c.Send("Please, send me your contact", h.makeShareContactKeyboard())
	})

	return nil
}

func (h *StartHandler) makeShareContactKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}

	kb.Reply(
		kb.Row(kb.Contact("Share contact")),
	)

	return kb
}
