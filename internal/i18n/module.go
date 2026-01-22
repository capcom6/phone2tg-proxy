package i18n

import (
	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"go.uber.org/fx"
)

// Module provides the i18n translation service.
func Module() fx.Option {
	return fx.Module(
		"i18n",
		fxutil.WithNamedLogger("i18n"),
		fx.Provide(
			NewService,
		),
	)
}
