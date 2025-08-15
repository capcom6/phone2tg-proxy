package http

import (
	"context"
	"fmt"
	"net"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"http",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("http")
		}),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, cfg Config, app *fiber.App, logger *zap.Logger, sd fx.Shutdowner) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Address)
					if err != nil {
						return fmt.Errorf("listen %s: %w", cfg.Address, err)
					}
					go func() {
						if err := app.Listener(ln); err != nil {
							logger.Error("server failed", zap.Error(err))
							_ = sd.Shutdown()
						}
					}()
					logger.Info("server listening", zap.String("address", ln.Addr().String()))
					return nil
				},
				OnStop: func(ctx context.Context) error {
					logger.Info("shutting down server")
					if err := app.ShutdownWithContext(ctx); err != nil {
						logger.Error("server shutdown failed", zap.Error(err))
						return fmt.Errorf("server shutdown failed: %w", err)
					}
					logger.Info("server shutdown completed")
					return nil
				},
			})
		}),
	)
}
