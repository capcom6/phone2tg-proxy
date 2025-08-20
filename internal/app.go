package internal

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/pkg/http"
	"github.com/capcom6/phone2tg-proxy/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Run() {
	fx.New(
		logger.Module(),
		fx.WithLogger(func(l *zap.Logger) fxevent.Logger {
			logOption := fxevent.ZapLogger{Logger: l}
			logOption.UseLogLevel(zapcore.DebugLevel)
			return &logOption
		}),
		config.Module(),
		http.Module(),
		server.Module(),
		bot.Module(),
		//
		fx.Invoke(func(logger *zap.Logger) {
			logger.Info("Hello, World!")
		}),
	).Run()
}
