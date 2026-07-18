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
	Token    string `koanf:"token"`
	ProxyURL string `koanf:"proxy_url"`
}

type redisConfig struct {
	URL string `koanf:"url"`
}

type storageConfig struct {
	Secret string `koanf:"secret"`
}

type i18nConfig struct {
	DefaultLanguage  string `koanf:"default_language"`
	TranslationsPath string `koanf:"translations_path"`
}

type Config struct {
	HTTP     httpConfig     `koanf:"http"`
	Telegram telegramConfig `koanf:"telegram"`
	Redis    redisConfig    `koanf:"redis"`
	Storage  storageConfig  `koanf:"storage"`
	I18n     i18nConfig     `koanf:"i18n"`
}

func New() (Config, error) {
	cfg := Config{
		HTTP: httpConfig{
			Address:     "127.0.0.1:3000",
			ProxyHeader: "X-Forwarded-For",
			Proxies:     []string{},
		},
		Telegram: telegramConfig{
			Token:    "",
			ProxyURL: "",
		},
		Redis: redisConfig{
			URL: "redis://localhost:6379/0",
		},
		Storage: storageConfig{
			Secret: "",
		},
		I18n: i18nConfig{
			DefaultLanguage:  "en",
			TranslationsPath: "i18n/locales",
		},
	}

	if err := config.Load(&cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}
