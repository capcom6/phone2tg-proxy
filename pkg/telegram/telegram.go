package telegram

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/telebot.v4"
)

// ErrInvalidToken is returned when the provided Telegram bot token is empty or invalid.
var ErrInvalidToken = errors.New("invalid token")

// New constructs a telebot.Bot using the provided Config.
func New(cfg Config) (*telebot.Bot, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrInvalidToken
	}

	pref := telebot.Settings{
		Token: cfg.Token,
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	return b, nil
}
