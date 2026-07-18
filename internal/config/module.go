package config

import (
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/pkg/redis"
	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
	"github.com/go-core-fx/fiberfx"
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
			func(cfg Config) telegram.Config {
				return telegram.Config{
					Token:    cfg.Telegram.Token,
					ProxyURL: cfg.Telegram.ProxyURL,
				}
			},
		),
		fx.Provide(func(cfg Config) redis.Config {
			return redis.Config{
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
