package telegram

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
	"gopkg.in/telebot.v4"
)

// ErrInvalidToken is returned when the provided Telegram bot token is empty or invalid.
var ErrInvalidToken = errors.New("invalid token")

// New constructs a telebot.Bot using the provided Config.
func New(cfg Config) (*telebot.Bot, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrInvalidToken
	}

	h, err := newProxyClient(cfg.ProxyURL)
	if err != nil {
		return nil, err
	}

	pref := telebot.Settings{
		Token:  cfg.Token,
		Client: h,
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	return b, nil
}

func newProxyClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse proxy URL: %w", ErrInvalidConfig, err)
	}

	dialer, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create proxy dialer: %w", ErrInvalidConfig, err)
	}

	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("%w: proxy dialer does not support context", ErrInvalidConfig)
	}

	transport := &http.Transport{
		DialContext: contextDialer.DialContext,
	}

	return &http.Client{
		Transport: transport,
	}, nil
}
