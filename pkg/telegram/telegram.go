package telegram

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/telebot.v4"
)

// ErrInvalidToken is returned when the provided Telegram bot token is empty or invalid.
var ErrInvalidToken = errors.New("invalid token")

// New constructs a telebot.Bot using the provided Config and the injected HTTP
// client. The client comes from the module's private provider, so the proxy is
// validated before this point and the token gate is the only check left here.
func New(cfg Config, client *http.Client) (*telebot.Bot, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrInvalidToken
	}

	pref := telebot.Settings{
		Token:  cfg.Token,
		Client: client,
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	return b, nil
}
