package config

import (
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
			// httpfx owns the transport: SOCKS5 routing from ProxyURL and the
			// Timeout that bounds dialing the proxy, TLS and reading the
			// response in one deadline. Every other field stays at its
			// inherit-the-defaults zero (pools, idle timeout, root CAs).
			// telegofx receives the configured *http.Client from the graph
			// (httpfx.Module) and only records call activity on it.
			func(cfg Config) httpfx.Config {
				return httpfx.Config{
					ProxyURL:            cfg.Telegram.ProxyURL,
					Timeout:             cfg.Telegram.Timeout,
					Bypass:              "",
					MaxIdleConns:        0,
					MaxIdleConnsPerHost: 0,
					IdleConnTimeout:     0,
					TLS: httpfx.TLSConfig{
						RootCAFile:          "",
						RootCAPEM:           "",
						RootCAReplaceSystem: false,
					},
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
