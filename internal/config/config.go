package config

import (
	"fmt"

	"github.com/go-core-fx/config"
)

type httpConfig struct {
	Address     string   `koanf:"address"`
	ProxyHeader string   `koanf:"proxy_header"`
	Proxies     []string `koanf:"proxies"`
}

type telegramConfig struct {
	Token string `koanf:"token"`
}

type redisConfig struct {
	URL string `koanf:"url"`
}

type storageConfig struct {
	Secret string `koanf:"secret"`
}

type Config struct {
	HTTP     httpConfig     `koanf:"http"`
	Telegram telegramConfig `koanf:"telegram"`
	Redis    redisConfig    `koanf:"redis"`
	Storage  storageConfig  `koanf:"storage"`
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
		Redis: redisConfig{
			URL: "redis://localhost:6379/0",
		},
		Storage: storageConfig{
			Secret: "",
		},
	}

	if err := config.Load(&cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}
