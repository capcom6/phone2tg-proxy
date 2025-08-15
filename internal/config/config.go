package config

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/pkg/config"
)

type httpConfig struct {
	Address     string   `koanf:"address"`
	ProxyHeader string   `koanf:"proxy_header"`
	Proxies     []string `koanf:"proxies"`
}

type telegramConfig struct {
	Token string `koanf:"token"`
}

type Config struct {
	HTTP     httpConfig     `koanf:"http"`
	Telegram telegramConfig `koanf:"telegram"`
}

func New() (Config, error) {
	cfg := Config{
		HTTP: httpConfig{
			Address:     "127.0.0.1:3000",
			ProxyHeader: "X-Forwarded-For",
			Proxies:     []string{},
		},
		Telegram: telegramConfig{
			Token: "",
		},
	}

	if err := config.Load(&cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}
