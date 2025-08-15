package handlers

import "gopkg.in/telebot.v4"

type StartHandler struct {
}

func NewStartHandler() *StartHandler {
	return &StartHandler{}
}

func (h *StartHandler) Register(bot *telebot.Bot) error {
	bot.Handle("/start", func(c telebot.Context) error {
		return c.Send("Please, send me your contact", h.makeShareContactKeyboard())
	})

	bot.Handle(telebot.OnContact, func(c telebot.Context) error {
		contact := c.Message().Contact
		if contact == nil {
			return c.Send("Please, send me your contact", h.makeShareContactKeyboard())
		}

		if contact.UserID != c.Chat().ID {
			return c.Send("You must share your contact with me")
		}

		return c.Send("Thanks for sharing your contact!")
	})

	return nil
}

func (h *StartHandler) makeShareContactKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{ResizeKeyboard: true}

	kb.Reply(
		kb.Row(kb.Contact("Share contact")),
	)

	return kb
}
