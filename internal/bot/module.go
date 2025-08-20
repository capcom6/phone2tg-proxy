package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"bot",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("bot")
		}),
		fx.Provide(func(logger *zap.Logger) (*log.Logger, error) {
			l, err := zap.NewStdLogAt(logger, zap.ErrorLevel)
			if err != nil {
				return nil, fmt.Errorf("create logger: %w", err)
			}

			return l, nil
		}, fx.Private),
		fx.Provide(func(cfg Config) (*gotgbot.Bot, error) {
			bot, err := gotgbot.NewBot(cfg.Token, nil)
			if err != nil {
				return nil, fmt.Errorf("create bot: %w", err)
			}

			return bot, nil
		}),
		fx.Provide(func() (*ext.Dispatcher, error) {
			dispatcher := ext.NewDispatcher(new(ext.DispatcherOpts))

			return dispatcher, nil
		}),
		fx.Provide(func(disp *ext.Dispatcher, logger *log.Logger) (*ext.Updater, error) {
			updater := ext.NewUpdater(disp, &ext.UpdaterOpts{ErrorLog: logger, UnhandledErrFunc: nil})

			return updater, nil
		}),
		fx.Invoke(func(lc fx.Lifecycle, updater *ext.Updater, bot *gotgbot.Bot, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("starting updater")
					if err := updater.StartPolling(bot, nil); err != nil {
						return fmt.Errorf("start polling: %w", err)
					}
					logger.Info("updater started")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("stopping updater")
					err := updater.Stop()
					if err != nil {
						return fmt.Errorf("stop updater: %w", err)
					}

					logger.Info("updater stopped")
					return nil
				},
			})
		}),
		fx.Invoke(func(d *ext.Dispatcher, logger *zap.Logger) {
			d.AddHandler(handlers.NewCommand("start", func(b *gotgbot.Bot, ctx *ext.Context) error {
				_, _ = b.SendMessage(
					ctx.EffectiveChat.Id,
					"Please provide your phone",
					&gotgbot.SendMessageOpts{
						ReplyMarkup: &gotgbot.ReplyKeyboardMarkup{
							Keyboard: [][]gotgbot.KeyboardButton{
								{
									{
										Text:           "Share Phone",
										RequestContact: true,
									},
								},
							},
							ResizeKeyboard:  true,
							OneTimeKeyboard: true,
						},
					},
				)
				return nil
			}))
		}),
	)
}
