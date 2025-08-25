package storage

import (
	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"storage",
		fxutil.WithNamedLogger("storage"),
		fx.Provide(
			newRepository,
			fx.Private,
		),
		fx.Provide(New),
	)
}
