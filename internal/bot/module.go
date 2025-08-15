package bot

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

func Module() fx.Option {
	return fx.Module(
		"bot",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("bot")
		}),
		fx.Provide(
			handlers.NewStartHandler,
			fx.Private,
		),

		fx.Invoke(func(
			bot *telebot.Bot,
			logger *zap.Logger,
			startHandler *handlers.StartHandler,
		) error {
			bot.Use(middleware.Logger(logger))

			if err := startHandler.Register(bot); err != nil {
				return fmt.Errorf("register start handler: %w", err)
			}
			return nil
		}),
	)
}
