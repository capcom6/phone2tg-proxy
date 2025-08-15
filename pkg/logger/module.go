package logger

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"logger",
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error { return nil },
				OnStop: func(_ context.Context) error {
					_ = logger.Sync()
					return nil
				},
			})
		}),
	)
}
