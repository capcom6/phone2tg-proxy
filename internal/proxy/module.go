package proxy

import (
	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"proxy",
		fxutil.WithNamedLogger("proxy"),
		fx.Provide(New),
	)
}
