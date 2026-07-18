package server

import (
	"github.com/capcom6/phone2tg-proxy/internal/server/handlers"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/logger"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"server",
		logger.WithNamedLogger("server"),

		fx.Provide(func(log *zap.Logger) fiberfx.Options {
			opts := fiberfx.Options{}
			opts.WithErrorHandler(fiberfx.NewCustomJSONErrorHandler(log, errorsFormatter))
			return opts
		}),

		fx.Provide(
			handlers.NewMessagesHandler,
			fx.Private,
		),

		fx.Invoke(func(app *fiber.App, messages *handlers.MessagesHandler) {
			api := app.Group("/api/v1")

			messages.Register(api.Group("/messages"))
		}),
	)
}
