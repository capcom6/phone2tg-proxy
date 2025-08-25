package validator

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"validator",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("validator")
		}),
		fx.Provide(New),
	)
}
