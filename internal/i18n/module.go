package i18n

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

// Module provides the i18n translation service.
func Module() fx.Option {
	return fx.Module(
		"i18n",
		logger.WithNamedLogger("i18n"),
		fx.Provide(
			NewService,
		),
	)
}
