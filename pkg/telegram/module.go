package telegram

import (
	"context"

	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

func Module() fx.Option {
	return fx.Module(
		"telegram",
		fxutil.WithNamedLogger("telegram"),
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
