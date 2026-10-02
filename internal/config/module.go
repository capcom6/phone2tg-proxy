package config

import (
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/openapi"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/redisfx"
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
			func(cfg Config) telegram.Config {
				return telegram.Config{
					Token:    cfg.Telegram.Token,
					ProxyURL: cfg.Telegram.ProxyURL,
				}
			},
			// httpfx.Module provides both Factory and a *http.Client, and NewFactory
			// needs an httpfx.Config that nothing else supplies. Without this
			// provider the graph fails to build with "missing type: httpfx.Config
			// (did you mean to Provide it?)". Telegram keeps its own client
			// module-private via fx.Private, so it does not collide with the
			// module-level one; dropping that fx.Private collides with
			// "cannot provide *http.Client ... already provided by
			// httpfx.Module.func1". The zero value is deliberate: ProxyURL is
			// supplied per-client via httpfx.WithProxyURL in pkg/telegram/module.go.
			// Do not delete this provider.
			func(_ Config) httpfx.Config {
				return httpfx.Config{}
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
