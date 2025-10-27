package http

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"http",
		fxutil.WithNamedLogger("http"),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, cfg Config, app *fiber.App, logger *zap.Logger, sd fx.Shutdowner) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					logger.Info("starting server")
					ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Address)
					if err != nil {
						return fmt.Errorf("listen %s: %w", cfg.Address, err)
					}
					go func() {
						if lsErr := app.Listener(ln); lsErr != nil && !errors.Is(lsErr, net.ErrClosed) {
							logger.Error("server failed", zap.Error(lsErr))
							if shErr := sd.Shutdown(); shErr != nil {
								logger.Error("fx shutdown failed", zap.Error(shErr))
							}
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
