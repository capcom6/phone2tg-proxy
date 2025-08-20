package config

import (
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/pkg/http"
	"github.com/capcom6/phone2tg-proxy/pkg/redis"
	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
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
			func(cfg Config) http.Config {
				return http.Config{
					Address:     cfg.HTTP.Address,
					ProxyHeader: cfg.HTTP.ProxyHeader,
					Proxies:     cfg.HTTP.Proxies,
				}
			},
			func(cfg Config) telegram.Config {
				return telegram.Config{
					Token: cfg.Telegram.Token,
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
	)
}
