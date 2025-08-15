package telegram

import (
	"fmt"

	"gopkg.in/telebot.v4"
)

func New(cfg Config) (*telebot.Bot, error) {
	pref := telebot.Settings{
		Token: cfg.Token,
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	return b, nil
}
