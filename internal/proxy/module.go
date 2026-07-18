package proxy

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"proxy",
		logger.WithNamedLogger("proxy"),
		fx.Provide(New),
	)
}
