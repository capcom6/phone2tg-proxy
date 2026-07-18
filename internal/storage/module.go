package storage

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"storage",
		logger.WithNamedLogger("storage"),
		fx.Provide(
			newRepository,
			fx.Private,
		),
		fx.Provide(New),
	)
}
