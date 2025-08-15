package config

import (
	"github.com/capcom6/phone2tg-proxy/pkg/http"
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
	)
}
