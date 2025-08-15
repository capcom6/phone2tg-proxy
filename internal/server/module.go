package server

import (
	"github.com/capcom6/phone2tg-proxy/pkg/http"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"server",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("server")
		}),

		fx.Provide(func() http.Options {
			return http.Options{}
		}),

		fx.Invoke(func(app *fiber.App) {
			app.Get("/", func(c *fiber.Ctx) error {
				return c.SendString("Hello, World!")
			})
		}),
	)
}
