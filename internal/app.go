package internal

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/pkg/http"
	"github.com/capcom6/phone2tg-proxy/pkg/logger"
	"github.com/capcom6/phone2tg-proxy/pkg/redis"
	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
	"github.com/capcom6/phone2tg-proxy/pkg/validator"
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
		telegram.Module(),
		redis.Module(),
		validator.Module(),
		//
		storage.Module(),
		server.Module(),
		bot.Module(),
		proxy.Module(),
	).Run()
}
