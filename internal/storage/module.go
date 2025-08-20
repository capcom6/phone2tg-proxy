package storage

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"storage",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("storage")
		}),
		fx.Provide(
			newRepository,
			fx.Private,
		),
		fx.Provide(New),
	)
}
