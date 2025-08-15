package main

import (
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	fx.New(
		logger.Module(),
		fx.WithLogger(func(l *zap.Logger) fxevent.Logger {
			logOption := fxevent.ZapLogger{Logger: l}
			logOption.UseLogLevel(zapcore.DebugLevel)
			return &logOption
		}),
		config.Module(),
		fx.Invoke(func(cfg config.Config, logger *zap.Logger) {
			logger.Info("Hello, World!")
			logger.Info("Config", zap.Any("config", cfg))
		}),
	).Run()
}
