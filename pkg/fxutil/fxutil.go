package fxutil

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func WithNamedLogger(name string) fx.Option {
	return fx.Decorate(func(log *zap.Logger) *zap.Logger { return log.Named(name) })
}
