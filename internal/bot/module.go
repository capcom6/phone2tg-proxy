package bot

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/middleware"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/gofiber/storage/memory/v2"
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
			func() fsm.Storage {
				return memory.New()
			},
			func(storage fsm.Storage) *fsm.Service {
				return fsm.New("fsm", 0, storage)
			},
			fx.Private,
		),
		fx.Provide(
			handlers.NewStartHandler,
			handlers.NewStopHandler,
			fx.Private,
		),

		fx.Invoke(func(
			bot *telebot.Bot,
			fsm *fsm.Service,
			logger *zap.Logger,
			startHandler *handlers.StartHandler,
			stopHandler *handlers.StopHandler,
		) error {
			bot.Use(middleware.Logger(logger))

			rt := router.New(fsm, bot)

			if err := startHandler.Register(rt); err != nil {
				return fmt.Errorf("register start handler: %w", err)
			}

			if err := stopHandler.Register(rt); err != nil {
				return fmt.Errorf("register stop handler: %w", err)
			}

			return nil
		}),
	)
}
