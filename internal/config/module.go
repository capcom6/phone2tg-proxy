package config

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/openapi"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/telegofx"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"config",
		fx.Provide(
			New,
			fx.Private,
		),
		fx.Provide(
			func(cfg Config) fiberfx.Config {
				return fiberfx.Config{
					Address:     cfg.HTTP.Address,
					ProxyHeader: cfg.HTTP.ProxyHeader,
					Proxies:     cfg.HTTP.Proxies,
				}
			},
			func(cfg Config) openapi.Config {
				return openapi.Config{
					Enabled:    cfg.HTTP.OpenAPI.Enabled,
					PublicHost: cfg.HTTP.OpenAPI.PublicHost,
					PublicPath: cfg.HTTP.OpenAPI.PublicPath,
				}
			},
			func() httpfx.Config {
				return httpfx.Config{}
			},
			func(cfg Config) bot.Config {
				return bot.Config{
					ProxyURL: cfg.Telegram.ProxyURL,
					Timeout:  cfg.Telegram.Timeout,
				}
			},
			func(cfg Config) telegofx.Config {
				return telegofx.Config{
					Token: cfg.Telegram.Token,
					Mode:  telegofx.ModePolling,
				}
			},
		),
		fx.Provide(func(cfg Config) redisfx.Config {
			return redisfx.Config{
				URL: cfg.Redis.URL,
			}
		}),
		fx.Provide(func(cfg Config) storage.Config {
			return storage.Config{
				Secret: []byte(cfg.Storage.Secret),
			}
		}),
		fx.Provide(func(cfg Config) i18n.Config {
			return i18n.Config{
				Language:         cfg.I18n.DefaultLanguage,
				TranslationsPath: cfg.I18n.TranslationsPath,
			}
		}),
	)
}
