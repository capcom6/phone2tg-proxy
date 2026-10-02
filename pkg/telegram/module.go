package telegram

import (
	"context"
	"net/http"

	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

func Module() fx.Option {
	return fx.Module(
		"telegram",
		logger.WithNamedLogger("telegram"),
		fx.Provide(func(cfg Config, factory httpfx.Factory) (*http.Client, error) {
			return factory.NewClient(httpfx.WithProxyURL(cfg.ProxyURL, ""))
		}, fx.Private),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, bot *telebot.Bot, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("starting telegram bot")

					go func() {
						bot.Start()
					}()

					logger.Info("telegram bot started")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("stopping telegram bot")
					bot.Stop()
					logger.Info("telegram bot stopped")

					return nil
				},
			})
		}),
	)
}
